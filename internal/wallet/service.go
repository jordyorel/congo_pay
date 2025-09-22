package wallet

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"

    "github.com/congo-pay/congo_pay/internal/ledger"
)

const (
    statusActive = "active"
)

// Service exposes wallet operations backed by the ledger.
type Service struct {
    repo   Repository
    ledger ledger.Ledger
}

// NewService builds a wallet service instance.
func NewService(repo Repository, ledger ledger.Ledger) *Service {
    return &Service{repo: repo, ledger: ledger}
}

// CreateInput captures data required to create a wallet.
type CreateInput struct {
    OwnerID  string
    Currency string
}

// Create provisions a wallet and associated ledger account.
func (s *Service) Create(ctx context.Context, input CreateInput) (Wallet, error) {
    walletID := uuid.New().String()
    accountCode := fmt.Sprintf("wallet:%s", walletID)

    if _, err := uuid.Parse(input.OwnerID); err != nil {
        return Wallet{}, err
    }

    if err := s.ledger.EnsureAccount(ctx, accountCode); err != nil {
        return Wallet{}, err
    }

    currency := input.Currency
    if currency == "" {
        currency = "XAF"
    }

    wallet := Wallet{
        ID:          walletID,
        OwnerID:     input.OwnerID,
        AccountCode: accountCode,
        Currency:    currency,
        Status:      statusActive,
        CreatedAt:   time.Now().UTC(),
    }

    if err := s.repo.Create(ctx, wallet); err != nil {
        return Wallet{}, err
    }

    return wallet, nil
}

// Get retrieves wallet metadata.
func (s *Service) Get(ctx context.Context, id string) (Wallet, error) {
    return s.repo.Get(ctx, id)
}

// Balance returns the ledger balance for the wallet.
func (s *Service) Balance(ctx context.Context, id string) (Balance, error) {
    wallet, err := s.repo.Get(ctx, id)
    if err != nil {
        return Balance{}, err
    }
    amount, err := s.ledger.Balance(ctx, wallet.AccountCode)
    if err != nil {
        return Balance{}, err
    }
    return Balance{WalletID: wallet.ID, Amount: amount, AsOf: time.Now().UTC()}, nil
}
