# Rate Limiting

## Why It Exists

`go-hermes` applies rate limiting to sensitive endpoints to reduce:

- brute-force login attempts
- repeated money-movement calls caused by misbehaving clients
- accidental retry storms during network instability

The goal is not to replace authentication or idempotency. The goal is to reduce noisy or abusive request volume before it becomes an operational problem.

## Sensitive Endpoints

Rate limiting is applied to:

- `POST /api/v1/auth/login`
- `POST /api/v1/wallets/me/top-up`
- `POST /api/v1/transfers`

## Strategy

- login is effectively limited per caller identity, which falls back to client IP for unauthenticated traffic
- top up and transfer are limited per authenticated user
- counters are stored in Redis
- the time window is configurable
- limits are configurable per endpoint category

## Configuration

Relevant environment variables:

- `REDIS_HOST`
- `REDIS_PORT`
- `REDIS_PASSWORD`
- `REDIS_DB`
- `RATE_LIMIT_WINDOW_SECONDS`
- `RATE_LIMIT_LOGIN`
- `RATE_LIMIT_TOPUP`
- `RATE_LIMIT_TRANSFER`

Example:

```env
RATE_LIMIT_WINDOW_SECONDS=60
RATE_LIMIT_LOGIN=10
RATE_LIMIT_TOPUP=20
RATE_LIMIT_TRANSFER=20
```

## Response Behavior

When the limit is exceeded, the API returns:

- HTTP `429 Too Many Requests`
- error code `RATE_LIMIT_EXCEEDED`

The response also includes useful headers:

- `X-RateLimit-Limit`
- `X-RateLimit-Remaining`
- `X-RateLimit-Reset`

If Redis is unavailable or the rate-limit backend errors, the middleware currently fails open:

- the request is allowed to continue
- a warning is logged with `failure_policy=fail_open`

This is an intentional availability-first choice so login and money-movement requests are not blocked by limiter backend outages. The tradeoff is reduced abuse protection during that failure window.

## Tradeoffs

- the implementation is intentionally simple and readable
- it uses Redis `INCR` plus `EXPIRE`, which is a practical portfolio-grade approach for many API workloads
- it does not implement distributed weighted quotas or advanced abuse scoring

## Local Development

Redis is included in `docker-compose.yml`.

For local app execution with dependencies in Docker:

```bash
docker compose up -d postgres redis
```

For full stack in Docker:

```bash
docker compose up --build
```
