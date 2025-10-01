package routes

import (
    "log/slog"
    "net/http"

    "github.com/gofiber/fiber/v2"

    "github.com/congo-pay/congo_pay/internal/identity"
    "github.com/congo-pay/congo_pay/internal/wallet"
)

// RegisterIdentityRoutes wires identity endpoints and auto‑provisions a wallet on registration.
func RegisterIdentityRoutes(r fiber.Router, ids *identity.Service, wallets *wallet.Service, logger *slog.Logger) {
    // Register with auto-provisioned wallet
    r.Post("/identity/register", func(c *fiber.Ctx) error {
        var req struct {
            Phone    string `json:"phone"`
            PIN      string `json:"pin"`
            DeviceID string `json:"device_id"`
        }
        if err := c.BodyParser(&req); err != nil {
            return fiber.NewError(http.StatusBadRequest, err.Error())
        }
        user, err := ids.Register(c.UserContext(), identity.Credentials{Phone: req.Phone, PIN: req.PIN, DeviceID: req.DeviceID})
        if err != nil {
            return fiber.NewError(http.StatusBadRequest, err.Error())
        }
        var walletID string
        if wallets != nil {
            w, _ := wallets.Create(c.UserContext(), wallet.CreateInput{OwnerID: user.ID, Currency: "XAF"})
            walletID = w.ID
        }
        if logger != nil {
            logger.Info("identity.register completed",
                slog.String("user_id", user.ID),
                slog.String("phone", user.Phone),
                slog.String("wallet_id", walletID),
                slog.Int("status", http.StatusCreated),
            )
        }
        return c.Status(http.StatusCreated).JSON(fiber.Map{
            "user_id":   user.ID,
            "phone":     user.Phone,
            "tier":      user.Tier,
            "device_id": user.DeviceID,
            "wallet_id": walletID,
        })
    })

    // Plain authenticate (no tokens) remains for compatibility
    r.Post("/identity/authenticate", func(c *fiber.Ctx) error {
        var req struct {
            Phone    string `json:"phone"`
            PIN      string `json:"pin"`
            DeviceID string `json:"device_id"`
        }
        if err := c.BodyParser(&req); err != nil {
            return fiber.NewError(http.StatusBadRequest, err.Error())
        }
        user, err := ids.Authenticate(c.UserContext(), identity.Credentials{Phone: req.Phone, PIN: req.PIN, DeviceID: req.DeviceID})
        if err != nil {
            return fiber.NewError(http.StatusUnauthorized, err.Error())
        }
        return c.Status(http.StatusOK).JSON(fiber.Map{
            "user_id":   user.ID,
            "phone":     user.Phone,
            "tier":      user.Tier,
            "device_id": user.DeviceID,
        })
    })
}
