package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists users.
type Repository interface {
	Create(ctx context.Context, user User) error
	FindByPhone(ctx context.Context, phone string) (User, error)
	UpdateDevice(ctx context.Context, id, deviceID string) error
}

// PostgresRepository implements Repository using PostgreSQL.
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository builds a Postgres-backed identity repository.
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Create inserts a new user.
func (r *PostgresRepository) Create(ctx context.Context, user User) error {
	userID, err := uuid.Parse(user.ID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `INSERT INTO users (id, phone, tier, pin_hash, device_id, created_at)
        VALUES ($1, $2, $3, $4, $5, $6)`, userID, user.Phone, user.Tier, user.PINHash, user.DeviceID, user.CreatedAt.UTC())
	return err
}

// FindByPhone fetches a user by phone number.
func (r *PostgresRepository) FindByPhone(ctx context.Context, phone string) (User, error) {
	row := r.db.QueryRow(ctx, `SELECT id, phone, tier, pin_hash, device_id, created_at FROM users WHERE phone = $1`, phone)
	var (
		id        uuid.UUID
		createdAt time.Time
		user      User
	)
	if err := row.Scan(&id, &user.Phone, &user.Tier, &user.PINHash, &user.DeviceID, &createdAt); err != nil {
		return User{}, err
	}
	user.ID = id.String()
	user.CreatedAt = createdAt.UTC()
	return user, nil
}

// UpdateDevice stores the users bound device identifier.
func (r *PostgresRepository) UpdateDevice(ctx context.Context, id, deviceID string) error {
	userID, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	cmd, err := r.db.Exec(ctx, `UPDATE users SET device_id = $1 WHERE id = $2`, deviceID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}
