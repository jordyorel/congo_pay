package ledger

import (
	"context"
	"errors"
)

var (
	// ErrInsufficientFunds occurs when the source account lacks available balance
	// to cover a requested posting.
	ErrInsufficientFunds = errors.New("insufficient funds")

	// ErrDuplicateTransaction indicates the provided client transaction identifier
	// already exists and therefore the operation should be treated as idempotent.
	ErrDuplicateTransaction = errors.New("duplicate transaction")
)

const (
	// FundingStatusPendingSettlement indicates a card transaction awaiting settlement confirmation.
	FundingStatusPendingSettlement = "pending_settlement"
	// FundingStatusCompleted represents a settled transaction.
	FundingStatusCompleted = "completed"
	// CardSuspenseAccountCode is the ledger account used to park card transactions pre-settlement.
	CardSuspenseAccountCode = "suspense:card"
)

// TransactionResult captures the outcome of a ledger posting.
type TransactionResult struct {
	TransactionID string
	FromBalance   int64
	ToBalance     int64
}

// FundingResult captures the outcome of a card funding transaction.
type FundingResult struct {
	TransactionID string
	WalletBalance int64
	Status        string
}

// Ledger defines the contract implemented by ledger backends (e.g. Postgres).
type Ledger interface {
	EnsureAccount(ctx context.Context, code string) error
	Balance(ctx context.Context, code string) (int64, error)
	Transfer(ctx context.Context, fromCode, toCode, kind, clientTxID string, amount int64) (TransactionResult, error)
	CardIn(ctx context.Context, walletCode, clientTxID string, amount int64) (FundingResult, error)
	CardOut(ctx context.Context, walletCode, clientTxID string, amount int64) (FundingResult, error)
}
