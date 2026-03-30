package usecase_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/idempotency"
	"go-hermes/internal/usecase"
	"go-hermes/tests/testkit"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newReconciliationUsecaseForTest() (*usecase.ReconciliationUsecase, *usecase.TransactionUsecase, *testkit.MemoryRepositories) {
	repos := testkit.NewMemoryRepositories()
	reconciliationUsecase := usecase.NewReconciliationUsecase(repos.Reconciliation, repos.Audits)
	transactionUsecase := usecase.NewTransactionUsecase(repos.TxManager, repos.Wallets, repos.Transactions, repos.Ledgers, repos.Idempotency, repos.Audits, &usecase.NoopWebhookService{})
	return reconciliationUsecase, transactionUsecase, repos
}

func seedAdminActor(t *testing.T, repos *testkit.MemoryRepositories) *entity.User {
	t.Helper()

	admin, _ := testkit.NewUserBuilder().WithRole(entity.RoleAdmin).WithEmail("admin-reconciliation@example.com").Build(t)
	adminWallet := testkit.NewWalletBuilder().WithUserID(admin.ID).Build()
	require.NoError(t, repos.Users.Create(context.Background(), admin))
	require.NoError(t, repos.Wallets.Create(context.Background(), adminWallet))
	return admin
}

func TestReconciliationHealthyState(t *testing.T) {
	reconciliationUsecase, transactionUsecase, repos := newReconciliationUsecaseForTest()
	admin := seedAdminActor(t, repos)

	senderUserID, senderWallet := seedWalletOwner(t, repos, 0)
	_, recipientWallet := seedWalletOwner(t, repos, 0)

	topUpHash, err := idempotency.HashPayload(map[string]interface{}{"amount": int64(5000)})
	require.NoError(t, err)
	_, err = transactionUsecase.TopUp(context.Background(), usecase.TopUpRequest{
		UserID:         senderUserID.String(),
		Amount:         5000,
		IdempotencyKey: "recon-topup",
		RequestHash:    topUpHash,
	})
	require.NoError(t, err)

	transferHash, err := idempotency.HashPayload(map[string]interface{}{
		"recipient_wallet_id": recipientWallet.ID.String(),
		"amount":              int64(1500),
	})
	require.NoError(t, err)
	_, err = transactionUsecase.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            senderUserID.String(),
		RecipientWalletID: recipientWallet.ID.String(),
		Amount:            1500,
		IdempotencyKey:    "recon-transfer",
		RequestHash:       transferHash,
	})
	require.NoError(t, err)

	report, err := reconciliationUsecase.Run(context.Background(), admin.ID.String())
	require.NoError(t, err)
	require.True(t, report.Healthy)
	require.Empty(t, report.Wallets)
	require.Empty(t, report.Transactions)
	require.Empty(t, report.LedgerEntries)
	require.Equal(t, 3, report.Summary.WalletsChecked)
	require.Equal(t, 2, report.Summary.TransactionsChecked)
	require.Equal(t, 3, report.Summary.LedgerEntriesChecked)

	currentSender, ok := repos.Store.WalletByUserID(senderUserID)
	require.True(t, ok)
	require.Equal(t, senderWallet.Balance+3500, currentSender.Balance)
}

