package testkit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type SuccessEnvelope[T any] struct {
	Message string          `json:"message"`
	Data    T               `json:"data"`
	Meta    json.RawMessage `json:"meta"`
}

type ErrorEnvelope struct {
	Message   string                   `json:"message"`
	ErrorCode string                   `json:"error_code"`
	Details   []map[string]interface{} `json:"details"`
}

func PerformJSONRequest(t *testing.T, app *fiber.App, method, path string, body interface{}, headers map[string]string) (*http.Response, []byte) {
	t.Helper()

	var payload io.Reader
	if body != nil {
		rawBody, err := json.Marshal(body)
		require.NoError(t, err)
		payload = bytes.NewBuffer(rawBody)
	}

	req := httptest.NewRequest(method, path, payload)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, responseBody
}

func DecodeSuccess[T any](t *testing.T, body []byte) SuccessEnvelope[T] {
	t.Helper()

	var envelope SuccessEnvelope[T]
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope
}

func DecodeError(t *testing.T, body []byte) ErrorEnvelope {
	t.Helper()

	var envelope ErrorEnvelope
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope
}
