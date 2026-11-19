package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"eneba-backend/internal/services"
)

var errCatalogNotEmpty = errors.New("catalog already has data")

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

const batchSize = 500

type seedSummary struct {
	Genres        int
	Sellers       int
	Products      int
	ProductGenres int
	Offers        int
	DigitalKeys   int
	UsersInserted int
	UsersSkipped  int
}

func ensureCatalogEmpty(ctx context.Context, tx pgx.Tx) error {
	var hasProducts bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM products)").Scan(&hasProducts); err != nil {
		return fmt.Errorf("check products: %w", err)
	}
	if hasProducts {
		return errCatalogNotEmpty
	}
	return nil
}

// One TRUNCATE may name tables that reference each other. Users and refresh tokens are kept;
// carts and orders go too, because their items reference offers and keys. It runs before the
// migrations, so tables that don't exist yet (fresh database) are skipped.
func truncateCatalog(ctx context.Context, pool *pgxpool.Pool) error {
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('products') IS NOT NULL").Scan(&exists); err != nil {
		return fmt.Errorf("check catalog tables: %w", err)
	}
	if !exists {
		return nil
	}

	tables := []string{"digital_keys", "offers", "sellers", "product_genres", "genres", "products"}
	for _, table := range []string{"cart_items", "carts", "order_items", "orders"} {
		var tableExists bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&tableExists); err != nil {
			return fmt.Errorf("check %s table: %w", table, err)
		}
		if tableExists {
			tables = append(tables, table)
		}
	}

	query := "TRUNCATE " + strings.Join(tables, ", ")

	_, err := pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("truncate catalog: %w", err)
	}
	return nil
}

// Parents go first (genres, sellers, products) so the later foreign keys are valid.
func seedCatalog(ctx context.Context, tx pgx.Tx, data *seedFile, keyCipher *services.KeyCipher) (seedSummary, error) {
	var summary seedSummary

	genreIDs, err := seedGenres(ctx, tx, data.Genres)
	if err != nil {
		return summary, err
	}
	summary.Genres = len(genreIDs)

	sellerIDs, err := seedSellers(ctx, tx, data.Sellers)
	if err != nil {
		return summary, err
	}
	summary.Sellers = len(sellerIDs)

	platformIDs, err := loadPlatformIDs(ctx, tx)
	if err != nil {
		return summary, err
	}

	productIDs, productGenres, err := seedProducts(ctx, tx, data.Products, genreIDs, platformIDs)
	if err != nil {
		return summary, err
	}
	summary.Products = len(productIDs)
	summary.ProductGenres = productGenres

	offers, keys, err := seedOffers(ctx, tx, data.Offers, productIDs, sellerIDs, keyCipher)
	if err != nil {
		return summary, err
	}
	summary.Offers = offers
	summary.DigitalKeys = keys

	return summary, nil
}

func seedGenres(ctx context.Context, tx pgx.Tx, genres []seedGenre) (map[int]uuid.UUID, error) {
	ids := make(map[int]uuid.UUID, len(genres))
	rows := make([][]any, 0, len(genres))
	for _, g := range genres {
		id := uuid.New()
		ids[g.ID] = id
		rows = append(rows, []any{id, g.Name, g.Slug})
	}

	err := insertRows(ctx, tx, "genres", []string{"id", "name", "slug"}, rows)
	return ids, err
}

func seedSellers(ctx context.Context, tx pgx.Tx, sellers []seedSeller) (map[int]uuid.UUID, error) {
	ids := make(map[int]uuid.UUID, len(sellers))
	rows := make([][]any, 0, len(sellers))
	for _, s := range sellers {
		id := uuid.New()
		ids[s.ID] = id
		rating := s.Rating * 2 // the dump rates sellers 0 to 5, our column is 0 to 10
		rows = append(rows, []any{id, s.Name, s.LogoURL, rating})
	}

	err := insertRows(ctx, tx, "sellers", []string{"id", "name", "logo_url", "rating"}, rows)
	return ids, err
}

// Platforms are reference data inserted by a migration, so they are looked up, not seeded.
func loadPlatformIDs(ctx context.Context, tx pgx.Tx) (map[string]uuid.UUID, error) {
	rows, err := tx.Query(ctx, "SELECT slug, id FROM platforms")
	if err != nil {
		return nil, fmt.Errorf("load platforms: %w", err)
	}

	type platformRow struct {
		Slug string
		ID   uuid.UUID
	}
	platforms, err := pgx.CollectRows(rows, pgx.RowToStructByPos[platformRow])
	if err != nil {
		return nil, fmt.Errorf("load platforms: %w", err)
	}

	ids := make(map[string]uuid.UUID, len(platforms))
	for _, p := range platforms {
		ids[p.Slug] = p.ID
	}
	return ids, nil
}

