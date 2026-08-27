# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project purpose

A URL shortener built as a learning playground for Go, HTTP server best practices, and performance
optimization (see README.md). It is early-stage: config, structured logging, and the HTTP server
skeleton are wired up, but no routes/handlers, storage, or shortening logic exist yet.

## Commands

- `make build` — build binary to `bin/url-shortener`
- `make run` — `go run ./cmd`
- `make test` — `go test ./...` (run a single test: `go test ./internal/server -run TestName`)
- `make vet` — `go vet ./...`
- `make fmt` — `gofmt -w .`
- `make lint` — runs `vet` then `gofmt -l .`
- `make tidy` — `go mod tidy`

Configuration is via environment variables (see `internal/config/config.go`), loaded from a `.env`
file in the working directory if present (via `godotenv`, silently ignored if missing). Required
vars: `ENV`, `HTTP_PORT`, `LOG_LEVEL`.

## Architecture

Dependencies are wired by hand in `cmd/main.go` (no DI framework) — this is the intentional pattern
for this project's current size: `config.New()` → `logging.NewLogger()` → `server.New()`.

- `internal/config` — env-based config struct (`caarlos0/env`), parsed once via `config.New()`,
  returns `(*Config, error)`.
- `internal/logging` — `logging.NewLogger(logLevel string) *slog.Logger` builds a JSON `slog.Logger`
  to stdout with `AddSource: true`; invalid/empty level strings fall back to `info`.
- `internal/server` — `server.New(port int, logger *slog.Logger) *Server` builds a Chi router with
  `ClientIPFromRemoteAddr`, `RequestID`, `Recoverer`, and a `Timeout` middleware, plus an
  `http.Server` with explicit timeouts. `Start()` runs `ListenAndServe` (blocking); `Shutdown(ctx)`
  wraps graceful shutdown. `cmd/main.go` runs `Start()` in a goroutine and shuts down on
  `SIGINT`/`SIGTERM` via `signal.NotifyContext`.

### Notable deliberate choices (don't "fix" without reason)

- `server.New` uses `middleware.ClientIPFromRemoteAddr`, which assumes the server is directly
  exposed to the internet with no reverse proxy/load balancer in front of it. If a proxy is added
  before deploying, this must change to `ClientIPFromXFFTrustedProxies(n)` — see the comment at the
  call site in `internal/server/server.go`.
- HTTP server timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`) and the
  per-request `middleware.Timeout` are tuned for a URL shortener's small-request/small-response
  profile (no uploads, no streaming) — see the comments on the constants in
  `internal/server/server.go` before changing them.
- `config.New()` returns an error rather than panicking, so callers (currently just `main.go`) decide
  how to fail on missing/invalid env vars.

## Planned (not yet implemented, per README)

- SSE/streaming response support
- Full containerized environment
- Unit tests
- Swagger docs
- Prometheus metrics
- Local Grafana setup
