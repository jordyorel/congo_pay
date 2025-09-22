package funding

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/congo-pay/congo_pay/internal/ledger"
	"github.com/congo-pay/congo_pay/internal/wallet"
)

// Service coordinates card funding and withdrawal operations using the ledger and acquirer connector.
type Service struct {
	ledger   ledger.Ledger
	wallets  *wallet.Service
	acquirer Acquirer
}

// NewService prepares a funding service ensuring the card suspense account exists.
func NewService(ctx context.Context, ledgerBackend ledger.Ledger, wallets *wallet.Service, acquirer Acquirer) (*Service, error) {
	if wallets == nil {
		return nil, fmt.Errorf("wallet service is required")
	}
	if acquirer == nil {
		acquirer = StaticAcquirer{}
	}
	if err := ledgerBackend.EnsureAccount(ctx, ledger.CardSuspenseAccountCode); err != nil {
		return nil, err
	}
	return &Service{ledger: ledgerBackend, wallets: wallets, acquirer: acquirer}, nil
}

// CardInInput captures the required data for a card top-up.
type CardInInput struct {
	WalletID   string
	Amount     int64
	ClientTxID string
	CardNumber string
	Expiry     string
	CVV        string
}

// CardOutInput captures the required data for a card withdrawal.
type CardOutInput struct {
	WalletID   string
	Amount     int64
	ClientTxID string
	CardNumber string
}

// FundingResult represents the domain outcome of a card operation.
type FundingResult struct {
	TransactionID     string
	Status            string
	WalletBalance     int64
	AcquirerReference string
	CompletedAt       time.Time
}

// CardIn authorizes and records a card top-up into the specified wallet.
func (s *Service) CardIn(ctx context.Context, input CardInInput) (FundingResult, error) {
	if err := validateCardNumber(input.CardNumber); err != nil {
		return FundingResult{}, err
	}
	if input.Amount <= 0 {
		return FundingResult{}, fmt.Errorf("amount must be positive")
	}
	if input.ClientTxID == "" {
		input.ClientTxID = uuid.NewString()
	}

	w, err := s.wallets.Get(ctx, input.WalletID)
	if err != nil {
		return FundingResult{}, err
	}

	decision, err := s.acquirer.AuthorizeCardIn(ctx, CardInAuthorization{
		CardNumber: input.CardNumber,
		Expiry:     input.Expiry,
		CVV:        input.CVV,
		Amount:     input.Amount,
	})
	if err != nil {
		return FundingResult{}, err
	}

	ledgerResult, err := s.ledger.CardIn(ctx, w.AccountCode, input.ClientTxID, input.Amount)
	if err != nil {
		if errors.Is(err, ledger.ErrDuplicateTransaction) || errors.Is(err, ledger.ErrInsufficientFunds) {
			return FundingResult{
				TransactionID:     ledgerResult.TransactionID,
				Status:            ledgerResult.Status,
				WalletBalance:     ledgerResult.WalletBalance,
				AcquirerReference: decision.Reference,
				CompletedAt:       time.Now().UTC(),
			}, err
		}
		return FundingResult{}, err
	}

	return FundingResult{
		TransactionID:     ledgerResult.TransactionID,
		Status:            ledgerResult.Status,
		WalletBalance:     ledgerResult.WalletBalance,
		AcquirerReference: decision.Reference,
		CompletedAt:       time.Now().UTC(),
	}, nil
}

// CardOut authorizes and records a withdrawal to the provided card.
func (s *Service) CardOut(ctx context.Context, input CardOutInput) (FundingResult, error) {
	if err := validateCardNumber(input.CardNumber); err != nil {
		return FundingResult{}, err
	}
	if input.Amount <= 0 {
		return FundingResult{}, fmt.Errorf("amount must be positive")
	}
	if input.ClientTxID == "" {
		input.ClientTxID = uuid.NewString()
	}

	w, err := s.wallets.Get(ctx, input.WalletID)
	if err != nil {
		return FundingResult{}, err
	}

	decision, err := s.acquirer.AuthorizeCardOut(ctx, CardOutAuthorization{
		CardNumber: input.CardNumber,
		Amount:     input.Amount,
	})
	if err != nil {
		return FundingResult{}, err
	}

	ledgerResult, err := s.ledger.CardOut(ctx, w.AccountCode, input.ClientTxID, input.Amount)
	if err != nil {
		if errors.Is(err, ledger.ErrDuplicateTransaction) || errors.Is(err, ledger.ErrInsufficientFunds) {
			return FundingResult{
				TransactionID:     ledgerResult.TransactionID,
				Status:            ledgerResult.Status,
				WalletBalance:     ledgerResult.WalletBalance,
				AcquirerReference: decision.Reference,
				CompletedAt:       time.Now().UTC(),
			}, err
		}
		return FundingResult{}, err
	}

	return FundingResult{
		TransactionID:     ledgerResult.TransactionID,
		Status:            ledgerResult.Status,
		WalletBalance:     ledgerResult.WalletBalance,
		AcquirerReference: decision.Reference,
		CompletedAt:       time.Now().UTC(),
	}, nil
}

func validateCardNumber(card string) error {
	digits := strings.ReplaceAll(card, " ", "")
	if len(digits) < 12 || len(digits) > 19 {
		return fmt.Errorf("card number must be between 12 and 19 digits")
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return fmt.Errorf("card number must be numeric")
		}
	}
	return nil
}
