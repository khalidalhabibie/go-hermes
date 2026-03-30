# go-hermes

`go-hermes` is a production-minded digital wallet backend written in Go. It demonstrates a modular Fiber API with PostgreSQL, Redis, GORM, JWT authentication, idempotent money movement, immutable ledger entries, rate limiting, audit trails, webhook delivery with retry, Prometheus-friendly metrics, Swagger docs, Dockerized local setup, and focused automated tests.

## Features

- User registration and login with bcrypt password hashing and JWT access tokens
- Automatic wallet provisioning with one main wallet per user
- Wallet balance lookup, top up, internal transfer, transaction history, and ledger history
- Idempotency protection for top up and transfer via `Idempotency-Key`
- Redis-backed rate limiting for login and money-movement endpoints
- Audit logs for sensitive actions and admin reads
- Webhook delivery records, async processing, and retry support for successful transaction events
- Structured logging, request/trace correlation, Prometheus metrics, recovery middleware, and health checks
- PostgreSQL schema migration with `golang-migrate`
- Dockerized local stack for app, database, and Redis

## Architecture Summary

The service follows a modular clean-ish architecture:

- `cmd/api`: application bootstrap
- `internal/config`: environment-driven configuration
- `internal/delivery/http`: Fiber handlers, DTOs, route registration, response writers
- `internal/middleware`: auth, role checks, request logging
- `internal/usecase`: business logic and orchestration
- `internal/repository`: GORM-backed persistence and transaction manager
- `internal/entity`: domain entities and enum-like types
- `internal/pkg`: cross-cutting helpers like JWT, hashing, logger, pagination, validation
- `migrations`: SQL schema migrations
- `docs`: OpenAPI specification and operational notes

Detailed architecture notes, ERD, sequence flows, and design decisions are documented in [architecture.md](/home/themisteriousone/Code/go-herems/docs/architecture.md).
Rate limiting is documented in [rate-limiting.md](/home/themisteriousone/Code/go-herems/docs/rate-limiting.md), and webhook delivery behavior is documented in [webhooks.md](/home/themisteriousone/Code/go-herems/docs/webhooks.md).
Operational visibility is summarized in [observability.md](/home/themisteriousone/Code/go-herems/docs/observability.md).

## Folder Structure

