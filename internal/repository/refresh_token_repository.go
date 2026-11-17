package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RefreshTokenRepository struct {
	db DBTX
}

func NewRefreshTokenRepository(db DBTX) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

// WithTx returns a copy of the repository that runs its queries inside tx.
func (r *RefreshTokenRepository) WithTx(tx pgx.Tx) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: tx}
}

// Create stores the hash of a new refresh token for the user.
func (r *RefreshTokenRepository) Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	query, args, err := psql.
		Insert("refresh_tokens").
		Columns("user_id", "token_hash", "expires_at").
		Values(userID, tokenHash, expiresAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert refresh token: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}

	return nil
}
