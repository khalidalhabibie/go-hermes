package tests

import (
	"context"
	"strings"
	"testing"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/idempotency"
	"go-hermes/internal/repository"
	"go-hermes/internal/usecase"
	"go-hermes/tests/testkit"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type postgresReconciliationHarness struct {
	DB             *gorm.DB
	Reconciliation *usecase.ReconciliationUsecase
	Transactions   *usecase.TransactionUsecase
	Admin          *entity.User
}

func TestPostgresReconciliationDetectsCorruptedFirstLedgerEntry(t *testing.T) {
	h := newPostgresReconciliationHarness(t)
	_, wallet := seedPostgresWalletOwner(t, h.DB, "recon-first-ledger@example.com", 0)

	topUp(t, h.Transactions, wallet.UserID.String(), 1000, "pg-recon-first-ledger")

	var entry entity.LedgerEntry
	require.NoError(t, h.DB.WithContext(context.Background()).Where("wallet_id = ?", wallet.ID).First(&entry).Error)

	withUnsafeLedgerMutation(t, h.DB, "UPDATE ledger_entries SET balance_before = ?, balance_after = ? WHERE id = ?", 100, 1100, entry.ID)

	report, err := h.Reconciliation.Run(context.Background(), h.Admin.ID.String())
	require.NoError(t, err)
	require.False(t, report.Healthy)
	require.True(t, containsLedgerIssueWithReason(report.LedgerEntries, entry.ID.String(), "first ledger entry must start from wallet genesis balance 0"))
}

func TestPostgresReconciliationDetectsOrphanLedgerEntry(t *testing.T) {
	h := newPostgresReconciliationHarness(t)
	_, wallet := seedPostgresWalletOwner(t, h.DB, "recon-orphan@example.com", 0)

	result := topUp(t, h.Transactions, wallet.UserID.String(), 1000, "pg-recon-orphan")
	transactionID := result.Data.Transaction.ID

	var entry entity.LedgerEntry
	require.NoError(t, h.DB.WithContext(context.Background()).Where("transaction_id = ?", transactionID).First(&entry).Error)

	withUnsafeLedgerMutation(t, h.DB, "UPDATE ledger_entries SET transaction_id = ? WHERE id = ?", uuid.New(), entry.ID)

	report, err := h.Reconciliation.Run(context.Background(), h.Admin.ID.String())
	require.NoError(t, err)
	require.False(t, report.Healthy)
	require.True(t, containsLedgerIssueWithReason(report.LedgerEntries, entry.ID.String(), "orphaned from any transaction"))
	require.True(t, containsTransactionIssueWithReason(report.Transactions, transactionID, "must have exactly one ledger entry"))
}

func TestPostgresReconciliationDetectsMissingTransferDebitAndCredit(t *testing.T) {
	testCases := []struct {
		name       string
		entryType  entity.LedgerEntryType
		wantReason string
	}{
		{
			name:       "missing debit",
			entryType:  entity.LedgerEntryTypeCredit,
			wantReason: "missing the source-wallet DEBIT ledger entry",
		},
		{
			name:       "missing credit",
			entryType:  entity.LedgerEntryTypeDebit,
			wantReason: "missing the destination-wallet CREDIT ledger entry",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := newPostgresReconciliationHarness(t)
			_, senderWallet := seedPostgresWalletOwner(t, h.DB, "recon-transfer-sender-"+tc.name+"@example.com", 5000)
			_, recipientWallet := seedPostgresWalletOwner(t, h.DB, "recon-transfer-recipient-"+tc.name+"@example.com", 0)

			result := transfer(t, h.Transactions, senderWallet.UserID.String(), recipientWallet.ID.String(), 1000, "pg-recon-transfer-"+tc.name)
			transactionID := result.Data.Transaction.ID

			var entry entity.LedgerEntry
			require.NoError(t, h.DB.WithContext(context.Background()).
				Where("transaction_id = ? AND wallet_id = ?", transactionID, walletIDForMissingLegCase(senderWallet, recipientWallet, tc.name)).
				First(&entry).Error)

			withUnsafeLedgerMutation(t, h.DB, "UPDATE ledger_entries SET entry_type = ? WHERE id = ?", tc.entryType, entry.ID)

			report, err := h.Reconciliation.Run(context.Background(), h.Admin.ID.String())
			require.NoError(t, err)
			require.False(t, report.Healthy)
			require.True(t, containsTransactionIssueWithReason(report.Transactions, transactionID, tc.wantReason))
		})
	}
}

func TestPostgresReconciliationDetectsTransactionAmountMismatch(t *testing.T) {
	h := newPostgresReconciliationHarness(t)
	_, wallet := seedPostgresWalletOwner(t, h.DB, "recon-amount@example.com", 0)

	result := topUp(t, h.Transactions, wallet.UserID.String(), 1000, "pg-recon-amount")
	transactionID := result.Data.Transaction.ID

	require.NoError(t, h.DB.WithContext(context.Background()).
		Model(&entity.Transaction{}).
		Where("id = ?", transactionID).
		Update("amount", 900).Error)

	report, err := h.Reconciliation.Run(context.Background(), h.Admin.ID.String())
	require.NoError(t, err)
	require.False(t, report.Healthy)
	require.True(t, containsTransactionIssueWithReason(report.Transactions, transactionID, "top up ledger amount does not match transaction amount"))
}

