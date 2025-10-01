package auth

import (
    "net/http"

    "github.com/gofiber/fiber/v2"

    "github.com/congo-pay/congo_pay/internal/identity"
    "github.com/congo-pay/congo_pay/internal/wallet"
)

// Handler exposes auth endpoints for login/refresh/logout.
type Handler struct {
    ids *identity.Service
    svc *Service
    wallets *wallet.Service
}

func NewHandler(ids *identity.Service, svc *Service, wallets *wallet.Service) *Handler {
    return &Handler{ids: ids, svc: svc, wallets: wallets}
}

type loginRequest struct {
    Phone    string `json:"phone"`
    PIN      string `json:"pin"`
    DeviceID string `json:"device_id"`
}

type loginResponse struct {
    UserID       string `json:"user_id"`
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
    TokenVersion int    `json:"token_version"`
    WalletID     string `json:"wallet_id,omitempty"`
}

// Login validates credentials and returns a token pair.
func (h *Handler) Login(c *fiber.Ctx) error {
    var req loginRequest
    if err := c.BodyParser(&req); err != nil {
        return fiber.NewError(http.StatusBadRequest, err.Error())
    }
    user, err := h.ids.Authenticate(c.UserContext(), identity.Credentials{Phone: req.Phone, PIN: req.PIN, DeviceID: req.DeviceID})
    if err != nil {
        return fiber.NewError(http.StatusUnauthorized, err.Error())
    }
    pair, err := h.svc.Login(user)
    if err != nil {
        return fiber.NewError(http.StatusInternalServerError, err.Error())
    }
    var wid string
    if h.wallets != nil {
        if w, err := h.wallets.GetByOwner(c.UserContext(), user.ID); err == nil {
            wid = w.ID
        }
    }
    return c.Status(http.StatusOK).JSON(loginResponse{UserID: user.ID, AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresIn: pair.ExpiresIn, TokenVersion: user.TokenVersion, WalletID: wid})
}

type refreshRequest struct {
    RefreshToken string `json:"refresh_token"`
}

// Refresh issues a new access token using a valid refresh token.
func (h *Handler) Refresh(c *fiber.Ctx) error {
    var req refreshRequest
    if err := c.BodyParser(&req); err != nil {
        return fiber.NewError(http.StatusBadRequest, err.Error())
    }
    token, exp, err := h.svc.Refresh(c.UserContext(), req.RefreshToken)
    if err != nil {
        return fiber.NewError(http.StatusUnauthorized, err.Error())
    }
    return c.Status(http.StatusOK).JSON(fiber.Map{"access_token": token, "expires_in": exp})
}

type logoutRequest struct {
    UserID string `json:"user_id"`
}

// Logout invalidates existing tokens by bumping the token version.
func (h *Handler) Logout(c *fiber.Ctx) error {
    var req logoutRequest
    if err := c.BodyParser(&req); err != nil {
        return fiber.NewError(http.StatusBadRequest, err.Error())
    }
    if req.UserID == "" {
        return fiber.NewError(http.StatusBadRequest, "user_id is required")
    }
    if err := h.svc.Logout(c.UserContext(), req.UserID); err != nil {
        return fiber.NewError(http.StatusBadRequest, err.Error())
    }
    return c.Status(http.StatusOK).JSON(fiber.Map{"status": "logged_out"})
}
