package repository

import (
	"context"
	"errors"
	"sort"
	"time"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/pagination"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type txContextKey struct{}

type GormTransactionManager struct {
	db *gorm.DB
}

func NewTransactionManager(db *gorm.DB) *GormTransactionManager {
	return &GormTransactionManager{db: db}
}

func (m *GormTransactionManager) WithinTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txContextKey{}, tx)
		return fn(txCtx)
	})
}

type baseRepository struct {
	db *gorm.DB
}

func (r *baseRepository) dbFromContext(ctx context.Context) *gorm.DB {
	tx, ok := ctx.Value(txContextKey{}).(*gorm.DB)
	if ok && tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

type GormUserRepository struct {
	baseRepository
}

func NewUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{baseRepository{db: db}}
}

func (r *GormUserRepository) Create(ctx context.Context, user *entity.User) error {
	return r.dbFromContext(ctx).Create(user).Error
}

func (r *GormUserRepository) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	var user entity.User
	if err := r.dbFromContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	var user entity.User
	if err := r.dbFromContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

type GormWalletRepository struct {
	baseRepository
}

func NewWalletRepository(db *gorm.DB) *GormWalletRepository {
	return &GormWalletRepository{baseRepository{db: db}}
}

func (r *GormWalletRepository) Create(ctx context.Context, wallet *entity.Wallet) error {
	return r.dbFromContext(ctx).Create(wallet).Error
}

