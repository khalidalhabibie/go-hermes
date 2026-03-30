# go-hermes

`go-hermes` is a production-minded digital wallet backend written in Go. It is designed as a portfolio-grade service that demonstrates realistic backend engineering practices around authentication, balance mutation, idempotency, ledgering, rate limiting, webhook delivery, observability, testing, and operational setup.

## Overview

The service exposes a REST API for:

- user registration and login
- automatic wallet provisioning
- wallet balance lookup
- top up and internal transfer
- transaction and ledger history
- admin audit and webhook inspection
- admin reconciliation reporting

The project is intentionally small enough to run locally, but structured to show senior backend concerns:

- transactional money movement
- idempotent write endpoints
- append-only ledger records
- Redis-backed abuse protection
- async webhook processing with retry
- PostgreSQL constraints for domain invariants
- request and operational visibility
- automated verification in unit, HTTP, and Postgres-backed integration tests

## Tech Stack

- Go
- Fiber
- PostgreSQL
- GORM
- Redis
- JWT
- `golang-migrate`
- `zerolog`
- Swagger / OpenAPI
- Testify
- Docker / Docker Compose
- GitHub Actions

## Key Features

- JWT authentication with `user` and `admin` roles
- one wallet per user, created automatically at registration
- top up and transfer flows execute inside database transactions and are protected by idempotency keys
- wallet balance updates use deterministic row-level locking to reduce deadlock risk and double-spend races
- transaction records plus immutable ledger entries for every balance mutation
- admin reconciliation checks wallet, ledger, and transaction consistency from persisted state under explicit model assumptions
- Postgres-backed tests cover row locking, contention, rollback behavior, and explicit corrupted-state reconciliation cases
- Redis-backed rate limiting on login and money-movement endpoints
- webhook delivery persistence with retry and async processing
- audit logging for important user and admin actions
- Prometheus-compatible metrics and structured logs with request correlation
- fail-fast runtime hardening for JWT configuration, metrics exposure, and admin seeding outside development
- SQL migrations and Dockerized local setup

## Architecture

The codebase follows a modular clean architecture approach:

- `cmd/api`
  Application bootstrap and dependency wiring
- `internal/config`
  Environment-driven configuration
- `internal/delivery/http`
  Fiber handlers, DTOs, route registration, and response writing
- `internal/middleware`
  Auth, role enforcement, request lifecycle, rate limiting, metrics, and tracing helpers
- `internal/usecase`
  Core business rules and orchestration
- `internal/repository`
  GORM repositories, transaction manager, and health integrations
- `internal/entity`
  Domain entities and enum-like types
- `internal/pkg`
  Shared helpers such as JWT, hashing, pagination, metrics, validation, and idempotency hashing
- `migrations`
  SQL schema changes
- `docs`
  Architecture notes, API docs, and operational references

Supporting documentation:

- [Architecture](docs/architecture.md)
- [Rate Limiting](docs/rate-limiting.md)
- [Webhooks](docs/webhooks.md)
- [Observability](docs/observability.md)
- [Reconciliation](docs/reconciliation.md)
- [Testing](docs/testing.md)
- [OpenAPI Spec](docs/openapi.yaml)

## Project Structure

```text
.
├── .github/workflows
├── cmd/api
├── docs
├── internal
│   ├── config
│   ├── delivery/http
│   ├── entity
│   ├── middleware
│   ├── pkg
│   ├── repository
│   └── usecase
├── migrations
├── tests
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

## Quick Start

### Prerequisites

- Go `1.20+`
- Docker and Docker Compose
- `golang-migrate` installed locally if you want to run migrations from your host machine

### Option 1: Run API Locally, Dependencies In Docker

1. Copy environment variables:

```bash
cp .env.example .env
```

2. Start PostgreSQL and Redis:

```bash
docker compose up -d postgres redis
```

3. Run migrations:

```bash
export DB_DSN='postgres://postgres:postgres@localhost:5432/go_hermes?sslmode=disable'
make migrate-up
```

4. Start the API:

```bash
make run
```

5. Open:

```text
Swagger: http://localhost:8080/swagger
Metrics: http://localhost:8080/metrics
Health:  http://localhost:8080/health
```

In development, `/metrics` stays open unless you set `METRICS_TOKEN`. Outside development, the API fails fast if metrics are enabled without that token.

This mode is best for local debugging and fast iteration with native Go tooling.

### Option 2: Run Full Stack In Docker

```bash
cp .env.example .env
docker compose up --build
```

This starts:

- PostgreSQL
- Redis
- migration runner
- API server

Available endpoints:

```text
Swagger: http://localhost:8080/swagger
Metrics: http://localhost:8080/metrics
Health:  http://localhost:8080/health
```

When `APP_ENV` is not `development`, scrape `/metrics` with `Authorization: Bearer <METRICS_TOKEN>` or `X-Metrics-Token: <METRICS_TOKEN>`.

## Environment Variables

See [.env.example](.env.example) for the full list.

Important values:

```env
APP_PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=go_hermes

