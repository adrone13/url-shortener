# Performance: benchmarking & profiling

Notes from establishing a baseline for `POST /api/shorten` / `GET /api/{code}` and
looking for real optimization opportunities. Everything here was measured on a
single dev machine (app, Postgres, and the `hey` load generator all running
locally) — treat the numbers as relative comparisons between runs, not
production capacity figures.

## Tooling

- **Load generation**: [`hey`](https://github.com/rakyll/hey) (`go install
  github.com/rakyll/hey@latest`), driven via `Makefile` targets.
- **Postgres pool visibility**: `cmd/main.go` logs `pgxpool.Pool.Stat()`
  every 5s (`max_cons`, `idle_cons`, `acquire_duration`, `acquire_count`,
  `empty_acquire_count`).
- **Go profiling**: `net/http/pprof`, served on `localhost:6060` (separate
  from the app's public port), wired in `cmd/main.go`.

## Commands

```bash
make db-up                          # start Postgres
make migrate-up                     # apply schema
make run                            # start the app

# fixed request count
make bench-write n=10000 c=100      # POST /api/shorten
make bench-read  n=10000 c=100 code=abc1234   # GET /api/{code}

# fixed duration — use this when you need a load window long enough
# to overlap a CPU profile capture (profile-cpu below runs for 30s)
make bench-write-duration z=30s c=100
make bench-read-duration  z=30s c=100 code=abc1234

# profiling (run one of these while a *-duration bench is hitting the app)
make profile-cpu                    # captures 30s CPU profile, saved under ~/pprof
make profile-allocs                 # captures a heap/alloc snapshot

# view a captured profile in the browser
make profile-serve path=~/pprof/pprof.cmd.samples.cpu.001.pb.gz
```

Results from `bench-write`/`bench-read` are written to `bench-runs/<write|read>-n<N>-c<C>-<timestamp>.txt`
(or `-z<duration>-` for the duration variants) — gitignored, local only.

## Findings

### 1. Connection pool size was the dominant bottleneck, not app code

`pgxpool` defaults `MaxConns` to `max(4, NumCPU())` — 12 on this machine. Sweeping
concurrency (`c=10` → `c=125`) at that default showed throughput flatten almost
immediately (~11.7k rps from `c=25` onward) while p99 latency kept climbing —
classic queuing-on-a-saturated-resource shape, confirmed directly via the pool
stat logs: `Σ acquire_duration ÷ concurrency` accounted for **65-80% of total
wall-clock request time** at `c=100`.

Raising `pool_max_conns` (set via `DATABASE_URL`, e.g.
`...?pool_max_conns=100`) to match concurrency removed almost all of that
queuing:

| pool size | rps (c=100) | p50 | p99 |
|---|---|---|---|
| 12 (default) | ~11,925 | ~8.3ms | ~11.2ms |
| 20 | 12,829 | 7.2ms | 26.5ms *(new tail — see below)* |
| 100 | 19,450 → 21,787 (cold → warm) | 4.3ms | 17.0 → 19.1ms |

### 2. A "cold" pool pays real one-time connection-dial cost

The first burst against a freshly-resized pool showed a small number of
extreme stragglers (up to ~92ms) that vanished on an immediate re-run against
the same (now warm) pool — each new pooled connection pays a real TCP +
Postgres-auth handshake once, and that cost shows up as tail latency for
whichever unlucky requests trigger it. Re-running warm dropped `Slowest`
91.8ms → 26.0ms with no code or config change.

### 3. `synchronous_commit=off` improved the *whole* distribution, not just the tail

Every `INSERT` does a WAL fsync under Postgres's default
`synchronous_commit=on`. Disabling it (diagnostic only — trades durability
for latency, see caveat below) shifted p50 down ~30% and raised rps ~39% at
`c=100` warm — because *every* write pays that fsync, not just the slow ones:

| | sync=on warm | sync=off warm |
|---|---|---|
| rps | 21,787 | 30,231 |
| p50 | 4.3ms | 3.0ms |
| p99 | 19.1ms | 10.3ms |

Toggle for testing via `docker-compose.yml`:
```yaml
command: postgres -c synchronous_commit=off
```

### 4. Go-level profiling: the service is I/O-bound, not CPU-bound

CPU profile (30s, `c=100`, sustained writes) — top of the flat-time list:

```
78.6%  syscall.rawsyscalln       (blocked on network/DB I/O, not "busy")
```

`generateCode`, `json` (de)serialization, and `url.Parse` — the app-level
functions most likely to be "optimizable" — never appeared in the profile at
all (below the noise floor). There is no hot loop or expensive computation in
this codebase to chase; the earlier pool/fsync work was the correct place to
have spent effort.

Allocation profile (10GB over 30s) breaks down as:
- Most of it is inherent `net/http`/middleware machinery — per-request
  context wrapping across 3 middlewares, header parsing/cloning, and
  `middleware.Timeout` building a fresh timer-backed context for every
  request (a deliberate safety tradeoff, not a bug).
- The one real, app-owned, *optional* target: `json.NewDecoder` per request
  (~10% of allocations) for decoding a one-field struct.
- `generateCode`'s `strings.Builder` usage: no measurable allocation.
  Already efficient.

**Takeaway**: at this request shape, there's no meaningful Go-code
optimization left to chase — the ceiling is set by network/DB I/O, which is
where 1-3 already addressed the real cost.

## Reading a pprof profile, briefly

- **flat** = time/bytes spent in that function's own code. **cum**
  (cumulative) = flat + everything it called. Sort by `-cum` to find which
  code path costs the most overall; sort by `-flat` (default) to find the
  actual leaf cost.
- A Go **CPU profile** samples whichever thread is running when the sampler
  fires — including one blocked inside a syscall — so a large
  `syscall.rawsyscalln` number usually means "waiting on I/O," not "burning
  CPU." An **alloc profile** measures bytes allocated, not time — a different
  axis entirely.
- `go tool pprof -list <func>` shows the profile attributed to your actual
  source lines — the most actionable view once you've found a function worth
  looking at. `-http=:PORT` (or `make profile-serve`) gives the flame-graph
  / graph view.
- A big number in the table isn't automatically a problem — most of it is
  unavoidable framework/OS cost every Go HTTP service pays. The judgment call
  is whether a line is "inherent cost of the job" or "something the code
  chose to do inefficiently."

## Caveats

- All numbers are from one dev machine with client, app, and DB co-located —
  useful for *relative* before/after comparisons, not absolute capacity
  planning.
- `synchronous_commit=off` is a diagnostic toggle only. Do not run it enabled
  outside local benchmarking — it trades crash-safe durability for latency.
