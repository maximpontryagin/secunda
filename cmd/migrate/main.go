package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pressly/goose/v3"

	"secunda/internal/config"
	"secunda/internal/platform/mysql"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: migrate <up|down|status|create> [name sql]")
	}
	if err := goose.SetDialect("mysql"); err != nil {
		return err
	}
	if os.Args[1] == "create" {
		return goose.RunContext(context.Background(), "create", nil, "migrations", os.Args[2:]...)
	}
	cfg, err := config.LoadMySQL()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := mysql.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	return goose.RunContext(ctx, os.Args[1], db, "migrations", os.Args[2:]...)
}