REDIS_HOST=localhost
REDIS_PORT=6379

JWT_SECRET=super-secret-change-me
JWT_ISSUER=go-hermes
JWT_EXPIRY_MINUTES=60

RATE_LIMIT_WINDOW_SECONDS=60
RATE_LIMIT_LOGIN=10
RATE_LIMIT_TOPUP=20
RATE_LIMIT_TRANSFER=20

WEBHOOK_ENABLED=false
WEBHOOK_TARGET_URL=
WEBHOOK_SECRET=
WEBHOOK_MAX_RETRY=3
WEBHOOK_RETRY_INTERVAL_SECONDS=30

METRICS_ENABLED=true
METRICS_TOKEN=

SEED_ADMIN_ENABLED=true
SEED_ADMIN_EMAIL=admin@gohermes.local
SEED_ADMIN_PASSWORD=ChangeMe123!
```

Operational defaults:

- outside development, startup fails if `JWT_SECRET` is empty, uses a known placeholder, or is shorter than 32 characters
- JWT parsing enforces the configured `JWT_ISSUER`
- admin seeding only runs in development, even if `SEED_ADMIN_ENABLED=true` is set elsewhere
- when metrics are enabled outside development, `METRICS_TOKEN` is required and protects `GET /metrics`
- rate limiter backend errors are handled explicitly with a documented `fail_open` policy

## Core Endpoints

### Public

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /health`

### Authenticated User

- `GET /api/v1/users/me`
- `GET /api/v1/wallets/me`
- `GET /api/v1/wallets/me/balance`
- `POST /api/v1/wallets/me/top-up`
- `POST /api/v1/transfers`
- `GET /api/v1/transactions/me`
- `GET /api/v1/transactions/:id`
- `GET /api/v1/ledgers/me`
- `GET /api/v1/ledgers/transactions/:transactionId`

### Admin

- `GET /api/v1/admin/audit-logs`
- `GET /api/v1/admin/reconciliation`
- `GET /api/v1/admin/transactions`
- `GET /api/v1/admin/webhooks`
- `GET /api/v1/admin/webhooks/:id`

## Sample API Flow

### Register

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Alice",
    "email": "alice@example.com",
    "password": "Password123"
  }'
```

### Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "alice@example.com",
    "password": "Password123"
  }'
```

### Top Up

```bash
curl -X POST http://localhost:8080/api/v1/wallets/me/top-up \
  -H 'Authorization: Bearer <TOKEN>' \
  -H 'Idempotency-Key: topup-001' \
  -H 'Content-Type: application/json' \
  -d '{
    "amount": 50000,
    "description": "Initial funding"
  }'
```

### Transfer

```bash
curl -X POST http://localhost:8080/api/v1/transfers \
  -H 'Authorization: Bearer <TOKEN>' \
  -H 'Idempotency-Key: transfer-001' \
  -H 'Content-Type: application/json' \
  -d '{
    "recipient_wallet_id": "<RECIPIENT_WALLET_ID>",
    "amount": 10000,
    "description": "Peer transfer"
  }'
```

## Rate Limiting

Redis-backed rate limiting is applied to:

- `POST /api/v1/auth/login`
- `POST /api/v1/wallets/me/top-up`
- `POST /api/v1/transfers`

Behavior:

