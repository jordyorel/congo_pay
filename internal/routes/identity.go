package routes

import (
    "github.com/gofiber/fiber/v2"

    "github.com/congo-pay/congo_pay/internal/identity"
)

// RegisterIdentityRoutes wires identity endpoints under the provided router group.
func RegisterIdentityRoutes(r fiber.Router, h *identity.Handler) {
    r.Post("/identity/register", h.Register)
    r.Post("/identity/authenticate", h.Authenticate)
}

