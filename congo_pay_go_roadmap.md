# CongoPay – Extended with Card Funding Service

This update adds a **card funding/withdrawal service** on top of the cash/agent model.

---

## Progress Tracker (working doc)

- [x] Repo scaffolding and base configs (`.gitignore`, `.editorconfig`, `.gitattributes`)
- [x] Environment template added (`.env.example`)
- [x] Dockerfile (multi-stage) and `docker-compose.yml` (Postgres, Redis, API)
- [x] Makefile helpers (`build`, `test`, `run`, `compose-*`)
- [x] Initialize Go module and minimal API (Fiber) with `/healthz`
- [x] CI workflow (GitHub Actions) for tidy/build/test + Docker build
- [x] Postman collection + local environment
- [ ] Identity users: DB schema (users), repo wired, register/auth tested
- [ ] Persistence wiring (pgx pool, Redis client) and readiness checks
- [ ] Database migrations for ledger schema (Phase 1)
- [ ] Core services (wallet, payments) fully implemented and tested
- [ ] Card funding/withdrawal end‑to‑end with acquirer stub + migrations
- [ ] USSD/SMS/QR baseline channels
- [ ] Observability (metrics/logs/traces) and IaC

## 1 New service in repo scaffold

```
internal/
  ├── funding/
  │   ├── handlers.go      # REST endpoints for card in/out
  │   ├── dto.go           # request/response structs
  │   ├── service.go       # business logic (ledger + acquirer API)
  │   └── acquirer.go      # connector to bank/acquirer API (stub)
```

### internal/routes/routes.go (add endpoints)
```go
p := payments.NewHandler(pg)
api.Post("/wallets/:walletId/p2p", p.P2P)
api.Post("/merchants/:merchantId/collect", p.Collect)
api.Post("/payments/:txId/refund", p.Refund)

f := funding.NewHandler(pg)
api.Post("/wallets/:walletId/fund/card", f.CardIn)
api.Post("/wallets/:walletId/withdraw/card", f.CardOut)
```

### internal/funding/handlers.go (sample)
```go
package funding

import (
    "context"
    "net/http"
    "github.com/gofiber/fiber/v2"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/google/uuid"
)

type Handler struct { db *pgxpool.Pool }

func NewHandler(db *pgxpool.Pool) *Handler { return &Handler{db: db} }

type CardInRequest struct {
    CardNumber string `json:"card_number"`
    Expiry     string `json:"expiry"`
    CVV        string `json:"cvv"`
    Amount     int64  `json:"amount_cfa"`
    ClientTxID string `json:"client_tx_id"`
}

type CardOutRequest struct {
    CardNumber string `json:"card_number"`
    Amount     int64  `json:"amount_cfa"`
    ClientTxID string `json:"client_tx_id"`
}

type TxResponse struct { TxID string `json:"tx_id"`; Status string `json:"status"` }

// Card top-up
func (h *Handler) CardIn(c *fiber.Ctx) error {
    walletId := c.Params("walletId")
    var req CardInRequest
    if err := c.BodyParser(&req); err != nil { return fiber.NewError(http.StatusBadRequest, err.Error()) }

    txID := uuid.NewString()
    ctx := context.Background()

    // TODO: call acquirer API here (simulate success)
    status := "pending_settlement"

    // Post to ledger: credit wallet, debit suspense account
    _, err := h.db.Exec(ctx, `
      SELECT ledger_card_in($1,$2,$3,$4,$5)`,
      walletId, req.Amount, req.ClientTxID, txID, "card_in",
    )
    if err != nil { return fiber.NewError(http.StatusBadRequest, err.Error()) }

    return c.Status(http.StatusCreated).JSON(TxResponse{TxID: txID, Status: status})
}

// Card withdrawal
func (h *Handler) CardOut(c *fiber.Ctx) error {
    walletId := c.Params("walletId")
    var req CardOutRequest
    if err := c.BodyParser(&req); err != nil { return fiber.NewError(http.StatusBadRequest, err.Error()) }

    txID := uuid.NewString()
    ctx := context.Background()

    // TODO: call acquirer payout API here
    status := "pending_settlement"

    // Post to ledger: debit wallet, credit suspense
    _, err := h.db.Exec(ctx, `
      SELECT ledger_card_out($1,$2,$3,$4,$5)`,
      walletId, req.Amount, req.ClientTxID, txID, "card_out",
    )
    if err != nil { return fiber.NewError(http.StatusBadRequest, err.Error()) }

    return c.Status(http.StatusCreated).JSON(TxResponse{TxID: txID, Status: status})
}
```

---

