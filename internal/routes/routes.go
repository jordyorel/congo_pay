package routes

import (
    "context"
    "fmt"
    "log/slog"
    "net/http"
    "strings"
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

// Deps aggregates shared dependencies required to wire routes.
type Deps struct {
    Cfg    config.Config
    DB     *pgxpool.Pool
    Cache  *redis.Client
    Logger *slog.Logger
}

// Setup configures middlewares and all application routes.
func Setup(app *fiber.App, d Deps) error {
    // Enforce DB/Redis presence outside of dev, even though main also checks.
    if !isDev(d.Cfg.Env) {
        if d.DB == nil {
            return fmt.Errorf("database is required when APP_ENV=%s", d.Cfg.Env)
        }
        if d.Cache == nil {
            return fmt.Errorf("redis is required when APP_ENV=%s", d.Cfg.Env)
        }
    }
    // Middlewares
    app.Use(recover.New())
    app.Use(middleware.RequestID())
    app.Use(middleware.Audit(d.Logger))
    if d.Cache != nil {
        app.Use(middleware.Idempotency(d.Cache, d.Cfg.IdempotencyTTL, d.Logger))
    }

    // Health
    RegisterHealthRoutes(app, d)

    // Services and handlers
    var ledgerBackend ledger.Ledger
    if d.DB != nil {
        ledgerBackend = ledger.NewPostgresLedger(d.DB)
    } else {
        ledgerBackend = ledger.NewInMemory()
        _ = ledgerBackend.EnsureAccount(context.Background(), ledger.CardSuspenseAccountCode)
    }

    var walletRepo wallet.Repository
    if d.DB != nil {
        walletRepo = wallet.NewPostgresRepository(d.DB)
    } else {
        walletRepo = wallet.NewMemoryRepository()
    }
    walletSvc := wallet.NewService(walletRepo, ledgerBackend)
    notifier := notification.NewLoggerNotifier(d.Logger)
    paymentSvc := payments.NewService(ledgerBackend, walletSvc, notifier)
    var identityRepo identity.Repository
    if d.DB != nil {
        identityRepo = identity.NewPostgresRepository(d.DB)
    } else {
        identityRepo = identity.NewMemoryRepository()
    }
    identitySvc := identity.NewService(identityRepo)
    fundingSvc, err := funding.NewService(context.Background(), ledgerBackend, walletSvc, nil)
    if err != nil {
        return err
    }

    fundingHandler := funding.NewHandler(fundingSvc)
    paymentHandler := payments.NewHandler(paymentSvc)
    walletHandler := wallet.NewHandler(walletSvc)
    identityHandler := identity.NewHandler(identitySvc)

    // API routes
    api := app.Group("/api/v1")
    api.Get("/ping", func(c *fiber.Ctx) error {
        reqID, _ := c.Locals("X-Request-ID").(string)
        return c.Status(http.StatusOK).JSON(fiber.Map{
            "status": "ok",
            "request_id": reqID,
            "timestamp": time.Now().UTC().Format(time.RFC3339Nano),
        })
    })

    // Delegate to sub-route modules for clarity
    RegisterIdentityRoutes(api, identityHandler)
    RegisterWalletRoutes(api, walletHandler)
    RegisterFundingRoutes(api, fundingHandler)
    RegisterPaymentRoutes(api, paymentHandler)

    return nil
}

func isDev(env string) bool {
    switch strings.ToLower(env) {
    case "dev", "development", "local":
        return true
    default:
        return false
    }
}
