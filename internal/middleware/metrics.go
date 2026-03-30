package middleware

import (
	"time"

	"go-hermes/internal/pkg/apperror"
	"go-hermes/internal/pkg/metrics"

	"github.com/gofiber/fiber/v2"
)

func Metrics(collector *metrics.Collector) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if collector == nil {
			return c.Next()
		}

		startedAt := time.Now()
		err := c.Next()

		statusCode := c.Response().StatusCode()
		if err != nil {
			if fiberErr, ok := err.(*fiber.Error); ok {
				statusCode = fiberErr.Code
			} else {
				statusCode = apperror.As(err).Status
			}
		}

		route := c.Path()
		if matched := c.Route(); matched != nil && matched.Path != "" {
			route = matched.Path
		}

		collector.ObserveHTTPRequest(c.Method(), route, statusCode, time.Since(startedAt))
		return err
	}
}
