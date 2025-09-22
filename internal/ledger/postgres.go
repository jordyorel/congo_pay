package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresLedger persists ledger entries in PostgreSQL ensuring double-entry balance.
type PostgresLedger struct {
	db *pgxpool.Pool
}

// NewPostgresLedger constructs a Postgres-backed ledger implementation.
func NewPostgresLedger(db *pgxpool.Pool) *PostgresLedger {
	return &PostgresLedger{db: db}
}

// EnsureAccount guarantees an account exists for the provided code.
func (l *PostgresLedger) EnsureAccount(ctx context.Context, code string) error {
	_, err := l.db.Exec(ctx, `INSERT INTO accounts (id, code) VALUES ($1, $2)
        ON CONFLICT (code) DO NOTHING`, uuid.New(), code)
	return err
}

// Balance returns the summed balance for the specified account code.
func (l *PostgresLedger) Balance(ctx context.Context, code string) (int64, error) {
	const query = `
        SELECT COALESCE(SUM(e.amount), 0)
        FROM entries e
        INNER JOIN accounts a ON a.id = e.account_id
        WHERE a.code = $1`
	var balance int64
	if err := l.db.QueryRow(ctx, query, code).Scan(&balance); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("account %s not found", code)
		}
		return 0, err
	}
	return balance, nil
}

// Transfer records a balanced posting between two accounts.
func (l *PostgresLedger) Transfer(ctx context.Context, fromCode, toCode, kind, clientTxID string, amount int64) (TransactionResult, error) {
	if amount <= 0 {
		return TransactionResult{}, fmt.Errorf("amount must be positive")
	}

	tx, err := l.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TransactionResult{}, err
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	accountsQuery := `SELECT id FROM accounts WHERE code = $1 FOR UPDATE`

	var fromAccountID uuid.UUID
	if err := tx.QueryRow(ctx, accountsQuery, fromCode).Scan(&fromAccountID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TransactionResult{}, fmt.Errorf("from account %s not found", fromCode)
		}
		return TransactionResult{}, err
	}

	var toAccountID uuid.UUID
	if err := tx.QueryRow(ctx, accountsQuery, toCode).Scan(&toAccountID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TransactionResult{}, fmt.Errorf("to account %s not found", toCode)
		}
		return TransactionResult{}, err
	}

	const existingTxQuery = `SELECT id FROM transactions WHERE client_tx_id = $1 AND kind = $2`
	var existingTxID uuid.UUID
	if err := tx.QueryRow(ctx, existingTxQuery, clientTxID, kind).Scan(&existingTxID); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return TransactionResult{}, err
		}
	} else {
		fromBal, err := balanceForAccount(ctx, tx, fromAccountID)
		if err != nil {
			return TransactionResult{}, err
		}
		toBal, err := balanceForAccount(ctx, tx, toAccountID)
		if err != nil {
			return TransactionResult{}, err
		}
		return TransactionResult{TransactionID: existingTxID.String(), FromBalance: fromBal, ToBalance: toBal}, ErrDuplicateTransaction
	}

	fromBalance, err := balanceForAccount(ctx, tx, fromAccountID)
	if err != nil {
		return TransactionResult{}, err
	}
	if fromBalance < amount {
		return TransactionResult{}, ErrInsufficientFunds
	}

	txID := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO transactions (id, client_tx_id, kind, status) VALUES ($1, $2, $3, $4)`, txID, clientTxID, kind, FundingStatusCompleted); err != nil {
		return TransactionResult{}, err
	}

	if _, err := tx.Exec(ctx, `INSERT INTO entries (id, transaction_id, account_id, amount) VALUES ($1, $2, $3, $4)`, uuid.New(), txID, fromAccountID, -amount); err != nil {
		return TransactionResult{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO entries (id, transaction_id, account_id, amount) VALUES ($1, $2, $3, $4)`, uuid.New(), txID, toAccountID, amount); err != nil {
		return TransactionResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return TransactionResult{}, err
	}

	// Retrieve updated balances after commit.
	fromBal, err := l.Balance(ctx, fromCode)
	if err != nil {
		return TransactionResult{}, err
	}
	toBal, err := l.Balance(ctx, toCode)
	if err != nil {
		return TransactionResult{}, err
	}

	return TransactionResult{TransactionID: txID.String(), FromBalance: fromBal, ToBalance: toBal}, nil
}

