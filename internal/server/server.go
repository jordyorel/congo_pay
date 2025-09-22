package server

import (
    "context"
    "log/slog"
    "time"

    "github.com/gofiber/fiber/v2"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"

    "github.com/congo-pay/congo_pay/internal/config"
    "github.com/congo-pay/congo_pay/internal/routes"
)

// Server wraps the Fiber application and shared dependencies.
type Server struct {
    app   *fiber.App
    cfg   config.Config
    db    *pgxpool.Pool
    cache *redis.Client
}

// New instantiates the HTTP server and delegates route wiring to routes.Setup.
func New(cfg config.Config, db *pgxpool.Pool, cache *redis.Client, logger *slog.Logger) (*Server, error) {
    app := fiber.New(fiber.Config{
        AppName:      cfg.AppName,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
    })

    if err := routes.Setup(app, routes.Deps{Cfg: cfg, DB: db, Cache: cache, Logger: logger}); err != nil {
        return nil, err
    }

    return &Server{app: app, cfg: cfg, db: db, cache: cache}, nil
}

// Listen starts the HTTP server.
func (s *Server) Listen() error {
    return s.app.Listen(s.cfg.Address())
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
    return s.app.ShutdownWithContext(ctx)
}
