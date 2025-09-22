package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppName         = "CongoPay"
	defaultAppEnv          = "development"
	defaultPort            = "8080"
	defaultLogLevel        = "info"
	defaultShutdownDelay   = 10 * time.Second
	defaultIdempotencyTTL  = 24 * time.Hour
	idemTTLSecondsEnvVar   = "IDEMPOTENCY_TTL_SECONDS"
	idemTTLDurEnvVar       = "IDEMPOTENCY_TTL"
	shutdownSecondsEnvVar  = "SHUTDOWN_TIMEOUT_SECONDS"
	shutdownDurationEnvVar = "SHUTDOWN_TIMEOUT"
)

// Config captures application runtime configuration loaded from environment variables.
type Config struct {
	AppName        string
	AppEnv         string
	Port           string
	LogLevel       string
	DatabaseURL    string
	RedisURL       string
	ShutdownPeriod time.Duration
	IdempotencyTTL time.Duration
}

// Load reads configuration values from the environment and populates a Config instance.
func Load() (Config, error) {
	cfg := Config{
		AppName:        getEnv("APP_NAME", defaultAppName),
		AppEnv:         getEnv("APP_ENV", defaultAppEnv),
		Port:           getEnv("PORT", defaultPort),
		LogLevel:       strings.ToLower(getEnv("LOG_LEVEL", defaultLogLevel)),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		RedisURL:       os.Getenv("REDIS_URL"),
		ShutdownPeriod: defaultShutdownDelay,
		IdempotencyTTL: defaultIdempotencyTTL,
	}

	if v := os.Getenv(shutdownSecondsEnvVar); v != "" {
		seconds, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", shutdownSecondsEnvVar, err)
		}
		cfg.ShutdownPeriod = time.Duration(seconds) * time.Second
	} else if v := os.Getenv(shutdownDurationEnvVar); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", shutdownDurationEnvVar, err)
		}
		cfg.ShutdownPeriod = d
	}

	if v := os.Getenv(idemTTLSecondsEnvVar); v != "" {
		seconds, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", idemTTLSecondsEnvVar, err)
		}
		cfg.IdempotencyTTL = time.Duration(seconds) * time.Second
	} else if v := os.Getenv(idemTTLDurEnvVar); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", idemTTLDurEnvVar, err)
		}
		cfg.IdempotencyTTL = d
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL must be set")
	}

	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL must be set")
	}

	return cfg, nil
}

// Address returns the listen address in the format Fiber expects.
func (c Config) Address() string {
	if strings.HasPrefix(c.Port, ":") {
		return c.Port
	}
	return fmt.Sprintf(":%s", c.Port)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
