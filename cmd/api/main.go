package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"secunda/internal/config"
	"secunda/internal/handler"
	"secunda/internal/platform/auth"
	"secunda/internal/platform/mysql"
	"secunda/internal/platform/redis"
	"secunda/internal/platform/transaction"
	"secunda/internal/repository"
	"secunda/internal/service"
)

// @title Secunda API
// @version 1.0
// @description Team task management API.
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	db, err := mysql.Open(startup, cfg.MySQL)
	if err != nil {
		return err
	}
	defer db.Close()
	cache, err := redis.Open(startup, cfg.Redis)
	if err != nil {
		return err
	}
	defer cache.Close()
	passwords, err := auth.NewPasswords()
	if err != nil {
		return err
	}
	tokens, err := auth.NewTokens(cfg.JWT.Secret, cfg.JWT.TTL)
	if err != nil {
		return err
	}
	users := repository.NewUsers(db)
	teams := repository.NewTeams(db)
	authService := service.NewAuth(users, passwords, tokens)
	transactions := transaction.New(db)
	teamService := service.NewTeams(teams, users, transactions)
	taskService := service.NewTasks(repository.NewTasks(db), teams, transactions)
	router := handler.NewRouter(handler.NewAuth(authService, logger), handler.NewTeams(teamService, logger), handler.NewTasks(taskService, logger), tokens)
	server := &http.Server{
		Addr: cfg.HTTP.Addr, Handler: router,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout, IdleTimeout: cfg.HTTP.IdleTimeout,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	logger.Info("http server starting", "address", cfg.HTTP.Addr)
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			server.Close()
			return err
		}
		return nil
	}
}
