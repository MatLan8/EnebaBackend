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

// WithTx returns a copy of the repository that runs its queries inside tx.
func (r *CartRepository) WithTx(tx pgx.Tx) *CartRepository {
	return &CartRepository{db: tx}
}

// unsoldKeyCount counts the unsold digital keys of offer "o", i.e. its stock.
const unsoldKeyCount = "(SELECT COUNT(*) FROM digital_keys k WHERE k.offer_id = o.id AND k.order_item_id IS NULL)"

// cartItemRow is one row of the GetCart query, in the same order as the selected columns.
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
}

func (row cartItemRow) toResponse() responses.CartItemResponse {
	return responses.CartItemResponse{
		OfferID:           row.OfferID,
		Quantity:          row.Quantity,
		AvailableQuantity: row.AvailableQuantity,
		PriceCents:        row.PriceCents,
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

// newCartResponse maps the rows to the cart. Every offer is in EUR today, so the
// cart's currency is taken from the first item; an empty cart gets the default.
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

// GetCart returns the cart's items, oldest first. Prices and stock are read live
// from the offers. A cart with no items, or a cart ID that doesn't exist, gives an empty cart.
func (r *CartRepository) GetCart(ctx context.Context, cartID uuid.UUID) (responses.CartResponse, error) {
	query, args, err := psql.
		Select(
			"ci.offer_id", "ci.quantity", unsoldKeyCount, "o.price_cents", "o.currency",
			"p.slug", "p.title", "p.image_url", "p.type",
			"s.name", "s.logo_url",
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

// CreateCart inserts a new, empty cart and returns its ID. The ID is what the
// browser keeps in the cart_token cookie.
func (r *CartRepository) CreateCart(ctx context.Context) (uuid.UUID, error) {
	// Every column has a default, and squirrel can't build DEFAULT VALUES, so the SQL is written out.
	const query = "INSERT INTO carts DEFAULT VALUES RETURNING id"

	var cartID uuid.UUID
	if err := r.db.QueryRow(ctx, query).Scan(&cartID); err != nil {
		return uuid.Nil, fmt.Errorf("insert cart: %w", err)
	}

	return cartID, nil
}

// LockCart reports whether the cart exists and, if it does, locks its row until
// the transaction ends, so a cart that is being changed can't be deleted meanwhile.
// It must be called on a repository from WithTx.
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

// Touch sets the cart's updated_at to now. Call it whenever the cart's items change.
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

// LockActiveOffer checks that the offer exists and is active, and locks its row
// until the transaction ends. Two adds of the same offer then run one after the
// other, so the quantity and stock checks can't both pass on stale numbers.
// Lock order in every cart change: offer, then cart, then cart item, so two
// requests never wait on each other in a circle.
// It must be called on a repository from WithTx. Returns models.ErrNotFound if
// the offer doesn't exist or isn't active.
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

// GetQuantityForUpdate returns how many keys of the offer are in the cart and
// locks that cart_items row until the transaction ends.
// Returns models.ErrNotFound if the offer isn't in the cart, or there is no such cart.
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

// CountUnsoldKeys returns the offer's stock: how many of its keys are not sold yet.
func (r *CartRepository) CountUnsoldKeys(ctx context.Context, offerID uuid.UUID) (int, error) {
	query, args, err := psql.
		Select("COUNT(*)").
		From("digital_keys").
		Where(sq.Eq{"offer_id": offerID, "order_item_id": nil}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count unsold keys: %w", err)
	}

	var count int
	if err := r.db.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unsold keys: %w", err)
	}

	return count, nil
}

// AddItem puts one key of the offer in the cart: a new row with quantity 1,
// or one more on the existing row. The caller checks the limit and the stock first.
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

// SetQuantity replaces the quantity of an offer in the cart.
// Returns models.ErrNotFound if the offer isn't in the cart.
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

// RemoveItem takes the offer out of the cart.
// Returns models.ErrNotFound if the offer isn't in the cart.
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

// Clear removes every item from the cart but keeps the cart itself, so the
// browser's cookie stays valid. An empty or unknown cart is not an error.
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
