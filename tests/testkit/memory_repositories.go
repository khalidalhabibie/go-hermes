package testkit

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/pagination"
	"go-hermes/internal/repository"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type MemoryStore struct {
	mu                 sync.Mutex
	users              map[uuid.UUID]entity.User
	usersByEmail       map[string]uuid.UUID
	wallets            map[uuid.UUID]entity.Wallet
	walletsByUserID    map[uuid.UUID]uuid.UUID
	transactions       map[uuid.UUID]entity.Transaction
	transactionOrder   []uuid.UUID
	ledgerEntries      map[uuid.UUID]entity.LedgerEntry
	ledgerOrder        []uuid.UUID
	idempotencyRecords map[string]entity.IdempotencyRecord
	auditLogs          map[uuid.UUID]entity.AuditLog
	auditOrder         []uuid.UUID
	webhookDeliveries  map[uuid.UUID]entity.WebhookDelivery
	webhookOrder       []uuid.UUID
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:              make(map[uuid.UUID]entity.User),
		usersByEmail:       make(map[string]uuid.UUID),
		wallets:            make(map[uuid.UUID]entity.Wallet),
		walletsByUserID:    make(map[uuid.UUID]uuid.UUID),
		transactions:       make(map[uuid.UUID]entity.Transaction),
		ledgerEntries:      make(map[uuid.UUID]entity.LedgerEntry),
		idempotencyRecords: make(map[string]entity.IdempotencyRecord),
		auditLogs:          make(map[uuid.UUID]entity.AuditLog),
		webhookDeliveries:  make(map[uuid.UUID]entity.WebhookDelivery),
	}
}

func (s *MemoryStore) TransactionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.transactions)
}

func (s *MemoryStore) UserCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.users)
}

func (s *MemoryStore) WalletCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.wallets)
}

func (s *MemoryStore) LedgerCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ledgerEntries)
}

func (s *MemoryStore) WebhookCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.webhookDeliveries)
}

func (s *MemoryStore) WebhookByID(id uuid.UUID) (entity.WebhookDelivery, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delivery, ok := s.webhookDeliveries[id]
	return delivery, ok
}

func (s *MemoryStore) WebhookIDs() []uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]uuid.UUID, len(s.webhookOrder))
	copy(ids, s.webhookOrder)
	return ids
}

func (s *MemoryStore) WalletByUserID(userID uuid.UUID) (entity.Wallet, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	walletID, ok := s.walletsByUserID[userID]
	if !ok {
		return entity.Wallet{}, false
	}
	wallet, exists := s.wallets[walletID]
	return wallet, exists
}

func cloneUser(user entity.User) *entity.User {
	cloned := user
	return &cloned
}

func cloneWallet(wallet entity.Wallet) *entity.Wallet {
	cloned := wallet
	return &cloned
}

func cloneTransaction(transaction entity.Transaction) *entity.Transaction {
	cloned := transaction
	return &cloned
}

type MemoryTransactionManager struct{}

func (m *MemoryTransactionManager) WithinTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

type MemoryUserRepository struct {
	store *MemoryStore
}

func NewMemoryUserRepository(store *MemoryStore) *MemoryUserRepository {
	return &MemoryUserRepository{store: store}
}

func (r *MemoryUserRepository) Create(_ context.Context, user *entity.User) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	if _, exists := r.store.usersByEmail[user.Email]; exists {
		return errors.New("duplicate email")
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = user.CreatedAt
	}

	r.store.users[user.ID] = *cloneUser(*user)
	r.store.usersByEmail[user.Email] = user.ID
	return nil
}

func (r *MemoryUserRepository) GetByEmail(_ context.Context, email string) (*entity.User, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	userID, ok := r.store.usersByEmail[email]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return cloneUser(r.store.users[userID]), nil
}

