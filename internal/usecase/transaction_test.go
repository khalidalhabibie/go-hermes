package usecase_test

import (
	"context"
	"testing"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/idempotency"
	"go-hermes/internal/usecase"
	"go-hermes/tests/testkit"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newTransactionUsecaseForTest() (*usecase.TransactionUsecase, *testkit.MemoryRepositories) {
	repos := testkit.NewMemoryRepositories()
	return usecase.NewTransactionUsecase(repos.TxManager, repos.Wallets, repos.Transactions, repos.Ledgers, repos.Idempotency, repos.Audits, &usecase.NoopWebhookService{}), repos
}

func seedWalletOwner(t *testing.T, repos *testkit.MemoryRepositories, balance int64) (uuid.UUID, *entity.Wallet) {
	t.Helper()

	user, _ := testkit.NewUserBuilder().Build(t)
	wallet := testkit.NewWalletBuilder().WithUserID(user.ID).WithBalance(balance).Build()
	require.NoError(t, repos.Users.Create(context.Background(), user))
	require.NoError(t, repos.Wallets.Create(context.Background(), wallet))
	return user.ID, wallet
}

func TestTopUpSuccess(t *testing.T) {
	svc, repos := newTransactionUsecaseForTest()
	userID, wallet := seedWalletOwner(t, repos, 1000)

	hashValue, err := idempotency.HashPayload(map[string]interface{}{"amount": 5000})
	require.NoError(t, err)

	result, err := svc.TopUp(context.Background(), usecase.TopUpRequest{
		UserID:         userID.String(),
		Amount:         5000,
		IdempotencyKey: "topup-success",
		RequestHash:    hashValue,
	})

	require.NoError(t, err)
	require.Equal(t, 200, result.StatusCode)
	currentWallet, ok := repos.Store.WalletByUserID(userID)
	require.True(t, ok)
	require.Equal(t, wallet.Balance+5000, currentWallet.Balance)
	require.Equal(t, 1, repos.Store.TransactionCount())
	require.Equal(t, 1, repos.Store.LedgerCount())
}

func TestTopUpInvalidAmount(t *testing.T) {
	svc, _ := newTransactionUsecaseForTest()

	result, err := svc.TopUp(context.Background(), usecase.TopUpRequest{
		UserID:         uuid.NewString(),
		Amount:         0,
		IdempotencyKey: "topup-invalid",
		RequestHash:    "hash",
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validation error")
}

func TestTopUpDuplicateIdempotencySamePayload(t *testing.T) {
	svc, repos := newTransactionUsecaseForTest()
	userID, wallet := seedWalletOwner(t, repos, 1000)

	hashValue, err := idempotency.HashPayload(map[string]interface{}{"amount": 5000})
	require.NoError(t, err)

	first, err := svc.TopUp(context.Background(), usecase.TopUpRequest{
		UserID:         userID.String(),
		Amount:         5000,
		IdempotencyKey: "topup-replay",
		RequestHash:    hashValue,
	})
	require.NoError(t, err)

	second, err := svc.TopUp(context.Background(), usecase.TopUpRequest{
		UserID:         userID.String(),
		Amount:         5000,
		IdempotencyKey: "topup-replay",
		RequestHash:    hashValue,
	})

	require.NoError(t, err)
	require.True(t, second.Replay)
	require.Equal(t, first.StatusCode, second.StatusCode)
	require.JSONEq(t, string(first.Body), string(second.Body))

	currentWallet, ok := repos.Store.WalletByUserID(userID)
	require.True(t, ok)
	require.Equal(t, wallet.Balance+5000, currentWallet.Balance)
	require.Equal(t, 1, repos.Store.TransactionCount())
	require.Equal(t, 1, repos.Store.LedgerCount())
}

func TestTopUpDuplicateIdempotencyDifferentPayload(t *testing.T) {
	svc, repos := newTransactionUsecaseForTest()
	userID, wallet := seedWalletOwner(t, repos, 1000)

	hashValueOne, err := idempotency.HashPayload(map[string]interface{}{"amount": 5000})
	require.NoError(t, err)
	hashValueTwo, err := idempotency.HashPayload(map[string]interface{}{"amount": 7000})
	require.NoError(t, err)

	_, err = svc.TopUp(context.Background(), usecase.TopUpRequest{
		UserID:         userID.String(),
		Amount:         5000,
		IdempotencyKey: "topup-conflict",
		RequestHash:    hashValueOne,
	})
	require.NoError(t, err)

	result, err := svc.TopUp(context.Background(), usecase.TopUpRequest{
		UserID:         userID.String(),
		Amount:         7000,
		IdempotencyKey: "topup-conflict",
		RequestHash:    hashValueTwo,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "different payload")
	currentWallet, ok := repos.Store.WalletByUserID(userID)
	require.True(t, ok)
	require.Equal(t, wallet.Balance+5000, currentWallet.Balance)
	require.Equal(t, 1, repos.Store.TransactionCount())
	require.Equal(t, 1, repos.Store.LedgerCount())
}

func TestTransferSuccess(t *testing.T) {
	svc, repos := newTransactionUsecaseForTest()
	senderUserID, senderWallet := seedWalletOwner(t, repos, 10000)
	_, recipientWallet := seedWalletOwner(t, repos, 1000)

	hashValue, err := idempotency.HashPayload(map[string]interface{}{
		"recipient_wallet_id": recipientWallet.ID.String(),
		"amount":              3000,
	})
	require.NoError(t, err)

	result, err := svc.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            senderUserID.String(),
		RecipientWalletID: recipientWallet.ID.String(),
		Amount:            3000,
		IdempotencyKey:    "transfer-success",
		RequestHash:       hashValue,
	})

	require.NoError(t, err)
	require.Equal(t, 200, result.StatusCode)
	senderAfter, _ := repos.Store.WalletByUserID(senderUserID)
	recipientAfter, _ := repos.Store.WalletByUserID(recipientWallet.UserID)
	require.Equal(t, senderWallet.Balance-3000, senderAfter.Balance)
	require.Equal(t, int64(4000), recipientAfter.Balance)
	require.Equal(t, 1, repos.Store.TransactionCount())
	require.Equal(t, 2, repos.Store.LedgerCount())
}

func TestTransferDuplicateIdempotencySamePayload(t *testing.T) {
	svc, repos := newTransactionUsecaseForTest()
	senderUserID, senderWallet := seedWalletOwner(t, repos, 10000)
	_, recipientWallet := seedWalletOwner(t, repos, 1000)

	hashValue, err := idempotency.HashPayload(map[string]interface{}{
		"recipient_wallet_id": recipientWallet.ID.String(),
		"amount":              3000,
	})
	require.NoError(t, err)

	first, err := svc.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            senderUserID.String(),
		RecipientWalletID: recipientWallet.ID.String(),
		Amount:            3000,
		IdempotencyKey:    "transfer-replay",
		RequestHash:       hashValue,
	})
	require.NoError(t, err)

	second, err := svc.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            senderUserID.String(),
		RecipientWalletID: recipientWallet.ID.String(),
		Amount:            3000,
		IdempotencyKey:    "transfer-replay",
		RequestHash:       hashValue,
	})

	require.NoError(t, err)
	require.True(t, second.Replay)
	require.Equal(t, first.StatusCode, second.StatusCode)
	require.JSONEq(t, string(first.Body), string(second.Body))

	senderAfter, _ := repos.Store.WalletByUserID(senderUserID)
	recipientAfter, _ := repos.Store.WalletByUserID(recipientWallet.UserID)
	require.Equal(t, senderWallet.Balance-3000, senderAfter.Balance)
	require.Equal(t, int64(4000), recipientAfter.Balance)
	require.Equal(t, 1, repos.Store.TransactionCount())
	require.Equal(t, 2, repos.Store.LedgerCount())
}