// CardIn records a card funding authorization and holds it in suspense until settlement.
func (l *PostgresLedger) CardIn(ctx context.Context, walletCode, clientTxID string, amount int64) (FundingResult, error) {
	if amount <= 0 {
		return FundingResult{}, fmt.Errorf("amount must be positive")
	}

	tx, err := l.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return FundingResult{}, err
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	walletAccountID, err := accountIDForCode(ctx, tx, walletCode)
	if err != nil {
		return FundingResult{}, err
	}
	suspenseAccountID, err := accountIDForCode(ctx, tx, CardSuspenseAccountCode)
	if err != nil {
		return FundingResult{}, err
	}

	const existingQuery = `SELECT id, status FROM transactions WHERE client_tx_id = $1 AND kind = 'card_in'`
	var existingTxID uuid.UUID
	var existingStatus string
	if err := tx.QueryRow(ctx, existingQuery, clientTxID).Scan(&existingTxID, &existingStatus); err == nil {
		walletBal, balErr := balanceForAccount(ctx, tx, walletAccountID)
		if balErr != nil {
			return FundingResult{}, balErr
		}
		return FundingResult{TransactionID: existingTxID.String(), WalletBalance: walletBal, Status: existingStatus}, ErrDuplicateTransaction
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return FundingResult{}, err
	}

	txID := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO transactions (id, client_tx_id, kind, status) VALUES ($1, $2, $3, $4)`, txID, clientTxID, "card_in", FundingStatusPendingSettlement); err != nil {
		return FundingResult{}, err
	}

	if _, err := tx.Exec(ctx, `INSERT INTO entries (id, transaction_id, account_id, amount) VALUES ($1, $2, $3, $4)`, uuid.New(), txID, walletAccountID, amount); err != nil {
		return FundingResult{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO entries (id, transaction_id, account_id, amount) VALUES ($1, $2, $3, $4)`, uuid.New(), txID, suspenseAccountID, -amount); err != nil {
		return FundingResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return FundingResult{}, err
	}

	walletBalance, err := l.Balance(ctx, walletCode)
	if err != nil {
		return FundingResult{}, err
	}

	return FundingResult{TransactionID: txID.String(), WalletBalance: walletBalance, Status: FundingStatusPendingSettlement}, nil
}

// CardOut records a card withdrawal request by debiting the wallet and crediting suspense until settlement.
func (l *PostgresLedger) CardOut(ctx context.Context, walletCode, clientTxID string, amount int64) (FundingResult, error) {
	if amount <= 0 {
		return FundingResult{}, fmt.Errorf("amount must be positive")
	}

	tx, err := l.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return FundingResult{}, err
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	walletAccountID, err := accountIDForCode(ctx, tx, walletCode)
	if err != nil {
		return FundingResult{}, err
	}
	suspenseAccountID, err := accountIDForCode(ctx, tx, CardSuspenseAccountCode)
	if err != nil {
		return FundingResult{}, err
	}

	const existingQuery = `SELECT id, status FROM transactions WHERE client_tx_id = $1 AND kind = 'card_out'`
	var existingTxID uuid.UUID
	var existingStatus string
	if err := tx.QueryRow(ctx, existingQuery, clientTxID).Scan(&existingTxID, &existingStatus); err == nil {
		walletBal, balErr := balanceForAccount(ctx, tx, walletAccountID)
		if balErr != nil {
			return FundingResult{}, balErr
		}
		return FundingResult{TransactionID: existingTxID.String(), WalletBalance: walletBal, Status: existingStatus}, ErrDuplicateTransaction
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return FundingResult{}, err
	}

	walletBalance, err := balanceForAccount(ctx, tx, walletAccountID)
	if err != nil {
		return FundingResult{}, err
	}
	if walletBalance < amount {
		return FundingResult{}, ErrInsufficientFunds
	}

	txID := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO transactions (id, client_tx_id, kind, status) VALUES ($1, $2, $3, $4)`, txID, clientTxID, "card_out", FundingStatusPendingSettlement); err != nil {
		return FundingResult{}, err
	}

	if _, err := tx.Exec(ctx, `INSERT INTO entries (id, transaction_id, account_id, amount) VALUES ($1, $2, $3, $4)`, uuid.New(), txID, walletAccountID, -amount); err != nil {
		return FundingResult{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO entries (id, transaction_id, account_id, amount) VALUES ($1, $2, $3, $4)`, uuid.New(), txID, suspenseAccountID, amount); err != nil {
		return FundingResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return FundingResult{}, err
	}

	updatedBalance, err := l.Balance(ctx, walletCode)
	if err != nil {
		return FundingResult{}, err
	}

	return FundingResult{TransactionID: txID.String(), WalletBalance: updatedBalance, Status: FundingStatusPendingSettlement}, nil
}

func accountIDForCode(ctx context.Context, tx pgx.Tx, code string) (uuid.UUID, error) {
	const query = `SELECT id FROM accounts WHERE code = $1 FOR UPDATE`
	var id uuid.UUID
	if err := tx.QueryRow(ctx, query, code).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("account %s not found", code)
		}
		return uuid.Nil, err
	}
	return id, nil
}

func balanceForAccount(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) (int64, error) {
	const query = `SELECT COALESCE(SUM(amount), 0) FROM entries WHERE account_id = $1`
	var balance int64
	if err := tx.QueryRow(ctx, query, accountID).Scan(&balance); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return balance, nil
}
