package middleware

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/congo-pay/congo_pay/internal/logging"
)

func setupTestApp(t *testing.T) (*fiber.App, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}

	cache := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	app := fiber.New()
	logger := logging.Discard()
	app.Use(Idempotency(cache, time.Minute, logger))
	app.Post("/resource", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
	})

	cleanup := func() {
		cache.Close()
		mr.Close()
	}

	return app, cleanup
}

func TestIdempotencyRequiresHeader(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	req := httptest.NewRequest(fiber.MethodPost, "/resource", strings.NewReader("{}"))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected %d got %d", fiber.StatusBadRequest, resp.StatusCode)
	}
}

func TestIdempotencyReturnsCachedResponse(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	body := strings.NewReader("{}")
	req := httptest.NewRequest(fiber.MethodPost, "/resource", body)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(idempotencyKeyHeader, "abc123")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("first request: %v", err)
	}

	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected status %d got %d", fiber.StatusCreated, resp.StatusCode)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	resp.Body.Close()

	// Second request should return the cached response without invoking handler again.
	req2 := httptest.NewRequest(fiber.MethodPost, "/resource", strings.NewReader("{}"))
	req2.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req2.Header.Set(idempotencyKeyHeader, "abc123")

	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("second request: %v", err)
	}

	if resp2.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected cached status %d got %d", fiber.StatusCreated, resp2.StatusCode)
	}

	cachedPayload, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("read cached body: %v", err)
	}
	resp2.Body.Close()

	if string(cachedPayload) != string(payload) {
		t.Fatalf("expected cached payload %s got %s", string(payload), string(cachedPayload))
	}

	var decoded map[string]any
	if err := json.Unmarshal(cachedPayload, &decoded); err != nil {
		t.Fatalf("cached payload invalid json: %v", err)
	}
}
