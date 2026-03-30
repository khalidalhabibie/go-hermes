package middleware

import (
	"strings"

	"go-hermes/internal/pkg/apperror"

	"github.com/gofiber/fiber/v2"
)

func ProtectMetrics(token string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if token == "" {
			return c.Next()
		}

		if c.Get("X-Metrics-Token") == token {
			return c.Next()
		}

		header := c.Get("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && parts[1] == token {
			return c.Next()
		}

		return apperror.Unauthorized("invalid metrics token")
	}
}
