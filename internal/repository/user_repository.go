package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"eneba-backend/internal/models"
)

// userColumns is the column list used by every query, in the order scanUser expects.
var userColumns = []string{"id", "name", "email", "password_hash", "created_at", "updated_at"}

type UserRepository struct {
	db DBTX
}

func NewUserRepository(db DBTX) *UserRepository {
	return &UserRepository{db: db}
}

// WithTx returns a copy of the repository that runs its queries inside tx.
func (r *UserRepository) WithTx(tx pgx.Tx) *UserRepository {
	return &UserRepository{db: tx}
}

// Create inserts a user and returns the stored row.
// It returns models.ErrEmailTaken if the email is already registered.
func (r *UserRepository) Create(ctx context.Context, name, email, passwordHash string) (*models.User, error) {
	query, args, err := psql.
		Insert("users").
		Columns("name", "email", "password_hash").
		Values(name, email, passwordHash).
		Suffix("RETURNING " + strings.Join(userColumns, ", ")).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert user: %w", err)
	}

	user, err := scanUser(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, models.ErrEmailTaken
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return user, nil
}

// GetByEmail returns models.ErrNotFound if no user has this email.
// The email column is CITEXT, so the match is case-insensitive.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.getOne(ctx, sq.Eq{"email": email})
}

// GetByID returns models.ErrNotFound if the user doesn't exist.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return r.getOne(ctx, sq.Eq{"id": id})
}

// getOne runs a SELECT for a single user matching where.
func (r *UserRepository) getOne(ctx context.Context, where sq.Sqlizer) (*models.User, error) {
	query, args, err := psql.
		Select(userColumns...).
		From("users").
		Where(where).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select user: %w", err)
	}

	user, err := scanUser(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("select user: %w", err)
	}

	return user, nil
}

// scanUser reads one row into a models.User. The order must match userColumns.
func scanUser(row pgx.Row) (*models.User, error) {
	var u models.User

	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &u, nil
}
