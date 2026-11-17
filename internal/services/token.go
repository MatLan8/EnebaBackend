package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"eneba-backend/internal/config"
	"eneba-backend/internal/models"
)

// AccessClaims is the payload of an access token.
// Embedding jwt.RegisteredClaims adds the standard fields
// (iss, sub, aud, exp, iat, jti); Name is our own custom claim.
type AccessClaims struct {
	Name string `json:"name"`
	jwt.RegisteredClaims
}

// UserID returns the user ID stored in the "sub" claim.
func (c *AccessClaims) UserID() (uuid.UUID, error) {
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: invalid subject", models.ErrInvalidToken)
	}
	return id, nil
}

// IssuedRefreshToken is a freshly generated refresh token.
// Raw goes into the user's cookie, Hash goes into the database.
type IssuedRefreshToken struct {
	Raw       string
	Hash      string
	ExpiresAt time.Time
}

// TokenService creates and validates access tokens and generates refresh tokens.
type TokenService struct {
	secret     []byte
	issuer     string
	audience   string
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time // replaceable in tests
}

func NewTokenService(cfg *config.Config) *TokenService {
	return &TokenService{
		secret:     []byte(cfg.JWTSecret),
		issuer:     cfg.JWTIssuer,
		audience:   cfg.JWTAudience,
		accessTTL:  cfg.AccessTokenTTL,
		refreshTTL: cfg.RefreshTokenTTL,
		now:        time.Now,
	}
}

// NewAccessToken signs a short-lived JWT for the user and returns it with its expiry time.
func (s *TokenService) NewAccessToken(user *models.User) (string, time.Time, error) {
	now := s.now()
	expiresAt := now.Add(s.accessTTL)

	claims := AccessClaims{
		Name: user.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   user.ID.String(),
			Audience:  jwt.ClaimStrings{s.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}

	// The header ({"alg":"HS256","typ":"JWT"}) is added by the library.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

// ParseAccessToken verifies the signature, algorithm, issuer, audience and
// expiry of an access token and returns its claims.
// Any problem is reported as models.ErrInvalidToken.
func (s *TokenService) ParseAccessToken(tokenString string) (*AccessClaims, error) {
	claims := &AccessClaims{}

	keyFunc := func(t *jwt.Token) (any, error) {
		return s.secret, nil
	}

	_, err := jwt.ParseWithClaims(tokenString, claims, keyFunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), // blocks "alg":"none" and algorithm swapping
		jwt.WithIssuer(s.issuer),
		jwt.WithAudience(s.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", models.ErrInvalidToken, err)
	}

	return claims, nil
}

// NewRefreshToken generates a random, unguessable refresh token.
func (s *TokenService) NewRefreshToken() IssuedRefreshToken {
	// rand.Text returns 26 random base32 characters (about 130 bits of entropy)
	// from the operating system's secure random generator.
	raw := rand.Text()

	return IssuedRefreshToken{
		Raw:       raw,
		Hash:      HashRefreshToken(raw),
		ExpiresAt: s.now().Add(s.refreshTTL),
	}
}

// HashRefreshToken returns the SHA-256 hex digest of a raw refresh token.
// The same input always gives the same hash, so it can be looked up in the database.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// IssuedTokens is everything a client gets when it signs in.
type IssuedTokens struct {
	AccessToken          string
	AccessTokenExpiresAt time.Time
	RefreshToken         IssuedRefreshToken
}

// RefreshTokenCreator is the one repository method IssueTokens needs.
// *repository.RefreshTokenRepository satisfies it, with or without a transaction.
type RefreshTokenCreator interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
}

// IssueTokens creates an access token and a refresh token for the user and
// stores the refresh token's hash. Used by register, login and refresh.
func (s *TokenService) IssueTokens(ctx context.Context, store RefreshTokenCreator, user *models.User) (*IssuedTokens, error) {
	accessToken, accessExpiresAt, err := s.NewAccessToken(user)
	if err != nil {
		return nil, err
	}

	refresh := s.NewRefreshToken()
	if err := store.Create(ctx, user.ID, refresh.Hash, refresh.ExpiresAt); err != nil {
		return nil, err
	}

	return &IssuedTokens{
		AccessToken:          accessToken,
		AccessTokenExpiresAt: accessExpiresAt,
		RefreshToken:         refresh,
	}, nil
}