## 2) Schema updates (Postgres)

Extend `transactions.kind` to support card operations:
```sql
ALTER TYPE tx_status ADD VALUE IF NOT EXISTS 'pending_settlement';

-- New transaction kinds: card_in, card_out
-- Extend ledger functions
CREATE OR REPLACE FUNCTION ledger_card_in(wallet_code TEXT, amount NUMERIC, client_tx_id TEXT, server_tx_id UUID, kind TEXT)
RETURNS TEXT AS $$
DECLARE
  acct UUID; suspense UUID; tx UUID;
BEGIN
  SELECT id INTO acct FROM accounts WHERE code = wallet_code FOR UPDATE;
  SELECT id INTO suspense FROM accounts WHERE code = 'suspense:card' FOR UPDATE;
  IF acct IS NULL OR suspense IS NULL THEN RAISE EXCEPTION 'account not found'; END IF;

  INSERT INTO transactions(id, ext_id, kind, status)
    VALUES (server_tx_id, client_tx_id, kind, 'pending_settlement') RETURNING id INTO tx;

  INSERT INTO entries(tx_id, account_id, side, amount) VALUES
    (tx, acct, 'debit', amount),
    (tx, suspense, 'credit', amount);

  RETURN 'pending_settlement';
END;$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION ledger_card_out(wallet_code TEXT, amount NUMERIC, client_tx_id TEXT, server_tx_id UUID, kind TEXT)
RETURNS TEXT AS $$
DECLARE
  acct UUID; suspense UUID; tx UUID;
BEGIN
  SELECT id INTO acct FROM accounts WHERE code = wallet_code FOR UPDATE;
  SELECT id INTO suspense FROM accounts WHERE code = 'suspense:card' FOR UPDATE;
  IF acct IS NULL OR suspense IS NULL THEN RAISE EXCEPTION 'account not found'; END IF;

  IF (SELECT available FROM account_balances WHERE id = acct) < amount THEN
    RAISE EXCEPTION 'insufficient funds';
  END IF;

  INSERT INTO transactions(id, ext_id, kind, status)
    VALUES (server_tx_id, client_tx_id, kind, 'pending_settlement') RETURNING id INTO tx;

  INSERT INTO entries(tx_id, account_id, side, amount) VALUES
    (tx, acct, 'credit', amount),
    (tx, suspense, 'debit', amount);

  RETURN 'pending_settlement';
END;$$ LANGUAGE plpgsql;

-- Add suspense account for card
INSERT INTO accounts(code,type) VALUES ('suspense:card','suspense') ON CONFLICT DO NOTHING;
```

---

## 3) Settlement flow

1. **User top-up (card_in)** → Ledger marks `pending_settlement`, money sits in `suspense:card`.
2. **Acquirer settles** → Ops cron job moves from `suspense:card` to `fees:revenue` (minus MDR) + confirms user credit.
3. **User withdraw (card_out)** → Debit wallet → credit `suspense:card`. Ops job waits for acquirer confirmation, then mark as `posted`.

---

## 4) Why this works in Congo
- You can support **urban/bank-linked users** with debit cards, while the mass market stays with **cash/agents/USSD**.
- Suspense accounts isolate risk until acquirer settlement is final.
- Same **double-entry discipline** as cash/QR, just a new rail.

---

This gives you: **wallet ↔ agents ↔ cards** all in one ledger.

---

Do you want me to also extend the **USSD flow** to show “Fund by Card” and “Withdraw to Card” options, or keep them app-only? 


---

# 7) End‑to‑end implementation roadmap (very detailed)

> This roadmap sequences engineering, compliance, partnerships, agent ops, and GTM. It’s **phase‑gated with exit criteria**, so you can parallelize safely. Timeboxes are left to your team capacity; focus on the **exit criteria** to move forward.

## Tracks & owners
- **ENG‑CORE** (Go/Fiber, DB, services)
- **ENG‑MOBILE** (Android merchant, user app)
- **ENG‑CHANNELS** (USSD/SMS/QR/NFC)
- **RISK/COMPLIANCE** (KYC/AML, policies, audits)
- **PARTNERSHIPS** (bank sponsor, acquirer, USSD aggregator, SMS)
- **OPS** (agents, support, L1/L2 runbooks)
- **SRE/SEC** (infra, observability, PCI path, DR)
- **DATA** (reporting, ClickHouse, growth analytics)
- **GTM** (merchant acquisition, pricing, comms)

---

## Phase 0 – Foundations & regulatory alignment
**Goals:** establish the legal/operating scaffolding so you can lawfully pilot.

