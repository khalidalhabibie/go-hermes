package usecase

import (
	"context"
	"encoding/json"
	"time"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/apperror"
	"go-hermes/internal/pkg/pagination"
	"go-hermes/internal/repository"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type AdminUsecase struct {
	audits       repository.AuditLogRepository
	transactions repository.TransactionRepository
	webhooks     repository.WebhookDeliveryRepository
}

func NewAdminUsecase(audits repository.AuditLogRepository, transactions repository.TransactionRepository, webhooks repository.WebhookDeliveryRepository) *AdminUsecase {
	return &AdminUsecase{
		audits:       audits,
		transactions: transactions,
		webhooks:     webhooks,
	}
}

func (u *AdminUsecase) ListAuditLogs(ctx context.Context, actorUserID string, params pagination.Params) ([]AuditLogResponse, map[string]interface{}, error) {
	actorID, err := uuid.Parse(actorUserID)
	if err != nil {
		return nil, nil, apperror.BadRequest("invalid user id")
	}

	logs, total, err := u.audits.List(ctx, params)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}

	items := make([]AuditLogResponse, 0, len(logs))
	for i := range logs {
		var metadata interface{}
		if len(logs[i].Metadata) > 0 {
			_ = json.Unmarshal(logs[i].Metadata, &metadata)
		}

		var actor *string
		if logs[i].ActorUserID != nil {
			value := logs[i].ActorUserID.String()
			actor = &value
		}
		var entityID *string
		if logs[i].EntityID != nil {
			value := logs[i].EntityID.String()
			entityID = &value
		}

		items = append(items, AuditLogResponse{
			ID:          logs[i].ID.String(),
			ActorUserID: actor,
			Action:      logs[i].Action,
			EntityType:  logs[i].EntityType,
			EntityID:    entityID,
			Metadata:    metadata,
			CreatedAt:   logs[i].CreatedAt,
		})
	}

	metadata, _ := json.Marshal(map[string]interface{}{
		"page":  params.Page,
		"limit": params.Limit,
	})
	_ = u.audits.Create(ctx, &entity.AuditLog{
		ID:          uuid.New(),
		ActorUserID: &actorID,
		Action:      "ADMIN_READ_AUDIT_LOGS",
		EntityType:  "audit_log",
		Metadata:    datatypes.JSON(metadata),
		CreatedAt:   time.Now(),
	})

	return items, pagination.Meta(total, params), nil
}

func (u *AdminUsecase) ListTransactions(ctx context.Context, actorUserID string, params pagination.Params) ([]TransactionResponse, map[string]interface{}, error) {
	actorID, err := uuid.Parse(actorUserID)
	if err != nil {
		return nil, nil, apperror.BadRequest("invalid user id")
	}

	transactions, total, err := u.transactions.ListAll(ctx, params)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}

	items := make([]TransactionResponse, 0, len(transactions))
	for i := range transactions {
		items = append(items, toTransactionResponse(&transactions[i]))
	}

	metadata, _ := json.Marshal(map[string]interface{}{
		"page":  params.Page,
		"limit": params.Limit,
	})
	_ = u.audits.Create(ctx, &entity.AuditLog{
		ID:          uuid.New(),
		ActorUserID: &actorID,
		Action:      "ADMIN_READ_TRANSACTIONS",
		EntityType:  "transaction",
		Metadata:    datatypes.JSON(metadata),
		CreatedAt:   time.Now(),
	})

	return items, pagination.Meta(total, params), nil
}

func (u *AdminUsecase) ListWebhooks(ctx context.Context, actorUserID string, filter repository.WebhookDeliveryFilter, params pagination.Params) ([]WebhookDeliveryResponse, map[string]interface{}, error) {
	actorID, err := uuid.Parse(actorUserID)
	if err != nil {
		return nil, nil, apperror.BadRequest("invalid user id")
	}

	deliveries, total, err := u.webhooks.List(ctx, filter, params)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}

	items := make([]WebhookDeliveryResponse, 0, len(deliveries))
	for i := range deliveries {
		items = append(items, toWebhookDeliveryResponse(&deliveries[i]))
	}

	metadata, _ := json.Marshal(map[string]interface{}{
		"page":            params.Page,
		"limit":           params.Limit,
		"event_type":      filter.EventType,
		"status":          filter.Status,
		"transaction_ref": filter.TransactionRef,
	})
	_ = u.audits.Create(ctx, &entity.AuditLog{
		ID:          uuid.New(),
		ActorUserID: &actorID,
		Action:      "ADMIN_READ_WEBHOOKS",
		EntityType:  "webhook_delivery",
		Metadata:    datatypes.JSON(metadata),
		CreatedAt:   time.Now(),
	})

	return items, pagination.Meta(total, params), nil
}

func (u *AdminUsecase) GetWebhook(ctx context.Context, actorUserID, deliveryID string) (*WebhookDeliveryResponse, error) {
	actorID, err := uuid.Parse(actorUserID)
	if err != nil {
		return nil, apperror.BadRequest("invalid user id")
	}
	parsedDeliveryID, err := uuid.Parse(deliveryID)
	if err != nil {
		return nil, apperror.BadRequest("invalid webhook id")
	}

	delivery, err := u.webhooks.GetByID(ctx, parsedDeliveryID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apperror.NotFound("webhook delivery not found")
		}
		return nil, apperror.Internal(err)
	}

	metadata, _ := json.Marshal(map[string]interface{}{
		"delivery_id": parsedDeliveryID.String(),
	})
	_ = u.audits.Create(ctx, &entity.AuditLog{
		ID:          uuid.New(),
		ActorUserID: &actorID,
		Action:      "ADMIN_READ_WEBHOOK",
		EntityType:  "webhook_delivery",
		EntityID:    &parsedDeliveryID,
		Metadata:    datatypes.JSON(metadata),
		CreatedAt:   time.Now(),
	})

	response := toWebhookDeliveryResponse(delivery)
	return &response, nil
}

type HealthUsecase struct {
	health repository.HealthRepository
}

func NewHealthUsecase(health repository.HealthRepository) *HealthUsecase {
	return &HealthUsecase{health: health}
}

func (u *HealthUsecase) Check(ctx context.Context) error {
	if err := u.health.Ping(ctx); err != nil {
		return apperror.Internal(err)
	}
	return nil
}
