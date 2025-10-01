package payments

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/congo-pay/congo_pay/internal/ledger"
)

// Handler exposes payment endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a payment handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type transferRequest struct {
	FromWalletID string `json:"from_wallet_id"`
	ToWalletID   string `json:"to_wallet_id"`
	Amount       int64  `json:"amount"`
	ClientTxID   string `json:"client_tx_id"`
}

// P2P processes a wallet-to-wallet transfer.
func (h *Handler) P2P(c *fiber.Ctx) error {
    var req transferRequest
    if err := c.BodyParser(&req); err != nil {
        return fiber.NewError(http.StatusBadRequest, err.Error())
    }
    uid, _ := c.Locals("user_id").(string)

    res, err := h.service.Transfer(c.UserContext(), TransferInput{
        FromWalletID: req.FromWalletID,
        ToWalletID:   req.ToWalletID,
        Amount:       req.Amount,
        ClientTxID:   req.ClientTxID,
        RequestorUserID: uid,
    })
    if err != nil {
        switch {
        case errors.Is(err, ledger.ErrInsufficientFunds):
            return fiber.NewError(http.StatusBadRequest, "insufficient funds")
        case errors.Is(err, ledger.ErrDuplicateTransaction):
            return fiber.NewError(http.StatusConflict, "duplicate transaction")
        case errors.Is(err, ErrNotOwner):
            return fiber.NewError(http.StatusForbidden, "not owner of source wallet")
        default:
            return fiber.NewError(http.StatusInternalServerError, err.Error())
        }
    }

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"transaction_id": res.TransactionID,
		"from_balance":   res.FromBalance,
		"to_balance":     res.ToBalance,
		"completed_at":   res.CompletedAt,
	})
}
