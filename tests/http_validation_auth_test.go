package tests

import (
	"net/http"
	"testing"

	"go-hermes/internal/entity"
	"go-hermes/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestRegisterValidationMissingRequiredFields(t *testing.T) {
	h := testkit.NewTestHarness(t)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{}, nil)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "INVALID_REQUEST", errorResponse.ErrorCode)
}

func TestRegisterValidationMalformedEmail(t *testing.T) {
	h := testkit.NewTestHarness(t)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"name":     "Alice",
		"email":    "not-an-email",
		"password": "Password123",
	}, nil)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "INVALID_REQUEST", errorResponse.ErrorCode)
}

func TestRegisterValidationShortPassword(t *testing.T) {
	h := testkit.NewTestHarness(t)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"name":     "Alice",
		"email":    "alice@example.com",
		"password": "short",
	}, nil)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "INVALID_REQUEST", errorResponse.ErrorCode)
}

func TestLoginValidationMissingFields(t *testing.T) {
	h := testkit.NewTestHarness(t)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{}, nil)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "INVALID_REQUEST", errorResponse.ErrorCode)
}

func TestTopUpValidationNegativeAmount(t *testing.T) {
	h := testkit.NewTestHarness(t)
	user := h.SeedUser(t, testkit.NewUserBuilder(), 0)
	token := h.MustIssueToken(t, user.User)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/wallets/me/top-up", map[string]interface{}{
		"amount": -100,
	}, map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": "topup-negative",
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "INVALID_REQUEST", errorResponse.ErrorCode)
}

func TestTopUpValidationMissingIdempotencyKey(t *testing.T) {
	h := testkit.NewTestHarness(t)
	user := h.SeedUser(t, testkit.NewUserBuilder(), 0)
	token := h.MustIssueToken(t, user.User)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/wallets/me/top-up", map[string]interface{}{
		"amount": 1000,
	}, map[string]string{
		"Authorization": "Bearer " + token,
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "INVALID_REQUEST", errorResponse.ErrorCode)
}

func TestTransferValidationZeroAmount(t *testing.T) {
	h := testkit.NewTestHarness(t)
	sender := h.SeedUser(t, testkit.NewUserBuilder(), 5000)
	recipient := h.SeedUser(t, testkit.NewUserBuilder(), 0)
	token := h.MustIssueToken(t, sender.User)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/transfers", map[string]interface{}{
		"recipient_wallet_id": recipient.Wallet.ID.String(),
		"amount":              0,
	}, map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": "transfer-zero",
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "INVALID_REQUEST", errorResponse.ErrorCode)
}

func TestTransferValidationInvalidRecipientWalletID(t *testing.T) {
	h := testkit.NewTestHarness(t)
	sender := h.SeedUser(t, testkit.NewUserBuilder(), 5000)
	token := h.MustIssueToken(t, sender.User)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/transfers", map[string]interface{}{
		"recipient_wallet_id": "not-a-uuid",
		"amount":              1000,
	}, map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": "transfer-invalid-recipient",
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "INVALID_REQUEST", errorResponse.ErrorCode)
}

func TestTransferValidationMissingRecipientWalletID(t *testing.T) {
	h := testkit.NewTestHarness(t)
	sender := h.SeedUser(t, testkit.NewUserBuilder(), 5000)
	token := h.MustIssueToken(t, sender.User)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/transfers", map[string]interface{}{
		"amount": 1000,
	}, map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": "transfer-missing-recipient",
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "INVALID_REQUEST", errorResponse.ErrorCode)
}

func TestTransferValidationMissingIdempotencyKey(t *testing.T) {
	h := testkit.NewTestHarness(t)
	sender := h.SeedUser(t, testkit.NewUserBuilder(), 5000)
	recipient := h.SeedUser(t, testkit.NewUserBuilder(), 0)
	token := h.MustIssueToken(t, sender.User)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/transfers", map[string]interface{}{
		"recipient_wallet_id": recipient.Wallet.ID.String(),
		"amount":              1000,
	}, map[string]string{
		"Authorization": "Bearer " + token,
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "INVALID_REQUEST", errorResponse.ErrorCode)
}

func TestUnauthorizedAccessWithoutJWT(t *testing.T) {
	h := testkit.NewTestHarness(t)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodGet, "/api/v1/users/me", nil, nil)

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "UNAUTHORIZED", errorResponse.ErrorCode)
}

func TestUnauthorizedAccessWithInvalidJWT(t *testing.T) {
	h := testkit.NewTestHarness(t)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodGet, "/api/v1/users/me", nil, map[string]string{
		"Authorization": "Bearer invalid-token",
	})

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "UNAUTHORIZED", errorResponse.ErrorCode)
}

func TestForbiddenAdminEndpointForNormalUser(t *testing.T) {
	h := testkit.NewTestHarness(t)
	user := h.SeedUser(t, testkit.NewUserBuilder().WithRole(entity.RoleUser), 0)
	token := h.MustIssueToken(t, user.User)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodGet, "/api/v1/admin/audit-logs", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})

	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "FORBIDDEN", errorResponse.ErrorCode)
}

func TestTransactionDetailRejectsInvalidUUIDParameter(t *testing.T) {
	h := testkit.NewTestHarness(t)
	user := h.SeedUser(t, testkit.NewUserBuilder(), 0)
	token := h.MustIssueToken(t, user.User)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodGet, "/api/v1/transactions/not-a-uuid", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "BAD_REQUEST", errorResponse.ErrorCode)
}

func TestLedgerByTransactionRejectsInvalidUUIDParameter(t *testing.T) {
	h := testkit.NewTestHarness(t)
	user := h.SeedUser(t, testkit.NewUserBuilder(), 0)
	token := h.MustIssueToken(t, user.User)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodGet, "/api/v1/ledgers/transactions/not-a-uuid", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	errorResponse := testkit.DecodeError(t, body)
	require.Equal(t, "BAD_REQUEST", errorResponse.ErrorCode)
}
