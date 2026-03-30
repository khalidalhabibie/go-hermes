package usecase

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go-hermes/internal/config"
	"go-hermes/internal/entity"
	"go-hermes/internal/repository"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/datatypes"
)

type WebhookNotifier interface {
	CreateTopUpDelivery(ctx context.Context, transaction *entity.Transaction, wallet *entity.Wallet) (*entity.WebhookDelivery, error)
	CreateTransferDelivery(ctx context.Context, transaction *entity.Transaction, sourceWallet *entity.Wallet, destinationWallet *entity.Wallet) (*entity.WebhookDelivery, error)
	Enqueue(ctx context.Context, deliveryID uuid.UUID)
}

type NoopWebhookService struct{}

func (s *NoopWebhookService) CreateTopUpDelivery(_ context.Context, _ *entity.Transaction, _ *entity.Wallet) (*entity.WebhookDelivery, error) {
	return nil, nil
}

func (s *NoopWebhookService) CreateTransferDelivery(_ context.Context, _ *entity.Transaction, _ *entity.Wallet, _ *entity.Wallet) (*entity.WebhookDelivery, error) {
	return nil, nil
}

func (s *NoopWebhookService) Enqueue(_ context.Context, _ uuid.UUID) {}

type WebhookService struct {
	config     config.WebhookConfig
	repository repository.WebhookDeliveryRepository
	client     *http.Client
	log        zerolog.Logger
	queue      chan uuid.UUID
}

func NewWebhookService(cfg config.WebhookConfig, repository repository.WebhookDeliveryRepository, client *http.Client, log zerolog.Logger) *WebhookService {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	queueSize := cfg.WorkerBatchSize
	if queueSize < 1 {
		queueSize = 20
	}

	return &WebhookService{
		config:     cfg,
		repository: repository,
		client:     client,
		log:        log,
		queue:      make(chan uuid.UUID, queueSize*2),
	}
}

func (s *WebhookService) CreateTopUpDelivery(ctx context.Context, transaction *entity.Transaction, wallet *entity.Wallet) (*entity.WebhookDelivery, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"event_type":            "wallet.top_up.success",
		"transaction_id":        transaction.ID,
		"transaction_ref":       transaction.TransactionRef,
		"source_wallet_id":      nil,
		"destination_wallet_id": wallet.ID,
		"amount":                transaction.Amount,
		"currency":              transaction.Currency,
		"status":                transaction.Status,
		"occurred_at":           time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}

	return s.createDelivery(ctx, "wallet.top_up.success", transaction, payload)
}

func (s *WebhookService) CreateTransferDelivery(ctx context.Context, transaction *entity.Transaction, sourceWallet *entity.Wallet, destinationWallet *entity.Wallet) (*entity.WebhookDelivery, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"event_type":            "wallet.transfer.success",
		"transaction_id":        transaction.ID,
		"transaction_ref":       transaction.TransactionRef,
		"source_wallet_id":      sourceWallet.ID,
		"destination_wallet_id": destinationWallet.ID,
		"amount":                transaction.Amount,
		"currency":              transaction.Currency,
		"status":                transaction.Status,
		"occurred_at":           time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}

	return s.createDelivery(ctx, "wallet.transfer.success", transaction, payload)
}

func (s *WebhookService) createDelivery(ctx context.Context, eventType string, transaction *entity.Transaction, payload []byte) (*entity.WebhookDelivery, error) {
	if !s.config.Enabled || s.config.TargetURL == "" {
		return nil, nil
	}

	delivery := &entity.WebhookDelivery{
		ID:             uuid.New(),
		EventType:      eventType,
		TransactionID:  &transaction.ID,
		TransactionRef: transaction.TransactionRef,
		TargetURL:      s.config.TargetURL,
		Payload:        datatypes.JSON(payload),
		Status:         entity.WebhookDeliveryStatusPending,
		RetryCount:     0,
		MaxRetry:       s.normalizedMaxRetry(),
	}
	if s.config.Secret != "" {
		secret := s.config.Secret
		delivery.Secret = &secret
	}

	if err := s.repository.Create(ctx, delivery); err != nil {
		return nil, err
	}

	s.log.Info().
		Str("delivery_id", delivery.ID.String()).
		Str("event_type", delivery.EventType).
		Str("transaction_ref", delivery.TransactionRef).
		Str("target_url", delivery.TargetURL).
		Msg("webhook delivery created")

	return delivery, nil
}

func (s *WebhookService) Enqueue(_ context.Context, deliveryID uuid.UUID) {
	if !s.config.Enabled {
		return
	}

	select {
	case s.queue <- deliveryID:
	default:
		s.log.Warn().Str("delivery_id", deliveryID.String()).Msg("webhook queue full, delivery will be picked up by retry scanner")
	}
}

