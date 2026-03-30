package repository

import (
	"context"
	"time"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/pagination"

	"github.com/google/uuid"
)

type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
}

type WalletRepository interface {
	Create(ctx context.Context, wallet *entity.Wallet) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*entity.Wallet, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Wallet, error)
	LockByIDs(ctx context.Context, ids []uuid.UUID) ([]entity.Wallet, error)
	Update(ctx context.Context, wallet *entity.Wallet) error
}

type TransactionRepository interface {
	Create(ctx context.Context, transaction *entity.Transaction) error
	ListByUser(ctx context.Context, userID uuid.UUID, params pagination.Params) ([]entity.Transaction, int64, error)
	GetByIDForUser(ctx context.Context, transactionID, userID uuid.UUID) (*entity.Transaction, error)
	ListAll(ctx context.Context, params pagination.Params) ([]entity.Transaction, int64, error)
	GetByID(ctx context.Context, transactionID uuid.UUID) (*entity.Transaction, error)
}

type LedgerRepository interface {
	CreateMany(ctx context.Context, entries []entity.LedgerEntry) error
	ListByUser(ctx context.Context, userID uuid.UUID, params pagination.Params) ([]entity.LedgerEntry, int64, error)
	ListByTransactionForUser(ctx context.Context, transactionID, userID uuid.UUID) ([]entity.LedgerEntry, error)
}

type IdempotencyRepository interface {
	Reserve(ctx context.Context, record *entity.IdempotencyRecord) (bool, error)
	Get(ctx context.Context, key string, userID uuid.UUID, endpoint string) (*entity.IdempotencyRecord, error)
	MarkCompleted(ctx context.Context, recordID uuid.UUID, statusCode int, responseBody []byte) error
}

type AuditLogRepository interface {
	Create(ctx context.Context, log *entity.AuditLog) error
	List(ctx context.Context, params pagination.Params) ([]entity.AuditLog, int64, error)
}

type HealthRepository interface {
	Ping(ctx context.Context) error
}

type ReconciliationRepository interface {
	ListWallets(ctx context.Context) ([]entity.Wallet, error)
	ListTransactions(ctx context.Context) ([]entity.Transaction, error)
	ListLedgerEntries(ctx context.Context) ([]entity.LedgerEntry, error)
}

type WebhookDeliveryFilter struct {
	EventType      string
	Status         string
	TransactionRef string
}

type WebhookDeliveryRepository interface {
	Create(ctx context.Context, delivery *entity.WebhookDelivery) error
	Update(ctx context.Context, delivery *entity.WebhookDelivery) error
	GetByID(ctx context.Context, id uuid.UUID) (*entity.WebhookDelivery, error)
	List(ctx context.Context, filter WebhookDeliveryFilter, params pagination.Params) ([]entity.WebhookDelivery, int64, error)
	ListDue(ctx context.Context, now time.Time, limit int) ([]entity.WebhookDelivery, error)
}
