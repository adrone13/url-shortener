package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/adrone13/url-shortener/internal/config"
	"github.com/adrone13/url-shortener/internal/handler"
	"github.com/adrone13/url-shortener/internal/logging"
	"github.com/adrone13/url-shortener/internal/server"
	"github.com/adrone13/url-shortener/internal/shortener"
	"github.com/adrone13/url-shortener/internal/storage/postgres"
)

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

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := postgres.New(pool)
	svc := shortener.New(repo)
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
