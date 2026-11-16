package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds every setting the app reads from environment variables.
// It is loaded once at startup and passed to whatever needs it.
type Config struct {
	Env         string        // "development" or "production"
	Port        string        // HTTP port; Render sets PORT automatically
	DatabaseURL string        // Postgres connection string (Supabase pooler URL)
	JWTSecret   string        // key used to sign auth tokens
	JWTTTL      time.Duration // how long a token stays valid
	CORSOrigins []string      // frontend origins allowed to call the API
}

// Load reads the environment, applies defaults and fails fast
// if a required value is missing, so a misconfigured deploy never starts.
func Load() (*Config, error) {
	cfg := &Config{
		Env:         getEnv("APP_ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		CORSOrigins: splitCSV(getEnv("CORS_ORIGINS", "http://localhost:5173")),
	}

	ttl, err := time.ParseDuration(getEnv("JWT_TTL", "24h"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_TTL: %w", err)
	}
	cfg.JWTTTL = ttl

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
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

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