- configurable via environment variables
- caller-aware, using authenticated user identity where available and IP fallback otherwise
- returns HTTP `429` with `RATE_LIMIT_EXCEEDED`
- emits standard rate-limit headers
- fails open if Redis rate-limit checks error, and logs that policy explicitly so request availability wins over accidental denial

Details: [docs/rate-limiting.md](docs/rate-limiting.md)

## Webhooks

Successful balance-changing events can emit outbound webhooks:

- `wallet.top_up.success`
- `wallet.transfer.success`

Design:

- webhook deliveries are persisted in PostgreSQL
- delivery is attempted asynchronously
- failed attempts are retried with persisted retry state
- admin users can inspect deliveries via API

Details: [docs/webhooks.md](docs/webhooks.md)

## Observability

The service includes a practical baseline for local and CI visibility:

- structured logs
- `X-Request-ID`
- `traceparent` correlation into `trace_id`
- Prometheus-compatible metrics at `GET /metrics`
- health checks covering PostgreSQL and Redis

Outside development, metrics scraping requires `METRICS_TOKEN`.

Details: [docs/observability.md](docs/observability.md)

## Reconciliation

Admin users can run a reconciliation report to check persisted balances and transaction structure against ledger history.

This checker assumes:

- wallets start at balance `0`
- every balance mutation is represented by append-only ledger entries

The report checks:

- wallet balance equals ledger-derived balance
- the first ledger entry starts from the expected wallet genesis balance
- each ledger entry balance delta matches its entry type and amount
- each wallet ledger chain remains continuous from the first entry onward
- top up and transfer transactions have the expected ledger shape
- orphan ledger entries are detected
- ledger and transaction amounts agree

Endpoint:

- `GET /api/v1/admin/reconciliation`

Details: [docs/reconciliation.md](docs/reconciliation.md)

## Testing

The test strategy is layered:

- unit tests for use cases and core business rules
- integration-style HTTP tests using in-memory repositories
- Postgres-backed integration tests for row locking, transactionality, rollback behavior, and idempotency under contention
- Postgres-backed reconciliation tests for wallet drift and explicit corrupted-state cases such as orphan entries, missing transfer legs, and amount mismatches

The strongest correctness evidence in this repo comes from the Postgres-backed suite. The in-memory tests provide fast request and use-case coverage, but they do not replace database-backed validation of locking and persisted-state reconciliation.

Common commands:

```bash
make lint
make test
make test-race
make test-unit
make test-integration
```

Run Postgres-backed integration tests explicitly:

```bash
export TEST_DATABASE_DSN='host=localhost port=5432 user=postgres password=postgres dbname=go_hermes sslmode=disable TimeZone=UTC'
make test-postgres-integration
```

More detail: [docs/testing.md](docs/testing.md)

## Design Decisions

- Money values use `int64` to avoid floating-point precision problems
- Top up and transfer always run inside database transactions
- Wallet rows are locked in deterministic order to reduce deadlock risk
- Ledger entries are append-only and protected from mutation at the database layer
- Idempotency is enforced with `(idempotency_key, user_id, endpoint)` uniqueness plus payload hashing
- Webhook delivery is decoupled from the request critical path
- Database constraints backstop core domain invariants, not just application code
- Reconciliation checks wallet, ledger, and transaction consistency from persisted state under explicit model assumptions
- Redis is used for practical cross-instance rate limiting
- Admin seeding is transactional to avoid partial bootstrap state

For deeper reasoning behind idempotency, locking, sequence flow, and ledger modeling, see [docs/architecture.md](docs/architecture.md).

## CI

GitHub Actions runs:

- `golangci-lint`
- `go mod tidy`
- `go test ./...`
- `go test -race ./...`

The workflow provisions PostgreSQL for the Postgres-backed integration suite.

## Current Limitations

- Reconciliation is a practical consistency checker, not a formal proof system
- The reconciliation model assumes wallet history starts from balance `0` and does not cover imported opening balances or ledger backfills
- Swagger UI currently loads assets from a public CDN
- Webhook processing is in-process, not a distributed job system
- There is no refresh-token or session revocation flow yet
- Tracing is correlation-friendly but not a full OpenTelemetry implementation

## Future Improvements

- refresh token support and session management
- outbox-based event delivery
- OpenTelemetry tracing
- stronger admin permission matrix
- dedicated load and benchmark coverage
