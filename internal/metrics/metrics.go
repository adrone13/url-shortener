// Package metrics holds this app's custom Prometheus metrics: signals the
// generic RED (rate/errors/duration) HTTP metrics can't see on their own —
// internal cache/pool/retry behavior validated during local benchmarking
// (see docs/performance.md). Labels stay low-cardinality by design (tier,
// role) — never label by a value like a short code.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// CacheLookups counts cache lookups by which tier answered them:
// "lru", "redis", or "miss" (fell through to the DB).
var CacheLookups = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "cache_lookups_total",
	Help: "Cache lookups by tier that answered them (lru, redis, miss).",
}, []string{"tier"})

// ShortenCodeCollisions counts how often Shorten had to regenerate a code
// after a unique-constraint collision on the primary.
var ShortenCodeCollisions = promauto.NewCounter(prometheus.CounterOpts{
	Name: "shorten_code_collisions_total",
	Help: "Code collisions encountered while generating a short code.",
})

// ResolveDedup counts singleflight calls whose result was shared with at
// least one other concurrent caller for the same code — i.e. cache-stampede
// protection actually kicking in. Counts every caller in a shared group
// (including the one that did the real work), not just the ones spared a
// DB call, so it's a "dedup happened" signal, not an exact avoided-call count.
var ResolveDedup = promauto.NewCounter(prometheus.CounterOpts{
	Name: "resolve_dedup_total",
	Help: "Resolve calls whose result was shared via singleflight.",
})
