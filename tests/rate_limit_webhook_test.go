package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go-hermes/internal/entity"
	"go-hermes/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestRateLimitExceededOnLogin(t *testing.T) {
	h := testkit.NewTestHarness(t, testkit.WithRateLimit(1, 0, 0, time.Minute))
	user := h.SeedUser(t, testkit.NewUserBuilder().WithEmail("login-rate@example.com"), 0)

	firstResp, _ := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{
		"email":    user.User.Email,
		"password": user.Password,
	}, nil)
	require.Equal(t, http.StatusOK, firstResp.StatusCode)

	secondResp, secondBody := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{
		"email":    user.User.Email,
		"password": user.Password,
	}, nil)
	require.Equal(t, http.StatusTooManyRequests, secondResp.StatusCode)

	errorResult := testkit.DecodeError(t, secondBody)
	require.Equal(t, "RATE_LIMIT_EXCEEDED", errorResult.ErrorCode)
}

func TestWebhookDeliveryRecordCreatedAfterSuccessfulTopUp(t *testing.T) {
	h := testkit.NewTestHarness(t, testkit.WithWebhook("http://example.test/webhooks", "secret", 3, time.Second, &http.Client{Timeout: time.Second}, false))
	user := h.SeedUser(t, testkit.NewUserBuilder().WithEmail("topup-webhook@example.com"), 1000)
	token := h.MustIssueToken(t, user.User)

	resp, _ := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/wallets/me/top-up", map[string]interface{}{
		"amount": 5000,
	}, map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": "topup-webhook",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, h.Repos.Store.WebhookCount())

	webhookIDs := h.Repos.Store.WebhookIDs()
	require.Len(t, webhookIDs, 1)
	delivery, ok := h.Repos.Store.WebhookByID(webhookIDs[0])
	require.True(t, ok)
	require.Equal(t, entity.WebhookDeliveryStatusPending, delivery.Status)
	require.Equal(t, "wallet.top_up.success", delivery.EventType)
	require.NotNil(t, delivery.TransactionID)
}

func TestWebhookDeliveryRecordCreatedAfterSuccessfulTransfer(t *testing.T) {
	h := testkit.NewTestHarness(t, testkit.WithWebhook("http://example.test/webhooks", "secret", 3, time.Second, &http.Client{Timeout: time.Second}, false))
	sender := h.SeedUser(t, testkit.NewUserBuilder().WithEmail("sender-webhook@example.com"), 10000)
	recipient := h.SeedUser(t, testkit.NewUserBuilder().WithEmail("recipient-webhook@example.com"), 1000)
	token := h.MustIssueToken(t, sender.User)

	resp, _ := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/transfers", map[string]interface{}{
		"recipient_wallet_id": recipient.Wallet.ID.String(),
		"amount":              3000,
	}, map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": "transfer-webhook",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, h.Repos.Store.WebhookCount())

	webhookIDs := h.Repos.Store.WebhookIDs()
	require.Len(t, webhookIDs, 1)
	delivery, ok := h.Repos.Store.WebhookByID(webhookIDs[0])
	require.True(t, ok)
	require.Equal(t, entity.WebhookDeliveryStatusPending, delivery.Status)
	require.Equal(t, "wallet.transfer.success", delivery.EventType)
	require.NotNil(t, delivery.TransactionID)
}

func TestAdminWebhookListEndpointRequiresAdminRole(t *testing.T) {
	h := testkit.NewTestHarness(t, testkit.WithWebhook("http://example.test/webhooks", "secret", 3, time.Second, &http.Client{Timeout: time.Second}, false))
	user := h.SeedUser(t, testkit.NewUserBuilder().WithRole(entity.RoleUser), 0)
	token := h.MustIssueToken(t, user.User)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodGet, "/api/v1/admin/webhooks", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})

	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	errorResult := testkit.DecodeError(t, body)
	require.Equal(t, "FORBIDDEN", errorResult.ErrorCode)
}

func TestWebhookWorkerEventualSuccessWithRetry(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			http.Error(w, "fail once", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	h := testkit.NewTestHarness(t, testkit.WithWebhook(server.URL, "secret", 3, time.Second, server.Client(), false))
	user := h.SeedUser(t, testkit.NewUserBuilder().WithEmail("worker-webhook@example.com"), 1000)
	token := h.MustIssueToken(t, user.User)

	resp, _ := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/wallets/me/top-up", map[string]interface{}{
		"amount": 5000,
	}, map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": "topup-worker",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	webhookIDs := h.Repos.Store.WebhookIDs()
	require.Len(t, webhookIDs, 1)
	deliveryID := webhookIDs[0]

	require.NoError(t, h.WebhookService.ProcessDelivery(context.Background(), deliveryID))
	require.NoError(t, h.WebhookService.ProcessDelivery(context.Background(), deliveryID))

	delivery, ok := h.Repos.Store.WebhookByID(deliveryID)
	require.True(t, ok)
	require.Equal(t, entity.WebhookDeliveryStatusSuccess, delivery.Status)
	require.Equal(t, 1, delivery.RetryCount)
	require.Equal(t, int32(2), attempts.Load())
}