**Work:**
- RISK/COMPLIANCE: Draft & approve KYC/AML program; appoint MLRO; SAR templates; record retention.
- PARTNERSHIPS: Bank sponsorship (PSP/EMI umbrella), data residency attestation; acquirer shortlist; USSD aggregator NDA; reserve USSD short code; claim SMS sender ID.
- SEC: DPIA (data protection impact assessment); encryption/HSM plan; key management policy; vendor risk reviews.
- OPS: Support model (WhatsApp + hotline); dispute/chargeback policy; complaint SLAs.

**Exit criteria:** (1) Signed bank sponsorship/MOU. (2) Reserved USSD code + SMS sender ID. (3) AML/KYC policies approved. (4) PCI scope defined (SAQ‑A path, no PAN storage).

---

## Phase 1 – Core ledger & identity
**Goals:** immutable double‑entry ledger live; baseline identity & auth.

**Work (ENG‑CORE):**
- Apply `0001_init.sql` migration; provision `accounts` (seed users/fees/suspense).
- Implement idempotency, request‑id, audit middleware.
- Services: `wallet‑svc`, `payment‑svc` (P2P), `identity‑svc` (Tier0/1, PIN, device bind), `notif‑svc` (SMS provider stub).
- Unit tests: ledger invariants; concurrent postings; idempotency replays; available‑balance calc; hold/release.
- SEC: Vault integration for secrets; JWT via Keycloak; mTLS for internal services.

**Exit criteria:** All ledger property tests pass; 99.99% idempotency reliability under fault injection; PII encrypted at rest; threat model reviewed.

### Implementation log
- [x] Bootstrap minimal API server (Fiber), config loader, `/healthz`, and `/api/v1/ping`.
- [x] Repo infra: Docker/Compose, Makefile, CI, env templates, Postman.
- [x] Request‑id + audit middleware, idempotency (enabled when Redis present).
- [x] Postgres/Redis connectors (optional in dev) and health checks.
- [ ] Identity: persistent users (DB schema + repo), register/login e2e, tests.
- [ ] Wallet provisioning, P2P transfers, notification stubs, and base SQL schema.
- [ ] Card funding/withdrawal flows with suspense ledger and acquirer stub.

---

## Next Up (Identity First)

1) Users (Identity)
- Add migration: `migrations/0001_users.sql` with `users(id uuid pk, phone text unique, tier text, pin_hash bytea, device_id text, created_at timestamptz)`
- Wire repository to Postgres (already scaffolded) and add unit tests
- Expose endpoints: `POST /api/v1/identity/register`, `POST /api/v1/identity/authenticate` (already scaffolded)
- Happy-path Postman requests for register/authenticate

2) Wallets
- Migration: `wallets(id uuid pk, owner_id uuid fk users(id), account_code text unique, currency text, status text, created_at timestamptz)`
- Wire repository + service; create on demand for new users

3) Ledger base
- Migration set for accounts, transactions, entries; constraints and indexes
- Implement balance query and transfer invariants; add property tests

---

## Phase 2 – Channels baseline (QR static, USSD, SMS)
**Goals:** usable MVP for P2P and merchant QR without app stores friction.

**Work (ENG‑CHANNELS / ENG‑MOBILE):**
- USSD main menu implemented per `docs/ussd-flows.mmd`; aggregator webhook spec + HMAC verification.
- Static QR (EMVCo content) – merchant‑presented; user scans via app or USSD “Pay Merchant”.
- SMS command handlers: `BAL`, `SEND`, `CONFIRM`, `PAY`, `HELP` with rate limits, replay protection.
- Merchant Android (alpha): show QR, view recent transactions, manual refund.

**Exit criteria:** P2P & Pay‑merchant complete through (a) app, (b) USSD, (c) SMS. Load test: sustained ≥100 TPS on postings with <200ms p95 API latency in‑region.

---

## Phase 3 – Agents & cash in/out
**Goals:** deploy real liquidity via agents; commissions; basic risk guards.

**Work (OPS / ENG‑CORE):**
- `agent‑svc`: float wallet, tiered commissions, KYC for agents; geo‑binding; liquidity alerts.
- Cash‑in/out flows (USSD + app) with agent code, receipts, dispute trails.
- Reconciliation jobs for agent float; daily agent statements.

**Exit criteria:** 20+ pilot agents on‑boarded; <2% cashout failures; liquidity alerts functioning; audit trail downloadable for each cash event.

---

## Phase 4 – Risk/AML v1 & dispute ops
**Goals:** stop obvious fraud; comply with SAR; operationalize case review.