func TestPostgresReconciliationDetectsWalletBalanceDrift(t *testing.T) {
	h := newPostgresReconciliationHarness(t)
	user, wallet := seedPostgresWalletOwner(t, h.DB, "recon-drift@example.com", 0)

	topUp(t, h.Transactions, wallet.UserID.String(), 1000, "pg-recon-drift")

	require.NoError(t, h.DB.WithContext(context.Background()).
		Model(&entity.Wallet{}).
		Where("id = ?", wallet.ID).
		Update("balance", 1300).Error)

	report, err := h.Reconciliation.Run(context.Background(), h.Admin.ID.String())
	require.NoError(t, err)
	require.False(t, report.Healthy)
	require.True(t, containsWalletIssueWithReason(report.Wallets, wallet.ID.String(), "wallet balance does not match ledger-derived balance"))
	require.Equal(t, user.ID.String(), walletIssueUserID(report.Wallets, wallet.ID.String()))
}

func newPostgresReconciliationHarness(t *testing.T) postgresReconciliationHarness {
	t.Helper()

	db := testkit.NewPostgresTestDB(t).DB
	txManager := repository.NewTransactionManager(db)
	userRepo := repository.NewUserRepository(db)
	walletRepo := repository.NewWalletRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	ledgerRepo := repository.NewLedgerRepository(db)
	idempotencyRepo := repository.NewIdempotencyRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	reconciliationRepo := repository.NewReconciliationRepository(db)

	admin, _ := testkit.NewUserBuilder().WithRole(entity.RoleAdmin).WithEmail("pg-reconciliation-admin@example.com").Build(t)
	adminWallet := testkit.NewWalletBuilder().WithUserID(admin.ID).Build()
	require.NoError(t, userRepo.Create(context.Background(), admin))
	require.NoError(t, walletRepo.Create(context.Background(), adminWallet))

	return postgresReconciliationHarness{
		DB:             db,
		Reconciliation: usecase.NewReconciliationUsecase(reconciliationRepo, auditRepo),
		Transactions: usecase.NewTransactionUsecase(
			txManager,
			walletRepo,
			transactionRepo,
			ledgerRepo,
			idempotencyRepo,
			auditRepo,
			&usecase.NoopWebhookService{},
		),
		Admin: admin,
	}
}

func seedPostgresWalletOwner(t *testing.T, db *gorm.DB, email string, balance int64) (*entity.User, *entity.Wallet) {
	t.Helper()

	userRepo := repository.NewUserRepository(db)
	walletRepo := repository.NewWalletRepository(db)

	user, _ := testkit.NewUserBuilder().WithEmail(email).Build(t)
	wallet := testkit.NewWalletBuilder().WithUserID(user.ID).WithBalance(balance).Build()

	require.NoError(t, userRepo.Create(context.Background(), user))
	require.NoError(t, walletRepo.Create(context.Background(), wallet))
	return user, wallet
}

func withUnsafeLedgerMutation(t *testing.T, db *gorm.DB, query string, args ...interface{}) {
	t.Helper()

	require.NoError(t, db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL session_replication_role = replica").Error; err != nil {
			return err
		}
		return tx.Exec(query, args...).Error
	}))
}

func containsLedgerIssueWithReason(issues []usecase.ReconciliationLedgerIssueResponse, ledgerEntryID, reason string) bool {
	for _, issue := range issues {
		if issue.LedgerEntryID == ledgerEntryID && strings.Contains(issue.Reason, reason) {
			return true
		}
	}
	return false
}

func containsTransactionIssueWithReason(issues []usecase.ReconciliationTransactionIssueResponse, transactionID, reason string) bool {
	for _, issue := range issues {
		if issue.TransactionID == transactionID && strings.Contains(issue.Reason, reason) {
			return true
		}
	}
	return false
}

func containsWalletIssueWithReason(issues []usecase.ReconciliationWalletIssueResponse, walletID, reason string) bool {
	for _, issue := range issues {
		if issue.WalletID == walletID && strings.Contains(issue.Reason, reason) {
			return true
		}
	}
	return false
}

func walletIssueUserID(issues []usecase.ReconciliationWalletIssueResponse, walletID string) string {
	for _, issue := range issues {
		if issue.WalletID == walletID {
			return issue.UserID
		}
	}
	return ""
}

func walletIDForMissingLegCase(senderWallet, recipientWallet *entity.Wallet, name string) uuid.UUID {
	if name == "missing debit" {
		return senderWallet.ID
	}
	return recipientWallet.ID
}

func topUp(t *testing.T, transactionUsecase *usecase.TransactionUsecase, userID string, amount int64, idempotencyKey string) testkit.SuccessEnvelope[topUpData] {
	t.Helper()

	requestHash, err := idempotency.HashPayload(map[string]interface{}{
		"amount": amount,
	})
	require.NoError(t, err)

	result, err := transactionUsecase.TopUp(context.Background(), usecase.TopUpRequest{
		UserID:         userID,
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
		RequestHash:    requestHash,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	return testkit.DecodeSuccess[topUpData](t, result.Body)
}

func transfer(t *testing.T, transactionUsecase *usecase.TransactionUsecase, userID, recipientWalletID string, amount int64, idempotencyKey string) testkit.SuccessEnvelope[transferData] {
	t.Helper()

	requestHash, err := idempotency.HashPayload(map[string]interface{}{
		"recipient_wallet_id": recipientWalletID,
		"amount":              amount,
	})
	require.NoError(t, err)

	result, err := transactionUsecase.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            userID,
		RecipientWalletID: recipientWalletID,
		Amount:            amount,
		IdempotencyKey:    idempotencyKey,
		RequestHash:       requestHash,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	return testkit.DecodeSuccess[transferData](t, result.Body)
}
