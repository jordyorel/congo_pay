package wallet

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// Handler exposes wallet HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler builds a wallet HTTP handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type createRequest struct {
	OwnerID  string `json:"owner_id"`
	Currency string `json:"currency"`
}

type walletResponse struct {
	ID          string `json:"id"`
	OwnerID     string `json:"owner_id"`
	AccountCode string `json:"account_code"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
}

// Create provisions a wallet for the authenticated owner.
func (h *Handler) Create(c *fiber.Ctx) error {
	var req createRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	wallet, err := h.service.Create(c.UserContext(), CreateInput{OwnerID: req.OwnerID, Currency: req.Currency})
	if err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	return c.Status(http.StatusCreated).JSON(walletResponse{
		ID:          wallet.ID,
		OwnerID:     wallet.OwnerID,
		AccountCode: wallet.AccountCode,
		Currency:    wallet.Currency,
		Status:      wallet.Status,
	})
}

// Balance returns the wallet balance.
func (h *Handler) Balance(c *fiber.Ctx) error {
	walletID := c.Params("walletId")
	balance, err := h.service.Balance(c.UserContext(), walletID)
	if err != nil {
		return fiber.NewError(http.StatusNotFound, err.Error())
	}
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"wallet_id": walletID,
		"balance":   balance.Amount,
		"timestamp": balance.AsOf,
	})
}
