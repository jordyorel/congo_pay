package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/congo-pay/congo_pay/internal/config"
	"github.com/congo-pay/congo_pay/internal/infra"
	"github.com/congo-pay/congo_pay/internal/logging"
	"github.com/congo-pay/congo_pay/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.LogLevel)

	ctx := context.Background()

	db, err := infra.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	cache, err := infra.NewRedisClient(ctx, cfg.RedisURL)
	if err != nil {
		logger.Error("connect redis", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			logger.Warn("close redis", "error", err)
		}
	}()

	srv, err := server.New(cfg, db, cache, logger)
	if err != nil {
		logger.Error("build server", "error", err)
		os.Exit(1)
	}

	srvErrCh := make(chan error, 1)
	go func() {
		srvErrCh <- srv.Listen()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("shutdown signal received", "signal", sig.String())
	case err := <-srvErrCh:
		if err != nil {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownPeriod)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	logger.Info("server exited cleanly")
}