func (r *GormWalletRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*entity.Wallet, error) {
	var wallet entity.Wallet
	if err := r.dbFromContext(ctx).Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (r *GormWalletRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Wallet, error) {
	var wallet entity.Wallet
	if err := r.dbFromContext(ctx).Where("id = ?", id).First(&wallet).Error; err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (r *GormWalletRepository) LockByIDs(ctx context.Context, ids []uuid.UUID) ([]entity.Wallet, error) {
	sortedIDs := append([]uuid.UUID(nil), ids...)
	sort.Slice(sortedIDs, func(i, j int) bool {
		return sortedIDs[i].String() < sortedIDs[j].String()
	})

	var wallets []entity.Wallet
	err := r.dbFromContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", sortedIDs).
		Order("id ASC").
		Find(&wallets).Error
	return wallets, err
}

func (r *GormWalletRepository) Update(ctx context.Context, wallet *entity.Wallet) error {
	return r.dbFromContext(ctx).Save(wallet).Error
}

type GormTransactionRepository struct {
	baseRepository
}

func NewTransactionRepository(db *gorm.DB) *GormTransactionRepository {
	return &GormTransactionRepository{baseRepository{db: db}}
}

func (r *GormTransactionRepository) Create(ctx context.Context, transaction *entity.Transaction) error {
	return r.dbFromContext(ctx).Create(transaction).Error
}

func (r *GormTransactionRepository) ListByUser(ctx context.Context, userID uuid.UUID, params pagination.Params) ([]entity.Transaction, int64, error) {
	query := r.dbFromContext(ctx).
		Model(&entity.Transaction{}).
		Joins("LEFT JOIN wallets sw ON sw.id = transactions.source_wallet_id").
		Joins("LEFT JOIN wallets dw ON dw.id = transactions.destination_wallet_id").
		Where("transactions.initiated_by_user_id = ? OR sw.user_id = ? OR dw.user_id = ?", userID, userID, userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var transactions []entity.Transaction
	err := query.
		Order("transactions.created_at DESC").
		Offset(params.Offset()).
		Limit(params.Limit).
		Find(&transactions).Error
	return transactions, total, err
}

func (r *GormTransactionRepository) GetByIDForUser(ctx context.Context, transactionID, userID uuid.UUID) (*entity.Transaction, error) {
	var transaction entity.Transaction
	err := r.dbFromContext(ctx).
		Model(&entity.Transaction{}).
		Joins("LEFT JOIN wallets sw ON sw.id = transactions.source_wallet_id").
		Joins("LEFT JOIN wallets dw ON dw.id = transactions.destination_wallet_id").
		Where("transactions.id = ?", transactionID).
		Where("transactions.initiated_by_user_id = ? OR sw.user_id = ? OR dw.user_id = ?", userID, userID, userID).
		First(&transaction).Error
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (r *GormTransactionRepository) ListAll(ctx context.Context, params pagination.Params) ([]entity.Transaction, int64, error) {
	query := r.dbFromContext(ctx).Model(&entity.Transaction{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var transactions []entity.Transaction
	err := query.
		Order("created_at DESC").
		Offset(params.Offset()).
		Limit(params.Limit).
		Find(&transactions).Error
	return transactions, total, err
}

func (r *GormTransactionRepository) GetByID(ctx context.Context, transactionID uuid.UUID) (*entity.Transaction, error) {
	var transaction entity.Transaction
	if err := r.dbFromContext(ctx).Where("id = ?", transactionID).First(&transaction).Error; err != nil {
		return nil, err
	}
	return &transaction, nil
}

type GormLedgerRepository struct {
	baseRepository
}

func NewLedgerRepository(db *gorm.DB) *GormLedgerRepository {
	return &GormLedgerRepository{baseRepository{db: db}}
}

func (r *GormLedgerRepository) CreateMany(ctx context.Context, entries []entity.LedgerEntry) error {
	return r.dbFromContext(ctx).Create(&entries).Error
}

func (r *GormLedgerRepository) ListByUser(ctx context.Context, userID uuid.UUID, params pagination.Params) ([]entity.LedgerEntry, int64, error) {
	query := r.dbFromContext(ctx).
		Model(&entity.LedgerEntry{}).
		Joins("JOIN wallets ON wallets.id = ledger_entries.wallet_id").
		Where("wallets.user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var entries []entity.LedgerEntry
	err := query.
		Order("ledger_entries.created_at DESC").
		Offset(params.Offset()).
		Limit(params.Limit).
		Find(&entries).Error
	return entries, total, err
}

func (r *GormLedgerRepository) ListByTransactionForUser(ctx context.Context, transactionID, userID uuid.UUID) ([]entity.LedgerEntry, error) {
	var entries []entity.LedgerEntry
	err := r.dbFromContext(ctx).
		Model(&entity.LedgerEntry{}).
		Joins("JOIN wallets ON wallets.id = ledger_entries.wallet_id").
		Where("ledger_entries.transaction_id = ?", transactionID).
		Where("wallets.user_id = ?", userID).
		Order("ledger_entries.created_at ASC").
		Find(&entries).Error
	return entries, err
}

type GormIdempotencyRepository struct {
	baseRepository
}

func NewIdempotencyRepository(db *gorm.DB) *GormIdempotencyRepository {
	return &GormIdempotencyRepository{baseRepository{db: db}}
}

func (r *GormIdempotencyRepository) Reserve(ctx context.Context, record *entity.IdempotencyRecord) (bool, error) {
	result := r.dbFromContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "idempotency_key"}, {Name: "user_id"}, {Name: "endpoint"}},
			DoNothing: true,
		}).
		Create(record)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (r *GormIdempotencyRepository) Get(ctx context.Context, key string, userID uuid.UUID, endpoint string) (*entity.IdempotencyRecord, error) {
	var record entity.IdempotencyRecord
	if err := r.dbFromContext(ctx).
		Where("idempotency_key = ? AND user_id = ? AND endpoint = ?", key, userID, endpoint).
		First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *GormIdempotencyRepository) MarkCompleted(ctx context.Context, recordID uuid.UUID, statusCode int, responseBody []byte) error {
	return r.dbFromContext(ctx).
		Model(&entity.IdempotencyRecord{}).
		Where("id = ?", recordID).
		Updates(map[string]interface{}{
			"status_code":   statusCode,
			"response_body": datatypes.JSON(responseBody),
		}).Error
}

type GormAuditLogRepository struct {
	baseRepository
}

func NewAuditLogRepository(db *gorm.DB) *GormAuditLogRepository {
	return &GormAuditLogRepository{baseRepository{db: db}}
}

func (r *GormAuditLogRepository) Create(ctx context.Context, log *entity.AuditLog) error {
	return r.dbFromContext(ctx).Create(log).Error
}

func (r *GormAuditLogRepository) List(ctx context.Context, params pagination.Params) ([]entity.AuditLog, int64, error) {
	query := r.dbFromContext(ctx).Model(&entity.AuditLog{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []entity.AuditLog
	err := query.
		Order("created_at DESC").
		Offset(params.Offset()).
		Limit(params.Limit).
		Find(&logs).Error
	return logs, total, err
}

type GormHealthRepository struct {
	baseRepository
}

func NewHealthRepository(db *gorm.DB) *GormHealthRepository {
	return &GormHealthRepository{baseRepository{db: db}}
}

func (r *GormHealthRepository) Ping(ctx context.Context) error {
	sqlDB, err := r.dbFromContext(ctx).DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

type GormWebhookDeliveryRepository struct {
	baseRepository
}

func NewWebhookDeliveryRepository(db *gorm.DB) *GormWebhookDeliveryRepository {
	return &GormWebhookDeliveryRepository{baseRepository{db: db}}
}

func (r *GormWebhookDeliveryRepository) Create(ctx context.Context, delivery *entity.WebhookDelivery) error {
	return r.dbFromContext(ctx).Create(delivery).Error
}

func (r *GormWebhookDeliveryRepository) Update(ctx context.Context, delivery *entity.WebhookDelivery) error {
	return r.dbFromContext(ctx).Save(delivery).Error
}

func (r *GormWebhookDeliveryRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.WebhookDelivery, error) {
	var delivery entity.WebhookDelivery
	if err := r.dbFromContext(ctx).Where("id = ?", id).First(&delivery).Error; err != nil {
		return nil, err
	}
	return &delivery, nil
}

func (r *GormWebhookDeliveryRepository) List(ctx context.Context, filter WebhookDeliveryFilter, params pagination.Params) ([]entity.WebhookDelivery, int64, error) {
	query := r.dbFromContext(ctx).Model(&entity.WebhookDelivery{})

	if filter.EventType != "" {
		query = query.Where("event_type = ?", filter.EventType)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.TransactionRef != "" {
		query = query.Where("transaction_ref = ?", filter.TransactionRef)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var deliveries []entity.WebhookDelivery
	err := query.
		Order("created_at DESC").
		Offset(params.Offset()).
		Limit(params.Limit).
		Find(&deliveries).Error
	return deliveries, total, err
}

func (r *GormWebhookDeliveryRepository) ListDue(ctx context.Context, now time.Time, limit int) ([]entity.WebhookDelivery, error) {
	var deliveries []entity.WebhookDelivery
	err := r.dbFromContext(ctx).
		Model(&entity.WebhookDelivery{}).
		Where("status IN ?", []entity.WebhookDeliveryStatus{entity.WebhookDeliveryStatusPending, entity.WebhookDeliveryStatusRetrying}).
		Where("next_retry_at IS NULL OR next_retry_at <= ?", now).
		Order("created_at ASC").
		Limit(limit).
		Find(&deliveries).Error
	return deliveries, err
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
