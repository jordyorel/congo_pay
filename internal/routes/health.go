package routes

import (
    "context"
    "net/http"
    "time"

    "github.com/gofiber/fiber/v2"
)

// RegisterHealthRoutes adds liveness/readiness style endpoints.
func RegisterHealthRoutes(app *fiber.App, d Deps) {
    app.Get("/healthz", func(c *fiber.Ctx) error {
        dbStatus := "ok"
        redisStatus := "ok"

        ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
        defer cancel()
        if d.DB != nil {
            if err := d.DB.Ping(ctx); err != nil {
                dbStatus = err.Error()
            }
        }
        if d.Cache != nil {
            if err := d.Cache.Ping(ctx).Err(); err != nil {
                redisStatus = err.Error()
            }
        }
        status := http.StatusOK
        if dbStatus != "ok" || redisStatus != "ok" {
            status = http.StatusServiceUnavailable
        }
        return c.Status(status).JSON(fiber.Map{
            "status": fiber.Map{"postgres": dbStatus, "redis": redisStatus},
            "timestamp": time.Now().UTC().Format(time.RFC3339Nano),
        })
    })
}

