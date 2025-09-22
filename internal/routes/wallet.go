package routes

import (
    "github.com/gofiber/fiber/v2"

    "github.com/congo-pay/congo_pay/internal/wallet"
)

// RegisterWalletRoutes wires wallet-related endpoints.
func RegisterWalletRoutes(r fiber.Router, h *wallet.Handler) {
    r.Post("/wallets", h.Create)
    r.Get("/wallets/:walletId/balance", h.Balance)
}

