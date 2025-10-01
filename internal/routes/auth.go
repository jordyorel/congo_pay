package routes

import (
    "github.com/gofiber/fiber/v2"

    "github.com/congo-pay/congo_pay/internal/auth"
)

// RegisterAuthRoutes wires authentication endpoints.
func RegisterAuthRoutes(r fiber.Router, h *auth.Handler, rateLimiter fiber.Handler) {
    group := r.Group("/auth")
    if rateLimiter != nil {
        group.Post("/login", rateLimiter, h.Login)
    } else {
        group.Post("/login", h.Login)
    }
    group.Post("/refresh", h.Refresh)
    group.Post("/logout", h.Logout)
}
