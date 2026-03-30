package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type WalletStatus string

const (
	WalletStatusActive WalletStatus = "ACTIVE"
)

type TransactionType string

const (
	TransactionTypeTopUp    TransactionType = "TOP_UP"
	TransactionTypeTransfer TransactionType = "TRANSFER"
)

type TransactionStatus string

const (
	TransactionStatusSuccess TransactionStatus = "SUCCESS"
	TransactionStatusFailed  TransactionStatus = "FAILED"
	TransactionStatusPending TransactionStatus = "PENDING"
)

type LedgerEntryType string

const (
	LedgerEntryTypeDebit  LedgerEntryType = "DEBIT"
	LedgerEntryTypeCredit LedgerEntryType = "CREDIT"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name         string    `gorm:"size:150;not null" json:"name"`
	Email        string    `gorm:"size:150;uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"column:password_hash;not null" json:"-"`
	Role         Role      `gorm:"size:20;not null" json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Wallet struct {
	ID        uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID    `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	Balance   int64        `gorm:"not null" json:"balance"`
	Status    WalletStatus `gorm:"size:20;not null" json:"status"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type Transaction struct {
	ID                  uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	TransactionRef      string            `gorm:"size:100;uniqueIndex;not null" json:"transaction_ref"`
	Type                TransactionType   `gorm:"size:20;not null" json:"type"`
	Status              TransactionStatus `gorm:"size:20;not null" json:"status"`
	SourceWalletID      *uuid.UUID        `gorm:"type:uuid" json:"source_wallet_id"`
	DestinationWalletID *uuid.UUID        `gorm:"type:uuid" json:"destination_wallet_id"`
	Amount              int64             `gorm:"not null" json:"amount"`
	Currency            string            `gorm:"size:10;not null" json:"currency"`
	IdempotencyKey      string            `gorm:"size:255;not null" json:"idempotency_key"`
	InitiatedByUserID   uuid.UUID         `gorm:"type:uuid;not null" json:"initiated_by_user_id"`
	Description         *string           `gorm:"size:255" json:"description"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

type LedgerEntry struct {
	ID            uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	TransactionID uuid.UUID       `gorm:"type:uuid;index;not null" json:"transaction_id"`
	WalletID      uuid.UUID       `gorm:"type:uuid;index;not null" json:"wallet_id"`
	EntryType     LedgerEntryType `gorm:"size:20;not null" json:"entry_type"`
	Amount        int64           `gorm:"not null" json:"amount"`
	BalanceBefore int64           `gorm:"not null" json:"balance_before"`
	BalanceAfter  int64           `gorm:"not null" json:"balance_after"`
	CreatedAt     time.Time       `json:"created_at"`
}

type IdempotencyRecord struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	IdempotencyKey string         `gorm:"size:255;not null" json:"idempotency_key"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	RequestHash    string         `gorm:"size:255;not null" json:"request_hash"`
	Endpoint       string         `gorm:"size:255;not null" json:"endpoint"`
	StatusCode     int            `gorm:"not null" json:"status_code"`
	ResponseBody   datatypes.JSON `gorm:"type:jsonb" json:"response_body"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type AuditLog struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	ActorUserID *uuid.UUID     `gorm:"type:uuid" json:"actor_user_id"`
	Action      string         `gorm:"size:100;not null" json:"action"`
	EntityType  string         `gorm:"size:100;not null" json:"entity_type"`
	EntityID    *uuid.UUID     `gorm:"type:uuid" json:"entity_id"`
	Metadata    datatypes.JSON `gorm:"type:jsonb" json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

type WebhookDeliveryStatus string

const (
	WebhookDeliveryStatusPending  WebhookDeliveryStatus = "PENDING"
	WebhookDeliveryStatusSuccess  WebhookDeliveryStatus = "SUCCESS"
	WebhookDeliveryStatusFailed   WebhookDeliveryStatus = "FAILED"
	WebhookDeliveryStatusRetrying WebhookDeliveryStatus = "RETRYING"
)

type WebhookDelivery struct {
	ID             uuid.UUID             `gorm:"type:uuid;primaryKey" json:"id"`
	EventType      string                `gorm:"size:100;index;not null" json:"event_type"`
	TransactionID  *uuid.UUID            `gorm:"type:uuid;index" json:"transaction_id"`
	TransactionRef string                `gorm:"size:100;index" json:"transaction_ref"`
	TargetURL      string                `gorm:"size:500;not null" json:"target_url"`
	Payload        datatypes.JSON        `gorm:"type:jsonb;not null" json:"payload"`
	Secret         *string               `gorm:"size:255" json:"-"`
	Status         WebhookDeliveryStatus `gorm:"size:20;index;not null" json:"status"`
	RetryCount     int                   `gorm:"not null" json:"retry_count"`
	MaxRetry       int                   `gorm:"not null" json:"max_retry"`
	LastError      *string               `gorm:"type:text" json:"last_error"`
	LastHTTPStatus *int                  `json:"last_http_status"`
	NextRetryAt    *time.Time            `gorm:"index" json:"next_retry_at"`
	DeliveredAt    *time.Time            `json:"delivered_at"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}
