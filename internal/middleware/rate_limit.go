package middleware

import (
	"context"
	"fmt"
	"time"

	"go-hermes/internal/pkg/apiresponse"
	"go-hermes/internal/pkg/metrics"
	"go-hermes/internal/pkg/ratelimit"

	"github.com/gofiber/fiber/v2"
)

func RateLimit(name string, limiter ratelimit.Limiter, limit int, window time.Duration, collector *metrics.Collector) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if limiter == nil || limit <= 0 {
			return c.Next()
		}

		identifier := c.IP()
		if userID, ok := c.Locals(ContextUserID).(string); ok && userID != "" {
			identifier = userID
		}

		result, err := limiter.Allow(context.Background(), fmt.Sprintf("%s:%s", name, identifier), limit, window)
		if err != nil {
			return c.Next()
		}

		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", result.Limit))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
		c.Set("X-RateLimit-Reset", result.ResetAt.UTC().Format(time.RFC3339))

		if !result.Allowed {
			collector.IncrementRateLimitExceeded(name)
			return c.Status(fiber.StatusTooManyRequests).JSON(apiresponse.Error("rate limit exceeded", "RATE_LIMIT_EXCEEDED", []map[string]string{
				{"field": name, "message": "too many requests"},
			}))
		}

		return c.Next()
	}
}