func (r *MemoryUserRepository) GetByID(_ context.Context, id uuid.UUID) (*entity.User, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	user, ok := r.store.users[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return cloneUser(user), nil
}

type MemoryWalletRepository struct {
	store *MemoryStore
}

func NewMemoryWalletRepository(store *MemoryStore) *MemoryWalletRepository {
	return &MemoryWalletRepository{store: store}
}

func (r *MemoryWalletRepository) Create(_ context.Context, wallet *entity.Wallet) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	if _, exists := r.store.walletsByUserID[wallet.UserID]; exists {
		return errors.New("duplicate wallet")
	}
	if wallet.CreatedAt.IsZero() {
		wallet.CreatedAt = time.Now()
	}
	if wallet.UpdatedAt.IsZero() {
		wallet.UpdatedAt = wallet.CreatedAt
	}

	r.store.wallets[wallet.ID] = *cloneWallet(*wallet)
	r.store.walletsByUserID[wallet.UserID] = wallet.ID
	return nil
}

func (r *MemoryWalletRepository) GetByUserID(_ context.Context, userID uuid.UUID) (*entity.Wallet, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	walletID, ok := r.store.walletsByUserID[userID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return cloneWallet(r.store.wallets[walletID]), nil
}

func (r *MemoryWalletRepository) GetByID(_ context.Context, id uuid.UUID) (*entity.Wallet, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	wallet, ok := r.store.wallets[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return cloneWallet(wallet), nil
}

func (r *MemoryWalletRepository) LockByIDs(_ context.Context, ids []uuid.UUID) ([]entity.Wallet, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	sortedIDs := append([]uuid.UUID(nil), ids...)
	sort.Slice(sortedIDs, func(i, j int) bool {
		return sortedIDs[i].String() < sortedIDs[j].String()
	})

	wallets := make([]entity.Wallet, 0, len(sortedIDs))
	for _, id := range sortedIDs {
		wallet, ok := r.store.wallets[id]
		if !ok {
			return nil, gorm.ErrRecordNotFound
		}
		wallets = append(wallets, wallet)
	}
	return wallets, nil
}

func (r *MemoryWalletRepository) Update(_ context.Context, wallet *entity.Wallet) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	if _, ok := r.store.wallets[wallet.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	wallet.UpdatedAt = time.Now()
	r.store.wallets[wallet.ID] = *cloneWallet(*wallet)
	r.store.walletsByUserID[wallet.UserID] = wallet.ID
	return nil
}

type MemoryTransactionRepository struct {
	store *MemoryStore
}

func NewMemoryTransactionRepository(store *MemoryStore) *MemoryTransactionRepository {
	return &MemoryTransactionRepository{store: store}
}

func (r *MemoryTransactionRepository) Create(_ context.Context, transaction *entity.Transaction) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	if transaction.CreatedAt.IsZero() {
		transaction.CreatedAt = time.Now()
	}
	if transaction.UpdatedAt.IsZero() {
		transaction.UpdatedAt = transaction.CreatedAt
	}
	r.store.transactions[transaction.ID] = *cloneTransaction(*transaction)
	r.store.transactionOrder = append(r.store.transactionOrder, transaction.ID)
	return nil
}

func (r *MemoryTransactionRepository) ListByUser(_ context.Context, userID uuid.UUID, params pagination.Params) ([]entity.Transaction, int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	seen := make(map[uuid.UUID]struct{})
	filtered := make([]entity.Transaction, 0)
	for _, transactionID := range r.store.transactionOrder {
		transaction := r.store.transactions[transactionID]
		if r.transactionVisibleToUser(transaction, userID) {
			if _, exists := seen[transaction.ID]; !exists {
				filtered = append(filtered, transaction)
				seen[transaction.ID] = struct{}{}
			}
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := int64(len(filtered))
	start := params.Offset()
	if start >= len(filtered) {
		return []entity.Transaction{}, total, nil
	}
	end := start + params.Limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return append([]entity.Transaction(nil), filtered[start:end]...), total, nil
}

func (r *MemoryTransactionRepository) GetByIDForUser(_ context.Context, transactionID, userID uuid.UUID) (*entity.Transaction, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	transaction, ok := r.store.transactions[transactionID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	if !r.transactionVisibleToUser(transaction, userID) {
		return nil, gorm.ErrRecordNotFound
	}
	return cloneTransaction(transaction), nil
}

func (r *MemoryTransactionRepository) ListAll(_ context.Context, params pagination.Params) ([]entity.Transaction, int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	items := make([]entity.Transaction, 0, len(r.store.transactions))
	for _, transactionID := range r.store.transactionOrder {
		items = append(items, r.store.transactions[transactionID])
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	total := int64(len(items))
	start := params.Offset()
	if start >= len(items) {
		return []entity.Transaction{}, total, nil
	}
	end := start + params.Limit
	if end > len(items) {
		end = len(items)
	}
	return append([]entity.Transaction(nil), items[start:end]...), total, nil
}

func (r *MemoryTransactionRepository) GetByID(_ context.Context, transactionID uuid.UUID) (*entity.Transaction, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	transaction, ok := r.store.transactions[transactionID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return cloneTransaction(transaction), nil
}

func (r *MemoryTransactionRepository) transactionVisibleToUser(transaction entity.Transaction, userID uuid.UUID) bool {
	if transaction.InitiatedByUserID == userID {
		return true
	}
	if transaction.SourceWalletID != nil {
		if wallet, ok := r.store.wallets[*transaction.SourceWalletID]; ok && wallet.UserID == userID {
			return true
		}
	}
	if transaction.DestinationWalletID != nil {
		if wallet, ok := r.store.wallets[*transaction.DestinationWalletID]; ok && wallet.UserID == userID {
			return true
		}
	}
	return false
}

type MemoryLedgerRepository struct {
	store *MemoryStore
}

func NewMemoryLedgerRepository(store *MemoryStore) *MemoryLedgerRepository {
	return &MemoryLedgerRepository{store: store}
}

func (r *MemoryLedgerRepository) CreateMany(_ context.Context, entries []entity.LedgerEntry) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	for _, entry := range entries {
		if entry.CreatedAt.IsZero() {
			entry.CreatedAt = time.Now()
		}
		r.store.ledgerEntries[entry.ID] = entry
		r.store.ledgerOrder = append(r.store.ledgerOrder, entry.ID)
	}
	return nil
}

func (r *MemoryLedgerRepository) ListByUser(_ context.Context, userID uuid.UUID, params pagination.Params) ([]entity.LedgerEntry, int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	items := make([]entity.LedgerEntry, 0)
	for _, entryID := range r.store.ledgerOrder {
		entry := r.store.ledgerEntries[entryID]
		wallet, ok := r.store.wallets[entry.WalletID]
		if ok && wallet.UserID == userID {
			items = append(items, entry)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	total := int64(len(items))
	start := params.Offset()
	if start >= len(items) {
		return []entity.LedgerEntry{}, total, nil
	}
	end := start + params.Limit
	if end > len(items) {
		end = len(items)
	}
	return append([]entity.LedgerEntry(nil), items[start:end]...), total, nil
}

func (r *MemoryLedgerRepository) ListByTransactionForUser(_ context.Context, transactionID, userID uuid.UUID) ([]entity.LedgerEntry, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	items := make([]entity.LedgerEntry, 0)
	for _, entryID := range r.store.ledgerOrder {
		entry := r.store.ledgerEntries[entryID]
		if entry.TransactionID != transactionID {
			continue
		}
		wallet, ok := r.store.wallets[entry.WalletID]
		if ok && wallet.UserID == userID {
			items = append(items, entry)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, nil
}

type MemoryIdempotencyRepository struct {
	store *MemoryStore
}

func NewMemoryIdempotencyRepository(store *MemoryStore) *MemoryIdempotencyRepository {
	return &MemoryIdempotencyRepository{store: store}
}

func (r *MemoryIdempotencyRepository) Reserve(_ context.Context, record *entity.IdempotencyRecord) (bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	key := r.compositeKey(record.IdempotencyKey, record.UserID, record.Endpoint)
	if _, exists := r.store.idempotencyRecords[key]; exists {
		return false, nil
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	record.UpdatedAt = record.CreatedAt
	r.store.idempotencyRecords[key] = *record
	return true, nil
}

func (r *MemoryIdempotencyRepository) Get(_ context.Context, key string, userID uuid.UUID, endpoint string) (*entity.IdempotencyRecord, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	record, ok := r.store.idempotencyRecords[r.compositeKey(key, userID, endpoint)]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cloned := record
	return &cloned, nil
}

func (r *MemoryIdempotencyRepository) MarkCompleted(_ context.Context, recordID uuid.UUID, statusCode int, responseBody []byte) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	for key, record := range r.store.idempotencyRecords {
		if record.ID == recordID {
			record.StatusCode = statusCode
			record.ResponseBody = datatypes.JSON(responseBody)
			record.UpdatedAt = time.Now()
			r.store.idempotencyRecords[key] = record
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *MemoryIdempotencyRepository) compositeKey(key string, userID uuid.UUID, endpoint string) string {
	return userID.String() + ":" + endpoint + ":" + key
}

type MemoryAuditLogRepository struct {
	store *MemoryStore
}

func NewMemoryAuditLogRepository(store *MemoryStore) *MemoryAuditLogRepository {
	return &MemoryAuditLogRepository{store: store}
}

func (r *MemoryAuditLogRepository) Create(_ context.Context, log *entity.AuditLog) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	r.store.auditLogs[log.ID] = *log
	r.store.auditOrder = append(r.store.auditOrder, log.ID)
	return nil
}

func (r *MemoryAuditLogRepository) List(_ context.Context, params pagination.Params) ([]entity.AuditLog, int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	items := make([]entity.AuditLog, 0, len(r.store.auditLogs))
	for _, logID := range r.store.auditOrder {
		items = append(items, r.store.auditLogs[logID])
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	total := int64(len(items))
	start := params.Offset()
	if start >= len(items) {
		return []entity.AuditLog{}, total, nil
	}
	end := start + params.Limit
	if end > len(items) {
		end = len(items)
	}
	return append([]entity.AuditLog(nil), items[start:end]...), total, nil
}

type MemoryHealthRepository struct{}

func (r *MemoryHealthRepository) Ping(_ context.Context) error {
	return nil
}

type MemoryReconciliationRepository struct {
	store *MemoryStore
}

func NewMemoryReconciliationRepository(store *MemoryStore) *MemoryReconciliationRepository {
	return &MemoryReconciliationRepository{store: store}
}

func (r *MemoryReconciliationRepository) ListWallets(_ context.Context) ([]entity.Wallet, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	items := make([]entity.Wallet, 0, len(r.store.wallets))
	for _, walletID := range sortedWalletIDs(r.store.wallets) {
		items = append(items, r.store.wallets[walletID])
	}
	return items, nil
}

func (r *MemoryReconciliationRepository) ListTransactions(_ context.Context) ([]entity.Transaction, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	items := make([]entity.Transaction, 0, len(r.store.transactionOrder))
	for _, transactionID := range r.store.transactionOrder {
		items = append(items, r.store.transactions[transactionID])
	}
	return items, nil
}

func (r *MemoryReconciliationRepository) ListLedgerEntries(_ context.Context) ([]entity.LedgerEntry, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	items := make([]entity.LedgerEntry, 0, len(r.store.ledgerOrder))
	for _, entryID := range r.store.ledgerOrder {
		items = append(items, r.store.ledgerEntries[entryID])
	}
	return items, nil
}

type MemoryWebhookDeliveryRepository struct {
	store *MemoryStore
}

func NewMemoryWebhookDeliveryRepository(store *MemoryStore) *MemoryWebhookDeliveryRepository {
	return &MemoryWebhookDeliveryRepository{store: store}
}

func (r *MemoryWebhookDeliveryRepository) Create(_ context.Context, delivery *entity.WebhookDelivery) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	now := time.Now()
	if delivery.CreatedAt.IsZero() {
		delivery.CreatedAt = now
	}
	if delivery.UpdatedAt.IsZero() {
		delivery.UpdatedAt = delivery.CreatedAt
	}
	r.store.webhookDeliveries[delivery.ID] = *delivery
	r.store.webhookOrder = append(r.store.webhookOrder, delivery.ID)
	return nil
}

func (r *MemoryWebhookDeliveryRepository) Update(_ context.Context, delivery *entity.WebhookDelivery) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	if _, ok := r.store.webhookDeliveries[delivery.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	delivery.UpdatedAt = time.Now()
	r.store.webhookDeliveries[delivery.ID] = *delivery
	return nil
}

func (r *MemoryWebhookDeliveryRepository) GetByID(_ context.Context, id uuid.UUID) (*entity.WebhookDelivery, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	delivery, ok := r.store.webhookDeliveries[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cloned := delivery
	return &cloned, nil
}

func (r *MemoryWebhookDeliveryRepository) List(_ context.Context, filter repository.WebhookDeliveryFilter, params pagination.Params) ([]entity.WebhookDelivery, int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	items := make([]entity.WebhookDelivery, 0)
	for _, deliveryID := range r.store.webhookOrder {
		delivery := r.store.webhookDeliveries[deliveryID]
		if filter.EventType != "" && delivery.EventType != filter.EventType {
			continue
		}
		if filter.Status != "" && string(delivery.Status) != filter.Status {
			continue
		}
		if filter.TransactionRef != "" && delivery.TransactionRef != filter.TransactionRef {
			continue
		}
		items = append(items, delivery)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	total := int64(len(items))
	start := params.Offset()
	if start >= len(items) {
		return []entity.WebhookDelivery{}, total, nil
	}
	end := start + params.Limit
	if end > len(items) {
		end = len(items)
	}
	return append([]entity.WebhookDelivery(nil), items[start:end]...), total, nil
}

func (r *MemoryWebhookDeliveryRepository) ListDue(_ context.Context, now time.Time, limit int) ([]entity.WebhookDelivery, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	items := make([]entity.WebhookDelivery, 0)
	for _, deliveryID := range r.store.webhookOrder {
		delivery := r.store.webhookDeliveries[deliveryID]
		if delivery.Status != entity.WebhookDeliveryStatusPending && delivery.Status != entity.WebhookDeliveryStatusRetrying {
			continue
		}
		if delivery.NextRetryAt != nil && delivery.NextRetryAt.After(now) {
			continue
		}
		items = append(items, delivery)
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

type MemoryRepositories struct {
	Store          *MemoryStore
	TxManager      repository.TransactionManager
	Users          repository.UserRepository
	Wallets        repository.WalletRepository
	Transactions   repository.TransactionRepository
	Ledgers        repository.LedgerRepository
	Idempotency    repository.IdempotencyRepository
	Audits         repository.AuditLogRepository
	Health         repository.HealthRepository
	Reconciliation repository.ReconciliationRepository
	Webhooks       repository.WebhookDeliveryRepository
}

func NewMemoryRepositories() *MemoryRepositories {
	store := NewMemoryStore()
	return &MemoryRepositories{
		Store:          store,
		TxManager:      &MemoryTransactionManager{},
		Users:          NewMemoryUserRepository(store),
		Wallets:        NewMemoryWalletRepository(store),
		Transactions:   NewMemoryTransactionRepository(store),
		Ledgers:        NewMemoryLedgerRepository(store),
		Idempotency:    NewMemoryIdempotencyRepository(store),
		Audits:         NewMemoryAuditLogRepository(store),
		Health:         &MemoryHealthRepository{},
		Reconciliation: NewMemoryReconciliationRepository(store),
		Webhooks:       NewMemoryWebhookDeliveryRepository(store),
	}
}

func sortedWalletIDs(wallets map[uuid.UUID]entity.Wallet) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(wallets))
	for walletID := range wallets {
		ids = append(ids, walletID)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i].String() < ids[j].String()
	})
	return ids
}

var (
	_ repository.TransactionManager        = (*MemoryTransactionManager)(nil)
	_ repository.UserRepository            = (*MemoryUserRepository)(nil)
	_ repository.WalletRepository          = (*MemoryWalletRepository)(nil)
	_ repository.TransactionRepository     = (*MemoryTransactionRepository)(nil)
	_ repository.LedgerRepository          = (*MemoryLedgerRepository)(nil)
	_ repository.IdempotencyRepository     = (*MemoryIdempotencyRepository)(nil)
	_ repository.AuditLogRepository        = (*MemoryAuditLogRepository)(nil)
	_ repository.HealthRepository          = (*MemoryHealthRepository)(nil)
	_ repository.ReconciliationRepository  = (*MemoryReconciliationRepository)(nil)
	_ repository.WebhookDeliveryRepository = (*MemoryWebhookDeliveryRepository)(nil)
)
