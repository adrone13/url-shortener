# url-shortener
This project is a learning playground to study Go, best practices for writing web servers and performance optimizations.

See [docs/performance.md](docs/performance.md) for benchmarking/profiling methodology, commands, and findings.

See [docs/architecture-cheatsheet.md](docs/architecture-cheatsheet.md) for a reference on scaling/architecture terms (singleflight, LB/gateway/edge stack, horizontal scaling & statelessness).

Swagger UI is served at `/swagger/index.html` when the app is running. Regenerate the spec after changing handler annotations with `make swagger` (requires `go install github.com/swaggo/swag/cmd/swag@latest`).

Prometheus metrics are served at `/metrics`: standard RED HTTP metrics (`http_requests_total`, `http_request_duration_seconds`) plus custom ones — `pg_pool_*` (primary/replica pool stats), `cache_lookups_total{tier}` (lru/redis/miss), `resolve_dedup_total` (singleflight dedup), `shorten_code_collisions_total`.

## ToDo:
* write a response to an HTML page via SSE or streaming
* fully containerized environment
* local Grafana setup
* unit-tests