**Work (RISK/COMPLIANCE / ENG‑CORE / DATA):**
- Rules: velocity, amount spikes, device change, geo anomalies; per‑tier daily/monthly caps in DB.
- Case mgmt UI (basic): queue, notes, status, export SAR PDF.
- Blacklist/PEP/Sanctions screening (batch + real‑time on KYC).

**Exit criteria:** Rules measurable (TP/FP < 5% in pilot); SAR pack export works; KYC failures quarantined correctly; kill‑switch tested.

---

## Phase 5 – Merchant settlement & reconciliation
**Goals:** merchants can choose T+0/T+1; MDR/caps enforced; bank recon live.

**Work (ENG‑CORE / PARTNERSHIPS / DATA):**
- `billing‑svc`: MDR % + caps; fee postings to `fees:revenue`.
- `settlement‑svc`: daily CSV to bank (or ISO‑20022 pain.001 if supported); ingest bank statements; 3‑way recon (ledger ↔ gateway ↔ bank).
- Merchant app: auto‑sweep config; statement PDF; refund with auth/hold.

**Exit criteria:** Recon variance < 0.05%; 100% merchant payouts matched; MDR correctly capped across micro‑tx.

---

## Phase 6 – Offline vouchers & store‑and‑forward
**Goals:** operate in no‑data zones; reconcile later without revenue loss.

**Work (ENG‑CHANNELS):**
- One‑time signed vouchers (8–10 digits) valid ~15 min; merchant cache; conflict resolution policy.
- Idempotent retry queue; poison‑message handling; delayed posting strategy with risk flag.

**Exit criteria:** Offline success rate ≥ 98% in field tests; conflicts auto‑resolved with escrow/hold rule.

---

## Phase 7 – Card rails (card‑in / card‑out)
**Goals:** enable debit/credit cards for wallet top‑ups and withdrawals safely.

**Work (PARTNERSHIPS / ENG‑CORE / SEC):**
- Choose acquirer (matrix: fees, Visa Direct/MC Send support, settlement time, Webhooks, 3DS2, tokenization).
- Implement `funding‑svc` (already scaffolded):
  - **Card‑in:** hosted payment page/hosted fields (to keep SAQ‑A scope), 3DS challenge flow, webhook for auth/capture, post to ledger with `pending_settlement` → finalize on settlement file.
  - **Card‑out:** push‑to‑card payout API; suspense postings until acquirer confirms.
- PCI path: SAQ‑A, quarterly ASV scans, no PAN storage; Vault for tokens if acquirer provides network tokens.

**Exit criteria:** End‑to‑end card top‑up and payout demoed; failed/chargeback paths exercised; settlement recon green; PCI SAQ‑A signed.

---

## Phase 8 – Observability, reliability & DR
**Goals:** production‑grade reliability with clear SLOs.

**Work (SRE/SEC):**
- OpenTelemetry traces + metrics; dashboards: TPS, p95 latency, error budgets, queue lag, agent liquidity.
- SLOs: core payments 99.9% available; RPO ≤ 5m; RTO ≤ 15m; active‑active or pilot hot‑standby.
- Backups: PITR on Postgres; chaos drills; runbooks for outages and recon backlog.

**Exit criteria:** SLOs defined and monitored; DR drill pass; on‑call rotations + escalation tree live.

---

## Phase 9 – Pilot launch (limited geography)
**Goals:** real users, controlled blast radius.

**Work (GTM / OPS / DATA):**
- Target 2 neighborhoods; onboard 100 merchants, 20 agents, 2,000 users.
- Incentives: free P2P, merchant MDR promo, agent commission booster.
- Feedback loop: in‑app survey; NPS; merchant council weekly.

**Exit criteria:** Retention D30 ≥ 30% (users), merchant activity ≥ 60% weekly, incident rate < 0.5% of tx, fraud loss < 5 bps of volume.

---

## Phase 10 – Interop (bank rails, mobile money connectors)
**Goals:** move money in/out of banks and, optionally, mobile money.

**Work (PARTNERSHIPS / ENG‑CORE):**
- ISO‑20022 (pain.001/pain.002/camt.053) for EFT/ACH with partner bank; webhook ingestion.
- Optional mobile money connectors via partners; enforce clear fee policy.

**Exit criteria:** Bank transfer in/out live with full recon; limits/fees enforced; AML screening on external transfers.

---

## Phase 11 – Government & billers
**Goals:** make the wallet useful for public payments.

**Work (PARTNERSHIPS / ENG‑CORE):**
- Integrate utilities, school fees, fines; bulk disbursements for stipends.
- Split‑payments for fees/taxes (escrow account + auto split to treasury).