func TestTransferDuplicateIdempotencyDifferentPayload(t *testing.T) {
	svc, repos := newTransactionUsecaseForTest()
	senderUserID, senderWallet := seedWalletOwner(t, repos, 10000)
	_, recipientWallet := seedWalletOwner(t, repos, 1000)

	hashValueOne, err := idempotency.HashPayload(map[string]interface{}{
		"recipient_wallet_id": recipientWallet.ID.String(),
		"amount":              3000,
	})
	require.NoError(t, err)
	hashValueTwo, err := idempotency.HashPayload(map[string]interface{}{
		"recipient_wallet_id": recipientWallet.ID.String(),
		"amount":              4000,
	})
	require.NoError(t, err)

	_, err = svc.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            senderUserID.String(),
		RecipientWalletID: recipientWallet.ID.String(),
		Amount:            3000,
		IdempotencyKey:    "transfer-conflict",
		RequestHash:       hashValueOne,
	})
	require.NoError(t, err)

	result, err := svc.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            senderUserID.String(),
		RecipientWalletID: recipientWallet.ID.String(),
		Amount:            4000,
		IdempotencyKey:    "transfer-conflict",
		RequestHash:       hashValueTwo,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "different payload")

	senderAfter, _ := repos.Store.WalletByUserID(senderUserID)
	recipientAfter, _ := repos.Store.WalletByUserID(recipientWallet.UserID)
	require.Equal(t, senderWallet.Balance-3000, senderAfter.Balance)
	require.Equal(t, int64(4000), recipientAfter.Balance)
	require.Equal(t, 1, repos.Store.TransactionCount())
	require.Equal(t, 2, repos.Store.LedgerCount())
}

