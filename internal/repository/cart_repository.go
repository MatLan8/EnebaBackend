package repository

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"eneba-backend/internal/models"
	"eneba-backend/internal/responses"
)

type CartRepository struct {
	db DBTX
}

func NewCartRepository(db DBTX) *CartRepository {
	return &CartRepository{db: db}
}

func (r *CartRepository) WithTx(tx pgx.Tx) *CartRepository {
	return &CartRepository{db: tx}
}

// Fields follow the order of the selected columns (RowToStructByPos).
type cartItemRow struct {
	OfferID           uuid.UUID
	Quantity          int
	AvailableQuantity int
	PriceCents        int
	Currency          string
	Slug              string
	Title             string
	ImageURL          string
	Type              models.ProductType
	SellerName        string
	SellerLogoURL     *string
	IsActive          bool
}

// A deactivated offer can't be bought, so none of its keys count as available.
func (row cartItemRow) toResponse() responses.CartItemResponse {
	available := row.AvailableQuantity
	if !row.IsActive {
		available = 0
	}

	return responses.CartItemResponse{
		OfferID:           row.OfferID,
		Quantity:          row.Quantity,
		AvailableQuantity: available,
		Available:         row.Quantity <= available,
		PriceCents:        row.PriceCents,
		Currency:          row.Currency,
		Product: responses.CartProductResponse{
			Slug:     row.Slug,
			Title:    row.Title,
			ImageURL: row.ImageURL,
			IsDLC:    row.Type == models.ProductTypeDLC,
		},
		Seller: responses.CartSellerResponse{
			Name:    row.SellerName,
			LogoURL: row.SellerLogoURL,
		},
	}
}

// Every offer is in EUR today, so the currency comes from the first item; an empty cart gets the default.
func newCartResponse(rows []cartItemRow) responses.CartResponse {
	items := make([]responses.CartItemResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toResponse())
	}

	currency := ""
	if len(rows) > 0 {
		currency = rows[0].Currency
	}

	return responses.NewCartResponse(items, currency)
}

// Prices and stock are read live from the offers. An unknown cart ID gives an empty cart.
func (r *CartRepository) GetCart(ctx context.Context, cartID uuid.UUID) (responses.CartResponse, error) {
	query, args, err := psql.
		Select("ci.offer_id", "ci.quantity").
		Column(sq.Expr(availableForCartCount, cartID)).
		Columns(
			"o.price_cents", "o.currency",
			"p.slug", "p.title", "p.image_url", "p.type",
			"s.name", "s.logo_url", "o.is_active",
		).
		From("cart_items ci").
		Join("offers o ON o.id = ci.offer_id").
		Join("products p ON p.id = o.product_id").
		Join("sellers s ON s.id = o.seller_id").
		Where(sq.Eq{"ci.cart_id": cartID}).
		OrderBy("ci.created_at", "ci.offer_id").
		ToSql()
	if err != nil {
		return responses.CartResponse{}, fmt.Errorf("build select cart: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return responses.CartResponse{}, fmt.Errorf("select cart: %w", err)
	}

	cartRows, err := pgx.CollectRows(rows, pgx.RowToStructByPos[cartItemRow])
	if err != nil {
		return responses.CartResponse{}, fmt.Errorf("select cart: %w", err)
	}

	return newCartResponse(cartRows), nil
}

func (r *CartRepository) CreateCart(ctx context.Context) (uuid.UUID, error) {
	// squirrel can't build DEFAULT VALUES, so the SQL is written out.
	const query = "INSERT INTO carts DEFAULT VALUES RETURNING id"

	var cartID uuid.UUID
	if err := r.db.QueryRow(ctx, query).Scan(&cartID); err != nil {
		return uuid.Nil, fmt.Errorf("insert cart: %w", err)
	}

	return cartID, nil
}

// LockCart locks the cart row until the transaction ends, so a cart being changed can't be deleted meanwhile.
func (r *CartRepository) LockCart(ctx context.Context, cartID uuid.UUID) (bool, error) {
	query, args, err := psql.
		Select("id").
		From("carts").
		Where(sq.Eq{"id": cartID}).
		Suffix("FOR UPDATE").
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build lock cart: %w", err)
	}

	var id uuid.UUID
	if err := r.db.QueryRow(ctx, query, args...).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("lock cart: %w", err)
	}

	return true, nil
}

