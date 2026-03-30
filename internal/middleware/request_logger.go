package middleware

import (
	"time"

	"go-hermes/internal/pkg/apperror"
	"go-hermes/internal/pkg/requestid"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

func RequestLogger(log zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		startedAt := time.Now()
		err := c.Next()
		duration := time.Since(startedAt)
		statusCode := c.Response().StatusCode()

		event := log.Info()
		if err != nil {
			event = log.Error().Err(err)
			if fiberErr, ok := err.(*fiber.Error); ok {
				statusCode = fiberErr.Code
			} else {
				statusCode = apperror.As(err).Status
			}
		}

		event = event.
			Str("request_id", requestid.Get(c)).
			Str("method", c.Method()).
			Str("path", c.OriginalURL()).
			Int("status", statusCode).
			Dur("duration", duration)

		if traceID, ok := c.Locals(ContextTraceID).(string); ok && traceID != "" {
			event = event.Str("trace_id", traceID)
		}
		if userID, ok := c.Locals(ContextUserID).(string); ok && userID != "" {
			event = event.Str("user_id", userID)
		}

		event.Msg("http request")

		return err
	}
}
