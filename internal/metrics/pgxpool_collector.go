package metrics

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// pgxPoolCollector exposes pgxpool.Pool.Stat() to Prometheus, computed at
// scrape time rather than polled on a timer — avoids staleness between
// scrapes and the extra background goroutine a ticker would need. Pool
// contention was the dominant bottleneck found during local benchmarking
// (docs/performance.md, finding 1); this turns that ad hoc log-based
// visibility into a real, scraped metric.
//
// One collector instance covers every pool: Prometheus identifies a metric
// descriptor by its name + label names (not label values or collector
// identity), so registering a separate Collector per pool with identically
// named/labeled Descs trips its duplicate-registration check.
type pgxPoolCollector struct {
	pools map[string]*pgxpool.Pool // role -> pool

	maxConns          *prometheus.Desc
	idleConns         *prometheus.Desc
	acquireCount      *prometheus.Desc
	emptyAcquireCount *prometheus.Desc
	acquireDuration   *prometheus.Desc
}

// NewPgxPoolCollector returns a Collector reporting every pool in pools,
// keyed by role (e.g. "primary", "replica"). Register it once with
// prometheus.MustRegister.
func NewPgxPoolCollector(pools map[string]*pgxpool.Pool) prometheus.Collector {
	labels := []string{"role"}
	return &pgxPoolCollector{
		pools: pools,
		maxConns: prometheus.NewDesc("pg_pool_max_conns", "Configured maximum pool connections.",
			labels, nil),
		idleConns: prometheus.NewDesc("pg_pool_idle_conns", "Currently idle pool connections.",
			labels, nil),
		acquireCount: prometheus.NewDesc("pg_pool_acquire_count_total", "Total connection acquires.",
			labels, nil),
		emptyAcquireCount: prometheus.NewDesc("pg_pool_empty_acquire_count_total",
			"Total acquires that had to wait for a connection.", labels, nil),
		acquireDuration: prometheus.NewDesc("pg_pool_acquire_duration_seconds_total",
			"Cumulative time spent waiting to acquire a connection.", labels, nil),
	}
}

func (c *pgxPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.maxConns
	ch <- c.idleConns
	ch <- c.acquireCount
	ch <- c.emptyAcquireCount
	ch <- c.acquireDuration
}

func (c *pgxPoolCollector) Collect(ch chan<- prometheus.Metric) {
	for role, pool := range c.pools {
		stats := pool.Stat()

		ch <- prometheus.MustNewConstMetric(c.maxConns, prometheus.GaugeValue, float64(stats.MaxConns()), role)
		ch <- prometheus.MustNewConstMetric(c.idleConns, prometheus.GaugeValue, float64(stats.IdleConns()), role)
		ch <- prometheus.MustNewConstMetric(c.acquireCount, prometheus.CounterValue, float64(stats.AcquireCount()), role)
		ch <- prometheus.MustNewConstMetric(c.emptyAcquireCount, prometheus.CounterValue, float64(stats.EmptyAcquireCount()), role)
		ch <- prometheus.MustNewConstMetric(c.acquireDuration, prometheus.CounterValue, stats.AcquireDuration().Seconds(), role)
	}
}
