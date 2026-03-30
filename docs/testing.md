# Testing Strategy

## Overview

`go-hermes` uses a layered test strategy designed to give strong confidence in money movement behavior without overcomplicating the local developer experience.

There are three main categories of automated tests:

- Unit tests for use cases and business rules
- Integration-style HTTP tests that exercise routing, middleware, validation, auth, and handlers end-to-end using an isolated in-memory repository layer
- Postgres-backed integration tests for transactionality, lock ordering, and migration compatibility

## What Is Covered

### Unit Tests

The unit test suite focuses on the highest-risk business logic:

- register success
- register duplicate email
- login success
- login invalid password
- get current user
- get wallet details
- top up success
- top up invalid amount
- top up idempotency replay with same payload
- top up idempotency conflict with different payload
- transfer success
- transfer insufficient balance
- transfer to same wallet
- transfer invalid recipient wallet
- transfer invalid amount

### Integration-Style HTTP Tests

The HTTP tests exercise the application as a running API:

- register -> login -> get wallet
- top up flow end-to-end
- transfer flow end-to-end
- transaction listing flow
- rate limit exceeded response
- webhook delivery record creation after top up
- webhook delivery record creation after transfer
- webhook retry then success behavior
- admin reconciliation report healthy path
- validation failures for malformed input
- missing idempotency key handling
- missing JWT handling
- invalid JWT handling
- forbidden admin access for normal users
- resource isolation for transaction detail access

### Postgres Integration Tests

The Postgres-backed suite verifies behaviors that the in-memory test double cannot prove:

- migrations apply cleanly to a fresh schema
- `LockByIDs` returns a deterministic order suitable for deadlock reduction
- transfer updates remain atomic against a real PostgreSQL database
- failed transfers roll back balances, transactions, and ledger writes
- concurrent top ups using the same idempotency key create only one transaction and one ledger entry
- concurrent transfers using the same idempotency key create only one transaction and two ledger entries
- duplicate concurrent requests do not mutate balances twice
- competing transfers on the same wallet serialize correctly and leave exactly one successful mutation when funds are insufficient for both
- reciprocal transfers complete successfully under real row locking
- reconciliation logic can detect wallet drift, orphan ledger entries, and broken transaction ledger shapes

## Critical Flows Covered

The most important trust-sensitive flows are intentionally tested from multiple angles:

- top up:
  unit coverage for business rules and idempotency
  HTTP coverage for request validation and end-to-end behavior
- transfer:
  unit coverage for balance checks and recipient validation
  HTTP coverage for real route + auth + response behavior
- idempotency:
  replay behavior with same payload
  conflict behavior with different payload
  assertions that balance, transactions, and ledger counts do not change twice
  Postgres-backed proof that concurrent retries with the same key still mutate state only once
- concurrency:
  Postgres-backed proof that same-wallet contention does not overdraw balances
  rollback proof that failed contenders do not leave partial transactions or partial ledger writes
  reciprocal transfer proof under real row locking
- webhooks:
  delivery record creation after successful balance mutation
  retry and eventual success behavior for outbound callback processing
- rate limiting:
  explicit HTTP coverage for `429` responses on sensitive endpoints

## Test Approach And Tradeoffs

The default integration-style tests use an in-memory repository implementation instead of a real Postgres database.

This choice keeps tests:

- fast
- deterministic
- easy to run in any environment
- independent of Docker or external services

Tradeoffs:

- these tests do not validate real PostgreSQL behavior such as row locking semantics, SQL constraints, or migration compatibility
- GORM-specific query behavior is not deeply exercised by the test suite
- the Postgres-backed integration suite is opt-in through `TEST_DATABASE_DSN` to keep local setup lightweight
- the strongest concurrency guarantees in this repository are intentionally proven only in the Postgres-backed suite because they depend on real transaction and lock behavior

## What Remains Untested

The following areas still deserve deeper coverage in a larger production system:

- admin endpoint positive-path authorization tests
- failure-path tests for audit log persistence and partial infrastructure failures
- chaos-style tests for Redis outages during rate limit checks
- end-to-end webhook delivery against a real downstream stub in CI
- scheduled reconciliation runs and alerting on failed reports

## Running Tests Locally

Run all tests:

```bash
make test
```

Run unit tests only:

```bash
make test-unit
```

Run integration-style HTTP tests only:

```bash
make test-integration
```

Run only Postgres-backed integration tests:

```bash
export TEST_DATABASE_DSN='host=localhost port=5432 user=postgres password=postgres dbname=go_hermes sslmode=disable TimeZone=UTC'
make test-postgres-integration
```

You can also run raw Go commands:

```bash
go test ./...
go test ./internal/...
go test ./tests/...
```

The Postgres-backed tests automatically skip when `TEST_DATABASE_DSN` is not set.

## Notes

- The in-memory repositories are built only for tests and exist to reduce setup cost while preserving realistic request flows.
- The business-critical assertions prioritize balance correctness, idempotency correctness, and authorization behavior over broad but shallow endpoint coverage.
