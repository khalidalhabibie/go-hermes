package usecase

import (
	"encoding/json"
	"time"

	"go-hermes/internal/entity"
)

type RegisterRequest struct {
	Name     string
	Email    string
	Password string
}

type LoginRequest struct {
	Email    string
	Password string
}

type TopUpRequest struct {
	UserID         string
	Amount         int64
	Description    *string
	IdempotencyKey string
	RequestHash    string
}

type TransferRequest struct {
	UserID            string
	RecipientWalletID string
	Amount            int64
	Description       *string
	IdempotencyKey    string
	RequestHash       string
}

type UserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type WalletResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Balance   int64     `json:"balance"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BalanceResponse struct {
	WalletID  string `json:"wallet_id"`
	Balance   int64  `json:"balance"`
	Currency  string `json:"currency"`
	UpdatedAt string `json:"updated_at"`
}

type RegisterResponse struct {
	User   UserResponse   `json:"user"`
	Wallet WalletResponse `json:"wallet"`
}

type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	ExpiresAt   time.Time    `json:"expires_at"`
	User        UserResponse `json:"user"`
}

type TransactionResponse struct {
	ID                  string    `json:"id"`
	TransactionRef      string    `json:"transaction_ref"`
	Type                string    `json:"type"`
	Status              string    `json:"status"`
	SourceWalletID      *string   `json:"source_wallet_id"`
	DestinationWalletID *string   `json:"destination_wallet_id"`
	Amount              int64     `json:"amount"`
	Currency            string    `json:"currency"`
	IdempotencyKey      string    `json:"idempotency_key"`
	InitiatedByUserID   string    `json:"initiated_by_user_id"`
	Description         *string   `json:"description"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type LedgerResponse struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	WalletID      string    `json:"wallet_id"`
	EntryType     string    `json:"entry_type"`
	Amount        int64     `json:"amount"`
	BalanceBefore int64     `json:"balance_before"`
	BalanceAfter  int64     `json:"balance_after"`
	CreatedAt     time.Time `json:"created_at"`
}

type AuditLogResponse struct {
	ID          string      `json:"id"`
	ActorUserID *string     `json:"actor_user_id"`
	Action      string      `json:"action"`
	EntityType  string      `json:"entity_type"`
	EntityID    *string     `json:"entity_id"`
	Metadata    interface{} `json:"metadata"`
	CreatedAt   time.Time   `json:"created_at"`
}

