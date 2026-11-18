package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"eneba-backend/internal/models"
)

type RefreshTokenRepository struct {
	db DBTX
}

func NewRefreshTokenRepository(db DBTX) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) WithTx(tx pgx.Tx) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: tx}
}

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

// Unknown or already revoked tokens are left alone, so calling it twice is harmless.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, tokenHash string) error {
	query, args, err := psql.
		Update("refresh_tokens").
		Set("revoked_at", sq.Expr("now()")).
		Where(sq.Eq{"token_hash": tokenHash, "revoked_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build revoke refresh token: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	return nil
}

// In the order scanRefreshToken expects.
var refreshTokenColumns = []string{
	"id", "user_id", "token_hash", "expires_at", "revoked_at", "replaced_by_token_hash", "created_at",
}

// The row lock stops two requests from rotating the same token at once.
func (r *RefreshTokenRepository) GetByHashForUpdate(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	query, args, err := psql.
		Select(refreshTokenColumns...).
		From("refresh_tokens").
		Where(sq.Eq{"token_hash": tokenHash}).
		Suffix("FOR UPDATE").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select refresh token: %w", err)
	}

	token, err := scanRefreshToken(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("select refresh token: %w", err)
	}

	return token, nil
}

// A token with replaced_by_token_hash set has been used, so seeing it again means reuse.
func (r *RefreshTokenRepository) MarkReplaced(ctx context.Context, id uuid.UUID, newTokenHash string) error {
	query, args, err := psql.
		Update("refresh_tokens").
		Set("revoked_at", sq.Expr("now()")).
		Set("replaced_by_token_hash", newTokenHash).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build mark refresh token replaced: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark refresh token replaced: %w", err)
	}

	return nil
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	query, args, err := psql.
		Update("refresh_tokens").
		Set("revoked_at", sq.Expr("now()")).
		Where(sq.Eq{"user_id": userID, "revoked_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build revoke all refresh tokens: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("revoke all refresh tokens: %w", err)
	}

	return nil
}

// The order must match refreshTokenColumns.
func scanRefreshToken(row pgx.Row) (*models.RefreshToken, error) {
	var t models.RefreshToken

	err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.ReplacedByTokenHash, &t.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &t, nil
}
