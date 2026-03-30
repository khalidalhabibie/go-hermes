# Webhooks

## Overview

`go-hermes` emits outbound webhooks for successful balance-changing events:

- `wallet.top_up.success`
- `wallet.transfer.success`

The design is intentionally decoupled:

1. the transaction flow succeeds first
2. a `webhook_deliveries` record is created in the database
3. an async worker attempts delivery
4. failures are retried with persisted state

This keeps webhook delivery from being on the critical path of the user-facing API response.

## Delivery Record Model

The `webhook_deliveries` table stores:

- event type
- transaction id and transaction reference
- target URL
- payload
- optional secret
- delivery status
- retry count
- max retry
- last error
- last HTTP status
- next retry time
- delivered time
- created and updated timestamps

## Payload Shape

The webhook payload is JSON and includes:

- `event_type`
- `transaction_id`
- `transaction_ref`
- `source_wallet_id`
- `destination_wallet_id`
- `amount`
- `currency`
- `status`
- `occurred_at`

Example top up payload:

```json
{
  "event_type": "wallet.top_up.success",
  "transaction_id": "6d97d5a2-3cd3-41db-ae47-e0fe6f8f0d32",
  "transaction_ref": "TXN-8f1f2f99-3b3e-4f6f-932a-16d0778f49bb",
  "source_wallet_id": null,
  "destination_wallet_id": "f854186f-f4f2-4dc6-a13a-3fd3a0f903f0",
  "amount": 50000,
  "currency": "IDR",
  "status": "SUCCESS",
  "occurred_at": "2026-03-29T13:15:00Z"
}
```

## Signing

If `WEBHOOK_SECRET` is configured, outbound requests include:

- `X-Webhook-Signature: sha256=<hex>`

The signature is an HMAC-SHA256 over the raw JSON payload.

## Retry Strategy

Retries happen when:

- the HTTP request fails at network level
- the webhook target responds with non-2xx status

Behavior:

- a failed first attempt moves the delivery to `RETRYING`
- `retry_count` is incremented on every failed attempt
- `next_retry_at` is scheduled using a simple interval-based backoff
- once `max_retry` is exceeded, the delivery is marked `FAILED`
- a successful attempt marks the delivery `SUCCESS` and records `delivered_at`

This strategy is intentionally simple, auditable, and easy to explain in a portfolio project.

## Async Worker

The worker is lightweight and in-process:

- new deliveries are queued immediately after creation
- a periodic scan also picks up due retries
- if the in-memory queue is full or the process restarts, persisted deliveries can still be retried later because state lives in the database

## Admin Visibility

Admin endpoints:

- `GET /api/v1/admin/webhooks`
- `GET /api/v1/admin/webhooks/:id`

Supported filters:

- `event_type`
- `status`
- `transaction_ref`

## Failure Scenarios

The system intentionally tolerates webhook target instability:

- the core wallet transaction is not rolled back because a downstream callback endpoint is unavailable
- instead, the delivery state becomes visible and retryable
- operators can inspect failures through admin webhook endpoints and logs

## Local Testing

One practical way to test locally is to point `WEBHOOK_TARGET_URL` at a request inspector or local stub server.

Examples:

- `webhook.site`
- a small local HTTP server
- `nc -l` or a simple mock endpoint in another app

Suggested local flow:

1. start Postgres and Redis
2. set `WEBHOOK_ENABLED=true`
3. set `WEBHOOK_TARGET_URL` to your local capture endpoint
4. execute top up or transfer
5. inspect `webhook_deliveries` via admin endpoint and watch logs for retry behavior

## Limitations

- the worker is process-local, not horizontally coordinated
- the retry strategy is intentionally simple and not a full job scheduler
- repository-level integration tests against real Postgres still remain a useful next step
