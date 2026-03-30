package middleware

import (
	"strings"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/apperror"
	"go-hermes/internal/pkg/auth"

	"github.com/gofiber/fiber/v2"
)

const (
	ContextUserID = "user_id"
	ContextRole   = "role"
)

func Auth(jwtManager *auth.JWTManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" {
			return apperror.Unauthorized("missing authorization header")
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return apperror.Unauthorized("invalid authorization header")
		}

		claims, err := jwtManager.ParseToken(parts[1])
		if err != nil {
			return apperror.Unauthorized("invalid token")
		}

		c.Locals(ContextUserID, claims.UserID)
		c.Locals(ContextRole, claims.Role)
		return c.Next()
	}
}

func RequireRole(role entity.Role) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if currentRole, ok := c.Locals(ContextRole).(string); !ok || currentRole != string(role) {
			return apperror.Forbidden("insufficient role")
		}
		return c.Next()
	}
}
