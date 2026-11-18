package repository

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"eneba-backend/internal/models"
)

// keyReservationMinutes is how long checkout holds keys for a buyer. It is a bit
// longer than the Stripe session (31 minutes), so a buyer who pays at the last
// second still has their keys when the payment webhook arrives.
const keyReservationMinutes = 35

type OrderRepository struct {
	db DBTX
}

func NewOrderRepository(db DBTX) *OrderRepository {
	return &OrderRepository{db: db}
}

// WithTx returns a copy of the repository that runs its queries inside tx.
func (r *OrderRepository) WithTx(tx pgx.Tx) *OrderRepository {
	return &OrderRepository{db: tx}
}

// CheckoutLine is one cart item with everything checkout needs from its offer and product.
type CheckoutLine struct {
	OfferID    uuid.UUID
	Quantity   int
	PriceCents int // the offer's price right now
	Currency   string
	IsActive   bool
	ProductID  uuid.UUID
	Title      string
	ImageURL   string
}

// ListCheckoutLines returns the cart's items with their live offer prices, in
// the same order as the cart shows them.
// The offers are only read, not locked: the cart's add-item locks the offer and
// then the cart, so locking them here in the other order could deadlock.
func (r *OrderRepository) ListCheckoutLines(ctx context.Context, cartID uuid.UUID) ([]CheckoutLine, error) {
	query, args, err := psql.
		Select(
			"ci.offer_id", "ci.quantity", "o.price_cents", "o.currency", "o.is_active",
			"p.id", "p.title", "p.image_url",
		).
		From("cart_items ci").
		Join("offers o ON o.id = ci.offer_id").
		Join("products p ON p.id = o.product_id").
		Where(sq.Eq{"ci.cart_id": cartID}).
		OrderBy("ci.created_at", "ci.offer_id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select checkout lines: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select checkout lines: %w", err)
	}

	return pgx.CollectRows(rows, pgx.RowToStructByPos[CheckoutLine])
}

// LockAvailableKeys locks up to limit available keys of the offer, oldest first,
// and returns their IDs. Fewer than limit means there isn't enough stock.
//
// FOR UPDATE SKIP LOCKED: keys another checkout has locked right now are skipped
// instead of waited for, so two buyers of the same offer never get the same key
// and never block each other. Keys that are still in an order (e.g. a pending
// order whose reservation ran out) are skipped too, because order_items allows
// each key only once.
func (r *OrderRepository) LockAvailableKeys(ctx context.Context, offerID uuid.UUID, limit int) ([]uuid.UUID, error) {
	query, args, err := psql.
		Select("k.id").
		From("digital_keys k").
		Where(sq.Eq{"k.offer_id": offerID}).
		Where(availableKeyCondition).
		Where("NOT EXISTS (SELECT 1 FROM order_items oi WHERE oi.digital_key_id = k.id)").
		OrderBy("k.created_at", "k.id").
		Limit(uint64(limit)).
		Suffix("FOR UPDATE OF k SKIP LOCKED").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build lock available keys: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lock available keys: %w", err)
	}

	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// ReserveKeys holds the keys for a checkout for keyReservationMinutes. The time
// comes from the database clock, the same clock availableKeyCondition compares with.
func (r *OrderRepository) ReserveKeys(ctx context.Context, keyIDs []uuid.UUID) error {
	query, args, err := psql.
		Update("digital_keys").
		Set("reserved_until", sq.Expr("now() + make_interval(mins => ?)", keyReservationMinutes)).
		Where(sq.Eq{"id": keyIDs}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build reserve keys: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("reserve keys: %w", err)
	}

	return nil
}

// NewOrder holds the values of an order being created at checkout.
type NewOrder struct {
	PublicID   string
	UserID     uuid.UUID
	CartID     uuid.UUID
	TotalCents int
	Currency   string
}

// CreateOrder inserts a pending order and returns its ID. ok is false if the
// public ID is already taken; the caller then tries again with a new one.
// ON CONFLICT DO NOTHING is used instead of catching the unique violation,
// because any error inside a Postgres transaction aborts the whole transaction.
func (r *OrderRepository) CreateOrder(ctx context.Context, order NewOrder) (uuid.UUID, bool, error) {
	query, args, err := psql.
		Insert("orders").
		Columns("public_id", "user_id", "cart_id", "status", "total_cents", "currency").
		Values(order.PublicID, order.UserID, order.CartID, models.OrderStatusPending, order.TotalCents, order.Currency).
		Suffix("ON CONFLICT (public_id) DO NOTHING RETURNING id").
		ToSql()
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("build insert order: %w", err)
	}

	var orderID uuid.UUID
	if err := r.db.QueryRow(ctx, query, args...).Scan(&orderID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil // public ID taken, nothing inserted
		}
		return uuid.Nil, false, fmt.Errorf("insert order: %w", err)
	}

	return orderID, true, nil
}

// NewOrderItem is one key bought in an order, with the price it was sold at.
type NewOrderItem struct {
	OfferID        uuid.UUID
	DigitalKeyID   uuid.UUID
	UnitPriceCents int
}

