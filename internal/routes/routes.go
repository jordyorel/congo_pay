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
    "github.com/gofiber/fiber/v2/middleware/logger"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"

    "github.com/congo-pay/congo_pay/internal/config"
    "github.com/congo-pay/congo_pay/internal/auth"
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
    // Plain text access log in desired format: [HH:MM:SS] 200 -  145ms METHOD /path
    app.Use(logger.New(logger.Config{
        Format:     "[${time}] ${status} -  ${latency} ${method} ${path}\n",
        TimeFormat: "15:04:05",
        TimeZone:   "Local",
    }))
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
    authSvc := auth.NewService(d.Cfg, identityRepo)
    authHandler := auth.NewHandler(identitySvc, authSvc, walletSvc)
    fundingSvc, err := funding.NewService(context.Background(), ledgerBackend, walletSvc, nil)
    if err != nil {
        return err
    }

    fundingHandler := funding.NewHandler(fundingSvc)
    paymentHandler := payments.NewHandler(paymentSvc)
    walletHandler := wallet.NewHandler(walletSvc)
    // identityHandler not needed; using service directly for register/auth

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

    // Public routes
    RegisterIdentityRoutes(api, identitySvc, walletSvc, d.Logger)
    rateLimiter := middleware.LoginRateLimit(d.Cache, 5)
    RegisterAuthRoutes(api, authHandler, rateLimiter)

    // Protected routes
    jwtmw := middleware.JWTAuth(d.Cfg, identityRepo)
    protected := api.Group("", jwtmw)
    RegisterWalletMeRoute(protected, walletSvc, identityRepo)
    // Profile endpoint
    protected.Get("/me", func(c *fiber.Ctx) error {
        uid, _ := c.Locals("user_id").(string)
        if uid == "" { return c.SendStatus(http.StatusUnauthorized) }
        user, err := identityRepo.FindByID(c.UserContext(), uid)
        if err != nil { return fiber.NewError(http.StatusUnauthorized, "user not found") }
        return c.JSON(fiber.Map{
            "user_id": user.ID,
            "phone": user.Phone,
            "tier": user.Tier,
            "device_id": user.DeviceID,
            "token_version": user.TokenVersion,
            "created_at": user.CreatedAt,
            "last_login": user.LastLogin,
        })
    })
    RegisterWalletRoutes(protected, walletHandler)
    RegisterFundingRoutes(protected, fundingHandler)
    RegisterPaymentRoutes(protected, paymentHandler)

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