func (r *CartRepository) Touch(ctx context.Context, cartID uuid.UUID) error {
	query, args, err := psql.
		Update("carts").
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"id": cartID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build touch cart: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("touch cart: %w", err)
	}

	return nil
}

// Locking the offer makes two adds of it run one after the other, so the stock checks can't both
// pass on stale numbers. Lock order in every cart change is offer, cart, then cart item, to avoid deadlocks.
func (r *CartRepository) LockActiveOffer(ctx context.Context, offerID uuid.UUID) error {
	query, args, err := psql.
		Select("id").
		From("offers").
		Where(sq.Eq{"id": offerID, "is_active": true}).
		Suffix("FOR UPDATE").
		ToSql()
	if err != nil {
		return fmt.Errorf("build lock offer: %w", err)
	}

	var id uuid.UUID
	if err := r.db.QueryRow(ctx, query, args...).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ErrNotFound
		}
		return fmt.Errorf("lock offer: %w", err)
	}

	return nil
}

func (r *CartRepository) GetQuantityForUpdate(ctx context.Context, cartID, offerID uuid.UUID) (int, error) {
	query, args, err := psql.
		Select("quantity").
		From("cart_items").
		Where(sq.Eq{"cart_id": cartID, "offer_id": offerID}).
		Suffix("FOR UPDATE").
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build select cart quantity: %w", err)
	}

	var quantity int
	if err := r.db.QueryRow(ctx, query, args...).Scan(&quantity); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, models.ErrNotFound
		}
		return 0, fmt.Errorf("select cart quantity: %w", err)
	}

	return quantity, nil
}

func (r *CartRepository) CountAvailableKeys(ctx context.Context, cartID, offerID uuid.UUID) (int, error) {
	query, args, err := psql.
		Select("COUNT(*)").
		From("digital_keys k").
		Where(sq.Eq{"k.offer_id": offerID}).
		Where(sq.Expr(availableForCartCondition, cartID)).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count available keys: %w", err)
	}

	var count int
	if err := r.db.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count available keys: %w", err)
	}

	return count, nil
}

// The caller checks the limit and the stock first.
func (r *CartRepository) AddItem(ctx context.Context, cartID, offerID uuid.UUID) error {
	query, args, err := psql.
		Insert("cart_items").
		Columns("cart_id", "offer_id", "quantity").
		Values(cartID, offerID, 1).
		Suffix("ON CONFLICT (cart_id, offer_id) DO UPDATE SET quantity = cart_items.quantity + 1").
		ToSql()
	if err != nil {
		return fmt.Errorf("build add cart item: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("add cart item: %w", err)
	}

	return nil
}

func (r *CartRepository) SetQuantity(ctx context.Context, cartID, offerID uuid.UUID, quantity int) error {
	query, args, err := psql.
		Update("cart_items").
		Set("quantity", quantity).
		Where(sq.Eq{"cart_id": cartID, "offer_id": offerID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build set cart quantity: %w", err)
	}

	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("set cart quantity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}

	return nil
}

func (r *CartRepository) RemoveItem(ctx context.Context, cartID, offerID uuid.UUID) error {
	query, args, err := psql.
		Delete("cart_items").
		Where(sq.Eq{"cart_id": cartID, "offer_id": offerID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build remove cart item: %w", err)
	}

	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("remove cart item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}

	return nil
}

// Clear keeps the cart itself, so the browser's cookie stays valid.
func (r *CartRepository) Clear(ctx context.Context, cartID uuid.UUID) error {
	query, args, err := psql.
		Delete("cart_items").
		Where(sq.Eq{"cart_id": cartID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build clear cart: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("clear cart: %w", err)
	}

	return nil
}
