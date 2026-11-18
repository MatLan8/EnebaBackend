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
	"eneba-backend/internal/requests"
	"eneba-backend/internal/responses"
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

// firstOrderItemTitle joins the title of each order's first item onto "orders o"
// as first_item.title. LATERAL lets the subquery use o.id from the outer row, so
// it runs once per order and LIMIT 1 keeps only the first item. It is a LEFT
// JOIN, so an order without items (a cancelled one) is still listed, with a NULL title.
const firstOrderItemTitle = `LEFT JOIN LATERAL (
	SELECT p.title
	FROM order_items oi
	JOIN offers f ON f.id = oi.offer_id
	JOIN products p ON p.id = f.product_id
	WHERE oi.order_id = o.id
	ORDER BY oi.created_at, oi.id
	LIMIT 1
) first_item ON true`

// orderListRow is one row of the ListByUser query. Its fields are in the same
// order as the selected columns, so pgx.RowToStructByPos can fill it.
type orderListRow struct {
	ID           uuid.UUID
	PublicID     string
	CreatedAt    time.Time
	Status       models.OrderStatus
	ProductTitle *string // NULL when the order has no items
	TotalCents   int
	Currency     string
}

func (row orderListRow) toResponse() responses.OrderListItemResponse {
	return responses.OrderListItemResponse{
		ID:           row.ID,
		PublicID:     row.PublicID,
		CreatedAt:    row.CreatedAt,
		Status:       row.Status,
		ProductTitle: row.ProductTitle,
		TotalCents:   row.TotalCents,
		Currency:     row.Currency,
	}
}

// ListByUser returns one page of the user's orders, newest first, plus the
// total number of orders matching params.
func (r *OrderRepository) ListByUser(ctx context.Context, userID uuid.UUID, params requests.OrderListParams) ([]responses.OrderListItemResponse, int, error) {
	where := orderListWhere(userID, params)

	total, err := r.countList(ctx, where)
	if err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	// Orders created in the same instant would come back in random order, so one
	// could appear on two pages; id makes the order unique.
	query, args, err := psql.
		Select(
			"o.id", "o.public_id", "o.created_at", "o.status",
			"first_item.title", "o.total_cents", "o.currency",
		).
		From("orders o").
		JoinClause(firstOrderItemTitle).
		Where(where).
		OrderBy("o.created_at DESC", "o.id DESC").
		Limit(uint64(params.Limit)).
		Offset(uint64(params.Offset())).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build list orders query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}

	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByPos[orderListRow])
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}

	// make(..., 0, n) so an empty page is [] in the JSON, not null.
	items := make([]responses.OrderListItemResponse, 0, len(listRows))
	for _, row := range listRows {
		items = append(items, row.toResponse())
	}

	return items, total, nil
}

// countList counts every order matching where, ignoring paging. The filters
// only use orders columns, so no join is needed.
func (r *OrderRepository) countList(ctx context.Context, where sq.And) (int, error) {
	query, args, err := psql.
		Select("COUNT(*)").
		From("orders o").
		Where(where).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count orders query: %w", err)
	}

	var total int
	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}

	return total, nil
}

// orderListWhere limits the list to the user's own orders and applies the
// optional public ID search. ILIKE finds the text anywhere in the public ID, in
// any case ("K3X" -> "o-k3x9q2a"); escapeLike makes % and _ match literally.
func orderListWhere(userID uuid.UUID, params requests.OrderListParams) sq.And {
	where := sq.And{
		sq.Eq{"o.user_id": userID},
	}

	if params.PublicID != "" {
		where = append(where, sq.ILike{"o.public_id": "%" + escapeLike(params.PublicID) + "%"})
	}

	return where
}

// orderDetailRow is the order part of GET /api/v1/orders/{id}, in the same
// order as the columns selected by getDetailRow.
type orderDetailRow struct {
	ID         uuid.UUID
	PublicID   string
	CreatedAt  time.Time
	PaidAt     *time.Time
	Status     models.OrderStatus
	TotalCents int
	Currency   string
}

// orderDetailItemRow is one row of listDetailItems. Its fields are in the same
// order as the selected columns, so pgx.RowToStructByPos can fill it.
type orderDetailItemRow struct {
	ID             uuid.UUID
	Title          string
	ImageURL       string
	UnitPriceCents int
	SellerName     string
}

