package ledger

import (
	"context"
	"sync"
)

type inMemoryLedger struct {
	mu           sync.RWMutex
	balances     map[string]int64
	transactions map[string]TransactionResult
	fundingTx    map[string]FundingResult
}

// NewInMemory creates a concurrency-safe in-memory ledger useful for unit tests.
func NewInMemory() Ledger {
	return &inMemoryLedger{
		balances:     make(map[string]int64),
		transactions: make(map[string]TransactionResult),
		fundingTx:    make(map[string]FundingResult),
	}
}

func (l *inMemoryLedger) EnsureAccount(_ context.Context, code string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.balances[code]; !exists {
		l.balances[code] = 0
	}
	return nil
}

func (l *inMemoryLedger) Balance(_ context.Context, code string) (int64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	balance, exists := l.balances[code]
	if !exists {
		return 0, ErrInsufficientFunds
	}
	return balance, nil
}

func (l *inMemoryLedger) Transfer(_ context.Context, fromCode, toCode, kind, clientTxID string, amount int64) (TransactionResult, error) {
	if amount <= 0 {
		return TransactionResult{}, ErrInsufficientFunds
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if res, exists := l.transactions[kind+":"+clientTxID]; exists {
		return res, ErrDuplicateTransaction
	}

	fromBalance, ok := l.balances[fromCode]
	if !ok {
		return TransactionResult{}, ErrInsufficientFunds
	}
	toBalance, ok := l.balances[toCode]
	if !ok {
		return TransactionResult{}, ErrInsufficientFunds
	}

	if fromBalance < amount {
		return TransactionResult{}, ErrInsufficientFunds
	}

	fromBalance -= amount
	toBalance += amount

	l.balances[fromCode] = fromBalance
	l.balances[toCode] = toBalance

	res := TransactionResult{
		TransactionID: kind + ":" + clientTxID,
		FromBalance:   fromBalance,
		ToBalance:     toBalance,
	}

	l.transactions[kind+":"+clientTxID] = res
	return res, nil
}

func (l *inMemoryLedger) CardIn(_ context.Context, walletCode, clientTxID string, amount int64) (FundingResult, error) {
	if amount <= 0 {
		return FundingResult{}, ErrInsufficientFunds
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	key := "card_in:" + clientTxID
	if res, exists := l.fundingTx[key]; exists {
		return res, ErrDuplicateTransaction
	}

	walletBalance, ok := l.balances[walletCode]
	if !ok {
		return FundingResult{}, ErrInsufficientFunds
	}

	walletBalance += amount
	l.balances[walletCode] = walletBalance
	l.balances[CardSuspenseAccountCode] -= amount

	res := FundingResult{
		TransactionID: key,
		WalletBalance: walletBalance,
		Status:        FundingStatusPendingSettlement,
	}
	l.fundingTx[key] = res
	return res, nil
}

func (l *inMemoryLedger) CardOut(_ context.Context, walletCode, clientTxID string, amount int64) (FundingResult, error) {
	if amount <= 0 {
		return FundingResult{}, ErrInsufficientFunds
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	key := "card_out:" + clientTxID
	if res, exists := l.fundingTx[key]; exists {
		return res, ErrDuplicateTransaction
	}

	walletBalance, ok := l.balances[walletCode]
	if !ok {
		return FundingResult{}, ErrInsufficientFunds
	}
	if walletBalance < amount {
		return FundingResult{}, ErrInsufficientFunds
	}

	walletBalance -= amount
	l.balances[walletCode] = walletBalance
	l.balances[CardSuspenseAccountCode] += amount

	res := FundingResult{
		TransactionID: key,
		WalletBalance: walletBalance,
		Status:        FundingStatusPendingSettlement,
	}
	l.fundingTx[key] = res
	return res, nil
}
