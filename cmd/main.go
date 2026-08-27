package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/adrone13/url-shortener/internal/config"
	"github.com/adrone13/url-shortener/internal/logging"
	"github.com/adrone13/url-shortener/internal/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	logger := logging.NewLogger(cfg.LogLevel)
	srv := server.New(cfg.HttpPort, logger)

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
