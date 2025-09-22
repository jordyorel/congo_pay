# Congo Pay

Bootstrap configuration for a Go (Fiber) financial services/USSD/SMS project.

This repo includes a minimal Go Fiber API with a health route and config loader.

## Quick Start (Local)

- Copy env file: `cp .env.example .env` and update values.
- Start infra: `make compose-up` (starts PostgreSQL and Redis).
- Install deps: `go mod tidy`
- Run API: `make run` then visit `http://localhost:8080/healthz`
- Lint/format (optional): `golangci-lint run` and `go fmt ./...`.

## Environment Variables

See `.env.example` for a working set, including:
- App: `APP_ENV`, `PORT`, `LOG_LEVEL`.
- Postgres: `DATABASE_URL`, `POSTGRES_*`.
- Redis: `REDIS_URL`.
- Security: `JWT_SECRET`.
- SMS/USSD provider: `SMS_PROVIDER` and provider-specific keys.

## Docker

- `docker-compose.yml` includes Postgres, Redis, and the API service.
- `Dockerfile` builds the API binary.
- Start entire stack: `docker compose up --build -d`
- Check: `curl http://localhost:8080/healthz`

## Environments

- Development (`APP_ENV=development`, default):
  - DB/Redis are optional. The app falls back to in-memory stores so routes work without infra.
- Non‑development (e.g., `APP_ENV=staging`/`production`):
  - Requires `DATABASE_URL` and `REDIS_URL`. The app fails fast if missing.

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

1) Flesh out modules under `internal/` (ledger, ussd, sms, users, wallets).
2) Add persistence and migrations; wire `DATABASE_URL` and `REDIS_URL`.
3) Add handlers/routes, validation, and error handling.
4) Expand CI for linting and security checks.
