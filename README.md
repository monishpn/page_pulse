# Page Pulse

A production-grade website audit service. Submit a public HTTP/HTTPS URL and get back a structured audit of the page — status code, title, meta description, H1, headers, timing — optimized for reliability and observability rather than SEO analysis or crawling.

**Live URL:** https://page-pulse-red-six.vercel.app

**Live backend:** https://page-pulse-bwc6.onrender.com

Built as the Digital Heroes Training Task.

---

## Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Project structure](#project-structure)
- [API reference](#api-reference)
- [Configuration](#configuration)
- [Running the backend locally](#running-the-backend-locally)
- [Testing](#testing)
- [Docker](#docker)
- [CI/CD](#cicd)
- [Frontend](#frontend)

---

## Overview

**In scope:** URL validation (SSRF-safe), single-page HTTP fetch, HTML parsing, Redis caching with configurable TTL, per-IP rate limiting, structured JSON API, Docker packaging, CI.

**Out of scope:** authentication, user accounts, recurring/scheduled audits, JavaScript rendering, Lighthouse-style scoring, screenshots, multi-page crawling.

## Architecture

Request flow for `POST /audit`:

```
Request
  → Rate limiter middleware (per-IP, Redis-backed fixed window)
  → Handler: bind JSON body
  → Validator: reject invalid / unsafe URLs
  → Service:
      → Redis lookup (cache hit → return immediately)
      → HTTP GET the target URL (bounded by REQUEST_TIMEOUT)
      → Parse HTML (title, meta description, h1)
      → Build AuditResponse
      → Write-through to Redis (best-effort; failures are logged, not fatal)
  → JSON response
```

Each layer only knows about the one below it through an interface, wired together in `main.go`:

- **`handler`** — binds the request body, delegates to `AuditService`. Doesn't know or care whether the service is real or fake.
- **`validator`** — pure functions, no I/O. Enforces URL shape and rejects localhost/private-network/link-local targets before anything touches the network.
- **`service`** — the audit pipeline itself, depends on a `CacheStore` interface rather than Redis directly.
- **`store/cache`** — the only package that talks to Redis, via GoFr's DI-managed `ctx.Redis`.
- **`middleware`** — the rate limiter, also Redis-backed, but resolved lazily via `app.OnStart` since GoFr's DI container isn't reachable until after middleware is wired up (see comments in `middleware/rate_limit.go` for why).

This means the limit holds correctly even if the service is later scaled to multiple instances, since the counter lives in Redis rather than in-process memory.

## Project structure

```
main.go                 Wires everything together, registers routes/middleware
handler/                HTTP-facing layer: binds requests, maps errors to responses
service/                Core audit pipeline (cache lookup → fetch → parse → cache write)
validator/               URL validation (FR-2): length, scheme, SSRF-safety
store/cache/             Redis-backed CacheStore implementation
middleware/              Per-IP rate limiter (Redis-backed, fixed window)
models/                  Shared types: AuditResponse, CustomError
configs/                 .local.env (gitignored) / .prod.env (template) — GoFr env config
web/                     Minimal static frontend (see below)
Dockerfile               Multi-stage build → alpine runtime image
.github/workflows/       CI: build + test on every push
```

## API reference

### `POST /audit`

**Request**

```json
{ "url": "https://example.com" }
```

**Success — `201 Created`**

```json
{
  "url": "https://example.com",
  "finalUrl": "https://example.com",
  "statusCode": 200,
  "responseTimeMs": 271,
  "pageTitle": "Example Domain",
  "metaDescription": "",
  "h1": "Example Domain",
  "contentType": "text/html",
  "contentLength": 559,
  "server": "cloudflare",
  "https": true,
  "auditedAt": "2026-07-25T11:02:06Z"
}
```

**Errors** — always `{"error":{"message":"..."}}`:

| Status | Cause | Example message |
|---|---|---|
| `400` | Missing/invalid/unsafe URL (validator) | `url is required`, `url scheme must be http or https`, `url must not target localhost or a private network` |
| `400` | Malformed JSON body, or the target itself can't be parsed as a request | `invalid request body` |
| `429` | Rate limit exceeded for the client IP | `rate limit exceeded, please retry after 1 minute` |
| `502` | Target host unreachable / connection failed | `failed to reach url: ...` |
| `504` | Fetching the target exceeded `REQUEST_TIMEOUT` | `request timed out: ...` |

All of the above were verified against a running instance while writing this doc.

## Configuration

Everything is env-var driven (`app.Config.GetOrDefault` in `main.go`), loaded via GoFr's config loader from `configs/.local.env` locally, or from real environment variables in any deployed environment (env vars always take precedence over file values).

| Variable | Default | Notes |
|---|---|---|
| `HTTP_PORT` | `8000` | Port the API listens on |
| `REDIS_HOST` | — | Required for caching and rate limiting |
| `REDIS_PORT` | — | |
| `REQUEST_TIMEOUT` | `30` | **Seconds.** Upper bound on fetching the target URL |
| `REQUEST_TTL` | `10` | **Minutes.** Cache TTL for a successful audit |
| `RATE_LIMIT_MAX` | `10` | Requests allowed per client IP per window |
| `RATE_LIMIT_WINDOW_SECONDS` | `60` | Rate limit window size, in seconds |
| `TRUST_PROXY_HEADERS` | `false` | Set `true` behind a reverse proxy (e.g. Render/Railway) so rate limiting keys off `X-Forwarded-For`/`X-Real-IP` instead of the proxy's own IP |

`REQUEST_TIMEOUT` is seconds; `REQUEST_TTL` is minutes — easy to mix up, worth double-checking if audits are caching for longer/shorter than expected.

## Running the backend locally

**Prerequisites:** Go 1.26+, a Redis instance.

```bash
# Redis, if you don't already have one running:
docker run -d --name redis -p 2001:6379 redis:7

# configs/.local.env already points at REDIS_PORT=2001 and HTTP_PORT=8000
go run .
```

The API is now at `http://localhost:8000`.

```bash
curl -X POST http://localhost:8000/audit \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}'
```

## Testing

```bash
go test ./...              # run everything
go test ./... -v           # with per-test output
go test ./... -cover       # with coverage percentages
```

Current coverage: `handler` 100%, `validator` 100%, `service` 93.9%, `store/cache` 90.5%.

- `validator` — table-driven tests covering every rejection rule (empty/too-long/malformed/wrong-scheme/no-host/localhost/private-IP/link-local/unspecified) plus valid cases.
- `store/cache` — built against `container.NewMockContainer` + gomock's `Redis.EXPECT()`, covering hit/miss/error/malformed-payload.
- `handler` — a hand-written fake `AuditService`, verifying the service is only invoked once bind + validation both pass.
- `service` — a fake `CacheStore` plus real `httptest.Server`s (no HTTP mocking library needed) for fetch success, cache hit/miss, connection-refused, and timeout paths.

## Docker

```bash
docker build -t page_pulse .
docker run -p 8000:8000 \
  -e REDIS_HOST=<host> \
  -e REDIS_PORT=<port> \
  -e REQUEST_TIMEOUT=10 \
  -e REQUEST_TTL=10 \
  -e RATE_LIMIT_MAX=10 \
  -e RATE_LIMIT_WINDOW_SECONDS=60 \
  -e TRUST_PROXY_HEADERS=true \
  page_pulse
```

Multi-stage build: `golang:1.26-alpine` compiles a static binary (`CGO_ENABLED=0`), then a minimal `alpine:3.22` runtime image runs it as a non-root user. `ca-certificates` is installed explicitly in the runtime stage — without it, every `https://` audit would fail TLS verification, since the app fetches arbitrary user-supplied HTTPS URLs.

No config files are baked into the image; it's configured entirely via environment variables at `docker run` time, matching GoFr's own precedence rules (real env vars always win over file-based config).

## CI/CD

`.github/workflows/workflow.yml` runs on every push: checks out the repo, sets up Go from `go.mod`'s version, then `go build ./...` followed by `go test ./... -v`.

## Frontend

`web/` is a minimal static page — no framework, no build step: a URL input, a submit button, and the audit result rendered below. Talks to the backend via `fetch`.

```bash
cd web
python3 server.py
```

Open `http://localhost:3000`. It points at the deployed backend (`https://page-pulse-bwc6.onrender.com`) by default; override with `BACKEND_URL` in `web/.local.env` to hit a local backend instead. `PORT` controls what port the frontend itself serves on (default `3000`).
