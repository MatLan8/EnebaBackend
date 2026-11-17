package repository

import (
	"context"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the set of query methods shared by *pgxpool.Pool and pgx.Tx.
// Repositories depend on it, so the same code runs with or without a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// psql is a squirrel builder that writes Postgres placeholders ($1, $2, ...)
// instead of the default "?".
var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// uniqueViolationCode is the Postgres error code for a UNIQUE constraint failure.
const uniqueViolationCode = "23505"

// isUniqueViolation reports whether err came from a UNIQUE constraint.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}
