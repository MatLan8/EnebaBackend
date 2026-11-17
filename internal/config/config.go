package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"
)

// minJWTSecretLength is the shortest secret we accept for HS256 (32 bytes = 256 bits).
const minJWTSecretLength = 32

// Config holds every setting the app reads from environment variables.
// It is loaded once at startup and passed to whatever needs it.
type Config struct {
	Env         string   // "development" or "production"
	Port        string   // HTTP port; Render sets PORT automatically
	DatabaseURL string   // Postgres connection string (Supabase pooler URL)
	CORSOrigins []string // frontend origins allowed to call the API

	JWTSecret       string        // key used to sign access tokens
	JWTIssuer       string        // "iss" claim: who created the token
	JWTAudience     string        // "aud" claim: who the token is meant for
	AccessTokenTTL  time.Duration // lifetime of an access token (short)
	RefreshTokenTTL time.Duration // lifetime of a refresh token (long)

	KeyEncryptionSecret []byte // 32 bytes used to encrypt digital keys (AES-256)
}

// Load reads the environment, applies defaults and fails fast
// if a value is missing or invalid, so a misconfigured deploy never starts.
func Load() (*Config, error) {
	cfg := &Config{
		Env:         getEnv("APP_ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		CORSOrigins: splitCSV(getEnv("CORS_ORIGINS", "http://localhost:5173")),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		JWTIssuer:   getEnv("JWT_ISSUER", "eneba-backend"),
		JWTAudience: getEnv("JWT_AUDIENCE", "eneba-frontend"),
	}

	var err error
	if cfg.AccessTokenTTL, err = getDuration("JWT_ACCESS_TTL", 15*time.Minute); err != nil {
		return nil, err
	}
	if cfg.RefreshTokenTTL, err = getDuration("JWT_REFRESH_TTL", 7*24*time.Hour); err != nil {
		return nil, err
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if os.Getenv("KEY_ENCRYPTION_SECRET") == "" {
		missing = append(missing, "KEY_ENCRYPTION_SECRET")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	if len(cfg.JWTSecret) < minJWTSecretLength {
		return nil, fmt.Errorf("JWT_SECRET must be at least %d characters", minJWTSecretLength)
	}

	// The secret is stored as base64 because it is 32 random bytes, not text.
	cfg.KeyEncryptionSecret, err = base64.StdEncoding.DecodeString(os.Getenv("KEY_ENCRYPTION_SECRET"))
	if err != nil || len(cfg.KeyEncryptionSecret) != 32 {
		return nil, fmt.Errorf("KEY_ENCRYPTION_SECRET must be 32 bytes encoded as base64")
	}

	return cfg, nil
}

// IsProduction reports whether the app runs in production mode.
func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// getDuration parses values like "15m", "24h" or "168h".
func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := getEnv(key, "")
	if v == "" {
		return fallback, nil
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return d, nil
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
