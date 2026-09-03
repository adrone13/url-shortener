# url-shortener
This project is a learning playground to study Go, best practices for writing web servers and performance optimizations.

See [docs/performance.md](docs/performance.md) for benchmarking/profiling methodology, commands, and findings.

See [docs/architecture-cheatsheet.md](docs/architecture-cheatsheet.md) for a reference on scaling/architecture terms (singleflight, LB/gateway/edge stack, horizontal scaling & statelessness).

Swagger UI is served at `/swagger/index.html` when the app is running. Regenerate the spec after changing handler annotations with `make swagger` (requires `go install github.com/swaggo/swag/cmd/swag@latest`).

Prometheus metrics are served at `/metrics`: standard RED HTTP metrics (`http_requests_total`, `http_request_duration_seconds`) plus custom ones — `pg_pool_*` (primary/replica pool stats), `cache_lookups_total{tier}` (lru/redis/miss), `resolve_dedup_total` (singleflight dedup), `shorten_code_collisions_total`.

`make up` runs the whole stack (app included) in Docker — the app talks to `postgres`/`postgres-replica`/`redis` by service name. `make run` still runs the app on the host against the same compose-managed dependencies via `.env`'s `localhost` URLs, for faster local iteration. Either way, migrations still need to be applied once against the primary (`make migrate-up`).

Prometheus is at `localhost:9090`, scraping the app's `/metrics` every 5s. Grafana is at `localhost:3000` (anonymous access enabled for local dev, no login needed) with a "URL Shortener" dashboard auto-provisioned on startup — HTTP RED metrics, cache tier hit rate, DB pool health per role, collision/dedup rates, and Go runtime stats.

## ToDo:
* write a response to an HTML page via SSE or streaming
* unit-tests
