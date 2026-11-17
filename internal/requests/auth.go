package requests

import "strings"

// RegisterRequest is the body of POST /api/v1/auth/register.
type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=50"`
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,min=8,max=72"` // bcrypt only uses the first 72 bytes
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required"`
}

// Normalize trims surrounding whitespace before validation.
// The password is left untouched on purpose: spaces can be part of it.
func (r *RegisterRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Email = strings.TrimSpace(r.Email)
}

func (r *LoginRequest) Normalize() {
	r.Email = strings.TrimSpace(r.Email)
}
