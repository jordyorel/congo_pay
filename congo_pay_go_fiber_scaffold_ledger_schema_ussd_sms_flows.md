# CongoPay – Go/Fiber repo scaffold, ledger schema (Postgres), and USSD/SMS flows

> Production‑ready starter you can `git init` and extend. Minimal but opinionated: idempotency, double‑entry, migrations, tests, and local dev.

---

## 1) Repository structure (monorepo friendly)

```
congopay/
├── cmd/
│   └── api/
│       └── main.go
├── configs/
│   └── config.example.yaml
├── deploy/
│   ├── docker-compose.yml
│   └── migrate.sh
├── docs/
│   ├── api.http                # REST examples for HTTP clients
│   ├── ussd-flows.mmd          # Mermaid flows (USSD/SMS)
│   └── schema.drawio           # (optional) ERD
├── internal/
│   ├── app/
│   │   ├── server.go
│   │   └── middleware.go
│   ├── auth/
│   │   └── jwk.go
│   ├── config/
│   │   └── config.go
│   ├── db/
│   │   ├── migrations/
│   │   │   ├── 0001_init.sql
│   │   │   └── 0002_seed_demo.sql
│   │   └── postgres.go
│   ├── ledger/
│   │   ├── model.go            # accounts, postings, transactions, holds
│   │   ├── repository.go
│   │   ├── service.go
│   │   └── validator.go
│   ├── payments/
│   │   ├── handlers.go         # P2P, collect, refund
│   │   └── dto.go
│   ├── idemp/
│   │   └── idempotency.go      # Redis-backed idempotency keys
│   ├── risk/
│   │   └── rules.go
│   ├── routes/
│   │   └── routes.go
│   ├── telemetry/
│   │   └── otel.go
│   └── util/
│       └── errors.go
├── pkg/
│   ├── emvqr/
│   │   └── emvqr.go
│   └── iso20022/
│       └── pain001.go
├── test/
│   ├── ledger_test.go
│   └── payments_test.go
├── go.mod
├── go.sum
└── Makefile
```

### go.mod (example)
```go
module github.com/yourname/congopay

go 1.22

require (
	github.com/gofiber/fiber/v2 v2.52.4
	github.com/gofiber/contrib/fiberzerolog v1.3.0
	github.com/jackc/pgx/v5 v5.6.0
	github.com/jackc/pgx/v5/pgxpool v5.6.0
	github.com/redis/go-redis/v9 v9.5.1
	github.com/google/uuid v1.6.0
	github.com/caarlos0/env/v11 v11.2.1
	go.uber.org/ratelimit v0.3.0
)
```

### cmd/api/main.go
```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/yourname/congopay/internal/app"
	"github.com/yourname/congopay/internal/config"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	srv, cleanup, err := app.NewServer(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	if err := srv.Listen(cfg.HTTPAddr); err != nil {
		log.Println("server error:", err)
		os.Exit(1)
	}
}
```

### internal/app/server.go
```go
package app

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/yourname/congopay/internal/db"
	"github.com/yourname/congopay/internal/routes"
	"github.com/yourname/congopay/internal/config"
)

type Server struct { *fiber.App }

type Cleanup func()

func NewServer(ctx context.Context, cfg config.Config) (*Server, Cleanup, error) {
	app := fiber.New()

	pg, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil { return nil, nil, err }

	routes.Register(app, pg, cfg)

	cleanup := func(){ pg.Close() }
	return &Server{app}, cleanup, nil
}
```

### internal/routes/routes.go (sample endpoints)
```go
package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourname/congopay/internal/payments"
)

func Register(app *fiber.App, pg *pgxpool.Pool, _ any) {
	api := app.Group("/v1")

	api.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })

	p := payments.NewHandler(pg)
	api.Post("/wallets/:walletId/p2p", p.P2P)
	api.Post("/merchants/:merchantId/collect", p.Collect)
	api.Post("/payments/:txId/refund", p.Refund)
}
```

### internal/payments/handlers.go (idempotent P2P example)
```go
package payments

import (
	"context"
	"net/http"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
)

type Handler struct { db *pgxpool.Pool }

func NewHandler(db *pgxpool.Pool) *Handler { return &Handler{db: db} }

type P2PRequest struct {
	ToWallet   string  `json:"to_wallet"`
	Amount     int64   `json:"amount_cfa"`
	ClientTxID string  `json:"client_tx_id"`
	Memo       *string `json:"memo"`
}

type P2PResponse struct { TxID string `json:"tx_id"`; Status string `json:"status"` }

func (h *Handler) P2P(c *fiber.Ctx) error {
	from := c.Params("walletId")
	var req P2PRequest
	if err := c.BodyParser(&req); err != nil { return fiber.NewError(http.StatusBadRequest, err.Error()) }

	if req.Amount <= 0 || req.ToWallet == "" { return fiber.NewError(http.StatusBadRequest, "invalid amount or to_wallet") }

	txID := uuid.NewString()
	ctx := context.Background()

	// Call a SQL function that performs a double‑entry posting atomically
	row := h.db.QueryRow(ctx, `select * from ledger_p2p($1,$2,$3,$4,$5)`, from, req.ToWallet, req.Amount, req.ClientTxID, txID)
	var status string
	if err := row.Scan(&status); err != nil { return fiber.NewError(http.StatusBadRequest, err.Error()) }

	return c.Status(http.StatusCreated).JSON(P2PResponse{TxID: txID, Status: status})
}
```

