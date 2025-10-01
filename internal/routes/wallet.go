package routes

import (
    "github.com/gofiber/fiber/v2"

    "github.com/congo-pay/congo_pay/internal/wallet"
)

// RegisterWalletRoutes wires wallet-related endpoints.
func RegisterWalletRoutes(r fiber.Router, h *wallet.Handler) {
    // Wallets are auto-created on registration; expose GET to retrieve metadata
    r.Get("/wallets/:walletId", h.Get)
    r.Get("/wallets/:walletId/balance", h.Balance)
}
