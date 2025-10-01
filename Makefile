.PHONY: help tidy build test run lint fmt docker-build compose-up compose-down compose-logs migrate-up migrate-local reset-db reset-local

APP_NAME := congopay

help:
	@echo "Common targets:"
	@echo "  tidy           - go mod tidy"
	@echo "  build          - build all packages"
	@echo "  test           - run tests"
	@echo "  lint           - run golangci-lint"
	@echo "  fmt            - go fmt"
	@echo "  run            - go run ./cmd/api"
	@echo "  compose-up     - start postgres & redis"
	@echo "  compose-down   - stop stack"
	@echo "  compose-logs   - follow logs"
	@echo "  migrate-up     - apply SQL migrations to Postgres"
	@echo "  migrate-local  - apply SQL migrations via local psql (no Docker)"
	@echo "  reset-db       - DROP schema in Docker Postgres, then re-run migrations"
	@echo "  reset-local    - DROP schema via local psql, then re-run migrations"

tidy:
	go mod tidy

build:
	go build ./...

test:
	go test ./... -cover

lint:
	golangci-lint run || true

fmt:
	go fmt ./...

run:
	# Load .env if present so DATABASE_URL/REDIS_URL are available
	set -a; [ -f .env ] && . ./.env; set +a; \
	if command -v docker >/dev/null 2>&1 && docker ps --format '{{.Names}}' | grep -q '^congopay-db$$'; then \
		echo "Running migrations via docker (make migrate-up)"; \
		$(MAKE) migrate-up; \
	elif command -v psql >/dev/null 2>&1 && [ -n "$$DATABASE_URL" ]; then \
		echo "Running migrations via local psql (make migrate-local)"; \
		$(MAKE) migrate-local; \
	else \
		echo "WARNING: No Postgres detected (docker or local). Skipping migrations."; \
	fi; \
	go run ./cmd/api

docker-build:
	docker build -t $(APP_NAME):dev .

compose-up:
	docker compose up -d db redis

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f

migrate-up:
	bash scripts/migrate.sh

migrate-local:
	bash scripts/migrate_local.sh

reset-db:
	bash scripts/reset_docker.sh

reset-local:
	bash scripts/reset_local.sh
