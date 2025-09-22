package funding

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/congo-pay/congo_pay/internal/ledger"
)

// Handler exposes HTTP endpoints for card funding flows.
type Handler struct {
	service *Service
}

// NewHandler constructs a funding handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// CardIn processes wallet top-ups funded by cards.
func (h *Handler) CardIn(c *fiber.Ctx) error {
	walletID := c.Params("walletId")
	var req CardInRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}

	result, err := h.service.CardIn(c.UserContext(), CardInInput{
		WalletID:   walletID,
		Amount:     req.Amount,
		ClientTxID: req.ClientTxID,
		CardNumber: req.CardNumber,
		Expiry:     req.Expiry,
		CVV:        req.CVV,
	})
	if err != nil {
		switch {
		case errors.Is(err, ledger.ErrDuplicateTransaction):
			return c.Status(http.StatusOK).JSON(toResponse(result))
		case errors.Is(err, ledger.ErrInsufficientFunds):
			return fiber.NewError(http.StatusBadRequest, err.Error())
		default:
			return fiber.NewError(http.StatusBadRequest, err.Error())
		}
	}

	return c.Status(http.StatusCreated).JSON(toResponse(result))
}

// CardOut processes wallet withdrawals to cards.
func (h *Handler) CardOut(c *fiber.Ctx) error {
	walletID := c.Params("walletId")
	var req CardOutRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}

	result, err := h.service.CardOut(c.UserContext(), CardOutInput{
		WalletID:   walletID,
		Amount:     req.Amount,
		ClientTxID: req.ClientTxID,
		CardNumber: req.CardNumber,
	})
	if err != nil {
		switch {
		case errors.Is(err, ledger.ErrDuplicateTransaction):
			return c.Status(http.StatusOK).JSON(toResponse(result))
		case errors.Is(err, ledger.ErrInsufficientFunds):
			return fiber.NewError(http.StatusBadRequest, err.Error())
		default:
			return fiber.NewError(http.StatusBadRequest, err.Error())
		}
	}

	return c.Status(http.StatusCreated).JSON(toResponse(result))
}

func toResponse(result FundingResult) FundingResponse {
	return FundingResponse{
		TransactionID:     result.TransactionID,
		Status:            result.Status,
		WalletBalance:     result.WalletBalance,
		AcquirerReference: result.AcquirerReference,
	}
}
