# Makefile for GlobalPermits

PROJECT_ROOT := $(CURDIR)
API_DIR := $(PROJECT_ROOT)/services/api
BIN_DIR := $(PROJECT_ROOT)/bin
DB_MIGRATIONS := $(PROJECT_ROOT)/db/migrations

# Customize these envs for local dev
API_ADDR ?= :8080
POSTGRES_DSN ?= postgres://globalpermits:globalpermits@localhost:5432/globalpermits?sslmode=disable
REDIS_ADDR ?= localhost:6379

.PHONY: help
help:
	@echo "Targets:"
	@echo "  build-api         Build API binary (requires Go)"
	@echo "  run-api           Run API locally (requires Go + psql + Postgres running)"
	@echo "  migrate           Apply SQL migrations using psql"
	@echo "  docker-build-api  Build API Docker image"
	@echo "  compose-up        Start Postgres, Redis, and API via docker-compose"
	@echo "  compose-down      Stop docker-compose stack"
	@echo "  compose-logs      Tail logs from docker-compose"
	@echo "  build-loader      Build data loader CLI"
	@echo "  load-fixture      Load fixtures/permits.json using loader"

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

.PHONY: build-api
build-api: $(BIN_DIR)
	go build -o $(BIN_DIR)/api ./services/api

.PHONY: run-api
run-api: build-api
	API_ADDR=$(API_ADDR) POSTGRES_DSN=$(POSTGRES_DSN) REDIS_ADDR=$(REDIS_ADDR) $(BIN_DIR)/api

.PHONY: migrate
migrate:
	psql "$(POSTGRES_DSN)" -f $(DB_MIGRATIONS)/001_init.sql

.PHONY: seed
seed:
	psql "$(POSTGRES_DSN)" -f $(PROJECT_ROOT)/db/seeds/001_seed.sql

.PHONY: docker-build-api
docker-build-api:
	docker build -t globalpermits/api:dev $(API_DIR)

.PHONY: compose-up
compose-up:
	docker compose up -d --build

.PHONY: compose-down
compose-down:
	docker compose down -v

.PHONY: compose-logs
compose-logs:
	docker compose logs -f --tail=200

.PHONY: build-loader
build-loader: $(BIN_DIR)
	go build -o $(BIN_DIR)/loader ./cmd/loader

.PHONY: load-fixture
load-fixture: build-loader
	POSTGRES_DSN=$(POSTGRES_DSN) $(BIN_DIR)/loader -file $(PROJECT_ROOT)/fixtures/permits.json -format json

