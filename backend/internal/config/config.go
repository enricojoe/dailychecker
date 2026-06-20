// Package config loads application configuration from environment variables,
// optionally seeded from a .env file.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the application.
type Config struct {
	AppEnv              string
	Port                string
	DatabaseURL         string
	JWTSecret           string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	Timezone            string
	DigestHour          int
	TelegramBotToken    string
	TelegramBotUsername string
	AppPublicURL        string

	// CORS — A1 (M9)
	// CORSAllowedOrigins is the list of origins permitted for cross-origin
	// requests. Populated from CORS_ALLOWED_ORIGINS (comma-separated);
	// defaults to ["http://localhost:5173"].
	CORSAllowedOrigins []string

	// Telegram webhook mode — A3 (M9)
	// TelegramMode selects the update delivery strategy: "polling" (default)
	// or "webhook". Ignored when TelegramBotToken is empty.
	TelegramMode string
	// TelegramWebhookURL is the public base URL the server is reachable at,
	// e.g. "https://example.com". The webhook path "/api/telegram/webhook"
	// is appended automatically. Only required when TelegramMode="webhook".
	TelegramWebhookURL string
	// TelegramWebhookSecret is the shared secret validated in the
	// X-Telegram-Bot-Api-Secret-Token header on every incoming webhook
	// delivery. Never logged.
	TelegramWebhookSecret string
}

// Load reads configuration from envFile (if the file exists) and then from
// real environment variables. Real env vars take precedence over the file.
func Load(envFile string) (*Config, error) {
	// Silently ignore a missing .env file; production relies on real env vars.
	_ = godotenv.Load(envFile)

	cfg := &Config{
		AppEnv:              getEnv("APP_ENV", "development"),
		Port:                getEnv("PORT", "8080"),
		Timezone:            getEnv("TIMEZONE", "Asia/Jakarta"),
		TelegramBotToken:    os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramBotUsername: os.Getenv("TELEGRAM_BOT_USERNAME"),
		AppPublicURL:        os.Getenv("APP_PUBLIC_URL"),
		CORSAllowedOrigins:  parseCORSOrigins(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		TelegramMode:        getEnv("TELEGRAM_MODE", "polling"),
		TelegramWebhookURL:  os.Getenv("TELEGRAM_WEBHOOK_URL"),
		TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
	}

	var err error

	if cfg.DatabaseURL, err = requireEnv("DATABASE_URL"); err != nil {
		return nil, err
	}
	if cfg.JWTSecret, err = requireEnv("JWT_SECRET"); err != nil {
		return nil, err
	}

	if cfg.AccessTokenTTL, err = parseDuration("ACCESS_TOKEN_TTL", "15m"); err != nil {
		return nil, err
	}
	if cfg.RefreshTokenTTL, err = parseDuration("REFRESH_TOKEN_TTL", "720h"); err != nil {
		return nil, err
	}
	if cfg.DigestHour, err = parseInt("DIGEST_HOUR", 22); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("config: required environment variable %q is not set", key)
	}
	return v, nil
}

func parseDuration(key, fallback string) (time.Duration, error) {
	raw := getEnv(key, fallback)
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s=%q is not a valid duration: %w", key, raw, err)
	}
	return d, nil
}

func parseInt(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s=%q is not a valid integer: %w", key, raw, err)
	}
	return v, nil
}

// parseCORSOrigins splits a comma-separated origins string, trims each entry,
// and discards empty entries.  e.g. "http://localhost:5173,https://app.example.com"
// → []string{"http://localhost:5173", "https://app.example.com"}.
func parseCORSOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}