type WebhookDeliveryResponse struct {
	ID             string      `json:"id"`
	EventType      string      `json:"event_type"`
	TransactionID  *string     `json:"transaction_id"`
	TransactionRef string      `json:"transaction_ref"`
	TargetURL      string      `json:"target_url"`
	Payload        interface{} `json:"payload"`
	Status         string      `json:"status"`
	RetryCount     int         `json:"retry_count"`
	MaxRetry       int         `json:"max_retry"`
	LastError      *string     `json:"last_error"`
	LastHTTPStatus *int        `json:"last_http_status"`
	NextRetryAt    *time.Time  `json:"next_retry_at"`
	DeliveredAt    *time.Time  `json:"delivered_at"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type ReconciliationSummaryResponse struct {
	WalletsChecked             int `json:"wallets_checked"`
	TransactionsChecked        int `json:"transactions_checked"`
	LedgerEntriesChecked       int `json:"ledger_entries_checked"`
	WalletBalanceMismatchCount int `json:"wallet_balance_mismatch_count"`
	TransactionIssueCount      int `json:"transaction_issue_count"`
	LedgerIssueCount           int `json:"ledger_issue_count"`
	IssueCount                 int `json:"issue_count"`
}

type ReconciliationWalletIssueResponse struct {
	WalletID         string `json:"wallet_id"`
	UserID           string `json:"user_id"`
	StoredBalance    int64  `json:"stored_balance"`
	DerivedBalance   int64  `json:"derived_balance"`
	LedgerEntryCount int    `json:"ledger_entry_count"`
	Reason           string `json:"reason"`
}

type ReconciliationTransactionIssueResponse struct {
	TransactionID  string   `json:"transaction_id"`
	TransactionRef string   `json:"transaction_ref"`
	Type           string   `json:"type"`
	Amount         int64    `json:"amount"`
	LedgerEntryIDs []string `json:"ledger_entry_ids"`
	Reason         string   `json:"reason"`
}

type ReconciliationLedgerIssueResponse struct {
	LedgerEntryID string `json:"ledger_entry_id"`
	TransactionID string `json:"transaction_id"`
	WalletID      string `json:"wallet_id"`
	Reason        string `json:"reason"`
}

type ReconciliationResponse struct {
	CheckedAt     time.Time                                `json:"checked_at"`
	Healthy       bool                                     `json:"healthy"`
	Summary       ReconciliationSummaryResponse            `json:"summary"`
	Wallets       []ReconciliationWalletIssueResponse      `json:"wallets"`
	Transactions  []ReconciliationTransactionIssueResponse `json:"transactions"`
	LedgerEntries []ReconciliationLedgerIssueResponse      `json:"ledger_entries"`
}

type OperationResult struct {
	StatusCode int
	Body       []byte
	Replay     bool
}

func toUserResponse(user *entity.User) UserResponse {
	return UserResponse{
		ID:        user.ID.String(),
		Name:      user.Name,
		Email:     user.Email,
		Role:      string(user.Role),
		CreatedAt: user.CreatedAt,
	}
}

func toWalletResponse(wallet *entity.Wallet) WalletResponse {
	return WalletResponse{
		ID:        wallet.ID.String(),
		UserID:    wallet.UserID.String(),
		Balance:   wallet.Balance,
		Status:    string(wallet.Status),
		CreatedAt: wallet.CreatedAt,
		UpdatedAt: wallet.UpdatedAt,
	}
}

func toTransactionResponse(transaction *entity.Transaction) TransactionResponse {
	response := TransactionResponse{
		ID:                transaction.ID.String(),
		TransactionRef:    transaction.TransactionRef,
		Type:              string(transaction.Type),
		Status:            string(transaction.Status),
		Amount:            transaction.Amount,
		Currency:          transaction.Currency,
		IdempotencyKey:    transaction.IdempotencyKey,
		InitiatedByUserID: transaction.InitiatedByUserID.String(),
		Description:       transaction.Description,
		CreatedAt:         transaction.CreatedAt,
		UpdatedAt:         transaction.UpdatedAt,
	}

	if transaction.SourceWalletID != nil {
		value := transaction.SourceWalletID.String()
		response.SourceWalletID = &value
	}
	if transaction.DestinationWalletID != nil {
		value := transaction.DestinationWalletID.String()
		response.DestinationWalletID = &value
	}
	return response
}

func toLedgerResponse(entry *entity.LedgerEntry) LedgerResponse {
	return LedgerResponse{
		ID:            entry.ID.String(),
		TransactionID: entry.TransactionID.String(),
		WalletID:      entry.WalletID.String(),
		EntryType:     string(entry.EntryType),
		Amount:        entry.Amount,
		BalanceBefore: entry.BalanceBefore,
		BalanceAfter:  entry.BalanceAfter,
		CreatedAt:     entry.CreatedAt,
	}
}

func toWebhookDeliveryResponse(delivery *entity.WebhookDelivery) WebhookDeliveryResponse {
	var payload interface{}
	if len(delivery.Payload) > 0 {
		_ = json.Unmarshal(delivery.Payload, &payload)
	}

	response := WebhookDeliveryResponse{
		ID:             delivery.ID.String(),
		EventType:      delivery.EventType,
		TransactionRef: delivery.TransactionRef,
		TargetURL:      delivery.TargetURL,
		Payload:        payload,
		Status:         string(delivery.Status),
		RetryCount:     delivery.RetryCount,
		MaxRetry:       delivery.MaxRetry,
		LastError:      delivery.LastError,
		LastHTTPStatus: delivery.LastHTTPStatus,
		NextRetryAt:    delivery.NextRetryAt,
		DeliveredAt:    delivery.DeliveredAt,
		CreatedAt:      delivery.CreatedAt,
		UpdatedAt:      delivery.UpdatedAt,
	}
	if delivery.TransactionID != nil {
		transactionID := delivery.TransactionID.String()
		response.TransactionID = &transactionID
	}
	return response
}
