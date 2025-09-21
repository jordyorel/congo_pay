# Congo Pay

Bootstrap configuration for a Go (Fiber) financial services/USSD/SMS project.

This repo currently contains project configuration and infra scaffolding. Add your Go application code (e.g., `cmd/api`, `internal/...`) and enable the commented Docker build steps when ready.

## Quick Start

- Copy env file: `cp .env.example .env` and update values.
- Start infra: `make compose-up` (starts PostgreSQL and Redis).
- Initialize Go module (once you add code):
  - `go mod init github.com/your-org/congo_pay`
  - `go mod tidy`
- Lint/format (optional): `golangci-lint run` and `go fmt ./...`.

## Environment Variables

See `.env.example` for a working set, including:
- App: `APP_ENV`, `PORT`, `LOG_LEVEL`.
- Postgres: `DATABASE_URL`, `POSTGRES_*`.
- Redis: `REDIS_URL`.
- Security: `JWT_SECRET`.
- SMS/USSD provider: `SMS_PROVIDER` and provider-specific keys.

## Docker

- `docker-compose.yml` includes Postgres and Redis.
- `Dockerfile` is a multi-stage build; un-comment the build steps once your Go code and `go.mod` exist.
- Build image: `make docker-build`.

## Make Targets

Run `make help` to see common commands: `tidy`, `build`, `test`, `lint`, `fmt`, `compose-*`.

## CI

GitHub Actions workflow runs tidy/build/test when `go.mod` exists and attempts a Docker build. It will succeed even before code is added.

## Suggested Project Layout (when you add code)

- `cmd/api` – main entrypoint for the HTTP/USSD service.
- `internal/` – application modules (e.g., ledger, ussd, sms, users, wallets).
- `pkg/` – shared packages (optional).
- `migrations/` – SQL migrations (if using `golang-migrate` or similar).

## Next Steps

1) Decide on module path and run `go mod init`.
2) Scaffold `cmd/api/main.go` (e.g., with Fiber) and wire `DATABASE_URL` and `REDIS_URL`.
3) Add migrations and seed data as needed.
4) Un-comment Dockerfile steps and `api` service in `docker-compose.yml`.

