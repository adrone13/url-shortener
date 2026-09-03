package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/adrone13/url-shortener/internal/cache/lru"
	"github.com/adrone13/url-shortener/internal/cache/redis"
	"github.com/adrone13/url-shortener/internal/cache/tiered"
	"github.com/adrone13/url-shortener/internal/config"
	"github.com/adrone13/url-shortener/internal/handler"
	"github.com/adrone13/url-shortener/internal/logging"
	"github.com/adrone13/url-shortener/internal/metrics"
	"github.com/adrone13/url-shortener/internal/server"
	"github.com/adrone13/url-shortener/internal/shortener"
	"github.com/adrone13/url-shortener/internal/storage/postgres"
)

func logPgPoolStatsOnce(pool *pgxpool.Pool, role string, logger *slog.Logger) {
	poolStats := pool.Stat()
	logger.Info("PG pool stats",
		slog.String("role", role),
		slog.Any("max_cons", poolStats.MaxConns()),
		slog.Any("idle_cons", poolStats.IdleConns()),
		slog.Duration("acquire_duration", poolStats.AcquireDuration()),
		slog.Int64("acquire_count", poolStats.AcquireCount()),
		slog.Int64("empty_acquire_count", poolStats.EmptyAcquireCount()),
	)
}

func logPgPoolStats(ctx context.Context, pool *pgxpool.Pool, role string, logger *slog.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			logPgPoolStatsOnce(pool, role, logger)
		case <-ctx.Done():
			logger.Info("pg pool stats stopped")
			return
		}
	}
}

// @title           URL Shortener API
// @version         1.0
// @description     Shortens URLs and resolves short codes back to their original URL.
// @BasePath        /api
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	logger := logging.NewLogger(cfg.LogLevel, cfg.Env == "local")

	logger.Info("starting app", slog.Int("cpus", runtime.NumCPU()))

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL, logger, "primary")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()

	replicaPool, err := postgres.Connect(ctx, cfg.DatabaseReplicaURL, logger, "replica")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer replicaPool.Close()

	logPgPoolStatsOnce(pool, "primary", logger)
	logPgPoolStatsOnce(replicaPool, "replica", logger)
	// go logPgPoolStats(ctx, pool, "primary", logger)

	prometheus.MustRegister(metrics.NewPgxPoolCollector(map[string]*pgxpool.Pool{
		"primary": pool,
		"replica": replicaPool,
	}))

	go func() {
		logger.Info("starting pprof server", slog.String("addr", ":6060"))
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			logger.Error("pprof server failed", "error", err)
		}
	}()

	lruCache, err := lru.New(10)
	if err != nil {
		logger.Error("failed to create cache", "error", err)
		os.Exit(1)
	}

	redisCache, err := redis.Connect(ctx, cfg.RedisAddr, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cache := tiered.New(lruCache, redisCache)

	repo := postgres.New(pool, replicaPool)
	svc := shortener.New(repo, cache, logger)
	h := handler.NewShortenerHandler(logger, svc)
	routes := handler.Routes(h)

	srv := server.New(cfg.HttpPort, logger, routes)

	go func() {
		if err := srv.Start(); err != nil {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	logger.Info("shutting down")

	if err := srv.Shutdown(context.Background()); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
