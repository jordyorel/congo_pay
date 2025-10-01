package middleware

import (
    "net/http"
    "strings"
    "time"

    "github.com/gofiber/fiber/v2"
    "github.com/redis/go-redis/v9"
)

// LoginRateLimit limits login attempts per phone or IP using Redis if available.
func LoginRateLimit(cache *redis.Client, maxPerMin int) fiber.Handler {
    if maxPerMin <= 0 {
        maxPerMin = 5
    }
    return func(c *fiber.Ctx) error {
        if cache == nil {
            return c.Next() // no-op without Redis
        }
        var req struct{ Phone string `json:"phone"` }
        _ = c.BodyParser(&req)
        phone := strings.TrimSpace(req.Phone)
        if phone == "" {
            phone = c.IP()
        }
        key := "rl:login:" + phone
        cnt, err := cache.Incr(c.UserContext(), key).Result()
        if err == nil && cnt == 1 {
            cache.Expire(c.UserContext(), key, time.Minute)
        }
        if err != nil {
            return c.Next() // fail-open on cache errors
        }
        if cnt > int64(maxPerMin) {
            return fiber.NewError(http.StatusTooManyRequests, "too many login attempts, try again later")
        }
        return c.Next()
    }
}