**Exit criteria:** 3+ billers live; one gov use‑case in production; SLA with ministries.

---

## Phase 12 – Credit & value‑add (optional)
**Goals:** monetize via responsible lending.

**Work (DATA / RISK):**
- Merchant cash‑advance from cash‑flow; scorecards from on‑us data; capped exposure; collections via settlement sweep.

**Exit criteria:** NPL < 3%; automated limits; compliance sign‑off.

---

## Engineering WBS (work breakdown) by component

### Core ledger & payments
- DB migrations, constraints, indexes
- Posting functions: p2p, collect, refund, card_in, card_out, reversals
- Idempotency service (Redis); audit trail tables
- Fees engine (MDR, caps); escrow/holds logic
- Reconciliation jobs (bank, agents, cards)

### Channels
- USSD aggregator adapter (HMAC sigs; retry & idempotency)
- SMS provider adapter; anti‑spam throttling; abuse detection
- QR (EMVCo) encoder/decoder; dynamic invoice generator
- Offline voucher signer/validator

### Apps
- Merchant Android: QR display, transaction feed, refunds, settlement config, statements
- User app: wallet, P2P, QR pay, vouchers, KYC, notifications

### Risk/Compliance
- Screening integrations (PEP/sanctions)
- Rules engine (velocity, geo, device); case mgmt UI
- SAR export; audit logs; policy automation (tier limits)

### SRE/SEC
- IaC (Terraform) for VPC, Postgres, Redis, object store
- CI/CD (lint, tests, migrations, image scan)
- Observability stack; alerting; chaos/failure drills
- Key management (Vault/HSM), secrets rotation; app hardening (TLS, CSP)

### Data/Reporting
- ClickHouse ingestion from event bus
- Regulatory reports (daily/weekly BEAC/CEMAC), merchant/agent dashboards
- Growth analytics (cohorts, funnels, LTV/ARPU)

---

## Compliance & PCI checklist (card phases)
- Use **hosted payment page/fields** only; avoid PAN on your servers
- 3DS2 flows; frictionless where possible
- Webhook signature verification; replay protection
- Chargeback management process with evidence packs
- SAQ‑A annually; ASV scans quarterly; WAF/CDN for HPP endpoints

---

## Test plan (must pass before pilot)
- **Functional:** P2P, merchant collect, refund, cash in/out, card in/out, USSD/SMS
- **Property‑based:** ledger always balances to zero; no negative available balances
- **Load:** sustained ≥100 TPS, spike ≥300 TPS; idempotent retries under packet loss
- **Security:** PIN brute‑force lockouts; token theft simulation; TLS downgrade attempts blocked
- **Resilience:** DB failover; message bus lag; offline voucher conflict
- **UAT:** 30 merchants, 5 agents, 100 users complete scripted scenarios

---

## Operational playbooks
- Incident severities & comms templates (status page, WhatsApp groups)
- Recon backlog clearing; stuck payout unblocking
- Fraud surge response: tighten limits, enable escrow, merchant freeze
- Data subject request (DSR) handling; breach notification flow

---

## Go‑to‑market (pilot)
- Merchant verticals first (groceries, pharmacies, transit)
- Starter kit: QR standee, sticker, how‑to card; onboarding in 15 minutes
- Incentives: MDR promo, referral bonuses, free POS dongle (limited)
- Weekly ops review: heatmaps (fails, fraud), merchant feedback, agent liquidity

---

## Key metrics (North Star & guardrails)
- **North Star:** Weekly Active Merchants (WAM) & Payer→Merchant Transactions (PMT)
- **Guardrails:**
  - p95 API latency < 250ms; success rate > 99.5%
  - Fraud losses < 10 bps of processed volume
  - Recon variance < 0.05%
  - Support CSAT > 4.5/5

---

## Dependencies & decisions (gates)
- Bank sponsor signed → unlock settlements
- USSD aggregator signed → unlock feature phone channel
- Acquirer selected → unlock card rails
- PCI SAQ‑A confirmed → allow public card flows

---

## Deliverables pack you can hand to partners
- **Technical:** OpenAPI spec, webhook specs, IP allowlists, signing keys, sample payloads
- **Operational:** AML/KYC policy PDFs, SAR templates, dispute SLAs, incident runbooks
- **Commercial:** MDR/fee tables, agent commission plan, merchant onboarding kit

---

### What I can generate next
- A **checklist spreadsheet** (CSV/XLSX) for this roadmap
- **OpenAPI YAML** for `wallet`, `payments`, `funding`, `settlement`, `ussd`
- **Gantt view** (Mermaid) inside `/docs/roadmap.mmd`
