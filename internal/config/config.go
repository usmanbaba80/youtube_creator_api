package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr            string
	DatabaseURL         string
	APIKeys             []string
	CORSAllowedOrigins  []string
	RateLimitRPM        int
	DefaultPageSize     int
	MaxPageSize         int
	MaxAllPageSize      int
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	ShutdownTimeout     time.Duration
	ReadyOnly           bool
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		HTTPAddr:           env("HTTP_ADDR", ":8080"),
		DatabaseURL:        normalizeDatabaseURL(env("DATABASE_URL", "")),
		APIKeys:            splitCSV(env("API_KEYS", "")),
		CORSAllowedOrigins: splitCSV(env("CORS_ALLOWED_ORIGINS", "")),
		RateLimitRPM:       envInt("RATE_LIMIT_RPM", 120),
		DefaultPageSize:    envInt("DEFAULT_PAGE_SIZE", 20),
		MaxPageSize:        envInt("MAX_PAGE_SIZE", 100),
		MaxAllPageSize:     envInt("MAX_ALL_PAGE_SIZE", 5000),
		ReadTimeout:        envDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:       envDuration("WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:        envDuration("IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:    envDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		ReadyOnly:          envBool("READY_ONLY", true),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.APIKeys) == 0 {
		return nil, fmt.Errorf("API_KEYS is required (comma-separated)")
	}
	if cfg.DefaultPageSize < 1 {
		cfg.DefaultPageSize = 20
	}
	if cfg.MaxPageSize < cfg.DefaultPageSize {
		cfg.MaxPageSize = cfg.DefaultPageSize
	}
	if cfg.MaxAllPageSize < cfg.MaxPageSize {
		cfg.MaxAllPageSize = cfg.MaxPageSize
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// normalizeDatabaseURL accepts scraper-style SQLAlchemy URLs.
func normalizeDatabaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	replacements := []struct{ old, neu string }{
		{"postgresql+psycopg2://", "postgres://"},
		{"postgresql+psycopg://", "postgres://"},
		{"postgres+psycopg2://", "postgres://"},
		{"postgresql://", "postgres://"},
	}
	for _, r := range replacements {
		if strings.HasPrefix(raw, r.old) {
			return r.neu + strings.TrimPrefix(raw, r.old)
		}
	}
	return raw
}

func splitCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
