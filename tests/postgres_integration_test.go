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

func TestPostgresConcurrentTopUpsWithSameIdempotencyKeyMutateOnce(t *testing.T) {
	db := testkit.NewPostgresTestDB(t).DB
	transactionUsecase, wallet, _ := newPostgresTransactionUsecase(t, db, 1_000, 0)

	hashValue, err := idempotency.HashPayload(map[string]interface{}{
		"amount": int64(2_000),
	})
	require.NoError(t, err)

	results := make(chan *usecase.OperationResult, 2)
	errs := runConcurrent(t, 2, func() error {
		result, err := transactionUsecase.TopUp(context.Background(), usecase.TopUpRequest{
			UserID:         wallet.UserID.String(),
			Amount:         2_000,
			IdempotencyKey: "pg-concurrent-topup-same-key",
			RequestHash:    hashValue,
		})
		if err == nil {
			results <- result
		}
		return err
	})
	close(results)

	replayCount := 0
	nonReplayCount := 0
	for _, err := range errs {
		require.NoError(t, err)
	}
	for result := range results {
		require.NotNil(t, result)
		if result.Replay {
			replayCount++
		} else {
			nonReplayCount++
		}
	}
	require.Equal(t, 1, nonReplayCount)
	require.Equal(t, 1, replayCount)

	replayResult, err := transactionUsecase.TopUp(context.Background(), usecase.TopUpRequest{
		UserID:         wallet.UserID.String(),
		Amount:         2_000,
		IdempotencyKey: "pg-concurrent-topup-same-key",
		RequestHash:    hashValue,
	})
	require.NoError(t, err)
	if replayResult.Replay {
		replayCount++
	}
	require.GreaterOrEqual(t, replayCount, 2)

	verifyWalletBalance(t, db, wallet.ID.String(), 3_000)
	assertTableCount(t, db, &entity.Transaction{}, 1)
	assertTableCount(t, db, &entity.LedgerEntry{}, 1)
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

func TestPostgresConcurrentTransfersWithSameIdempotencyKeyMutateOnce(t *testing.T) {
	db := testkit.NewPostgresTestDB(t).DB
	transactionUsecase, senderWallet, recipientWallet := newPostgresTransactionUsecase(t, db, 10_000, 1_000)

	hashValue, err := idempotency.HashPayload(map[string]interface{}{
		"recipient_wallet_id": recipientWallet.ID.String(),
		"amount":              int64(3_000),
	})
	require.NoError(t, err)

	results := make(chan *usecase.OperationResult, 2)
	errs := runConcurrent(t, 2, func() error {
		result, err := transactionUsecase.Transfer(context.Background(), usecase.TransferRequest{
			UserID:            senderWallet.UserID.String(),
			RecipientWalletID: recipientWallet.ID.String(),
			Amount:            3_000,
			IdempotencyKey:    "pg-concurrent-transfer-same-key",
			RequestHash:       hashValue,
		})
		if err == nil {
			results <- result
		}
		return err
	})
	close(results)

	replayCount := 0
	nonReplayCount := 0
	for _, err := range errs {
		require.NoError(t, err)
	}
	for result := range results {
		require.NotNil(t, result)
		if result.Replay {
			replayCount++
		} else {
			nonReplayCount++
		}
	}
	require.Equal(t, 1, nonReplayCount)
	require.Equal(t, 1, replayCount)

	verifyWalletBalance(t, db, senderWallet.ID.String(), 7_000)
	verifyWalletBalance(t, db, recipientWallet.ID.String(), 4_000)
	assertTableCount(t, db, &entity.Transaction{}, 1)
	assertTableCount(t, db, &entity.LedgerEntry{}, 2)
}

func TestPostgresConcurrentTransfersCompetingOnSameWalletRemainConsistent(t *testing.T) {
	db := testkit.NewPostgresTestDB(t).DB

	txManager := repository.NewTransactionManager(db)
	userRepo := repository.NewUserRepository(db)
	walletRepo := repository.NewWalletRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	ledgerRepo := repository.NewLedgerRepository(db)
	idempotencyRepo := repository.NewIdempotencyRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)

	senderUser, _ := testkit.NewUserBuilder().WithEmail("competing-sender@example.com").Build(t)
	recipientOneUser, _ := testkit.NewUserBuilder().WithEmail("competing-recipient-1@example.com").Build(t)
	recipientTwoUser, _ := testkit.NewUserBuilder().WithEmail("competing-recipient-2@example.com").Build(t)
	senderWallet := testkit.NewWalletBuilder().WithUserID(senderUser.ID).WithBalance(5_000).Build()
	recipientOneWallet := testkit.NewWalletBuilder().WithUserID(recipientOneUser.ID).WithBalance(0).Build()
	recipientTwoWallet := testkit.NewWalletBuilder().WithUserID(recipientTwoUser.ID).WithBalance(0).Build()

	require.NoError(t, userRepo.Create(context.Background(), senderUser))
	require.NoError(t, userRepo.Create(context.Background(), recipientOneUser))
	require.NoError(t, userRepo.Create(context.Background(), recipientTwoUser))
	require.NoError(t, walletRepo.Create(context.Background(), senderWallet))
	require.NoError(t, walletRepo.Create(context.Background(), recipientOneWallet))
	require.NoError(t, walletRepo.Create(context.Background(), recipientTwoWallet))

	transactionUsecase := usecase.NewTransactionUsecase(
		txManager,
		walletRepo,
		transactionRepo,
		ledgerRepo,
		idempotencyRepo,
		auditRepo,
		&usecase.NoopWebhookService{},
	)

	successes := 0
	errs := runConcurrentWithResults(t, []func() error{
		func() error {
			hashValue, err := idempotency.HashPayload(map[string]interface{}{
				"recipient_wallet_id": recipientOneWallet.ID.String(),
				"amount":              int64(3_000),
			})
			if err != nil {
				return err
			}
			_, err = transactionUsecase.Transfer(context.Background(), usecase.TransferRequest{
				UserID:            senderWallet.UserID.String(),
				RecipientWalletID: recipientOneWallet.ID.String(),
				Amount:            3_000,
				IdempotencyKey:    "pg-competing-transfer-1",
				RequestHash:       hashValue,
			})
			return err
		},
		func() error {
			hashValue, err := idempotency.HashPayload(map[string]interface{}{
				"recipient_wallet_id": recipientTwoWallet.ID.String(),
				"amount":              int64(3_000),
			})
			if err != nil {
				return err
			}
			_, err = transactionUsecase.Transfer(context.Background(), usecase.TransferRequest{
				UserID:            senderWallet.UserID.String(),
				RecipientWalletID: recipientTwoWallet.ID.String(),
				Amount:            3_000,
				IdempotencyKey:    "pg-competing-transfer-2",
				RequestHash:       hashValue,
			})
			return err
		},
	})

	for _, err := range errs {
		if err == nil {
			successes++
			continue
		}
		require.Contains(t, err.Error(), "insufficient balance")
	}
	require.Equal(t, 1, successes)

	verifyWalletBalance(t, db, senderWallet.ID.String(), 2_000)

	recipientOneAfter := getWalletBalance(t, db, recipientOneWallet.ID.String())
	recipientTwoAfter := getWalletBalance(t, db, recipientTwoWallet.ID.String())
	require.Equal(t, int64(3_000), recipientOneAfter+recipientTwoAfter)
	assertTableCount(t, db, &entity.Transaction{}, 1)
	assertTableCount(t, db, &entity.LedgerEntry{}, 2)
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

func runConcurrent(t *testing.T, count int, fn func() error) []error {
	t.Helper()

	fns := make([]func() error, 0, count)
	for i := 0; i < count; i++ {
		fns = append(fns, fn)
	}
	return runConcurrentWithResults(t, fns)
}

func runConcurrentWithResults(t *testing.T, fns []func() error) []error {
	t.Helper()

	start := make(chan struct{})
	results := make([]error, len(fns))
	var wg sync.WaitGroup
	wg.Add(len(fns))

	for i, fn := range fns {
		go func(index int, call func() error) {
			defer wg.Done()
			<-start
			results[index] = call()
		}(i, fn)
	}

	close(start)
	wg.Wait()
	return results
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

func getWalletBalance(t *testing.T, db *gorm.DB, walletID string) int64 {
	t.Helper()

	var wallet entity.Wallet
	require.NoError(t, db.WithContext(context.Background()).Where("id = ?", walletID).First(&wallet).Error)
	return wallet.Balance
}

func assertTableCount(t *testing.T, db *gorm.DB, model interface{}, expected int64) {
	t.Helper()

	var total int64
	require.NoError(t, db.WithContext(context.Background()).Model(model).Count(&total).Error)
	require.Equal(t, expected, total)
}
