# Architecture & scaling cheatsheet

Quick reference for terms/patterns that came up while designing this project
for scale (system-design-interview style: "50M writes/day, 500M reads/day,
handle a hot key"). Not project-specific findings (see
[performance.md](performance.md) for those) — this is background reference.

## Singleflight — deduplicating concurrent identical work

Problem: a cache miss on a suddenly-hot key (e.g. a link just went viral)
means every concurrent request for that key independently queries the DB at
the same instant — a "cache stampede" / "thundering herd."

Fix: only let the *first* concurrent caller for a given key do the real
work; every other concurrent caller for that same key blocks and receives
the same result once it's ready. Different keys are unaffected and run
independently.

`golang.org/x/sync/singleflight` is the standard Go implementation:

```go
var g singleflight.Group

func (s *Shortener) Resolve(ctx context.Context, code string) (string, error) {
    v, err, _ := g.Do(code, func() (any, error) {
        return s.repo.Get(ctx, code)
    })
    return v.(string), err
}
```

`g.Do(key, fn)`: if `fn` is already running for `key`, the caller waits and
shares that result instead of starting a second `fn`. Doesn't prevent the
first miss from hitting the DB — prevents that one miss from becoming N
simultaneous DB hits.

## The request path: DNS → edge → load balancer → gateway → app

Terms that overlap a lot in casual use; here's what each layer actually does
and where it typically sits, client to server:

```
Client
  │
  ▼
DNS  (resolves your domain to an IP/anycast network)
  │
  ▼
CDN / Edge  (Cloudflare, Fastly, CloudFront)
  — geographically distributed; caches cacheable responses close to the
    user (e.g. a URL shortener's redirect response); also handles DDoS
    protection and TLS termination.
  │  (cache miss, or non-cacheable request, passes through)
  ▼
Load balancer  (AWS ALB/NLB, GCP LB, or self-run: Nginx, HAProxy, Envoy)
  — distributes incoming requests across multiple app instances; health
    checks instances and stops routing to dead ones. This is what makes
    horizontal scaling actually work in practice.
  │
  ▼
Reverse proxy / API gateway  (sometimes the same box as the LB, sometimes
  separate — Kong, Envoy, Nginx, or a managed cloud API Gateway)
  — auth, rate limiting, routing to the right backend service, TLS
    termination if the LB didn't already do it, request/response shaping.
  │
  ▼
App instances  (your service, N identical copies, behind the LB)
  │
  ▼
Cache (Redis) / DB (primary + read replica)
```

In practice several of these collapse into one piece of software at
small-to-medium scale — a single Nginx or Envoy instance commonly does LB +
reverse proxy + TLS termination together, and a CDN alone often covers edge
caching + DDoS + part of what a load balancer does. The diagram is "what
roles exist," not "what you must deploy five separate things for."

This project currently has none of these layers — the client hits the app's
port directly. That's fine for local dev/benchmarking; adding any of this
is the first step toward an actual deployed version, not something the
benchmarking work in `performance.md` needed.

## Postgres read replication: mechanism, tradeoffs, HA

**The core idea**: every change to the database is first written to the WAL
(write-ahead log) before being applied to the actual data files — originally
built for crash recovery, so a restart after a crash can replay the log and
reconstruct the exact prior state. Streaming replication reuses that same
mechanism instead of inventing a separate one: a replica takes one full copy
of the primary (`pg_basebackup`), then stays permanently in the same
recovery-replay loop Postgres normally runs once after a crash — just fed
continuously by a live WAL stream from the primary instead of a finite log
file that eventually runs out.

**Why a replica is read-only**: not a policy choice — a consequence of the
mechanism. Its data files are being actively overwritten by the replay
process in the background; a second writer modifying those same pages would
corrupt that process. Only the replayer is allowed to write to a standby's
data files.

**Tradeoffs beyond replication lag**:
- **Storage roughly doubles per replica** — each one holds a full physical
  copy of the primary's data. Worse: if a replication *slot* is used and a
  replica falls behind or disconnects, the primary retains WAL for it
  indefinitely — that can grow unbounded and fill the *primary's* disk, a
  real operational hazard, not just a replica-side cost.
- **No write scaling.** All writes still funnel through the one primary; a
  replica only ever helps the read side.
- **Multi-replica reads can go "back in time."** With more than one replica,
  each replays independently and can sit at a different point in the WAL
  stream — a client bounced between replica A then replica B can see a
  *newer* value followed by an *older* one on the very next request. Worse
  than plain staleness, since it isn't even internally consistent across
  requests.
- **Version lock-step.** Physical streaming replication requires the same
  major Postgres version on both sides — it can't bridge a version upgrade.
  (Logical replication can, at the cost of not replicating DDL/sequences
  automatically and needing per-table publications — a different tool for a
  different job, not a faster version of the same one.)
- **Recovery conflicts.** A long-running read query on a replica can get
  canceled ("canceling statement due to conflict with recovery") if
  incoming WAL removes data that query still needs — the replica has to
  choose between delaying replication or canceling the query, and defaults
  to canceling it.

**Replication ≠ HA.** A plain streaming replica is a warm, up-to-date copy —
nothing about it automatically promotes that replica if the primary dies.
**HA (High Availability)** is the broader goal of keeping a service running
through component failures, usually with orchestration on top of
replication (Patroni, repmgr, pg_auto_failover, or a cloud provider's
managed failover) to detect the failure, promote the standby, repoint
clients, and fence the old primary so it can't come back and cause a
split-brain. Replication is the mechanism HA is built on; it isn't HA by
itself.

See [performance.md, finding 7](performance.md) for how this project's
actual replica is set up and why it made sense for this app's traffic shape.

## Horizontal scaling vs. vertical scaling

- **Vertical scaling**: make one machine bigger (more CPU/RAM).
- **Horizontal scaling**: add more machines/processes to split the load.
  This is why a load balancer exists — it's the thing that spreads requests
  across, say, 10 copies of an app instead of running 1 large one.

## Stateless vs. stateful (why it matters for horizontal scaling)

**Stateless**: an app instance holds no data that only it has and that
another instance would need in order to answer the next request correctly.
If instance A handling request #1 and instance B handling request #2 (same
user, same key) both produce identical, correct results, the app is
stateless.

This is *why* horizontal scaling is safe: a load balancer can route any
request to any instance, replace/restart instances freely, and add more
under load, without breaking correctness — because no instance is holding
onto something the others need.

Concrete example from this project: the app is *almost* fully stateless
already — every instance talks to the same shared Postgres, so any instance
can serve any request correctly. The one thing that would break full
statelessness once there's more than one instance is the in-process LRU
cache added for the hot-key experiment (see performance.md, finding 6) —
each instance's LRU is local and private, different from every other
instance's. That's an acceptable kind of state to have per-instance
*because* it's only a cache: worst case, a "miss" on one instance just falls
through to Postgres — no correctness issue, unlike a database write, which
must never be instance-local.