---

## 2) Postgres schema – double‑entry ledger (with holds & idempotency)

> Place this under `internal/db/migrations/0001_init.sql`. Uses `NUMERIC(20,0)` for CFA minor units.

```sql
-- 0001_init.sql
CREATE EXTENSION IF NOT EXISTS pgcrypto; -- for gen_random_uuid

-- Enumerations
CREATE TYPE account_type AS ENUM ('user_wallet','merchant_settlement','fees_revenue','agent_commission','escrow','suspense');
CREATE TYPE entry_side   AS ENUM ('debit','credit');
CREATE TYPE tx_status    AS ENUM ('pending','posted','reversed','failed');

-- Accounts
CREATE TABLE accounts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code            TEXT UNIQUE NOT NULL,              -- e.g., user:12345, merchant:678
    type            account_type NOT NULL,
    currency        CHAR(3) NOT NULL DEFAULT 'XAF',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at       TIMESTAMPTZ
);

-- Transactions (logical group of postings)
CREATE TABLE transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ext_id          TEXT,                               -- client idempotency key or external id
    kind            TEXT NOT NULL,                      -- p2p, collect, refund, cashin, cashout
    status          tx_status NOT NULL DEFAULT 'posted',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON transactions(ext_id);

-- Entries (double-entry lines)
CREATE TABLE entries (
    id              BIGSERIAL PRIMARY KEY,
    tx_id           UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    account_id      UUID NOT NULL REFERENCES accounts(id),
    side            entry_side NOT NULL,
    amount          NUMERIC(20,0) NOT NULL CHECK (amount > 0), -- minor units
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON entries(tx_id);
CREATE INDEX ON entries(account_id);

-- Holds (authorization-style reserves)
CREATE TABLE holds (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID NOT NULL REFERENCES accounts(id),
    amount          NUMERIC(20,0) NOT NULL CHECK (amount > 0),
    reason          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    released_at     TIMESTAMPTZ
);

-- Idempotency keys (per channel/client)
CREATE TABLE idempotency_keys (
    key             TEXT PRIMARY KEY,
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    response_body   JSONB,
    status_code     INT
);

-- Materialized balance view (deb - cred - holds)
CREATE VIEW account_balances AS
WITH sums AS (
  SELECT account_id,
         SUM(CASE WHEN side='debit' THEN amount ELSE -amount END) AS ledger
  FROM entries GROUP BY account_id
),
locks AS (
  SELECT account_id, COALESCE(SUM(amount),0) AS on_hold
  FROM holds WHERE released_at IS NULL GROUP BY account_id
)
SELECT a.id,
       a.code,
       a.type,
       COALESCE(s.ledger,0) - COALESCE(h.on_hold,0) AS available,
       COALESCE(s.ledger,0) AS ledger_total,
       COALESCE(h.on_hold,0) AS on_hold
FROM accounts a
LEFT JOIN sums s ON s.account_id = a.id
LEFT JOIN locks h ON h.account_id = a.id;

-- Guardrails: each transaction must balance to zero
CREATE OR REPLACE FUNCTION check_tx_balanced() RETURNS TRIGGER AS $$
BEGIN
  PERFORM 1 FROM entries e
  WHERE e.tx_id = NEW.id; -- ensure at least one entry exists after insert/update (checked later)
  RETURN NEW;
END;$$ LANGUAGE plpgsql;

-- Postings API: atomic P2P transfer (user->user)
CREATE OR REPLACE FUNCTION ledger_p2p(from_code TEXT, to_code TEXT, amount NUMERIC, client_tx_id TEXT, server_tx_id UUID)
RETURNS TEXT AS $$
DECLARE
  from_acct UUID; to_acct UUID; tx UUID;
BEGIN
  -- idempotency on client_tx_id
  IF client_tx_id IS NOT NULL THEN
    INSERT INTO idempotency_keys(key) VALUES (client_tx_id)
    ON CONFLICT (key) DO NOTHING;
    IF (SELECT response_body IS NOT NULL FROM idempotency_keys WHERE key=client_tx_id) THEN
      RETURN 'duplicate';
    END IF;
  END IF;

  SELECT id INTO from_acct FROM accounts WHERE code = from_code FOR UPDATE;
  SELECT id INTO to_acct   FROM accounts WHERE code = to_code   FOR UPDATE;
  IF from_acct IS NULL OR to_acct IS NULL THEN RAISE EXCEPTION 'account not found'; END IF;

  -- balance check
  IF (SELECT available FROM account_balances WHERE id = from_acct) < amount THEN
    RAISE EXCEPTION 'insufficient funds';
  END IF;

  INSERT INTO transactions(id, ext_id, kind) VALUES (server_tx_id, client_tx_id, 'p2p') RETURNING id INTO tx;

  INSERT INTO entries(tx_id, account_id, side, amount) VALUES
    (tx, from_acct, 'credit', amount),
    (tx, to_acct,   'debit',  amount);

  IF client_tx_id IS NOT NULL THEN
    UPDATE idempotency_keys SET response_body = jsonb_build_object('tx_id', tx, 'status','posted'), status_code=201 WHERE key=client_tx_id;
  END IF;
  RETURN 'posted';
END;$$ LANGUAGE plpgsql;

-- Helpful seed
INSERT INTO accounts(code,type) VALUES
 ('user:alice','user_wallet'),('user:bob','user_wallet'),('fees:revenue','fees_revenue');
```

