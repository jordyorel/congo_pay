package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

// RequestID ensures each request has a stable request identifier for tracing and logging.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		reqID := c.Get(requestIDHeader)
		if reqID == "" {
			reqID = uuid.NewString()
			c.Set(requestIDHeader, reqID)
		}

		c.Locals(requestIDHeader, reqID)

		return c.Next()
	}
}
