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

// Name is our own custom claim on top of the standard ones.
type AccessClaims struct {
	Name string `json:"name"`
	jwt.RegisteredClaims
}

func (c *AccessClaims) UserID() (uuid.UUID, error) {
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: invalid subject", models.ErrInvalidToken)
	}
	return id, nil
}

// Raw goes into the user's cookie, Hash into the database.
type IssuedRefreshToken struct {
	Raw       string
	Hash      string
	ExpiresAt time.Time
}

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

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

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

func (s *TokenService) NewRefreshToken() IssuedRefreshToken {
	// About 130 bits of entropy from the OS secure random generator.
	raw := rand.Text()

	return IssuedRefreshToken{
		Raw:       raw,
		Hash:      HashRefreshToken(raw),
		ExpiresAt: s.now().Add(s.refreshTTL),
	}
}

func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

type IssuedTokens struct {
	AccessToken          string
	AccessTokenExpiresAt time.Time
	RefreshToken         IssuedRefreshToken
}

// Satisfied by *repository.RefreshTokenRepository, with or without a transaction.
type RefreshTokenCreator interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
}

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
