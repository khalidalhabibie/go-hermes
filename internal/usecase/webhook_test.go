package usecase_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"go-hermes/internal/config"
	"go-hermes/internal/entity"
	"go-hermes/internal/usecase"
	"go-hermes/tests/testkit"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestWebhookRetryBehaviorOnFailedDelivery(t *testing.T) {
	repos := testkit.NewMemoryRepositories()
	attempts := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, "temporary failure", http.StatusInternalServerError)
	}))
	defer server.Close()

	service := usecase.NewWebhookService(config.WebhookConfig{
		Enabled:              true,
		TargetURL:            server.URL,
		Secret:               "secret",
		MaxRetry:             3,
		RetryIntervalSeconds: 1,
		WorkerBatchSize:      10,
	}, repos.Webhooks, server.Client(), zerolog.Nop(), nil)

	transaction := testkit.NewTransactionBuilder().Build()
	wallet := testkit.NewWalletBuilder().Build()
	delivery, err := service.CreateTopUpDelivery(context.Background(), transaction, wallet)
	require.NoError(t, err)
	require.NotNil(t, delivery)

	require.NoError(t, service.ProcessDelivery(context.Background(), delivery.ID))

	storedDelivery, ok := repos.Store.WebhookByID(delivery.ID)
	require.True(t, ok)
	require.Equal(t, entity.WebhookDeliveryStatusRetrying, storedDelivery.Status)
	require.Equal(t, 1, storedDelivery.RetryCount)
	require.NotNil(t, storedDelivery.LastError)
	require.Equal(t, int32(1), attempts.Load())
}

func TestWebhookEventuallySucceedsAfterRetry(t *testing.T) {
	repos := testkit.NewMemoryRepositories()
	attempts := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := attempts.Add(1)
		if current == 1 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := usecase.NewWebhookService(config.WebhookConfig{
		Enabled:              true,
		TargetURL:            server.URL,
		Secret:               "secret",
		MaxRetry:             3,
		RetryIntervalSeconds: 1,
		WorkerBatchSize:      10,
	}, repos.Webhooks, server.Client(), zerolog.Nop(), nil)

	transaction := testkit.NewTransactionBuilder().Build()
	wallet := testkit.NewWalletBuilder().Build()
	delivery, err := service.CreateTopUpDelivery(context.Background(), transaction, wallet)
	require.NoError(t, err)
	require.NotNil(t, delivery)

	require.NoError(t, service.ProcessDelivery(context.Background(), delivery.ID))
	require.NoError(t, service.ProcessDelivery(context.Background(), delivery.ID))

	storedDelivery, ok := repos.Store.WebhookByID(delivery.ID)
	require.True(t, ok)
	require.Equal(t, entity.WebhookDeliveryStatusSuccess, storedDelivery.Status)
	require.Equal(t, 1, storedDelivery.RetryCount)
	require.NotNil(t, storedDelivery.DeliveredAt)
	require.Equal(t, int32(2), attempts.Load())
}
