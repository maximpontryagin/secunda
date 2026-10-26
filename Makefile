SHELL := /bin/sh

GO ?= go
BIN_DIR := $(CURDIR)/bin
SWAG_VERSION := v1.16.6
MOCKERY_VERSION := v2.53.5
SWAG := $(BIN_DIR)/swag-$(SWAG_VERSION)
MOCKERY := $(BIN_DIR)/mockery-$(MOCKERY_VERSION)

.PHONY: help tools swagger mocks generate build run test test-integration vet fmt migrate-up migrate-down migrate-status migrate-create compose-up compose-down infra-up

help:
	@printf '%s\n' 'tools: install pinned swag and mockery' 'generate: generate Swagger and mocks' 'build/run: build or run locally' 'test/vet/fmt: check Go code' 'migrate-up/down/status: local goose migrations using .env' 'migrate-create NAME=change_name: create SQL migration' 'infra-up: start MySQL and Redis' 'compose-up/down: start or stop the application stack'

tools: $(SWAG) $(MOCKERY)

$(SWAG):
	@mkdir -p $(BIN_DIR)
	GOBIN=$(BIN_DIR) $(GO) install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	mv $(BIN_DIR)/swag $(SWAG)

$(MOCKERY):
	@mkdir -p $(BIN_DIR)
	GOBIN=$(BIN_DIR) $(GO) install github.com/vektra/mockery/v2@$(MOCKERY_VERSION)
	mv $(BIN_DIR)/mockery $(MOCKERY)

swagger: $(SWAG)
	$(SWAG) init -g main.go -d cmd/api,internal/handler --parseInternal --outputTypes json,yaml -o docs

mocks: $(MOCKERY)
	$(MOCKERY) --config .mockery.yaml

generate: swagger mocks

build:
	$(GO) build -o $(BIN_DIR)/api ./cmd/api
	$(GO) build -o $(BIN_DIR)/migrate ./cmd/migrate

run:
	$(GO) run ./cmd/api

test:
	$(GO) test -race ./...

test-integration:
	$(GO) test -race -tags=integration ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w cmd internal

migrate-up:
	$(GO) run ./cmd/migrate up

migrate-down:
	$(GO) run ./cmd/migrate down

migrate-status:
	$(GO) run ./cmd/migrate status

migrate-create:
	@test -n "$(NAME)" || (printf '%s\n' 'Usage: make migrate-create NAME=change_name'; exit 1)
	$(GO) run ./cmd/migrate create "$(NAME)" sql

infra-up:
	docker compose up -d --wait mysql redis

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down
