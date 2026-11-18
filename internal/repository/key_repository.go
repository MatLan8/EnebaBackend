package repository

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"eneba-backend/internal/models"
	"eneba-backend/internal/requests"
	"eneba-backend/internal/responses"
)

// KeyRepository reads the keys users have bought (their key library).
type KeyRepository struct {
	db DBTX
}

func NewKeyRepository(db DBTX) *KeyRepository {
	return &KeyRepository{db: db}
}

// boughtKeys is the FROM part shared by List and countList: every order item
// with its order, offer, product and seller. One order item is one key.
const boughtKeys = `order_items oi
	JOIN orders o ON o.id = oi.order_id
	JOIN offers f ON f.id = oi.offer_id
	JOIN products p ON p.id = f.product_id
	JOIN sellers s ON s.id = f.seller_id`

// keyListRow is one row of the List query. Its fields are in the same order as
// the selected columns, so pgx.RowToStructByPos can fill it.
type keyListRow struct {
	ID              uuid.UUID
	OrderID         uuid.UUID
	PublicID        string
	PurchasedAt     time.Time // orders.paid_at, never NULL for a paid order
	SellerName      string
	ProductTitle    string
	ProductImageURL string
	PriceCents      int
}

func (row keyListRow) toResponse() responses.KeyListItemResponse {
	return responses.KeyListItemResponse{
		ID:              row.ID,
		OrderID:         row.OrderID,
		PublicID:        row.PublicID,
		PurchasedAt:     row.PurchasedAt,
		SellerName:      row.SellerName,
		ProductTitle:    row.ProductTitle,
		ProductImageURL: row.ProductImageURL,
		PriceCents:      row.PriceCents,
	}
}

// List returns one page of the keys the user bought in paid orders, newest
// purchase first, plus the total number of keys matching params.
// It never reads code_encrypted: the library only lists keys, it doesn't reveal them.
func (r *KeyRepository) List(ctx context.Context, userID uuid.UUID, params requests.KeyListParams) ([]responses.KeyListItemResponse, int, error) {
	where := keyListWhere(userID, params)

	total, err := r.countList(ctx, where)
	if err != nil {
		return nil, 0, fmt.Errorf("count keys: %w", err)
	}

	// Keys of the same order share paid_at, so they are ordered like the order's
	// items; oi.id makes the order unique, so no key appears on two pages.
	query, args, err := psql.
		Select(
			"oi.digital_key_id", "o.id", "o.public_id", "o.paid_at",
			"s.name", "p.title", "p.image_url", "oi.unit_price_cents",
		).
		From(boughtKeys).
		Where(where).
		OrderBy("o.paid_at DESC", "oi.created_at", "oi.id").
		Limit(uint64(params.Limit)).
		Offset(uint64(params.Offset())).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build list keys query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list keys: %w", err)
	}

	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByPos[keyListRow])
	if err != nil {
		return nil, 0, fmt.Errorf("list keys: %w", err)
	}

	// make(..., 0, n) so an empty page is [] in the JSON, not null.
	items := make([]responses.KeyListItemResponse, 0, len(listRows))
	for _, row := range listRows {
		items = append(items, row.toResponse())
	}

	return items, total, nil
}

// countList counts every key matching where, ignoring paging.
func (r *KeyRepository) countList(ctx context.Context, where sq.And) (int, error) {
	query, args, err := psql.
		Select("COUNT(*)").
		From(boughtKeys).
		Where(where).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count keys query: %w", err)
	}

	var total int
	if err := r.db.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}

	return total, nil
}

// keyListWhere limits the list to the user's paid orders and applies the
// optional search: ILIKE finds the text anywhere in the product title or the
// order's public ID, in any case ("elden" -> "Elden Ring", "K3X" -> "o-k3x9q2a").
// escapeLike makes % and _ match literally.
func keyListWhere(userID uuid.UUID, params requests.KeyListParams) sq.And {
	where := sq.And{
		sq.Eq{"o.user_id": userID},
		sq.Eq{"o.status": models.OrderStatusPaid},
	}

	if params.Search != "" {
		pattern := "%" + escapeLike(params.Search) + "%"
		where = append(where, sq.Or{
			sq.ILike{"p.title": pattern},
			sq.ILike{"o.public_id": pattern},
		})
	}

	return where
}
