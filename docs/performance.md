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

Raising the pool size to match concurrency removed almost all of that
queuing. (At the time, this was set via a `pool_max_conns` `DATABASE_URL`
query param — later moved to its own `PG_POOL_MAX_CONNS` env var, set
explicitly via `pgxpool.ParseConfig` + `MaxConns` in `postgres.Connect`,
once that DSN param turned out to break `migrate` and any other tool
sharing the same URL. Same knob, same numbers below — just no longer baked
into the DSN.)

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

### 5. Read-path concurrency sweep: locating the knee

Reads (`GET /api/{code}`, warm pool at 100) are cheaper than writes as
expected — no WAL fsync, no `INSERT` — so they scale to a noticeably higher
rps before saturating:

| c | rps | p50 | p99 | slowest |
|---|---|---|---|---|
| 10 | 16,185 | 0.6ms | 1.1ms | 5.2ms |
| 25 | 20,310 | 1.1ms | 2.3ms | 22.8ms |
| 100 | 25,643 | 3.3ms | 16.2ms | 61.6ms |
| 125 | 27,653 | 4.1ms | 15.6ms | 36.4ms |
| 130 | 29,702 | 4.0ms | 13.5ms | 28.5ms |
| 140 | 31,613 | 3.9ms | 18.0ms | 21.8ms |
| 150 | 28,413 | 4.7ms | 28.6ms | 34.1ms |
| 160 | 31,456 | 4.5ms | 18.9ms | 23.4ms |
| 175 | 28,888 | 5.3ms | 26.9ms | 37.9ms |
| 200 | 27,643 | 6.3ms | 40.4ms | 55.6ms |
| 225 | 30,868 | 6.1ms | 32.0ms | 62.5ms |
| 250 | 31,124 | 6.6ms | 37.4ms | 64.0ms |
| 275 | 28,729 | 8.5ms | 39.4ms | 50.1ms |

The `c=200` re-run confirms the original 32,007 rps reading was noise: rerun
at the same concurrency landed at 27,643 rps with a visibly worse tail (p99
40.4ms vs. the original 23.7ms) — a genuine capacity increase doesn't
disappear on a repeat. With the gaps filled in (130/140/160), the whole
`c=125`-`275` range turns out to be a noisy plateau rather than a smooth
curve with one clean elbow: rps oscillates between ~27.6k-31.6k throughout,
with no further systematic growth past `c≈130-140` — that's roughly where
peak throughput is first reached (140: 31,613 rps), not 225.

Latency tells a clearer story than throughput does here. p50 sits in a tight
3.9-4.7ms band across `c=125-160`, then trends upward from `c=175` onward
(5.3ms → 6.3ms → 6.1ms → 6.6ms → 8.5ms) even though rps isn't climbing to
match. **That's the real signal: latency climbing while throughput stops
climbing (or drops) is the definition of past-the-knee.** Combining both
signals: the throughput ceiling is reached by `c≈130-140`, and it becomes
unambiguous you're past it (rising latency, flat-to-falling throughput) by
`c≈175-200`; `225-275` is squarely past it, with `275` the clearest case
(rps drops to 28,729 — below the `c=175` value — while p50 hits its worst
point, 8.5ms).

`hey`'s runs are noisy enough at fixed `c` (see the `c=200` before/after
above) that any single reading near a boundary should be treated as
approximate — 2-3 repeats per concurrency level would narrow this further
better than adding yet more distinct `c` values would.

### 6. In-process LRU cache roughly doubled read throughput

Added a cache-aside layer (`internal/cache/lru`, wrapping
`hashicorp/golang-lru`) in front of `Repository.Get` — `Shortener.Resolve`
checks the cache first and falls back to Postgres on a miss, populating the
cache on both a miss and on `Shorten`'s initial write. Re-ran the read sweep
at `c=140-200` (same concurrency points as finding 5) with the cache in
place:

