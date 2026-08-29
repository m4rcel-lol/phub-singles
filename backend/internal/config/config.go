// Package config loads runtime configuration from the environment.
//
// Every knob has a production-safe default; nothing is read from disk and no
// secret is ever compiled into the binary.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully resolved configuration for one server process.
type Config struct {
	Addr            string        // TCP address the HTTP server listens on.
	DatabasePath    string        // SQLite file path.
	UploadDir       string        // Directory holding uploaded avatars.
	MaxUploadBytes  int64         // Hard limit for a single avatar upload.
	SessionTTL      time.Duration // Lifetime of an admin session cookie.
	ViewWindow      time.Duration // How long one visitor's page view is de-duplicated.
	SecureCookie    bool          // Set the Secure flag on the session cookie.
	CORSOrigins     []string      // Extra origins allowed to call the API.
	TrustedProxy    bool          // Honour X-Forwarded-For for client IPs.
	AdminUsername   string        // Bootstrap admin username.
	AdminPassword   string        // Bootstrap admin password (first run only).
	ForcePassword   bool          // Reset the admin password on every start.
	PublicURL       string        // Canonical public URL, used for logging/CORS.
	LogLevel        string        // debug | info | warn | error
	LogFormat       string        // json | text
	ShutdownTimeout time.Duration // Grace period for in-flight requests.
}

// Load reads the PHS_* environment variables and validates them.
func Load() (Config, error) {
	cfg := Config{
		Addr:            env("PHS_ADDR", ":8080"),
		DatabasePath:    env("PHS_DB_PATH", "./data/phs.db"),
		UploadDir:       env("PHS_UPLOAD_DIR", "./data/uploads"),
		MaxUploadBytes:  envInt64("PHS_MAX_UPLOAD_BYTES", 2<<20),
		SessionTTL:      envDuration("PHS_SESSION_TTL", 24*time.Hour),
		ViewWindow:      envDuration("PHS_VIEW_WINDOW", 12*time.Hour),
		SecureCookie:    envBool("PHS_SECURE_COOKIE", true),
		CORSOrigins:     envList("PHS_CORS_ORIGINS"),
		TrustedProxy:    envBool("PHS_TRUST_PROXY", true),
		AdminUsername:   env("PHS_ADMIN_USERNAME", "admin"),
		AdminPassword:   os.Getenv("PHS_ADMIN_PASSWORD"),
		ForcePassword:   envBool("PHS_ADMIN_PASSWORD_RESET", false),
		PublicURL:       strings.TrimRight(env("PHS_PUBLIC_URL", "http://localhost:8080"), "/"),
		LogLevel:        env("PHS_LOG_LEVEL", "info"),
		LogFormat:       env("PHS_LOG_FORMAT", "json"),
		ShutdownTimeout: envDuration("PHS_SHUTDOWN_TIMEOUT", 15*time.Second),
	}

	if cfg.AdminUsername == "" {
		return cfg, fmt.Errorf("PHS_ADMIN_USERNAME must not be empty")
	}
	if cfg.AdminPassword != "" && len(cfg.AdminPassword) < 8 {
		return cfg, fmt.Errorf("PHS_ADMIN_PASSWORD must be at least 8 characters")
	}
	if cfg.ForcePassword && cfg.AdminPassword == "" {
		return cfg, fmt.Errorf("PHS_ADMIN_PASSWORD_RESET requires PHS_ADMIN_PASSWORD")
	}
	if cfg.MaxUploadBytes <= 0 {
		return cfg, fmt.Errorf("PHS_MAX_UPLOAD_BYTES must be positive")
	}
	if cfg.SessionTTL <= 0 {
		return cfg, fmt.Errorf("PHS_SESSION_TTL must be positive")
	}
	if cfg.ViewWindow <= 0 {
		return cfg, fmt.Errorf("PHS_VIEW_WINDOW must be positive")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v, err := strconv.ParseBool(env(key, ""))
	if err != nil {
		return fallback
	}
	return v
}

func envInt64(key string, fallback int64) int64 {
	v, err := strconv.ParseInt(env(key, ""), 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(env(key, ""))
	if err != nil {
		return fallback
	}
	return v
}

// envList splits a comma separated variable and drops empty entries.
func envList(key string) []string {
	raw := strings.Split(os.Getenv(key), ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
