package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"

    "github.com/congo-pay/congo_pay/internal/config"
    "github.com/congo-pay/congo_pay/internal/infra"
    "github.com/congo-pay/congo_pay/internal/logging"
    "github.com/congo-pay/congo_pay/internal/server"
)

func main() {
    cfg := config.Load()
    logger := logging.New(cfg.LogLevel)

    // Enforce required dependencies outside of development
    if !isDevEnv(cfg.Env) {
        if cfg.DatabaseURL == "" {
            log.Fatal("DATABASE_URL is required when APP_ENV is not development")
        }
        if cfg.RedisURL == "" {
            log.Fatal("REDIS_URL is required when APP_ENV is not development")
        }
    }

    // Initialize DB and Redis if URLs provided
    var (
        db    *pgxpool.Pool
        cache *redis.Client
        err   error
    )

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if cfg.DatabaseURL != "" {
        db, err = infra.NewPostgresPool(ctx, cfg.DatabaseURL)
        if err != nil {
            log.Fatalf("postgres init failed: %v", err)
        }
        defer db.Close()
    }
    if cfg.RedisURL != "" {
        cache, err = infra.NewRedisClient(ctx, cfg.RedisURL)
        if err != nil {
            log.Fatalf("redis init failed: %v", err)
        }
        defer cache.Close()
    }

    srv, err := server.New(cfg, db, cache, logger)
    if err != nil {
        log.Fatalf("server init failed: %v", err)
    }

    // Start server in goroutine
    go func() {
        log.Printf("Starting %s on %s (env=%s)", cfg.AppName, cfg.Address(), cfg.Env)
        if err := srv.Listen(); err != nil {
            log.Printf("server stopped: %v", err)
        }
    }()

    // Graceful shutdown
    sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()
    <-sigCtx.Done()
    shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel2()
    if err := srv.Shutdown(shutdownCtx); err != nil {
        log.Printf("graceful shutdown error: %v", err)
    }
    _ = os.Stdout.Sync()
}

func isDevEnv(env string) bool {
    switch env {
    case "development", "dev", "local":
        return true
    default:
        return false
    }
}