| c | rps (no cache) | rps (LRU) | p50 (no cache) | p50 (LRU) | p99 (no cache) | p99 (LRU) |
|---|---|---|---|---|---|---|
| 140 | 31,613 | 60,193 | 3.9ms | 1.7ms | 18.0ms | 13.9ms |
| 150 | 28,413 | 63,383 | 4.7ms | 1.8ms | 28.6ms | 12.6ms |
| 160 | 31,456 | 63,243 | 4.5ms | 1.9ms | 18.9ms | 14.8ms |
| 175 | 28,888 | 62,561 | 5.3ms | 2.1ms | 26.9ms | 15.0ms |
| 200 | 27,643 | 63,370 | 6.3ms | 2.3ms | 40.4ms | 15.8ms |

Roughly 2-2.3x throughput and about half the p50 — expected, since a cache
hit replaces a Postgres round-trip with an in-process map lookup. More
telling than the averages: without the cache, rps was still a noisy plateau
across this range (finding 5); with it, rps is flat at ~60-63k regardless of
`c` — the bottleneck has moved off the app/DB entirely (likely the `hey`
client or local network stack), well past where the uncached knee was found.

Caveats on this specific test: `bench-read` always hits one fixed `code`, so
this is a 100%-hit-rate best case on a single hot key, and `lru.New(10)`
(capacity 10) means eviction was never exercised — this proves the mechanism
works, not how it holds up against a realistic spread of keys or actual
evictions. Also, four of the five runs completed slightly short of 10,000
responses (9,900-9,975, no reported errors) — consistent with the very fast
(~0.16s) runs being interrupted a moment early rather than a real failure;
only `c=200` completed cleanly, and it shows the same ~2x effect, so the
conclusion doesn't rest on the short runs alone.

**Production note**: a single in-process LRU only works cleanly for a
single instance. Once there's more than one app instance (the normal case in
production), each instance would keep an independent, colder cache with no
shared view — for a real multi-instance deployment, use either Redis alone,
or Redis as the shared cache with a small in-process LRU in front of it as a
local L1 (justified once Redis's own round-trip is shown to matter, not by
default). See the discussion this finding came out of for the reasoning
behind that split.

### 7. Postgres read replica: reasoning, mechanism, tradeoffs, HA

Added a streaming physical replica (`docker-compose.yml`: `postgres` as
`master`, `postgres-replica` as `slave`, both `bitnamilegacy/postgresql`)
and split `postgres.Repo` to send writes to a `primary` pool and reads
(`Get`, `List`) to a separate `replica` pool.

**Why**: this app's own traffic shape backs the case directly — reads
dominate writes (see finding 5 vs. the write findings above: reads sustain
noticeably higher rps at the same concurrency, and any real deployment of a
URL shortener is read-heavy by nature). A read replica offloads that
dominant load off the primary without touching write capacity, which
matches where this app's actual bottleneck sits. See
[architecture-cheatsheet.md](architecture-cheatsheet.md) for the general
mechanism (WAL, why a replica is read-only), the tradeoffs beyond lag, and
what HA actually requires on top of replication — not repeated here.

Confirmed locally: creating the `links` table on the primary (DDL, not just
data) appeared on the replica automatically with no migration run against
it — physical replication ships everything, schema included.

**Why this app is largely insulated from the staleness caveat**: `Shorten`
writes straight into both cache tiers (finding 6 / Redis addition) on
creation, so a freshly-created code is served from cache long before any
request would fall through to a (possibly lagging) replica — the classic
read-your-own-write problem doesn't surface here in practice.

**Practical note from setting this up**: `bitnami/postgresql` no longer
publishes pinned version tags on Docker Hub's free tier (as of mid-2025,
only a floating `latest`) — versioned tags now live under
`bitnamilegacy/postgresql`, which is what `docker-compose.yml` actually
uses, for the same reason every other dependency in this repo is pinned.

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
