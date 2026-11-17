package models

import "errors"

// Domain errors shared by repositories, services and handlers.
// They describe what went wrong in business terms; httpx.Error turns
// each of them into the matching HTTP status code.
var (
	ErrNotFound           = errors.New("not found")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrUnauthorized       = errors.New("authentication required")
)
