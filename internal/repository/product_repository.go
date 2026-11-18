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

// hasActiveOffer limits a query on "products p" to products that can be bought,
// i.e. products with at least one active offer.
var hasActiveOffer = sq.Expr("EXISTS (SELECT 1 FROM offers o WHERE o.product_id = p.id AND o.is_active)")

// GetFilters returns every value the store can be filtered by, with the number
// of products that have it, plus the highest active offer price.
// Only products with an active offer are counted, so no filter leads to an empty page.
// The rows are read straight into the response types, since this query only exists
// for GET /api/v1/products/filters.
func (r *ProductRepository) GetFilters(ctx context.Context) (responses.ProductFiltersResponse, error) {
	types, err := r.countProductsBy(ctx, "p.type")
	if err != nil {
		return responses.ProductFiltersResponse{}, fmt.Errorf("count products by type: %w", err)
	}

	platforms, err := r.countProductsBy(ctx, "p.platform")
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

	maxPriceCents, err := r.maxActivePrice(ctx)
	if err != nil {
		return responses.ProductFiltersResponse{}, fmt.Errorf("select max price: %w", err)
	}

	return responses.ProductFiltersResponse{
		Types:     types,
		Platforms: platforms,
		Regions:   regions,
		Genres:    genres,
		Price:     responses.NewPriceRange(maxPriceCents),
	}, nil
}

// countProductsBy groups products by column and counts each value, most common first.
// column goes into the SQL as-is, so it must always be a constant from this file, never user input.
func (r *ProductRepository) countProductsBy(ctx context.Context, column string) ([]responses.FilterOption, error) {
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
	// The json tags don't matter here, only the field order.
	return pgx.CollectRows(rows, pgx.RowToStructByPos[responses.FilterOption])
}

