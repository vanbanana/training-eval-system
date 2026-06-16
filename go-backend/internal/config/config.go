// Package config loads application configuration from TES_ prefixed environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from TES_ env vars / .env file.
type Config struct {
	Env             string        // "dev" | "test" | "prod"
	Debug           bool          // enable debug mode
	ListenAddr      string        // default ":8000"
	DBPath          string        // default "./data/app.db"
	UploadRoot      string        // default "./data/uploads"
	DistDir         string        // default "./dist"
	JWTSecret       string        // required, min 32 chars
	JWTAccessTTL    time.Duration // default 60m
	JWTRefreshTTL   time.Duration // default 7*24h
	LLMKeyMaster    string        // required, base64-encoded 32 bytes
	CORSOrigins     []string      // default ["http://localhost:5173","http://localhost:3000"]
	MaxUploadSizeMB int           // default 50
	WorkerCount     int           // default 4
	TaskBufferSize  int           // default 100
	LogLevel        string        // "debug" | "info" | "warn" | "error"
	BackupDir       string        // default "./data/backups"
	BackupInterval  time.Duration // default 24h
	BackupRetention time.Duration // default 7*24h
	DevToken        string        // dev/test only debug token

	// LLM provider settings
	LLMBaseURL    string // default "https://api.xiaomimimo.com/v1"
	LLMAPIKey     string // LLM API key (MiMo: passed via api-key header)
	LLMModel      string // default "mimo-v2.5-pro"
	LLMEmbedModel string // embedding model name (can be empty)
	LLMOCRModel   string // OCR model name for multimodal image recognition (e.g. "mimo-v2.5")

	// Use api-key header instead of Authorization: Bearer (MiMo style)
	LLMUseAPIKeyHeader bool // default true

	// system_config cache TTL
	SystemConfigCacheTTL time.Duration // default 60s
}

// Load reads TES_ prefixed env vars, with .env file fallback.
// Returns error if required fields (JWTSecret, LLMKeyMaster) are missing or invalid.
func Load() (*Config, error) {
	// Load .env file (ignore error if not found — env vars take precedence)
	_ = godotenv.Load()

	cfg := &Config{
		Env:                  envStr("TES_ENV", "dev"),
		Debug:                envBool("TES_DEBUG", false),
		ListenAddr:           envStr("TES_LISTEN_ADDR", ":8000"),
		DBPath:               envStr("TES_DB_PATH", "./data/app.db"),
		UploadRoot:           envStr("TES_UPLOAD_ROOT", "./data/uploads"),
		DistDir:              envStr("TES_DIST_DIR", "./dist"),
		JWTSecret:            envStr("TES_JWT_SECRET", ""),
		JWTAccessTTL:         envDuration("TES_JWT_ACCESS_TTL_MINUTES", 60*time.Minute),
		JWTRefreshTTL:        envDuration("TES_JWT_REFRESH_TTL_DAYS", 7*24*time.Hour),
		LLMKeyMaster:         envStr("TES_LLM_KEY_MASTER", ""),
		CORSOrigins:          envStringSlice("TES_CORS_ORIGINS", []string{"http://localhost:5173", "http://localhost:3000"}),
		MaxUploadSizeMB:      envInt("TES_MAX_UPLOAD_SIZE_MB", 50),
		WorkerCount:          envInt("TES_WORKER_COUNT", 32),
		TaskBufferSize:       envInt("TES_TASK_BUFFER_SIZE", 2000),
		LogLevel:             envStr("TES_LOG_LEVEL", "info"),
		BackupDir:            envStr("TES_BACKUP_DIR", "./data/backups"),
		BackupInterval:       envDuration("TES_BACKUP_INTERVAL_HOURS", 24*time.Hour),
		BackupRetention:      envDuration("TES_BACKUP_RETENTION_DAYS", 7*24*time.Hour),
		DevToken:             envStr("TES_DEV_TOKEN", "dev-token"),
		SystemConfigCacheTTL: envDuration("TES_SYSTEM_CONFIG_CACHE_TTL_SECONDS", 60*time.Second),

		// LLM provider settings (MiMo defaults)
		LLMBaseURL:         envStr("TES_LLM_BASE_URL", "https://token-plan-cn.xiaomimimo.com/v1"),
		LLMAPIKey:          envStr("TES_LLM_API_KEY", ""),
		LLMModel:           envStr("TES_LLM_MODEL", "mimo-v2.5-pro"),
		LLMEmbedModel:      envStr("TES_LLM_EMBED_MODEL", ""),
		LLMOCRModel:        envStr("TES_LLM_OCR_MODEL", ""),
		LLMUseAPIKeyHeader: envBool("TES_LLM_USE_API_KEY_HEADER", true),
	}

	// Validate environment
	switch cfg.Env {
	case "dev", "test", "prod":
	default:
		return nil, fmt.Errorf("config: TES_ENV must be one of dev/test/prod, got %q", cfg.Env)
	}

	// Validate required fields
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("config: TES_JWT_SECRET is required (min 32 chars)")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("config: TES_JWT_SECRET must be at least 32 characters, got %d", len(cfg.JWTSecret))
	}

	if cfg.LLMKeyMaster == "" {
		return nil, fmt.Errorf("config: TES_LLM_KEY_MASTER is required (base64-encoded 32 bytes)")
	}

	// Validate log level
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return nil, fmt.Errorf("config: TES_LOG_LEVEL must be one of debug/info/warn/error, got %q", cfg.LogLevel)
	}

	return cfg, nil
}

// IsDev returns true if running in development mode.
func (c *Config) IsDev() bool {
	return c.Env == "dev"
}

// IsTest returns true if running in test mode.
func (c *Config) IsTest() bool {
	return c.Env == "test"
}

// IsProd returns true if running in production mode.
func (c *Config) IsProd() bool {
	return c.Env == "prod"
}

// --- helper functions ---

func envStr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}

func envInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func envDuration(key string, defaultVal time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	// Try parsing as integer (interpret based on key suffix)
	if n, err := strconv.Atoi(v); err == nil {
		switch {
		case strings.HasSuffix(key, "_MINUTES"):
			return time.Duration(n) * time.Minute
		case strings.HasSuffix(key, "_HOURS"):
			return time.Duration(n) * time.Hour
		case strings.HasSuffix(key, "_DAYS"):
			return time.Duration(n) * 24 * time.Hour
		case strings.HasSuffix(key, "_SECONDS"):
			return time.Duration(n) * time.Second
		default:
			return time.Duration(n) * time.Second
		}
	}
	// Try parsing as Go duration string (e.g. "30m", "24h")
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return defaultVal
}

func envStringSlice(key string, defaultVal []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return defaultVal
	}
	return result
}
