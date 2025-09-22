package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/congo-pay/congo_pay/internal/config"
	"github.com/congo-pay/congo_pay/internal/funding"
	"github.com/congo-pay/congo_pay/internal/identity"
	"github.com/congo-pay/congo_pay/internal/ledger"
	"github.com/congo-pay/congo_pay/internal/middleware"
	"github.com/congo-pay/congo_pay/internal/notification"
	"github.com/congo-pay/congo_pay/internal/payments"
	"github.com/congo-pay/congo_pay/internal/wallet"
)

// Server wraps the Fiber application and shared dependencies.
type Server struct {
	app             *fiber.App
	cfg             config.Config
	db              *pgxpool.Pool
	cache           *redis.Client
	ledger          ledger.Ledger
	fundingHandler  *funding.Handler
	paymentHandler  *payments.Handler
	walletHandler   *wallet.Handler
	identityHandler *identity.Handler
}

// New instantiates the HTTP server with middlewares and baseline routes.
func New(cfg config.Config, db *pgxpool.Pool, cache *redis.Client, logger *slog.Logger) (*Server, error) {
	app := fiber.New(fiber.Config{
		AppName:      cfg.AppName,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	})

	app.Use(recover.New())
	app.Use(middleware.RequestID())
	app.Use(middleware.Audit(logger))
	app.Use(middleware.Idempotency(cache, cfg.IdempotencyTTL, logger))

	ledgerBackend := ledger.NewPostgresLedger(db)
	walletRepo := wallet.NewPostgresRepository(db)
	walletSvc := wallet.NewService(walletRepo, ledgerBackend)
	notifier := notification.NewLoggerNotifier(logger)
	paymentSvc := payments.NewService(ledgerBackend, walletSvc, notifier)
	identityRepo := identity.NewPostgresRepository(db)
	identitySvc := identity.NewService(identityRepo)
	fundingSvc, err := funding.NewService(context.Background(), ledgerBackend, walletSvc, nil)
	if err != nil {
		return nil, err
	}

	s := &Server{
		app:             app,
		cfg:             cfg,
		db:              db,
		cache:           cache,
		ledger:          ledgerBackend,
		fundingHandler:  funding.NewHandler(fundingSvc),
		paymentHandler:  payments.NewHandler(paymentSvc),
		walletHandler:   wallet.NewHandler(walletSvc),
		identityHandler: identity.NewHandler(identitySvc),
	}
	s.registerRoutes()
	return s, nil
}

func (s *Server) registerRoutes() {
	s.app.Get("/healthz", s.healthCheck)
	api := s.app.Group("/api/v1")
	api.Get("/ping", s.ping)
	api.Post("/identity/register", s.identityHandler.Register)
	api.Post("/identity/authenticate", s.identityHandler.Authenticate)
	api.Post("/wallets", s.walletHandler.Create)
	api.Get("/wallets/:walletId/balance", s.walletHandler.Balance)
	api.Post("/wallets/:walletId/fund/card", s.fundingHandler.CardIn)
	api.Post("/wallets/:walletId/withdraw/card", s.fundingHandler.CardOut)
	api.Post("/payments/p2p", s.paymentHandler.P2P)
}

// Listen starts the HTTP server.
func (s *Server) Listen() error {
	return s.app.Listen(s.cfg.Address())
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.app.ShutdownWithContext(ctx)
}

func (s *Server) healthCheck(c *fiber.Ctx) error {
	dbStatus := "ok"
	redisStatus := "ok"

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.db.Ping(ctx); err != nil {
		dbStatus = err.Error()
	}

	if err := s.cache.Ping(ctx).Err(); err != nil {
		redisStatus = err.Error()
	}

	status := http.StatusOK
	if dbStatus != "ok" || redisStatus != "ok" {
		status = http.StatusServiceUnavailable
	}

	return c.Status(status).JSON(fiber.Map{
		"status": fiber.Map{
			"postgres": dbStatus,
			"redis":    redisStatus,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (s *Server) ping(c *fiber.Ctx) error {
	requestID, _ := c.Locals("X-Request-ID").(string)
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"status":     "ok",
		"request_id": requestID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
	})
}