func TestReconciliationDetectsMismatchOrphanAndBrokenTransactionShape(t *testing.T) {
	reconciliationUsecase, _, repos := newReconciliationUsecaseForTest()
	admin := seedAdminActor(t, repos)

	userA, _ := testkit.NewUserBuilder().WithEmail("wallet-a@example.com").Build(t)
	userB, _ := testkit.NewUserBuilder().WithEmail("wallet-b@example.com").Build(t)
	walletA := testkit.NewWalletBuilder().WithUserID(userA.ID).WithBalance(900).Build()
	walletB := testkit.NewWalletBuilder().WithUserID(userB.ID).WithBalance(0).Build()

	require.NoError(t, repos.Users.Create(context.Background(), userA))
	require.NoError(t, repos.Users.Create(context.Background(), userB))
	require.NoError(t, repos.Wallets.Create(context.Background(), walletA))
	require.NoError(t, repos.Wallets.Create(context.Background(), walletB))

	now := time.Now()
	topUpTransaction := testkit.NewTransactionBuilder().
		WithType(entity.TransactionTypeTopUp).
		WithDestinationWalletID(walletA.ID).
		WithInitiatedByUserID(userA.ID).
		WithAmount(1000).
		Build()
	topUpTransaction.CreatedAt = now
	topUpTransaction.UpdatedAt = now
	require.NoError(t, repos.Transactions.Create(context.Background(), topUpTransaction))

	transferTransaction := testkit.NewTransactionBuilder().
		WithType(entity.TransactionTypeTransfer).
		WithSourceWalletID(walletA.ID).
		WithDestinationWalletID(walletB.ID).
		WithInitiatedByUserID(userA.ID).
		WithAmount(300).
		Build()
	transferTransaction.CreatedAt = now.Add(time.Second)
	transferTransaction.UpdatedAt = transferTransaction.CreatedAt
	require.NoError(t, repos.Transactions.Create(context.Background(), transferTransaction))

	require.NoError(t, repos.Ledgers.CreateMany(context.Background(), []entity.LedgerEntry{
		{
			ID:            uuid.New(),
			TransactionID: topUpTransaction.ID,
			WalletID:      walletA.ID,
			EntryType:     entity.LedgerEntryTypeCredit,
			Amount:        800,
			BalanceBefore: 0,
			BalanceAfter:  800,
			CreatedAt:     now,
		},
		{
			ID:            uuid.New(),
			TransactionID: transferTransaction.ID,
			WalletID:      walletA.ID,
			EntryType:     entity.LedgerEntryTypeDebit,
			Amount:        300,
			BalanceBefore: 800,
			BalanceAfter:  500,
			CreatedAt:     now.Add(time.Second),
		},
		{
			ID:            uuid.New(),
			TransactionID: uuid.New(),
			WalletID:      walletB.ID,
			EntryType:     entity.LedgerEntryTypeCredit,
			Amount:        50,
			BalanceBefore: 0,
			BalanceAfter:  50,
			CreatedAt:     now.Add(2 * time.Second),
		},
	}))

	report, err := reconciliationUsecase.Run(context.Background(), admin.ID.String())
	require.NoError(t, err)
	require.False(t, report.Healthy)
	require.Len(t, report.Wallets, 2)
	require.NotEmpty(t, report.Transactions)
	require.NotEmpty(t, report.LedgerEntries)
	require.Equal(t, len(report.Wallets)+len(report.Transactions)+len(report.LedgerEntries), report.Summary.IssueCount)

	require.True(t, containsWalletIssue(report.Wallets, walletA.ID.String(), "wallet balance does not match ledger-derived balance"))
	require.True(t, containsTransactionIssue(report.Transactions, topUpTransaction.ID.String(), "top up ledger amount does not match transaction amount"))
	require.True(t, containsTransactionIssue(report.Transactions, transferTransaction.ID.String(), "must have exactly two ledger entries"))
	require.True(t, containsLedgerIssue(report.LedgerEntries, "orphaned from any transaction"))
}

func containsWalletIssue(issues []usecase.ReconciliationWalletIssueResponse, walletID, reason string) bool {
	for _, issue := range issues {
		if issue.WalletID == walletID && strings.Contains(issue.Reason, reason) {
			return true
		}
	}
	return false
}

func containsTransactionIssue(issues []usecase.ReconciliationTransactionIssueResponse, transactionID, reason string) bool {
	for _, issue := range issues {
		if issue.TransactionID == transactionID && strings.Contains(issue.Reason, reason) {
			return true
		}
	}
	return false
}

func containsLedgerIssue(issues []usecase.ReconciliationLedgerIssueResponse, reason string) bool {
	for _, issue := range issues {
		if strings.Contains(issue.Reason, reason) {
			return true
		}
	}
	return false
}
