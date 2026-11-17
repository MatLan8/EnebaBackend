package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"eneba-backend/internal/config"
	"eneba-backend/internal/models"
)

func newTestTokenService() *TokenService {
	return NewTokenService(&config.Config{
		JWTSecret:       "test-secret-that-is-at-least-32-bytes-long",
		JWTIssuer:       "test-issuer",
		JWTAudience:     "test-audience",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	})
}

func TestAccessTokenRoundTrip(t *testing.T) {
	s := newTestTokenService()
	user := &models.User{ID: uuid.New(), Name: "Matas"}

	token, _, err := s.NewAccessToken(user)
	if err != nil {
		t.Fatalf("NewAccessToken: %v", err)
	}

	claims, err := s.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}

	id, err := claims.UserID()
	if err != nil || id != user.ID {
		t.Errorf("UserID = %v, %v; want %v", id, err, user.ID)
	}
	if claims.Name != user.Name {
		t.Errorf("Name = %q; want %q", claims.Name, user.Name)
	}
	if claims.ID == "" {
		t.Error("jti claim is empty")
	}
}

func TestAccessTokenRejected(t *testing.T) {
	user := &models.User{ID: uuid.New(), Name: "Matas"}
	validator := newTestTokenService()

	expired := newTestTokenService()
	expired.now = func() time.Time {
		return time.Now().Add(-time.Hour) // issued an hour ago, 15 minute lifetime
	}

	wrongSecret := newTestTokenService()
	wrongSecret.secret = []byte("another-secret-that-is-32-bytes-long!!")

	wrongAudience := newTestTokenService()
	wrongAudience.audience = "someone-else"

	tests := []struct {
		name   string
		issuer *TokenService
	}{
		{"expired", expired},
		{"wrong secret", wrongSecret},
		{"wrong audience", wrongAudience},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, _, err := tt.issuer.NewAccessToken(user)
			if err != nil {
				t.Fatalf("NewAccessToken: %v", err)
			}

			_, err = validator.ParseAccessToken(token)
			if !errors.Is(err, models.ErrInvalidToken) {
				t.Errorf("err = %v; want ErrInvalidToken", err)
			}
		})
	}

	t.Run("garbage", func(t *testing.T) {
		_, err := validator.ParseAccessToken("not-a-jwt")
		if !errors.Is(err, models.ErrInvalidToken) {
			t.Errorf("err = %v; want ErrInvalidToken", err)
		}
	})
}

func TestRefreshToken(t *testing.T) {
	s := newTestTokenService()

	a := s.NewRefreshToken()
	b := s.NewRefreshToken()

	if a.Raw == b.Raw {
		t.Error("two refresh tokens are identical")
	}
	if a.Hash != HashRefreshToken(a.Raw) {
		t.Error("hash does not match HashRefreshToken(raw)")
	}
	if len(a.Hash) != 64 {
		t.Errorf("hash length = %d; want 64 hex chars", len(a.Hash))
	}
	if !a.ExpiresAt.After(time.Now().Add(6 * 24 * time.Hour)) {
		t.Errorf("ExpiresAt = %v; want about 7 days from now", a.ExpiresAt)
	}
}
