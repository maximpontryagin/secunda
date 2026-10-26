package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTP  HTTP
	MySQL MySQL
	Redis Redis
	JWT   JWT
}

type HTTP struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type MySQL struct {
	Host            string
	Port            int
	Database        string
	User            string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type Redis struct {
	Host     string
	Port     int
	Username string
	Password string
	DB       int
}

type JWT struct {
	Secret string
	TTL    time.Duration
}

func LoadEnv() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load .env: %w", err)
	}
	return nil
}

func Load() (Config, error) {
	if err := LoadEnv(); err != nil {
		return Config{}, err
	}
	p := parser{}
	c := Config{
		HTTP: HTTP{
			Addr:            p.value("HTTP_ADDR", ":8080"),
			ReadTimeout:     p.duration("HTTP_READ_TIMEOUT", "10s"),
			WriteTimeout:    p.duration("HTTP_WRITE_TIMEOUT", "15s"),
			IdleTimeout:     p.duration("HTTP_IDLE_TIMEOUT", "60s"),
			ShutdownTimeout: p.duration("SHUTDOWN_TIMEOUT", "10s"),
		},
		MySQL: parseMySQL(&p),
		Redis: Redis{
			Host: p.required("REDIS_HOST"), Port: p.integer("REDIS_PORT", "6379", 1, 65535),
			Username: os.Getenv("REDIS_USERNAME"), Password: os.Getenv("REDIS_PASSWORD"),
			DB: p.integer("REDIS_DB", "0", 0, 15),
		},
		JWT: JWT{Secret: p.required("JWT_SECRET"), TTL: p.duration("JWT_TTL", "24h")},
	}
	if len(c.JWT.Secret) < 32 {
		p.errs = append(p.errs, errors.New("JWT_SECRET must contain at least 32 bytes"))
	}
	return c, errors.Join(p.errs...)
}

func LoadMySQL() (MySQL, error) {
	if err := LoadEnv(); err != nil {
		return MySQL{}, err
	}
	p := parser{}
	c := parseMySQL(&p)
	return c, errors.Join(p.errs...)
}

func parseMySQL(p *parser) MySQL {
	c := MySQL{
		Host: p.required("MYSQL_HOST"), Port: p.integer("MYSQL_PORT", "3306", 1, 65535),
		Database: p.required("MYSQL_DATABASE"), User: p.required("MYSQL_USER"), Password: p.required("MYSQL_PASSWORD"),
		MaxOpenConns:    p.integer("MYSQL_MAX_OPEN_CONNS", "20", 1, 10000),
		MaxIdleConns:    p.integer("MYSQL_MAX_IDLE_CONNS", "10", 0, 10000),
		ConnMaxLifetime: p.duration("MYSQL_CONN_MAX_LIFETIME", "5m"),
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		p.errs = append(p.errs, errors.New("MYSQL_MAX_IDLE_CONNS must not exceed MYSQL_MAX_OPEN_CONNS"))
	}
	return c
}

type parser struct{ errs []error }

func (p *parser) value(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func (p *parser) required(key string) string {
	value := os.Getenv(key)
	if value == "" {
		p.errs = append(p.errs, fmt.Errorf("%s is required", key))
	}
	return value
}

func (p *parser) integer(key, fallback string, min, max int) int {
	value, err := strconv.Atoi(p.value(key, fallback))
	if err != nil || value < min || value > max {
		p.errs = append(p.errs, fmt.Errorf("%s must be an integer between %d and %d", key, min, max))
	}
	return value
}

func (p *parser) duration(key, fallback string) time.Duration {
	value, err := time.ParseDuration(p.value(key, fallback))
	if err != nil || value <= 0 {
		p.errs = append(p.errs, fmt.Errorf("%s must be a positive duration", key))
	}
	return value
}
