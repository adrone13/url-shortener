# url-shortener
This project is a learning playground to study Go, best practices for writing web servers and performance optimizations.

See [docs/performance.md](docs/performance.md) for benchmarking/profiling methodology, commands, and findings.

See [docs/architecture-cheatsheet.md](docs/architecture-cheatsheet.md) for a reference on scaling/architecture terms (singleflight, LB/gateway/edge stack, horizontal scaling & statelessness).

Swagger UI is served at `/swagger/index.html` when the app is running. Regenerate the spec after changing handler annotations with `make swagger` (requires `go install github.com/swaggo/swag/cmd/swag@latest`).

## ToDo:
* write a response to an HTML page via SSE or streaming
* fully containerized environment
* Prometheus metrics setup
* local Grafana setup
* unit-tests