func seedProducts(
	ctx context.Context,
	tx pgx.Tx,
	products []seedProduct,
	genreIDs map[int]uuid.UUID,
	platformIDs map[string]uuid.UUID,
) (map[int]uuid.UUID, int, error) {
	// The dump has no sales numbers, so sold_count is random. The fixed seed keeps the
	// numbers (and the home page) the same after every reseed.
	soldCounts := rand.New(rand.NewPCG(2027, 1))

	ids := make(map[int]uuid.UUID, len(products))
	productRows := make([][]any, 0, len(products))
	var genreRows [][]any
	for _, p := range products {
		id := uuid.New()
		ids[p.ID] = id

		releaseDate, err := parseDate(p.ReleaseDate)
		if err != nil {
			return nil, 0, fmt.Errorf("product %q: %w", p.Slug, err)
		}

		platformID, ok := platformIDs[p.Platform]
		if !ok {
			return nil, 0, fmt.Errorf("product %q: unknown platform %q", p.Slug, p.Platform)
		}

		productRows = append(productRows, []any{
			id, p.Title, p.Slug, p.Description, p.ImageURL, p.Type, platformID, p.Region,
			p.Developer, p.Publisher, releaseDate, p.BasePriceCents, soldCounts.IntN(5000),
		})

		for _, genreID := range p.GenreIDs {
			genreUUID, ok := genreIDs[genreID]
			if !ok {
				return nil, 0, fmt.Errorf("product %q: unknown genre id %d", p.Slug, genreID)
			}
			genreRows = append(genreRows, []any{id, genreUUID})
		}
	}

	productColumns := []string{
		"id", "title", "slug", "description", "image_url", "type", "platform_id", "region",
		"developer", "publisher", "release_date", "base_price_cents", "sold_count",
	}
	if err := insertRows(ctx, tx, "products", productColumns, productRows); err != nil {
		return nil, 0, err
	}
	if err := insertRows(ctx, tx, "product_genres", []string{"product_id", "genre_id"}, genreRows); err != nil {
		return nil, 0, err
	}

	return ids, len(genreRows), nil
}

func seedOffers(
	ctx context.Context,
	tx pgx.Tx,
	offers []seedOffer,
	productIDs map[int]uuid.UUID,
	sellerIDs map[int]uuid.UUID,
	keyCipher *services.KeyCipher,
) (int, int, error) {
	offerRows := make([][]any, 0, len(offers))
	var keyRows [][]any
	usedCodes := make(map[string]bool) // code_hash is UNIQUE, so never reuse a code

	for i, o := range offers {
		productID, ok := productIDs[o.ProductID]
		if !ok {
			return 0, 0, fmt.Errorf("offer %d: unknown product id %d", i, o.ProductID)
		}
		sellerID, ok := sellerIDs[o.SellerID]
		if !ok {
			return 0, 0, fmt.Errorf("offer %d: unknown seller id %d", i, o.SellerID)
		}

		offerID := uuid.New()
		offerRows = append(offerRows, []any{
			offerID, productID, sellerID, o.PriceCents, o.Currency, o.IsActive,
		})

		// The dump's keys were encrypted by another app, so each offer gets new fake keys.
		for range o.KeyCount {
			code := generateKeyCode()
			for usedCodes[code] {
				code = generateKeyCode()
			}
			usedCodes[code] = true

			keyRows = append(keyRows, []any{offerID, keyCipher.Encrypt(code), services.HashKeyCode(code)})
		}
	}

	offerColumns := []string{
		"id", "product_id", "seller_id", "price_cents", "currency", "is_active",
	}
	if err := insertRows(ctx, tx, "offers", offerColumns, offerRows); err != nil {
		return 0, 0, err
	}
	if err := insertRows(ctx, tx, "digital_keys", []string{"offer_id", "code_encrypted", "code_hash"}, keyRows); err != nil {
		return 0, 0, err
	}

	return len(offerRows), len(keyRows), nil
}

// One INSERT with many rows is much faster than one per row; batches stay well under
// Postgres' limit of 65535 parameters.
func insertRows(ctx context.Context, tx pgx.Tx, table string, columns []string, rows [][]any) error {
	for start := 0; start < len(rows); start += batchSize {
		end := min(start+batchSize, len(rows))

		query := psql.Insert(table).Columns(columns...)
		for _, row := range rows[start:end] {
			query = query.Values(row...)
		}

		sql, args, err := query.ToSql()
		if err != nil {
			return fmt.Errorf("build insert into %s: %w", table, err)
		}
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return fmt.Errorf("insert into %s: %w", table, err)
		}
	}
	return nil
}

func parseDate(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, errors.New("missing release date")
	}

	t, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid release date %q: %w", value, err)
	}
	return t, nil
}