```text
.
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
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

## How To Run Locally

There are two supported ways to run the project:

1. PostgreSQL + Redis in Docker, API running locally on your machine
2. Full stack in Docker, including PostgreSQL, Redis, migration runner, and API

### Option 1: PostgreSQL And Redis In Docker, API Local

1. Copy environment variables:

```bash
cp .env.example .env
```

2. Start infrastructure services:

```bash
docker compose up -d postgres redis
```

3. Apply migrations from your local machine:

```bash
export DB_DSN='postgres://postgres:postgres@localhost:5432/go_hermes?sslmode=disable'
make migrate-up
```

4. Run the API locally:

```bash
make run
```

5. Open Swagger:

```text
http://localhost:8080/swagger
```

6. Inspect metrics if needed:

```text
http://localhost:8080/metrics
```

This mode is useful when you want:

- fast local iteration with Go tooling on your host machine
- PostgreSQL and Redis isolated in Docker
- easier debugging with local breakpoints and hot reload tooling

### Option 2: Full Stack In Docker

To run PostgreSQL, Redis, migrations, and the API all in containers:

```bash
cp .env.example .env
docker compose up --build
```

This starts:

- PostgreSQL
- Redis
- migration runner
- Fiber API on `:8080`

Open:

```text
http://localhost:8080/swagger
http://localhost:8080/metrics
```

## Docker Compose

The current [docker-compose.yml](/home/themisteriousone/Code/go-herems/docker-compose.yml) supports both workflows:

- `docker compose up -d postgres`
  Runs only the database dependencies when the API will be started locally
- `docker compose up --build`
  Runs the full stack in containers

There is no need for a separate compose file unless you want stricter environment separation later, such as `compose.dev.yml` and `compose.full.yml`.

## Migration Commands

The `Makefile` expects `golang-migrate` installed locally.

```bash
export DB_DSN='postgres://postgres:postgres@localhost:5432/go_hermes?sslmode=disable'
make migrate-up
make migrate-down
```

## Redis, Rate Limiting, And Webhooks

Redis is used for rate limiting on:

- `POST /api/v1/auth/login`
- `POST /api/v1/wallets/me/top-up`
- `POST /api/v1/transfers`

Configuration is environment-driven through:

- `RATE_LIMIT_WINDOW_SECONDS`
- `RATE_LIMIT_LOGIN`
- `RATE_LIMIT_TOPUP`
- `RATE_LIMIT_TRANSFER`

Webhook support emits:

- `wallet.top_up.success`
- `wallet.transfer.success`

Webhook delivery is persistence-backed and retryable:

- successful top up or transfer creates a `webhook_deliveries` record
- a lightweight background worker attempts delivery asynchronously
- failures move to `RETRYING`
- max retry exhaustion moves the record to `FAILED`

Observability additions:

- `GET /metrics` exposes Prometheus-compatible metrics
- request logs now include `request_id` and `trace_id` when `traceparent` is present
- webhook lifecycle and rate-limit rejections are exported as counters

Admin webhook endpoints:

- `GET /api/v1/admin/webhooks`
- `GET /api/v1/admin/webhooks/:id`

See:

- [rate-limiting.md](/home/themisteriousone/Code/go-herems/docs/rate-limiting.md)
- [webhooks.md](/home/themisteriousone/Code/go-herems/docs/webhooks.md)

## Testing

The repository includes both unit tests and integration-style HTTP tests.

Run everything:

```bash
make test
```

Run only unit tests:

```bash
make test-unit
```

Run only integration-style HTTP tests:

```bash
make test-integration
```

Run only Postgres-backed integration tests:

```bash
export TEST_DATABASE_DSN='host=localhost port=5432 user=postgres password=postgres dbname=go_hermes sslmode=disable TimeZone=UTC'
make test-postgres-integration
```

Detailed testing notes and tradeoffs are documented in [testing.md](/home/themisteriousone/Code/go-herems/docs/testing.md).

## Local Webhook Testing

To test webhooks locally:

1. set `WEBHOOK_ENABLED=true`
2. set `WEBHOOK_TARGET_URL` to a local mock endpoint or webhook inspector
3. run a successful top up or transfer
4. inspect delivery state from admin webhook endpoints
5. watch application logs for send, retry, success, and failure events

## Sample Environment Variables

See [.env.example](/home/themisteriousone/Code/go-herems/.env.example).

Important values:

- `APP_PORT=8080`
- `DB_HOST=localhost`
- `DB_PORT=5432`
- `REDIS_PORT=6379`
- `DB_USER=postgres`
- `DB_PASSWORD=postgres`
- `DB_NAME=go_hermes`
- `JWT_SECRET=super-secret-change-me`
- `RATE_LIMIT_LOGIN=10`
- `RATE_LIMIT_TOPUP=20`
- `RATE_LIMIT_TRANSFER=20`
- `WEBHOOK_ENABLED=false`
- `WEBHOOK_TARGET_URL=`
- `METRICS_ENABLED=true`
- `SEED_ADMIN_EMAIL=admin@gohermes.local`
- `SEED_ADMIN_PASSWORD=ChangeMe123!`

## Sample API Flow

Register:

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "name":"Alice",
    "email":"alice@example.com",
    "password":"Password123"
  }'
```

Login:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "email":"alice@example.com",
    "password":"Password123"
  }'
```

Top up:

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

Transfer:

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

## Design Decisions

- Amounts use `int64` to avoid floating-point precision issues
- Top up and transfer execute inside DB transactions
- Wallet rows are locked with deterministic ordering to reduce deadlocks
- Ledger entries are append-only and never updated
- Database-level `CHECK` constraints backstop critical invariants such as positive amounts, valid enum states, and valid transaction shapes
- Idempotency records use `(idempotency_key, user_id, endpoint)` uniqueness
- Rate limiting is Redis-backed to work consistently across app instances
- Webhook delivery is decoupled from the critical transaction response path
- Webhook retries are persisted so failed callbacks remain inspectable and recoverable
- Admin seeding now runs transactionally to avoid partial bootstrap state
- Prometheus-compatible metrics make request, webhook, and rate-limit behavior visible during local runs and CI
- Swagger is shipped as a manual OpenAPI file to keep local setup lightweight

For deeper reasoning behind idempotency, locking, and ledger modeling, see [architecture.md](/home/themisteriousone/Code/go-herems/docs/architecture.md).

## Future Improvements

- Refresh token and token revocation support
- Separate admin authentication policy and permission matrix
- Outbox/eventing for notifications and downstream integrations
- Metrics and tracing with OpenTelemetry
- Dedicated integration test suite with ephemeral Postgres
