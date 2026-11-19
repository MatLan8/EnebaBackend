package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"eneba-backend/internal/models"
	"eneba-backend/internal/requests"
	"eneba-backend/internal/responses"
)

type ProductRepository struct {
	db DBTX
}

func NewProductRepository(db DBTX) *ProductRepository {
	return &ProductRepository{db: db}
}

// Keeps exactly the products cheapestBuyableOffer keeps, so the filter counts add up to the store list's total.
var hasBuyableOffer = sq.Expr("EXISTS (SELECT 1 FROM offers o WHERE o.product_id = p.id AND o.is_active AND " + availableKeyExists + ")")

// Only buyable products are counted (the ones the store list shows), so no filter leads to an empty page.
func (r *ProductRepository) GetFilters(ctx context.Context) (responses.ProductFiltersResponse, error) {
	types, err := r.countProductsBy(ctx, "p.type")
	if err != nil {
		return responses.ProductFiltersResponse{}, fmt.Errorf("count products by type: %w", err)
	}

	platforms, err := r.countProductsByPlatform(ctx)
	if err != nil {
		return responses.ProductFiltersResponse{}, fmt.Errorf("count products by platform: %w", err)
	}

	regions, err := r.countProductsBy(ctx, "p.region")
	if err != nil {
		return responses.ProductFiltersResponse{}, fmt.Errorf("count products by region: %w", err)
	}

	genres, err := r.countProductsByGenre(ctx)
	if err != nil {
		return responses.ProductFiltersResponse{}, fmt.Errorf("count products by genre: %w", err)
	}

	return responses.ProductFiltersResponse{
		Types:     types,
		Platforms: platforms,
		Regions:   regions,
		Genres:    genres,
	}, nil
}

// column goes into the SQL as-is, so it must always be a constant from this file, never user input.
func (r *ProductRepository) countProductsBy(ctx context.Context, column string) ([]responses.FilterOption, error) {
	query, args, err := psql.
		Select(column, "COUNT(*)").
		From("products p").
		Where(hasBuyableOffer).
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

	// RowToStructByPos fills by column order (column -> Value, COUNT(*) -> Count); json tags don't matter.
	return pgx.CollectRows(rows, pgx.RowToStructByPos[responses.FilterOption])
}

// Columns are selected in LabeledFilterOption's field order: slug -> Value, name -> Label.
func (r *ProductRepository) countProductsByPlatform(ctx context.Context) ([]responses.LabeledFilterOption, error) {
	query, args, err := psql.
		Select("pl.slug", "pl.name", "COUNT(*)").
		From("products p").
		Join(joinPlatform).
		Where(hasBuyableOffer).
		GroupBy("pl.slug", "pl.name").
		OrderBy("COUNT(*) DESC", "pl.slug").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByPos[responses.LabeledFilterOption])
}

// Columns are selected in LabeledFilterOption's field order: slug -> Value, name -> Label.
func (r *ProductRepository) countProductsByGenre(ctx context.Context) ([]responses.LabeledFilterOption, error) {
	query, args, err := psql.
		Select("g.slug", "g.name", "COUNT(*)").
		From("genres g").
		Join("product_genres pg ON pg.genre_id = g.id").
		Join("products p ON p.id = pg.product_id").
		Where(hasBuyableOffer).
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

	return pgx.CollectRows(rows, pgx.RowToStructByPos[responses.LabeledFilterOption])
}

// The cheapest buyable offer is the first offer on the product page, so every list shows the price
// a buyer actually gets. It's an inner join, so products without a buyable offer (inactive, or all
// keys sold or reserved) drop out. o.id breaks price ties, so the same offer is picked every time.
const cheapestBuyableOffer = `JOIN LATERAL (
	SELECT o.id, o.price_cents, o.currency
	FROM offers o
	WHERE o.product_id = p.id AND o.is_active AND ` + availableKeyExists + `
	ORDER BY o.price_cents, o.id
	LIMIT 1
) co ON true`

// 0.5 lets "witcer" find "The Witcher 3" (0.57) without also matching "Monster Hunter" (0.43).
const searchSimilarityThreshold = 0.5

// Fixed ORDER BY strings, so user input never ends up in the SQL.
var productSortOrders = map[requests.ProductSort]string{
	requests.ProductSortPopular:   "p.sold_count DESC",
	requests.ProductSortPriceAsc:  "co.price_cents ASC",
	requests.ProductSortPriceDesc: "co.price_cents DESC",
	requests.ProductSortNewest:    "p.release_date DESC",
	requests.ProductSortOldest:    "p.release_date ASC",
	requests.ProductSortNameAsc:   "p.title ASC",
	requests.ProductSortNameDesc:  "p.title DESC",
}

