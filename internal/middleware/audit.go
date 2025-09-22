package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Audit emits structured logs for each request/response lifecycle event.
func Audit(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		status := c.Response().StatusCode()
		duration := time.Since(start)
		requestID, _ := c.Locals(requestIDHeader).(string)

		attrs := []any{
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", status),
			slog.Duration("duration", duration),
		}
		if requestID != "" {
			attrs = append(attrs, slog.String("request_id", requestID))
		}
		if err != nil {
			attrs = append(attrs, slog.Any("error", err))
			logger.Error("request completed", attrs...)
			return err
		}

		logger.Info("request completed", attrs...)
		return nil
	}
}
