package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"
)

// HS256 needs a secret of at least 256 bits.
const minJWTSecretLength = 32

type Config struct {
	Env         string // "development" or "production"
	Port        string // Render sets PORT automatically
	DatabaseURL string // Supabase pooler URL
	CORSOrigins []string

	JWTSecret       string
	JWTIssuer       string
	JWTAudience     string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	KeyEncryptionSecret []byte // 32 bytes (AES-256)

	StripeSecretKey     string // only the API needs it; see ValidateStripe
	StripeWebhookSecret string
	FrontendURL         string // no trailing slash
}

// Load fails fast on missing or invalid values, so a misconfigured deploy never starts.
func Load() (*Config, error) {
	cfg := &Config{
		Env:         getEnv("APP_ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		CORSOrigins: splitCSV(getEnv("CORS_ORIGINS", "http://localhost:5173")),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		JWTIssuer:   getEnv("JWT_ISSUER", "eneba-backend"),
		JWTAudience: getEnv("JWT_AUDIENCE", "eneba-frontend"),

		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		FrontendURL:         strings.TrimRight(getEnv("FRONTEND_URL", "http://localhost:5173"), "/"),
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

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

// ValidateStripe is separate from Load because the seed command doesn't need Stripe.
// Outside production only sk_test_ keys are accepted, so local runs never charge real cards.
func (c *Config) ValidateStripe() error {
	if c.StripeSecretKey == "" {
		return fmt.Errorf("missing required env var: STRIPE_SECRET_KEY")
	}
	if !c.IsProduction() && !strings.HasPrefix(c.StripeSecretKey, "sk_test_") {
		return fmt.Errorf("STRIPE_SECRET_KEY must be a test key (sk_test_...) outside production")
	}
	if c.IsProduction() && c.StripeWebhookSecret == "" {
		return fmt.Errorf("missing required env var: STRIPE_WEBHOOK_SECRET")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

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
