package config

import (
    "fmt"
    "os"
    "strconv"
    "time"
)

type Config struct {
    AppName       string
    Env           string
    Port          int
    LogLevel      string
    DatabaseURL   string
    RedisURL      string
    JWTSecret     string
    RefreshSecret string
    AccessTokenTTL  time.Duration
    RefreshTokenTTL time.Duration
    SMSProvider   string
    IdempotencyTTL time.Duration
}

func (c Config) Addr() string {
    return fmt.Sprintf(":%d", c.Port)
}

// Address provides a compatibility alias used by other packages.
func (c Config) Address() string { return c.Addr() }

func getenv(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}

func getint(key string, def int) int {
    if v := os.Getenv(key); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            return n
        }
    }
    return def
}

func getduration(key string, def time.Duration) time.Duration {
    if v := os.Getenv(key); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            return d
        }
    }
    return def
}

func Load() Config {
    return Config{
        AppName:        getenv("APP_NAME", "CongoPay"),
        Env:            getenv("APP_ENV", "development"),
        Port:           getint("PORT", 8080),
        LogLevel:       getenv("LOG_LEVEL", "info"),
        DatabaseURL:    getenv("DATABASE_URL", ""),
        RedisURL:       getenv("REDIS_URL", ""),
        JWTSecret:      getenv("JWT_SECRET", ""),
        RefreshSecret:  getenv("REFRESH_SECRET", getenv("JWT_SECRET", "")),
        AccessTokenTTL:  getduration("ACCESS_TTL", 15*time.Minute),
        RefreshTokenTTL: getduration("REFRESH_TTL", 720*time.Hour),
        SMSProvider:    getenv("SMS_PROVIDER", ""),
        IdempotencyTTL: getduration("IDEMPOTENCY_TTL", 10*time.Minute),
    }
}
