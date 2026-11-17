package repository

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"eneba-backend/internal/models"
)

type ProductRepository struct {
	db DBTX
}

func NewProductRepository(db DBTX) *ProductRepository {
	return &ProductRepository{db: db}
}

// hasActiveOffer limits a query on "products p" to products that can be bought,
// i.e. products with at least one active offer.
var hasActiveOffer = sq.Expr("EXISTS (SELECT 1 FROM offers o WHERE o.product_id = p.id AND o.is_active)")

// GetFilters returns every value the store can be filtered by, with the number
// of products that have it, plus the highest active offer price.
// Only products with an active offer are counted, so no filter leads to an empty page.
func (r *ProductRepository) GetFilters(ctx context.Context) (*models.ProductFilters, error) {
	types, err := r.countProductsBy(ctx, "p.type")
	if err != nil {
		return nil, fmt.Errorf("count products by type: %w", err)
	}

	platforms, err := r.countProductsBy(ctx, "p.platform")
	if err != nil {
		return nil, fmt.Errorf("count products by platform: %w", err)
	}

	regions, err := r.countProductsBy(ctx, "p.region")
	if err != nil {
		return nil, fmt.Errorf("count products by region: %w", err)
	}

	genres, err := r.countProductsByGenre(ctx)
	if err != nil {
		return nil, fmt.Errorf("count products by genre: %w", err)
	}

	maxPriceCents, err := r.maxActivePrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("select max price: %w", err)
	}

	return &models.ProductFilters{
		Types:         types,
		Platforms:     platforms,
		Regions:       regions,
		Genres:        genres,
		MaxPriceCents: maxPriceCents,
	}, nil
}

// countProductsBy groups products by column and counts each value, most common first.
// column goes into the SQL as-is, so it must always be a constant from this file, never user input.
func (r *ProductRepository) countProductsBy(ctx context.Context, column string) ([]models.FilterCount, error) {
	query, args, err := psql.
		Select(column, "COUNT(*)").
		From("products p").
		Where(hasActiveOffer).
		GroupBy(column).
		OrderBy("COUNT(*) DESC", column).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	// CollectRows reads every row and closes rows. RowToStructByPos fills the
	// struct fields in column order: column -> Value, COUNT(*) -> Count.
	return pgx.CollectRows(rows, pgx.RowToStructByPos[models.FilterCount])
}

// countProductsByGenre counts products per genre, most common first.
func (r *ProductRepository) countProductsByGenre(ctx context.Context) ([]models.GenreFilterCount, error) {
	query, args, err := psql.
		Select("g.slug", "g.name", "COUNT(*)").
		From("genres g").
		Join("product_genres pg ON pg.genre_id = g.id").
		Join("products p ON p.id = pg.product_id").
		Where(hasActiveOffer).
		GroupBy("g.slug", "g.name").
		OrderBy("COUNT(*) DESC", "g.slug").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByPos[models.GenreFilterCount])
}

// maxActivePrice returns the highest price among active offers, or 0 if there are none.
func (r *ProductRepository) maxActivePrice(ctx context.Context) (int, error) {
	query, args, err := psql.
		Select("COALESCE(MAX(price_cents), 0)"). // MAX of no rows is NULL
		From("offers").
		Where(sq.Eq{"is_active": true}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build query: %w", err)
	}

	var maxCents int
	if err := r.db.QueryRow(ctx, query, args...).Scan(&maxCents); err != nil {
		return 0, err
	}

	return maxCents, nil
}
