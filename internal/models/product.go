package models

import (
	"time"

	"github.com/google/uuid"
)

// BasePriceCents is the official price (or a gift card's face value) that offer discounts are measured against.
type Product struct {
	ID             uuid.UUID   `json:"id"`
	Title          string      `json:"title"`
	Slug           string      `json:"slug"`
	Description    string      `json:"description"`
	ImageURL       string      `json:"imageUrl"`
	Type           ProductType `json:"type"`
	Platform       Platform    `json:"platform"`
	Region         Region      `json:"region"`
	Developer      *string     `json:"developer"`
	Publisher      *string     `json:"publisher"`
	ReleaseDate    time.Time   `json:"releaseDate"`
	BasePriceCents int         `json:"basePriceCents"`
	SoldCount      int         `json:"soldCount"` // increased when an order is paid
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}
