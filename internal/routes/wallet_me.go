package routes

import (
    "net/http"

    "github.com/gofiber/fiber/v2"

    "github.com/congo-pay/congo_pay/internal/identity"
    "github.com/congo-pay/congo_pay/internal/wallet"
)

// RegisterWalletMeRoute exposes a GET endpoint to view the current user's wallet and profile.
func RegisterWalletMeRoute(r fiber.Router, wallets *wallet.Service, idRepo identity.Repository) {
    r.Get("/wallet", func(c *fiber.Ctx) error {
        uid, _ := c.Locals("user_id").(string)
        if uid == "" {
            return fiber.NewError(http.StatusUnauthorized, "unauthorized")
        }
        user, err := idRepo.FindByID(c.UserContext(), uid)
        if err != nil {
            return fiber.NewError(http.StatusNotFound, "user not found")
        }
        w, err := wallets.GetByOwner(c.UserContext(), uid)
        if err != nil {
            return fiber.NewError(http.StatusNotFound, "wallet not found")
        }
        bal, err := wallets.Balance(c.UserContext(), w.ID)
        if err != nil {
            return fiber.NewError(http.StatusInternalServerError, err.Error())
        }
        return c.Status(http.StatusOK).JSON(fiber.Map{
            "user": fiber.Map{
                "id":            user.ID,
                "phone":         user.Phone,
                "tier":          user.Tier,
                "device_id":     user.DeviceID,
                "token_version": user.TokenVersion,
                "created_at":    user.CreatedAt,
                "last_login":    user.LastLogin,
            },
            "wallet": fiber.Map{
                "id":           w.ID,
                "account_code": w.AccountCode,
                "currency":     w.Currency,
                "status":       w.Status,
                "created_at":   w.CreatedAt,
                "balance":      bal.Amount,
                "as_of":        bal.AsOf,
            },
        })
    })
}

