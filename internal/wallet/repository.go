package wallet

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists wallet metadata.
type Repository interface {
	Create(ctx context.Context, wallet Wallet) error
	Get(ctx context.Context, id string) (Wallet, error)
}

// PostgresRepository stores wallets in PostgreSQL.
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository builds a repository backed by PostgreSQL.
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Create inserts a wallet record.
func (r *PostgresRepository) Create(ctx context.Context, wallet Wallet) error {
	walletID, err := uuid.Parse(wallet.ID)
	if err != nil {
		return err
	}
	ownerID, err := uuid.Parse(wallet.OwnerID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `INSERT INTO wallets (id, owner_id, account_code, currency, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6)`, walletID, ownerID, wallet.AccountCode, wallet.Currency, wallet.Status, wallet.CreatedAt.UTC())
	return err
}

// Get fetches wallet metadata by identifier.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Wallet, error) {
	walletUUID, err := uuid.Parse(id)
	if err != nil {
		return Wallet{}, err
	}
	row := r.db.QueryRow(ctx, `SELECT id, owner_id, account_code, currency, status, created_at
        FROM wallets WHERE id = $1`, walletUUID)
	var w Wallet
	var createdAt time.Time
	var idVal uuid.UUID
	var ownerID uuid.UUID
	if err := row.Scan(&idVal, &ownerID, &w.AccountCode, &w.Currency, &w.Status, &createdAt); err != nil {
		return Wallet{}, err
	}
	w.ID = idVal.String()
	w.OwnerID = ownerID.String()
	w.CreatedAt = createdAt.UTC()
	return w, nil
}