// CreateOrderItems inserts one order_items row per key, all in one statement.
func (r *OrderRepository) CreateOrderItems(ctx context.Context, orderID uuid.UUID, items []NewOrderItem) error {
	insert := psql.
		Insert("order_items").
		Columns("order_id", "offer_id", "digital_key_id", "unit_price_cents")
	for _, item := range items {
		insert = insert.Values(orderID, item.OfferID, item.DigitalKeyID, item.UnitPriceCents)
	}

	query, args, err := insert.ToSql()
	if err != nil {
		return fmt.Errorf("build insert order items: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert order items: %w", err)
	}

	return nil
}

// SetStripeSession saves the ID of the Stripe Checkout session paying for the order.
func (r *OrderRepository) SetStripeSession(ctx context.Context, orderID uuid.UUID, sessionID string) error {
	query, args, err := psql.
		Update("orders").
		Set("stripe_session_id", sessionID).
		Where(sq.Eq{"id": orderID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build set stripe session: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("set stripe session: %w", err)
	}

	return nil
}

// orderColumns is the column list of GetForUpdate, in models.Order's field order.
var orderColumns = []string{
	"id", "public_id", "user_id", "status", "total_cents", "currency",
	"stripe_session_id", "created_at", "paid_at", "cart_id",
}

// GetForUpdate finds the order by its Stripe session ID, or by orderID when the
// session ID was never saved (see the checkout handler), and locks it until the
// transaction ends, so two deliveries of the same webhook run one after the other.
// Returns models.ErrNotFound if no order matches.
func (r *OrderRepository) GetForUpdate(ctx context.Context, sessionID string, orderID uuid.UUID) (models.Order, error) {
	query, args, err := psql.
		Select(orderColumns...).
		From("orders").
		Where(sq.Or{sq.Eq{"stripe_session_id": sessionID}, sq.Eq{"id": orderID}}).
		Limit(1).
		Suffix("FOR UPDATE").
		ToSql()
	if err != nil {
		return models.Order{}, fmt.Errorf("build select order: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return models.Order{}, fmt.Errorf("select order: %w", err)
	}

	order, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[models.Order])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Order{}, models.ErrNotFound
		}
		return models.Order{}, fmt.Errorf("select order: %w", err)
	}

	return order, nil
}

// orderKeyIDs selects the IDs of the keys in an order.
const orderKeyIDs = "SELECT oi.digital_key_id FROM order_items oi WHERE oi.order_id = ?"

// MarkPaid sets the order to paid. The session ID is filled in if checkout
// couldn't save it.
func (r *OrderRepository) MarkPaid(ctx context.Context, orderID uuid.UUID, sessionID string) error {
	query, args, err := psql.
		Update("orders").
		Set("status", models.OrderStatusPaid).
		Set("paid_at", sq.Expr("now()")).
		Set("stripe_session_id", sq.Expr("COALESCE(stripe_session_id, ?)", sessionID)).
		Where(sq.Eq{"id": orderID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build mark order paid: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark order paid: %w", err)
	}

	return nil
}

// MarkKeysSold sells every key of the order: sold_at is set and the
// reservation is no longer needed.
func (r *OrderRepository) MarkKeysSold(ctx context.Context, orderID uuid.UUID) error {
	query, args, err := psql.
		Update("digital_keys").
		Set("sold_at", sq.Expr("now()")).
		Set("reserved_until", nil).
		Where(sq.Expr("id IN ("+orderKeyIDs+")", orderID)).
		ToSql()
	if err != nil {
		return fmt.Errorf("build mark keys sold: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark keys sold: %w", err)
	}

	return nil
}

// IncrementSoldCounts adds the order's keys to products.sold_count, the number
// behind "best sellers" and the popularity sort. One key is one sale.
func (r *OrderRepository) IncrementSoldCounts(ctx context.Context, orderID uuid.UUID) error {
	const query = `UPDATE products p
SET sold_count = p.sold_count + sold.keys
FROM (
	SELECT o.product_id, COUNT(*) AS keys
	FROM order_items oi
	JOIN offers o ON o.id = oi.offer_id
	WHERE oi.order_id = $1
	GROUP BY o.product_id
) sold
WHERE p.id = sold.product_id`

	if _, err := r.db.Exec(ctx, query, orderID); err != nil {
		return fmt.Errorf("increment sold counts: %w", err)
	}

	return nil
}

// Cancel sets the order to cancelled, releases its keys and deletes its items.
// The items have to go: order_items.digital_key_id is UNIQUE, so a key that
// stays in a cancelled order could never be bought again.
func (r *OrderRepository) Cancel(ctx context.Context, orderID uuid.UUID) error {
	release, args, err := psql.
		Update("digital_keys").
		Set("reserved_until", nil).
		Where(sq.Expr("id IN ("+orderKeyIDs+")", orderID)).
		Where(sq.Eq{"sold_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build release keys: %w", err)
	}
	if _, err := r.db.Exec(ctx, release, args...); err != nil {
		return fmt.Errorf("release keys: %w", err)
	}

	deleteItems, args, err := psql.
		Delete("order_items").
		Where(sq.Eq{"order_id": orderID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete order items: %w", err)
	}
	if _, err := r.db.Exec(ctx, deleteItems, args...); err != nil {
		return fmt.Errorf("delete order items: %w", err)
	}

	cancel, args, err := psql.
		Update("orders").
		Set("status", models.OrderStatusCancelled).
		Where(sq.Eq{"id": orderID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build cancel order: %w", err)
	}
	if _, err := r.db.Exec(ctx, cancel, args...); err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}

	return nil
}
