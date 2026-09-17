# url-shortener

A URL shortener written in Go, built as a hands-on study of HTTP service
design and performance: connection pooling, caching layers,
read replicas, request deduplication, and the profiling/benchmarking work
behind each of those decisions. Built iteratively with AI pair-programming
(Claude).

## Stack

Go (chi) · Postgres (primary + read replica) · Redis · in-process LRU ·
Prometheus + Grafana · Swagger

## Quick start

```
make up          # full stack in Docker: app, Postgres primary+replica, Redis, Prometheus, Grafana
make migrate-up   # apply schema (once, against the primary)
```

`make run` runs the app on the host instead, against the same
compose-managed dependencies, for faster local iteration.

- App: `localhost:8080` — `POST /api/shorten`, `GET /api/{code}`, `GET /api/links`
- API docs: `localhost:8080/swagger/index.html`
- Metrics: `localhost:8080/metrics`
- Grafana: `localhost:3000` (dashboard auto-provisioned) · Prometheus: `localhost:9090`

## Further reading

- [docs/performance.md](docs/performance.md) — benchmarking and profiling methodology and findings (connection pooling, caching, replication, CPU/alloc profiles)
- [docs/architecture-cheatsheet.md](docs/architecture-cheatsheet.md) — reference notes on the scaling concepts applied here (singleflight, LB/gateway/edge stack, replication, horizontal scaling)

## ToDo

- unit tests
- SSE/streaming response support
