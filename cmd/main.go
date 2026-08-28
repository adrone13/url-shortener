package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/adrone13/url-shortener/internal/config"
	"github.com/adrone13/url-shortener/internal/handler"
	"github.com/adrone13/url-shortener/internal/logging"
	"github.com/adrone13/url-shortener/internal/server"
	"github.com/adrone13/url-shortener/internal/shortener"
	"github.com/adrone13/url-shortener/internal/storage/memory"
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
	repo := memory.New()
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
