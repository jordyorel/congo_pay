.PHONY: help tidy build test run lint fmt docker-build compose-up compose-down compose-logs dev

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
	go run ./cmd/api

docker-build:
	docker build -t $(APP_NAME):dev .

compose-up:
	docker compose up -d db redis

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f
