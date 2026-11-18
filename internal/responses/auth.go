package responses

import (
	"time"

	"github.com/google/uuid"

	"eneba-backend/internal/models"
)

// Deliberately has no password field.
type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewUserResponse(u *models.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
}

// The refresh token is not here: it travels only in the HttpOnly cookie.
type AuthResponse struct {
	AccessToken string       `json:"accessToken"`
	TokenType   string       `json:"tokenType"`
	ExpiresAt   time.Time    `json:"expiresAt"`
	User        UserResponse `json:"user"`
}