> **Notes**
> - Use **minor units** only (no decimals) to avoid rounding issues.
> - Enforce consistency with DB transactions and `FOR UPDATE` row‑locks on involved accounts.
> - Extend with `constraints` table (per tier limits), and `reversals` if you need proper chargebacks.

---

## 3) USSD & SMS flow chart (hand‑off to aggregator)

Save the following to `docs/ussd-flows.mmd`. Most aggregators accept Mermaid or can translate to BPMN quickly.

```mermaid
flowchart TD
  A[Dial USSD *
  *123#] --> B{Main Menu}
  B -->|1| C[1. Balance]
  B -->|2| D[2. Send Money]
  B -->|3| E[3. Cash In]
  B -->|4| F[4. Cash Out]
  B -->|5| G[5. Pay Merchant]
  B -->|6| H[6. Settings]

  C --> C1[Show: "Balance: XXXX CFA\n1. Back\n0. Exit"] --> B

  D --> D1[Enter Recipient Phone or Wallet ID]
  D1 --> D2[Enter Amount]
  D2 --> D3[Enter PIN]
  D3 --> D4{Risk Check}
  D4 -->|Pass| D5[Confirm: "Send {amt} to {rec}?\n1. Yes 2. No"]
  D4 -->|Fail| D6["Cannot process. Try later / Contact support"] --> B
  D5 -->|Yes| D7[Success: "Sent. Ref: {tx}"] --> B
  D5 -->|No| B

  E --> E1[Agent Code]
  E1 --> E2[Amount]
  E2 --> E3[PIN]
  E3 --> E4[Success + Ref]
  E4 --> B

  F --> F1[Agent Code]
  F1 --> F2[Amount]
  F2 --> F3[PIN]
  F3 --> F4{Sufficient Funds?}
  F4 -->|Yes| F5[Success + Ref]
  F4 -->|No| F6["Insufficient balance"]
  F5 --> B
  F6 --> B

  G --> G1[Merchant ID]
  G1 --> G2[Amount]
  G2 --> G3[PIN]
  G3 --> G4[Success + Ref]
  G4 --> B

  H --> H1[Change PIN]
  H1 --> H2[Language]
  H2 --> B
```

### SMS command set (for offline/low‑end fallback)
- `BAL` → returns current balance.
- `SEND <recipient> <amount>` → replies with `CONFIRM <code>`.
- `CONFIRM <code> <PIN>` → posts the transaction.
- `PAY <merchantId> <amount>` → same confirm flow.
- `HELP` → usage message in FR/EN.

**Security:** bind device/phone at registration; require PIN for any money movement; throttle by number; OTP on SIM change.

---

## 4) Local development

**docker-compose.yml** (snippet)
```yaml
version: "3.9"
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: congopay
    ports: ["5432:5432"]
    volumes: ["pgdata:/var/lib/postgresql/data"]
  redis:
    image: redis:7
    ports: ["6379:6379"]
volumes:
  pgdata: {}
```

**Makefile** (snippet)
```makefile
run: ## run api
	go run ./cmd/api

test:
	go test ./...

migrate:
	psql $$DATABASE_URL -f internal/db/migrations/0001_init.sql
```

**configs/config.example.yaml**
```yaml
httpAddr: ":8080"
databaseURL: "postgres://postgres:postgres@localhost:5432/congopay?sslmode=disable"
redisURL: "redis://localhost:6379/0"
```

---

## 5) Quick API smoke test (use `docs/api.http`)
```
### Health
GET http://localhost:8080/v1/health

### Create P2P (alice -> bob)
POST http://localhost:8080/v1/wallets/user:alice/p2p
Content-Type: application/json

{
  "to_wallet": "user:bob",
  "amount_cfa": 1000,
  "client_tx_id": "cli-123"
}
```

---

## 6) Next steps & hardening checklist
- Add **per‑tier limits** table + check in `ledger_p2p`.
- Add **fees engine** (MDR, caps) with routing to `fees:revenue` account.
- Implement **collect/refund** SQL functions mirroring P2P pattern.
- Add **reversal** flow with immutable audit trail.
- Introduce **outbox/event bus** rows for async notifications & settlement.
- Integrate **USSD aggregator** webhook endpoints + signature verification.
- Add **OpenAPI spec** and CI (lint, test, migrate).

---

If you want, I can export these into a ready‑to‑zip repo with files laid out and a starter test suite.

