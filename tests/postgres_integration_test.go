package tests

import (
	"context"
	"sync"
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

func TestPostgresWalletLockByIDsReturnsDeterministicOrder(t *testing.T) {
	db := testkit.NewPostgresTestDB(t).DB
	wallets := repository.NewWalletRepository(db)
	users := repository.NewUserRepository(db)

	firstUser, _ := testkit.NewUserBuilder().WithEmail("lock-a@example.com").Build(t)
	secondUser, _ := testkit.NewUserBuilder().WithEmail("lock-b@example.com").Build(t)
	firstWallet := testkit.NewWalletBuilder().WithUserID(firstUser.ID).Build()
	secondWallet := testkit.NewWalletBuilder().WithUserID(secondUser.ID).Build()

	require.NoError(t, users.Create(context.Background(), firstUser))
	require.NoError(t, users.Create(context.Background(), secondUser))
	require.NoError(t, wallets.Create(context.Background(), firstWallet))
	require.NoError(t, wallets.Create(context.Background(), secondWallet))

	locked, err := wallets.LockByIDs(context.Background(), []uuid.UUID{secondWallet.ID, firstWallet.ID})
	require.NoError(t, err)
	require.Len(t, locked, 2)
	require.LessOrEqual(t, locked[0].ID.String(), locked[1].ID.String())
}

func TestPostgresTransferIsAtomicAndCreatesLedgerEntries(t *testing.T) {
	db := testkit.NewPostgresTestDB(t).DB
	transactionUsecase, senderWallet, recipientWallet := newPostgresTransactionUsecase(t, db, 10_000, 1_000)

	hashValue, err := idempotency.HashPayload(map[string]interface{}{
		"recipient_wallet_id": recipientWallet.ID.String(),
		"amount":              3_000,
	})
	require.NoError(t, err)

	result, err := transactionUsecase.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            senderWallet.UserID.String(),
		RecipientWalletID: recipientWallet.ID.String(),
		Amount:            3_000,
		IdempotencyKey:    "pg-transfer-success",
		RequestHash:       hashValue,
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	verifyWalletBalance(t, db, senderWallet.ID.String(), 7_000)
	verifyWalletBalance(t, db, recipientWallet.ID.String(), 4_000)
	assertTableCount(t, db, &entity.Transaction{}, 1)
	assertTableCount(t, db, &entity.LedgerEntry{}, 2)
}

func TestPostgresTransferRollbackOnInsufficientBalance(t *testing.T) {
	db := testkit.NewPostgresTestDB(t).DB
	transactionUsecase, senderWallet, recipientWallet := newPostgresTransactionUsecase(t, db, 1_000, 1_000)

	hashValue, err := idempotency.HashPayload(map[string]interface{}{
		"recipient_wallet_id": recipientWallet.ID.String(),
		"amount":              3_000,
	})
	require.NoError(t, err)

	result, err := transactionUsecase.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            senderWallet.UserID.String(),
		RecipientWalletID: recipientWallet.ID.String(),
		Amount:            3_000,
		IdempotencyKey:    "pg-transfer-insufficient",
		RequestHash:       hashValue,
	})

	require.Nil(t, result)
	require.Error(t, err)

	verifyWalletBalance(t, db, senderWallet.ID.String(), 1_000)
	verifyWalletBalance(t, db, recipientWallet.ID.String(), 1_000)
	assertTableCount(t, db, &entity.Transaction{}, 0)
	assertTableCount(t, db, &entity.LedgerEntry{}, 0)
}

func TestPostgresReciprocalTransfersCompleteWithoutDeadlock(t *testing.T) {
	db := testkit.NewPostgresTestDB(t).DB
	transactionUsecase, firstWallet, secondWallet := newPostgresTransactionUsecase(t, db, 10_000, 10_000)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start
		hashValue, err := idempotency.HashPayload(map[string]interface{}{
			"recipient_wallet_id": secondWallet.ID.String(),
			"amount":              1_500,
		})
		if err != nil {
			errs <- err
			return
		}
		_, err = transactionUsecase.Transfer(context.Background(), usecase.TransferRequest{
			UserID:            firstWallet.UserID.String(),
			RecipientWalletID: secondWallet.ID.String(),
			Amount:            1_500,
			IdempotencyKey:    "pg-transfer-a-to-b",
			RequestHash:       hashValue,
		})
		errs <- err
	}()

	go func() {
		defer wg.Done()
		<-start
		hashValue, err := idempotency.HashPayload(map[string]interface{}{
			"recipient_wallet_id": firstWallet.ID.String(),
			"amount":              500,
		})
		if err != nil {
			errs <- err
			return
		}
		_, err = transactionUsecase.Transfer(context.Background(), usecase.TransferRequest{
			UserID:            secondWallet.UserID.String(),
			RecipientWalletID: firstWallet.ID.String(),
			Amount:            500,
			IdempotencyKey:    "pg-transfer-b-to-a",
			RequestHash:       hashValue,
		})
		errs <- err
	}()

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	verifyWalletBalance(t, db, firstWallet.ID.String(), 9_000)
	verifyWalletBalance(t, db, secondWallet.ID.String(), 11_000)
	assertTableCount(t, db, &entity.Transaction{}, 2)
	assertTableCount(t, db, &entity.LedgerEntry{}, 4)
}

func newPostgresTransactionUsecase(t *testing.T, db *gorm.DB, senderBalance, recipientBalance int64) (*usecase.TransactionUsecase, *entity.Wallet, *entity.Wallet) {
	t.Helper()

	txManager := repository.NewTransactionManager(db)
	userRepo := repository.NewUserRepository(db)
	walletRepo := repository.NewWalletRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	ledgerRepo := repository.NewLedgerRepository(db)
	idempotencyRepo := repository.NewIdempotencyRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)

	senderUser, _ := testkit.NewUserBuilder().WithEmail("sender-pg@example.com").Build(t)
	recipientUser, _ := testkit.NewUserBuilder().WithEmail("recipient-pg@example.com").Build(t)
	senderWallet := testkit.NewWalletBuilder().WithUserID(senderUser.ID).WithBalance(senderBalance).Build()
	recipientWallet := testkit.NewWalletBuilder().WithUserID(recipientUser.ID).WithBalance(recipientBalance).Build()

	require.NoError(t, userRepo.Create(context.Background(), senderUser))
	require.NoError(t, userRepo.Create(context.Background(), recipientUser))
	require.NoError(t, walletRepo.Create(context.Background(), senderWallet))
	require.NoError(t, walletRepo.Create(context.Background(), recipientWallet))

	return usecase.NewTransactionUsecase(
		txManager,
		walletRepo,
		transactionRepo,
		ledgerRepo,
		idempotencyRepo,
		auditRepo,
		&usecase.NoopWebhookService{},
	), senderWallet, recipientWallet
}

func verifyWalletBalance(t *testing.T, db *gorm.DB, walletID string, expected int64) {
	t.Helper()

	var wallet entity.Wallet
	require.NoError(t, db.WithContext(context.Background()).Where("id = ?", walletID).First(&wallet).Error)
	require.Equal(t, expected, wallet.Balance)
}

func assertTableCount(t *testing.T, db *gorm.DB, model interface{}, expected int64) {
	t.Helper()

	var total int64
	require.NoError(t, db.WithContext(context.Background()).Model(model).Count(&total).Error)
	require.Equal(t, expected, total)
}