// Fields follow the order of the selected columns (RowToStructByPos).
type productListRow struct {
	ID              uuid.UUID
	Slug            string
	Title           string
	ImageURL        string
	Type            models.ProductType
	PlatformSlug    string
	PlatformName    string
	PlatformIconURL string
	Region          models.Region
	CheapestOfferID uuid.UUID
	PriceCents      int
	BasePriceCents  int
	Currency        string
}

func (row productListRow) toResponse() responses.ProductListItemResponse {
	return responses.ProductListItemResponse{
		ID:              row.ID,
		Slug:            row.Slug,
		Title:           row.Title,
		ImageURL:        row.ImageURL,
		Type:            row.Type,
		Platform:        platformResponse(row.PlatformSlug, row.PlatformName, row.PlatformIconURL),
		Region:          row.Region,
		BasePriceCents:  row.BasePriceCents,
		Currency:        models.DefaultCurrency,
		Price:           responses.NewPriceResponse(row.PriceCents, row.BasePriceCents, row.Currency),
		CheapestOfferID: row.CheapestOfferID,
	}
}

func (r *ProductRepository) List(ctx context.Context, params requests.ProductListParams) ([]responses.ProductListItemResponse, int, error) {
	where := productListWhere(params)

	total, err := r.countList(ctx, where)
	if err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	query := psql.
		Select("p.id", "p.slug", "p.title", "p.image_url", "p.type").
		Columns(platformColumns...).
		Columns("p.region", "co.id", "co.price_cents", "p.base_price_cents", "co.currency").
		From("products p").
		Join(joinPlatform).
		JoinClause(cheapestBuyableOffer).
		Where(where).
		OrderBy(productSortOrders[params.Sort]).
		Limit(uint64(params.Limit)).
		Offset(uint64(params.Offset()))

	// title and id make the order unique, so no product appears on two pages;
	// name_desc already orders by title, so it only needs id.
	if params.Sort == requests.ProductSortNameDesc {
		query = query.OrderBy("p.id")
	} else {
		query = query.OrderBy("p.title", "p.id")
	}

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build list query: %w", err)
	}

	rows, err := r.db.Query(ctx, statement, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}

	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByPos[productListRow])
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}

	// make(..., 0, n) so an empty page is [] in the JSON, not null.
	items := make([]responses.ProductListItemResponse, 0, len(listRows))
	for _, row := range listRows {
		items = append(items, row.toResponse())
	}

	return items, total, nil
}

// Uses the same cheapestBuyableOffer join as List, so the total always matches the listed products.
func (r *ProductRepository) countList(ctx context.Context, where sq.And) (int, error) {
	query, args, err := psql.
		Select("COUNT(*)").
		From("products p").
		JoinClause(cheapestBuyableOffer).
		Where(where).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count query: %w", err)
	}

	var total int
	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}

	return total, nil
}

func productListWhere(params requests.ProductListParams) sq.And {
	where := sq.And{}

	if params.Search != "" {
		where = append(where, titleMatches(params.Search))
	}

	if len(params.Types) > 0 {
		where = append(where, sq.Eq{"p.type": params.Types})
	}
	if len(params.Platforms) > 0 {
		where = append(where, sq.Expr("p.platform_id IN (SELECT id FROM platforms WHERE slug = ANY(?))", params.Platforms))
	}
	if len(params.Regions) > 0 {
		where = append(where, sq.Eq{"p.region": params.Regions})
	}
	if len(params.Genres) > 0 {
		where = append(where, sq.Expr(
			`EXISTS (
				SELECT 1 FROM product_genres pg
				JOIN genres g ON g.id = pg.genre_id
				WHERE pg.product_id = p.id AND g.slug = ANY(?)
			)`,
			params.Genres,
		))
	}

	if params.MinPriceCents != nil {
		where = append(where, sq.GtOrEq{"co.price_cents": *params.MinPriceCents})
	}
	if params.MaxPriceCents != nil {
		where = append(where, sq.LtOrEq{"co.price_cents": *params.MaxPriceCents})
	}

	return where
}

// Escapes LIKE wildcards, so a search for "100%" looks for a real percent sign.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string {
	return likeEscaper.Replace(s)
}

// Shared by List and Suggest, so the dropdown finds the same products as the store page.
// word_similarity (pg_trgm) lets typos match ("eldn ring" -> "Elden Ring").
func titleMatches(search string) sq.Or {
	return sq.Or{
		sq.ILike{"p.title": "%" + escapeLike(search) + "%"},
		sq.Expr("word_similarity(?, p.title) >= ?", search, searchSimilarityThreshold),
	}
}

