package models

import (
	"time"

	"github.com/google/uuid"
)

// Seller mirrors a row in the sellers table.
type Seller struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	LogoURL      *string   `json:"logoUrl"`
	Rating       float64   `json:"rating"` // 0 to 10, NUMERIC(4,2) in the database
	ReviewsCount int       `json:"reviewsCount"`
	CreatedAt    time.Time `json:"createdAt"`
}