func (s *WebhookService) Start(ctx context.Context) {
	if !s.config.Enabled {
		return
	}

	ticker := time.NewTicker(s.retryInterval())
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case deliveryID := <-s.queue:
				if err := s.ProcessDelivery(ctx, deliveryID); err != nil {
					s.log.Error().Err(err).Str("delivery_id", deliveryID.String()).Msg("webhook immediate processing failed")
				}
			case <-ticker.C:
				if err := s.ProcessDue(ctx, s.batchSize()); err != nil {
					s.log.Error().Err(err).Msg("webhook retry scan failed")
				}
			}
		}
	}()
}

func (s *WebhookService) ProcessDue(ctx context.Context, limit int) error {
	deliveries, err := s.repository.ListDue(ctx, time.Now().UTC(), limit)
	if err != nil {
		return err
	}

	for _, delivery := range deliveries {
		if err := s.ProcessDelivery(ctx, delivery.ID); err != nil {
			s.log.Error().Err(err).Str("delivery_id", delivery.ID.String()).Msg("webhook due delivery failed")
		}
	}
	return nil
}

func (s *WebhookService) ProcessDelivery(ctx context.Context, deliveryID uuid.UUID) error {
	delivery, err := s.repository.GetByID(ctx, deliveryID)
	if err != nil {
		return err
	}
	if delivery.Status == entity.WebhookDeliveryStatusSuccess || delivery.Status == entity.WebhookDeliveryStatusFailed {
		return nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.TargetURL, bytes.NewReader(delivery.Payload))
	if err != nil {
		return s.markFailure(ctx, delivery, 0, err.Error())
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Webhook-Event", delivery.EventType)
	request.Header.Set("X-Webhook-Delivery-ID", delivery.ID.String())
	if delivery.Secret != nil && *delivery.Secret != "" {
		request.Header.Set("X-Webhook-Signature", signPayload(*delivery.Secret, delivery.Payload))
	}

	s.log.Info().
		Str("delivery_id", delivery.ID.String()).
		Str("event_type", delivery.EventType).
		Str("transaction_ref", delivery.TransactionRef).
		Int("retry_count", delivery.RetryCount).
		Str("target_url", delivery.TargetURL).
		Msg("sending webhook delivery")

	response, err := s.client.Do(request)
	if err != nil {
		return s.markFailure(ctx, delivery, 0, err.Error())
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return s.markFailure(ctx, delivery, response.StatusCode, fmt.Sprintf("unexpected status code %d", response.StatusCode))
	}

	now := time.Now().UTC()
	delivery.Status = entity.WebhookDeliveryStatusSuccess
	delivery.DeliveredAt = &now
	delivery.NextRetryAt = nil
	delivery.LastError = nil
	delivery.LastHTTPStatus = &response.StatusCode
	delivery.UpdatedAt = now

	if err := s.repository.Update(ctx, delivery); err != nil {
		return err
	}

	s.log.Info().
		Str("delivery_id", delivery.ID.String()).
		Str("event_type", delivery.EventType).
		Str("transaction_ref", delivery.TransactionRef).
		Int("status_code", response.StatusCode).
		Msg("webhook delivery succeeded")

	return nil
}

func (s *WebhookService) markFailure(ctx context.Context, delivery *entity.WebhookDelivery, statusCode int, failureMessage string) error {
	delivery.RetryCount++
	now := time.Now().UTC()
	delivery.UpdatedAt = now

	if statusCode > 0 {
		delivery.LastHTTPStatus = &statusCode
	}

	lastError := failureMessage
	delivery.LastError = &lastError

	if delivery.RetryCount >= delivery.MaxRetry {
		delivery.Status = entity.WebhookDeliveryStatusFailed
		delivery.NextRetryAt = nil
	} else {
		delivery.Status = entity.WebhookDeliveryStatusRetrying
		nextRetryAt := now.Add(s.retryInterval() * time.Duration(delivery.RetryCount))
		delivery.NextRetryAt = &nextRetryAt
	}

	if err := s.repository.Update(ctx, delivery); err != nil {
		return err
	}

	s.log.Warn().
		Str("delivery_id", delivery.ID.String()).
		Str("event_type", delivery.EventType).
		Str("transaction_ref", delivery.TransactionRef).
		Int("retry_count", delivery.RetryCount).
		Int("status_code", statusCode).
		Msg("webhook delivery failed")

	return nil
}

func (s *WebhookService) retryInterval() time.Duration {
	seconds := s.config.RetryIntervalSeconds
	if seconds < 1 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func (s *WebhookService) normalizedMaxRetry() int {
	if s.config.MaxRetry < 1 {
		return 3
	}
	return s.config.MaxRetry
}

func (s *WebhookService) batchSize() int {
	if s.config.WorkerBatchSize < 1 {
		return 20
	}
	return s.config.WorkerBatchSize
}

func signPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
