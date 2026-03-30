package tests

import (
	"net/http"
	"testing"

	"go-hermes/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestMetricsEndpointRequiresTokenWhenConfigured(t *testing.T) {
	h := testkit.NewTestHarness(t, testkit.WithMetricsToken("metrics-secret"))

	unauthorizedResp, unauthorizedBody := testkit.PerformJSONRequest(t, h.App, http.MethodGet, "/metrics", nil, nil)
	require.Equal(t, http.StatusUnauthorized, unauthorizedResp.StatusCode)
	errorResponse := testkit.DecodeError(t, unauthorizedBody)
	require.Equal(t, "UNAUTHORIZED", errorResponse.ErrorCode)

	authorizedResp, _ := testkit.PerformJSONRequest(t, h.App, http.MethodGet, "/metrics", nil, map[string]string{
		"X-Metrics-Token": "metrics-secret",
	})
	require.Equal(t, http.StatusOK, authorizedResp.StatusCode)
}
