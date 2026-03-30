# Observability

## Overview

`go-hermes` now exposes a practical observability baseline aimed at local operability and portfolio realism:

- structured request logs
- request ID and trace ID correlation
- Prometheus-compatible metrics
- webhook lifecycle metrics
- rate-limit rejection metrics
- health checks that verify both PostgreSQL and Redis

## Request Correlation

Every request passes through:

- `X-Request-ID` generation
- `traceparent` parsing when upstream tracing headers are present

This means logs can include:

- `request_id`
- `trace_id`
- `user_id` for authenticated requests

That keeps debugging sane when looking at failed auth, money movement, or admin reads.

## Metrics Endpoint

The API exposes:

- `GET /metrics`

The endpoint is Prometheus-friendly and can be scraped locally or in CI smoke checks.

Protection defaults:

- in development, `/metrics` is open unless `METRICS_TOKEN` is set
- outside development, startup fails if metrics stay enabled without `METRICS_TOKEN`
- when `METRICS_TOKEN` is set, send it as `Authorization: Bearer <token>` or `X-Metrics-Token: <token>`

It can be disabled with:

```env
METRICS_ENABLED=false
```

## Exported Metrics

Current metrics include:

- `go_hermes_http_requests_total`
- `go_hermes_http_request_duration_seconds`
- `go_hermes_rate_limit_exceeded_total`
- `go_hermes_webhook_delivery_events_total`

These cover the most operationally important surfaces in the project:

- request volume and latency
- rate-limit pressure
- webhook creation, retry scheduling, and success or failure

## Local Usage

Start the app normally, then open:

```text
http://localhost:8080/metrics
```

Useful checks:

- verify login traffic increments HTTP counters
- trigger repeated login failures and watch rate-limit counters
- run a top up or transfer with webhooks enabled and watch webhook lifecycle counters move

If you want to keep development closer to production, set `METRICS_TOKEN` locally and scrape with that token instead of relying on the open development default.

## Scope And Tradeoffs

This is intentionally not a full OpenTelemetry rollout with exporters, spans, and collectors.

The design goal is:

- strong baseline visibility
- low local setup cost
- clear extension point for future OpenTelemetry tracing or external metrics backends

If the project evolves further, the next step would be span-based tracing around transaction execution, idempotency checks, and webhook delivery attempts.
