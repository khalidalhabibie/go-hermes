package middleware

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-hermes/internal/pkg/metrics"
	"go-hermes/internal/pkg/ratelimit"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type failingLimiter struct{}

func (f failingLimiter) Allow(_ context.Context, _ string, _ int, _ time.Duration) (ratelimit.Result, error) {
	return ratelimit.Result{}, errors.New("redis unavailable")
}

func TestRateLimitFailsOpenAndLogsPolicy(t *testing.T) {
	var logBuffer bytes.Buffer
	log := zerolog.New(&logBuffer)

	app := fiber.New()
	app.Get("/login", RateLimit("login", failingLimiter{}, 1, time.Minute, metrics.NewCollector(), log), func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, logBuffer.String(), rateLimitFailurePolicy)
	require.Contains(t, logBuffer.String(), "allowing request")
}
