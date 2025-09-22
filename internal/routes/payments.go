package routes

import (
    "github.com/gofiber/fiber/v2"

    "github.com/congo-pay/congo_pay/internal/payments"
)

// RegisterPaymentRoutes wires payment endpoints.
func RegisterPaymentRoutes(r fiber.Router, h *payments.Handler) {
    r.Post("/payments/p2p", h.P2P)
}

