package models

import "github.com/google/uuid"

// Where a key is redeemed, e.g. Steam. The rows are inserted by migrations/000011_platforms.
type Platform struct {
	ID      uuid.UUID `json:"id"`
	Slug    string    `json:"slug"`
	Name    string    `json:"name"`
	IconURL string    `json:"iconUrl"`
}
