package routes

import (
    "github.com/gofiber/fiber/v2"

    "github.com/congo-pay/congo_pay/internal/funding"
)

// RegisterFundingRoutes wires card funding/withdrawal endpoints.
func RegisterFundingRoutes(r fiber.Router, h *funding.Handler) {
    r.Post("/wallets/:walletId/fund/card", h.CardIn)
    r.Post("/wallets/:walletId/withdraw/card", h.CardOut)
}

