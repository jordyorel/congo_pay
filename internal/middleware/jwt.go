package middleware

import (
    "net/http"
    "strings"

    "github.com/gofiber/fiber/v2"

    "github.com/congo-pay/congo_pay/internal/auth"
    "github.com/congo-pay/congo_pay/internal/config"
    "github.com/congo-pay/congo_pay/internal/identity"
)

// JWTAuth returns a middleware that validates JWT access tokens and checks token version.
func JWTAuth(cfg config.Config, repo identity.Repository) fiber.Handler {
    return func(c *fiber.Ctx) error {
        authz := c.Get(fiber.HeaderAuthorization)
        if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
            return fiber.NewError(http.StatusUnauthorized, "missing bearer token")
        }
        tokenStr := strings.TrimSpace(authz[len("Bearer "):])
        claims, err := auth.ParseAndVerifyHS256(tokenStr, []byte(cfg.JWTSecret))
        if err != nil {
            return fiber.NewError(http.StatusUnauthorized, "invalid token")
        }
        sub, _ := claims["sub"].(string)
        verFloat, _ := claims["ver"].(float64)
        ver := int(verFloat)

        user, err := repo.FindByID(c.UserContext(), sub)
        if err != nil || user.TokenVersion != ver {
            return fiber.NewError(http.StatusUnauthorized, "token invalidated")
        }

        c.Locals("user_id", sub)
        c.Locals("token_version", ver)
        return c.Next()
    }
}
