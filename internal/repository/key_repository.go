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

type KeyRepository struct {
	db DBTX
}

func NewKeyRepository(db DBTX) *KeyRepository {
	return &KeyRepository{db: db}
}

// One order item is one key.
const boughtKeys = `order_items oi
	JOIN orders o ON o.id = oi.order_id
	JOIN offers f ON f.id = oi.offer_id
	JOIN products p ON p.id = f.product_id
	JOIN ` + joinPlatform + `
	JOIN sellers s ON s.id = f.seller_id`

// Fields follow the order of the selected columns (RowToStructByPos).
type keyListRow struct {
	ID              uuid.UUID
	OrderID         uuid.UUID
	PublicID        string
	PurchasedAt     time.Time // never NULL for a paid order
	SellerName      string
	ProductTitle    string
	ProductImageURL string
	PlatformSlug    string
	PlatformName    string
	PlatformIconURL string
	PriceCents      int
	Currency        string
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
		Platform:        platformResponse(row.PlatformSlug, row.PlatformName, row.PlatformIconURL),
		PriceCents:      row.PriceCents,
		Currency:        row.Currency,
	}
}

// List never reads code_encrypted: the library only lists keys, it doesn't reveal them.
func (r *KeyRepository) List(ctx context.Context, userID uuid.UUID, params requests.KeyListParams) ([]responses.KeyListItemResponse, int, error) {
	where := keyListWhere(userID, params)

	total, err := r.countList(ctx, where)
	if err != nil {
		return nil, 0, fmt.Errorf("count keys: %w", err)
	}

	// oi.id makes the order unique, so no key appears on two pages.
	query, args, err := psql.
		Select("oi.digital_key_id", "o.id", "o.public_id", "o.paid_at", "s.name", "p.title", "p.image_url").
		Columns(platformColumns...).
		Columns("oi.unit_price_cents", "o.currency").
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

// The search matches the product title or the order's public ID; escapeLike makes % and _ literal.
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
