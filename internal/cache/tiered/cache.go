package tiered

import (
	"context"
	"errors"

	"github.com/adrone13/url-shortener/internal/metrics"
)

type cache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string) error
}

// Cache composes a fast, per-instance L1 in front of a shared L2: Get checks
// l1 first, falls back to l2 and backfills l1 on an l2 hit; Set writes
// through to both so a fresh entry is warm everywhere without needing a
// read to populate it.
type Cache struct {
	l1 cache
	l2 cache
}

func New(l1, l2 cache) *Cache {
	return &Cache{l1: l1, l2: l2}
}

func (c *Cache) Get(ctx context.Context, key string) (string, bool, error) {
	if val, ok, err := c.l1.Get(ctx, key); err == nil && ok {
		metrics.CacheLookups.WithLabelValues("lru").Inc()
		return val, true, nil
	}

	val, ok, err := c.l2.Get(ctx, key)
	if err != nil || !ok {
		metrics.CacheLookups.WithLabelValues("miss").Inc()
		return "", false, err
	}

	metrics.CacheLookups.WithLabelValues("redis").Inc()

	_ = c.l1.Set(ctx, key, val)

	return val, true, nil
}

func (c *Cache) Set(ctx context.Context, key, value string) error {
	return errors.Join(c.l1.Set(ctx, key, value), c.l2.Set(ctx, key, value))
}