func TestTransferInsufficientBalance(t *testing.T) {
	svc, repos := newTransactionUsecaseForTest()
	senderUserID, senderWallet := seedWalletOwner(t, repos, 1000)
	_, recipientWallet := seedWalletOwner(t, repos, 1000)

	hashValue, err := idempotency.HashPayload(map[string]interface{}{
		"recipient_wallet_id": recipientWallet.ID.String(),
		"amount":              3000,
	})
	require.NoError(t, err)

	result, err := svc.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            senderUserID.String(),
		RecipientWalletID: recipientWallet.ID.String(),
		Amount:            3000,
		IdempotencyKey:    "transfer-insufficient",
		RequestHash:       hashValue,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient balance")
	senderAfter, _ := repos.Store.WalletByUserID(senderUserID)
	require.Equal(t, senderWallet.Balance, senderAfter.Balance)
	require.Equal(t, 0, repos.Store.TransactionCount())
	require.Equal(t, 0, repos.Store.LedgerCount())
}

func TestTransferToSameWalletFails(t *testing.T) {
	svc, repos := newTransactionUsecaseForTest()
	senderUserID, senderWallet := seedWalletOwner(t, repos, 1000)

	hashValue, err := idempotency.HashPayload(map[string]interface{}{
		"recipient_wallet_id": senderWallet.ID.String(),
		"amount":              100,
	})
	require.NoError(t, err)

	result, err := svc.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            senderUserID.String(),
		RecipientWalletID: senderWallet.ID.String(),
		Amount:            100,
		IdempotencyKey:    "transfer-same-wallet",
		RequestHash:       hashValue,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be the same")
}

func TestTransferInvalidRecipientWallet(t *testing.T) {
	svc, repos := newTransactionUsecaseForTest()
	senderUserID, _ := seedWalletOwner(t, repos, 1000)

	result, err := svc.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            senderUserID.String(),
		RecipientWalletID: "not-a-uuid",
		Amount:            100,
		IdempotencyKey:    "transfer-invalid-wallet",
		RequestHash:       "hash",
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validation error")
}

func TestTransferInvalidAmount(t *testing.T) {
	svc, repos := newTransactionUsecaseForTest()
	senderUserID, _ := seedWalletOwner(t, repos, 1000)

	result, err := svc.Transfer(context.Background(), usecase.TransferRequest{
		UserID:            senderUserID.String(),
		RecipientWalletID: uuid.NewString(),
		Amount:            0,
		IdempotencyKey:    "transfer-invalid-amount",
		RequestHash:       "hash",
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validation error")
}