// countProductsByGenre counts products per genre, most common first.
// The columns are selected in GenreFilterOption's field order: slug -> Value, name -> Label.
func (r *ProductRepository) countProductsByGenre(ctx context.Context) ([]responses.GenreFilterOption, error) {
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

	return pgx.CollectRows(rows, pgx.RowToStructByPos[responses.GenreFilterOption])
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

// cheapestActiveOffer joins every product to its single cheapest active offer as "co".
// Only the offer's own price is needed; the discount compares it with p.base_price_cents.
// LATERAL lets the subquery use p.id from the outer query, so it runs once per product.
// It's an inner JOIN, so products without an active offer drop out of the list.
// o.id breaks ties between equal prices, so the same offer is picked every time.
const cheapestActiveOffer = `JOIN LATERAL (
	SELECT o.price_cents, o.currency
	FROM offers o
	WHERE o.product_id = p.id AND o.is_active
	ORDER BY o.price_cents, o.id
	LIMIT 1
) co ON true`

// searchSimilarityThreshold is how close (0 to 1) the search text must be to a
// word sequence in the title to count as a match. 0.5 lets "witcer" find
// "The Witcher 3" (0.57) without also matching "Monster Hunter" (0.43).
const searchSimilarityThreshold = 0.5

// productSortOrders maps each sort option to its ORDER BY clause. The clauses are
// fixed strings, so user input never ends up in the SQL.
var productSortOrders = map[requests.ProductSort]string{
	requests.ProductSortPopular:   "p.sold_count DESC",
	requests.ProductSortPriceAsc:  "co.price_cents ASC",
	requests.ProductSortPriceDesc: "co.price_cents DESC",
	requests.ProductSortNewest:    "p.release_date DESC",
	requests.ProductSortOldest:    "p.release_date ASC",
	requests.ProductSortNameAsc:   "p.title ASC",
	requests.ProductSortNameDesc:  "p.title DESC",
}

// productListRow is one row of the List query. Its fields are in the same order
// as the selected columns, so pgx.RowToStructByPos can fill it.
type productListRow struct {
	ID             uuid.UUID
	Slug           string
	Title          string
	ImageURL       string
	Type           models.ProductType
	Platform       models.Platform
	Region         models.Region
	PriceCents     int // the cheapest active offer
	BasePriceCents int // the product's official price, for the discount
	Currency       string
}

// toResponse turns the flat row into a product card with a nested price,
// discounted against the product's base price.
func (row productListRow) toResponse() responses.ProductListItemResponse {
	return responses.ProductListItemResponse{
		ID:             row.ID,
		Slug:           row.Slug,
		Title:          row.Title,
		ImageURL:       row.ImageURL,
		Type:           row.Type,
		Platform:       row.Platform,
		Region:         row.Region,
		BasePriceCents: row.BasePriceCents,
		Price:          responses.NewPriceResponse(row.PriceCents, row.BasePriceCents, row.Currency),
	}
}

// List returns one page of the store's products matching params as product
// cards, each priced by its cheapest active offer, plus the total number of matches.
func (r *ProductRepository) List(ctx context.Context, params requests.ProductListParams) ([]responses.ProductListItemResponse, int, error) {
	where := productListWhere(params)

	total, err := r.countList(ctx, where)
	if err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	query := psql.
		Select(
			"p.id", "p.slug", "p.title", "p.image_url", "p.type", "p.platform", "p.region",
			"co.price_cents", "p.base_price_cents", "co.currency",
		).
		From("products p").
		JoinClause(cheapestActiveOffer).
		Where(where).
		OrderBy(productSortOrders[params.Sort]).
		Limit(uint64(params.Limit)).
		Offset(uint64(params.Offset()))

	// Equal values (e.g. two products at the same price) would come back in random
	// order, so a product could appear on two pages. title and id make the order unique.
	// name_desc already orders by title, so only id is needed as a tie-breaker.
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

// countList counts every product matching where, ignoring paging.
// It joins the cheapest offer too, so the price filters work the same as in List.
func (r *ProductRepository) countList(ctx context.Context, where sq.And) (int, error) {
	query, args, err := psql.
		Select("COUNT(*)").
		From("products p").
		JoinClause(cheapestActiveOffer).
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

// productListWhere turns params into WHERE conditions, all joined with AND.
// Values always go in as ? placeholders, never into the SQL text.
func productListWhere(params requests.ProductListParams) sq.And {
	where := sq.And{}

	if params.Search != "" {
		// ILIKE finds the text anywhere in the title ("elden" -> "Elden Ring").
		// word_similarity (pg_trgm) compares letter triplets, so typos still match
		// ("eldn ring" -> "Elden Ring", "witcer" -> "The Witcher 3").
		where = append(where, sq.Or{
			sq.ILike{"p.title": "%" + escapeLike(params.Search) + "%"},
			sq.Expr("word_similarity(?, p.title) >= ?", params.Search, searchSimilarityThreshold),
		})
	}

	// Squirrel turns sq.Eq with a slice into "IN (...)": any of the values matches.
	if len(params.Types) > 0 {
		where = append(where, sq.Eq{"p.type": params.Types})
	}
	if len(params.Platforms) > 0 {
		where = append(where, sq.Eq{"p.platform": params.Platforms})
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

// likeEscaper puts a backslash before the characters that are special in LIKE
// patterns, so a search for "100%" looks for a real percent sign.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string {
	return likeEscaper.Replace(s)
}

// GetBySlug returns the product page for slug: the product, its genres and every
// offer that can be bought right now, cheapest first. It runs three small queries
// instead of one big join, so each part stays easy to read.
// It returns models.ErrNotFound if no product has this slug.
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

// productDetailRow is the product part of the product page, in the same order
// as the columns selected by getDetailRow.
type productDetailRow struct {
	ID             uuid.UUID
	Slug           string
	Title          string
	Description    string
	ImageURL       string
	Type           models.ProductType
	Platform       models.Platform
	Region         models.Region
	Developer      *string
	Publisher      *string
	ReleaseDate    time.Time // a DATE column, so the time is always midnight
	BasePriceCents int
}

// getDetailRow selects the product with this slug, or returns models.ErrNotFound.
func (r *ProductRepository) getDetailRow(ctx context.Context, slug string) (productDetailRow, error) {
	query, args, err := psql.
		Select(
			"p.id", "p.slug", "p.title", "p.description", "p.image_url", "p.type", "p.platform", "p.region",
			"p.developer", "p.publisher", "p.release_date", "p.base_price_cents",
		).
		From("products p").
		Where(sq.Eq{"p.slug": slug}).
		ToSql()
	if err != nil {
		return productDetailRow{}, fmt.Errorf("build select product: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return productDetailRow{}, fmt.Errorf("select product: %w", err)
	}

	// CollectExactlyOneRow returns pgx.ErrNoRows when nothing matched.
	product, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[productDetailRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return productDetailRow{}, models.ErrNotFound
		}
		return productDetailRow{}, fmt.Errorf("select product: %w", err)
	}

	return product, nil
}

// listGenres returns the product's genres sorted by name. The columns are
// selected in GenreResponse's field order: slug -> Slug, name -> Name.
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

// hasUnsoldKey limits a query on "offers o" to offers that still have at least
// one digital key left to sell. An offer without keys can't be bought.
var hasUnsoldKey = sq.Expr("EXISTS (SELECT 1 FROM digital_keys k WHERE k.offer_id = o.id AND k.order_item_id IS NULL)")

// offerRow is one row of the listBuyableOffers query: an offer with its seller,
// in the same order as the selected columns.
type offerRow struct {
	ID            uuid.UUID
	PriceCents    int
	Currency      string
	SellerID      uuid.UUID
	SellerName    string
	SellerLogoURL *string
	SellerRating  float64 // NUMERIC(4,2) in the database
}

// listBuyableOffers returns the product's active offers that still have unsold
// keys, cheapest first. Equal prices go to the better rated seller first, and
// o.id makes the order the same on every request.
func (r *ProductRepository) listBuyableOffers(ctx context.Context, productID uuid.UUID) ([]offerRow, error) {
	query, args, err := psql.
		Select(
			"o.id", "o.price_cents", "o.currency",
			"s.id", "s.name", "s.logo_url", "s.rating",
		).
		From("offers o").
		Join("sellers s ON s.id = o.seller_id").
		Where(sq.Eq{"o.product_id": productID, "o.is_active": true}).
		Where(hasUnsoldKey).
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

// toResponse builds the product page from the product row, its genres and its
// offers. Each offer's discount is measured against the product's base price.
func (row productDetailRow) toResponse(genres []responses.GenreResponse, offers []offerRow) responses.ProductDetailResponse {
	// make(..., 0, n) so a product with nothing to sell has [] offers, not null.
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
		Platform:       row.Platform,
		Region:         row.Region,
		Developer:      row.Developer,
		Publisher:      row.Publisher,
		ReleaseDate:    row.ReleaseDate.Format(time.DateOnly), // "2022-02-25"
		BasePriceCents: row.BasePriceCents,
		Genres:         genres,
		Offers:         offerResponses,
	}
}

// toResponse turns the flat offer row into an offer with a nested price and seller.
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