const maxSuggestions = 6

// Fields follow the order of the selected columns (RowToStructByPos).
type productSuggestionRow struct {
	Slug            string
	Title           string
	ImageURL        string
	Type            models.ProductType
	PlatformSlug    string
	PlatformName    string
	PlatformIconURL string
	PriceCents      int
	BasePriceCents  int
	Currency        string
}

func (row productSuggestionRow) toResponse() responses.ProductSuggestionResponse {
	return responses.ProductSuggestionResponse{
		Slug:           row.Slug,
		Title:          row.Title,
		ImageURL:       row.ImageURL,
		Platform:       platformResponse(row.PlatformSlug, row.PlatformName, row.PlatformIconURL),
		IsDLC:          row.Type == models.ProductTypeDLC,
		BasePriceCents: row.BasePriceCents,
		Currency:       models.DefaultCurrency,
		Price:          responses.NewPriceResponse(row.PriceCents, row.BasePriceCents, row.Currency),
	}
}

// Matches and prices like List, so the total equals the store page's total. Unlike List it orders
// by match quality, because with only a few rows the product being typed has to come first.
func (r *ProductRepository) Suggest(ctx context.Context, search string) (responses.ProductSuggestionsResponse, error) {
	where := sq.And{titleMatches(search)}

	total, err := r.countList(ctx, where)
	if err != nil {
		return responses.ProductSuggestionsResponse{}, fmt.Errorf("count suggestions: %w", err)
	}
	if total == 0 {
		return responses.NewProductSuggestionsResponse(nil, 0), nil
	}

	query, args, err := psql.
		Select("p.slug", "p.title", "p.image_url", "p.type").
		Columns(platformColumns...).
		Columns("co.price_cents", "p.base_price_cents", "co.currency").
		From("products p").
		Join(joinPlatform).
		JoinClause(cheapestBuyableOffer).
		Where(where).
		// Prefix matches first (true sorts after false, hence DESC), then the closest fuzzy matches,
		// then best sellers; title and id make the order unique.
		OrderByClause("p.title ILIKE ? DESC", escapeLike(search)+"%").
		OrderByClause("word_similarity(?, p.title) DESC", search).
		OrderBy("p.sold_count DESC", "p.title", "p.id").
		Limit(maxSuggestions).
		ToSql()
	if err != nil {
		return responses.ProductSuggestionsResponse{}, fmt.Errorf("build suggest query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return responses.ProductSuggestionsResponse{}, fmt.Errorf("suggest products: %w", err)
	}

	suggestionRows, err := pgx.CollectRows(rows, pgx.RowToStructByPos[productSuggestionRow])
	if err != nil {
		return responses.ProductSuggestionsResponse{}, fmt.Errorf("suggest products: %w", err)
	}

	items := make([]responses.ProductSuggestionResponse, 0, len(suggestionRows))
	for _, row := range suggestionRows {
		items = append(items, row.toResponse())
	}

	return responses.NewProductSuggestionsResponse(items, total), nil
}

// Three small queries instead of one big join, so each part stays easy to read.
func (r *ProductRepository) GetBySlug(ctx context.Context, slug string) (responses.ProductDetailResponse, error) {
	product, err := r.getDetailRow(ctx, slug)
	if err != nil {
		return responses.ProductDetailResponse{}, err
	}

	genres, err := r.listGenres(ctx, product.ID)
	if err != nil {
		return responses.ProductDetailResponse{}, fmt.Errorf("list product genres: %w", err)
	}

	offers, err := r.listBuyableOffers(ctx, product.ID)
	if err != nil {
		return responses.ProductDetailResponse{}, fmt.Errorf("list product offers: %w", err)
	}

	return product.toResponse(genres, offers), nil
}

// Fields follow the columns selected by getDetailRow.
type productDetailRow struct {
	ID              uuid.UUID
	Slug            string
	Title           string
	Description     string
	ImageURL        string
	Type            models.ProductType
	PlatformSlug    string
	PlatformName    string
	PlatformIconURL string
	Region          models.Region
	Developer       *string
	Publisher       *string
	ReleaseDate     time.Time // a DATE column, so the time is always midnight
	BasePriceCents  int
}

func (r *ProductRepository) getDetailRow(ctx context.Context, slug string) (productDetailRow, error) {
	query, args, err := psql.
		Select("p.id", "p.slug", "p.title", "p.description", "p.image_url", "p.type").
		Columns(platformColumns...).
		Columns("p.region", "p.developer", "p.publisher", "p.release_date", "p.base_price_cents").
		From("products p").
		Join(joinPlatform).
		Where(sq.Eq{"p.slug": slug}).
		ToSql()
	if err != nil {
		return productDetailRow{}, fmt.Errorf("build select product: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return productDetailRow{}, fmt.Errorf("select product: %w", err)
	}

	product, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[productDetailRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return productDetailRow{}, models.ErrNotFound
		}
		return productDetailRow{}, fmt.Errorf("select product: %w", err)
	}

	return product, nil
}

// Columns are selected in GenreResponse's field order.
func (r *ProductRepository) listGenres(ctx context.Context, productID uuid.UUID) ([]responses.GenreResponse, error) {
	query, args, err := psql.
		Select("g.slug", "g.name").
		From("genres g").
		Join("product_genres pg ON pg.genre_id = g.id").
		Where(sq.Eq{"pg.product_id": productID}).
		OrderBy("g.name").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	// CollectRows starts from an empty slice, so no genres is [] in the JSON, not null.
	return pgx.CollectRows(rows, pgx.RowToStructByPos[responses.GenreResponse])
}

// Fields follow the order of the selected columns (RowToStructByPos).
type offerRow struct {
	ID            uuid.UUID
	PriceCents    int
	Currency      string
	SellerID      uuid.UUID
	SellerName    string
	SellerLogoURL *string
	SellerRating  float64
}

// Cheapest first; equal prices go to the better rated seller, and o.id keeps the order stable.
func (r *ProductRepository) listBuyableOffers(ctx context.Context, productID uuid.UUID) ([]offerRow, error) {
	query, args, err := psql.
		Select(
			"o.id", "o.price_cents", "o.currency",
			"s.id", "s.name", "s.logo_url", "s.rating",
		).
		From("offers o").
		Join("sellers s ON s.id = o.seller_id").
		Where(sq.Eq{"o.product_id": productID, "o.is_active": true}).
		Where(hasAvailableKey).
		OrderBy("o.price_cents ASC", "s.rating DESC", "o.id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByPos[offerRow])
}

func (row productDetailRow) toResponse(genres []responses.GenreResponse, offers []offerRow) responses.ProductDetailResponse {
	// make(..., 0, n) so nothing to sell is [] offers, not null.
	offerResponses := make([]responses.OfferResponse, 0, len(offers))
	for _, offer := range offers {
		offerResponses = append(offerResponses, offer.toResponse(row.BasePriceCents))
	}

	return responses.ProductDetailResponse{
		ID:             row.ID,
		Slug:           row.Slug,
		Title:          row.Title,
		Description:    row.Description,
		ImageURL:       row.ImageURL,
		Type:           row.Type,
		Platform:       platformResponse(row.PlatformSlug, row.PlatformName, row.PlatformIconURL),
		Region:         row.Region,
		Developer:      row.Developer,
		Publisher:      row.Publisher,
		ReleaseDate:    row.ReleaseDate.Format(time.DateOnly),
		BasePriceCents: row.BasePriceCents,
		Currency:       models.DefaultCurrency,
		Genres:         genres,
		Offers:         offerResponses,
	}
}

func (row offerRow) toResponse(basePriceCents int) responses.OfferResponse {
	return responses.OfferResponse{
		ID:    row.ID,
		Price: responses.NewPriceResponse(row.PriceCents, basePriceCents, row.Currency),
		Seller: responses.SellerResponse{
			ID:      row.SellerID,
			Name:    row.SellerName,
			LogoURL: row.SellerLogoURL,
			Rating:  row.SellerRating,
		},
	}
}

func (r *ProductRepository) ListPopular(ctx context.Context, types []models.ProductType, limit int) ([]responses.ProductListItemResponse, error) {
	// Equal sold counts would come back in random order; title and id make it stable.
	query, args, err := psql.
		Select("p.id", "p.slug", "p.title", "p.image_url", "p.type").
		Columns(platformColumns...).
		Columns("p.region", "co.id", "co.price_cents", "p.base_price_cents", "co.currency").
		From("products p").
		Join(joinPlatform).
		JoinClause(cheapestBuyableOffer).
		Where(sq.Eq{"p.type": types}). // a slice becomes "IN (...)"
		OrderBy("p.sold_count DESC", "p.title", "p.id").
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build popular products query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list popular products: %w", err)
	}

	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByPos[productListRow])
	if err != nil {
		return nil, fmt.Errorf("list popular products: %w", err)
	}

	// make(..., 0, n) so no products is [] in the JSON, not null.
	items := make([]responses.ProductListItemResponse, 0, len(listRows))
	for _, row := range listRows {
		items = append(items, row.toResponse())
	}

	return items, nil
}
