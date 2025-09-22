package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

const (
	idempotencyKeyHeader = "Idempotency-Key"
	idempotencyPrefix    = "idempotency:v1:"
	inProgressMarker     = "__in_progress__"
)

type storedResponse struct {
	Status  int               `json:"status"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

// Idempotency enforces idempotent semantics across unsafe HTTP methods by
// persisting responses in Redis keyed by the provided Idempotency-Key header.
func Idempotency(cache *redis.Client, ttl time.Duration, logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		method := strings.ToUpper(c.Method())
		switch method {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			return c.Next()
		}

		key := c.Get(idempotencyKeyHeader)
		if key == "" {
			return fiber.NewError(fiber.StatusBadRequest, "missing Idempotency-Key header")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		cacheKey := idempotencyPrefix + key

		cached, err := cache.Get(ctx, cacheKey).Result()
		if err == nil {
			if cached == inProgressMarker {
				return fiber.NewError(fiber.StatusConflict, "duplicate request currently processing")
			}

			var stored storedResponse
			if err := json.Unmarshal([]byte(cached), &stored); err != nil {
				logger.Warn("failed to decode stored idempotent response", slog.String("key", key), slog.Any("error", err))
				return fiber.NewError(fiber.StatusConflict, "duplicate request")
			}

			for header, value := range stored.Headers {
				if strings.EqualFold(header, fiber.HeaderContentLength) {
					continue
				}
				c.Set(header, value)
			}
			return c.Status(stored.Status).SendString(stored.Body)
		}

		if err != redis.Nil {
			logger.Error("idempotency lookup failed", slog.String("key", key), slog.Any("error", err))
			return fiber.NewError(fiber.StatusInternalServerError, "idempotency store failure")
		}

		if err := cache.SetNX(ctx, cacheKey, inProgressMarker, ttl).Err(); err != nil {
			logger.Error("idempotency reservation failed", slog.String("key", key), slog.Any("error", err))
			return fiber.NewError(fiber.StatusInternalServerError, "idempotency reservation failure")
		}

		if err := c.Next(); err != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cache.Del(cleanupCtx, cacheKey) // best effort cleanup
			return err
		}

		stored := storedResponse{
			Status:  c.Response().StatusCode(),
			Body:    string(c.Response().Body()),
			Headers: map[string]string{},
		}

		c.Response().Header.VisitAll(func(k, v []byte) {
			stored.Headers[string(k)] = string(v)
		})

		payload, err := json.Marshal(stored)
		if err != nil {
			logger.Error("failed to encode idempotent response", slog.String("key", key), slog.Any("error", err))
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cache.Del(cleanupCtx, cacheKey)
			return fiber.NewError(fiber.StatusInternalServerError, "idempotency persistence failure")
		}

		persistCtx, persistCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer persistCancel()

		if err := cache.Set(persistCtx, cacheKey, payload, ttl).Err(); err != nil {
			logger.Error("failed to persist idempotent response", slog.String("key", key), slog.Any("error", err))
			cache.Del(persistCtx, cacheKey)
			return fiber.NewError(fiber.StatusInternalServerError, "idempotency persistence failure")
		}

		return nil
	}
}