// GetByUser returns the user's order with its items, or models.ErrNotFound when
// there is no such order or it belongs to someone else.
// It runs two simple queries, the order and then its items, instead of one big
// join, so an order without items (a cancelled one) needs no special case.
func (r *OrderRepository) GetByUser(ctx context.Context, userID, orderID uuid.UUID) (responses.OrderDetailResponse, error) {
	order, err := r.getDetailRow(ctx, userID, orderID)
	if err != nil {
		return responses.OrderDetailResponse{}, err
	}

	itemRows, err := r.listDetailItems(ctx, order.ID)
	if err != nil {
		return responses.OrderDetailResponse{}, err
	}

	// make(..., 0, n) so an order without items has [] in the JSON, not null.
	items := make([]responses.OrderDetailItemResponse, 0, len(itemRows))
	for _, row := range itemRows {
		items = append(items, responses.OrderDetailItemResponse{
			ID:             row.ID,
			Title:          row.Title,
			ImageURL:       row.ImageURL,
			UnitPriceCents: row.UnitPriceCents,
			SellerName:     row.SellerName,
		})
	}

	return responses.OrderDetailResponse{
		ID:          order.ID,
		PublicID:    order.PublicID,
		CreatedAt:   order.CreatedAt,
		PaidAt:      order.PaidAt,
		Status:      order.Status,
		SellerNames: responses.DistinctSellerNames(items),
		TotalCents:  order.TotalCents,
		Currency:    order.Currency,
		Items:       items,
	}, nil
}

// getDetailRow selects the order with this ID if it belongs to the user, or
// returns models.ErrNotFound. Checking user_id in the same WHERE means another
// user's order looks exactly like a missing one.
func (r *OrderRepository) getDetailRow(ctx context.Context, userID, orderID uuid.UUID) (orderDetailRow, error) {
	query, args, err := psql.
		Select("o.id", "o.public_id", "o.created_at", "o.paid_at", "o.status", "o.total_cents", "o.currency").
		From("orders o").
		Where(sq.Eq{"o.id": orderID, "o.user_id": userID}).
		ToSql()
	if err != nil {
		return orderDetailRow{}, fmt.Errorf("build select order: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return orderDetailRow{}, fmt.Errorf("select order: %w", err)
	}

	// CollectExactlyOneRow returns pgx.ErrNoRows when nothing matched.
	order, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[orderDetailRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orderDetailRow{}, models.ErrNotFound
		}
		return orderDetailRow{}, fmt.Errorf("select order: %w", err)
	}

	return order, nil
}

// listDetailItems returns the order's items with their product and seller, in
// the same order the order list uses to pick productTitle.
func (r *OrderRepository) listDetailItems(ctx context.Context, orderID uuid.UUID) ([]orderDetailItemRow, error) {
	query, args, err := psql.
		Select("oi.id", "p.title", "p.image_url", "oi.unit_price_cents", "s.name").
		From("order_items oi").
		Join("offers f ON f.id = oi.offer_id").
		Join("products p ON p.id = f.product_id").
		Join("sellers s ON s.id = f.seller_id").
		Where(sq.Eq{"oi.order_id": orderID}).
		OrderBy("oi.created_at", "oi.id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list order items: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}

	itemRows, err := pgx.CollectRows(rows, pgx.RowToStructByPos[orderDetailItemRow])
	if err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}

	return itemRows, nil
}

// GetByIDForUser returns the order with this ID if it belongs to the user, or
// models.ErrNotFound. Another user's order looks exactly like a missing one.
func (r *OrderRepository) GetByIDForUser(ctx context.Context, userID, orderID uuid.UUID) (models.Order, error) {
	query, args, err := psql.
		Select(orderColumns...).
		From("orders").
		Where(sq.Eq{"id": orderID, "user_id": userID}).
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

// OrderKeyRow is one key of an order with its product and seller. The code is
// still encrypted: the handler decrypts it, so plaintext codes never pass
// through the repository. Its fields are in the same order as the columns
// selected by ListKeys, so pgx.RowToStructByPos can fill it.
type OrderKeyRow struct {
	KeyID         uuid.UUID
	CodeEncrypted []byte
	Title         string
	ImageURL      string
	Type          models.ProductType
	Platform      models.Platform
	Region        models.Region
	SellerName    string
	SellerLogoURL *string
	SellerRating  float64
}

// ListKeys returns the keys of the order, in the same order as the order's items.
func (r *OrderRepository) ListKeys(ctx context.Context, orderID uuid.UUID) ([]OrderKeyRow, error) {
	query, args, err := psql.
		Select(
			"k.id", "k.code_encrypted",
			"p.title", "p.image_url", "p.type", "p.platform", "p.region",
			"s.name", "s.logo_url", "s.rating",
		).
		From("order_items oi").
		Join("digital_keys k ON k.id = oi.digital_key_id").
		Join("offers f ON f.id = oi.offer_id").
		Join("products p ON p.id = f.product_id").
		Join("sellers s ON s.id = f.seller_id").
		Where(sq.Eq{"oi.order_id": orderID}).
		OrderBy("oi.created_at", "oi.id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list order keys: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list order keys: %w", err)
	}

	keyRows, err := pgx.CollectRows(rows, pgx.RowToStructByPos[OrderKeyRow])
	if err != nil {
		return nil, fmt.Errorf("list order keys: %w", err)
	}

	return keyRows, nil
}
