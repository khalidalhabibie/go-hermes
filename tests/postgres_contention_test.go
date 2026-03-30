package tests

import (
	"context"
	"fmt"
	"testing"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/idempotency"
	"go-hermes/internal/usecase"
	"go-hermes/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestPostgresHighFanoutTopUpsWithSameIdempotencyKeyMutateOnce(t *testing.T) {
	db := testkit.NewPostgresTestDB(t).DB
	transactionUsecase, wallet, _ := newPostgresTransactionUsecase(t, db, 1_000, 0)

	fanout := 12
	rounds := 3
	hashValue, err := idempotency.HashPayload(map[string]interface{}{"amount": int64(2_000)})
	require.NoError(t, err)

	totalReplayCount, totalNonReplayCount := runRepeatedIdempotentRequests(t, fanout, rounds, func() (*usecase.OperationResult, error) {
		return transactionUsecase.TopUp(context.Background(), usecase.TopUpRequest{
			UserID:         wallet.UserID.String(),
			Amount:         2_000,
			IdempotencyKey: "pg-high-fanout-topup",
			RequestHash:    hashValue,
		})
	})

	require.Equal(t, 1, totalNonReplayCount)
	require.Equal(t, fanout*rounds-1, totalReplayCount)
	verifyWalletBalance(t, db, wallet.ID.String(), 3_000)
	assertTableCount(t, db, &entity.Transaction{}, 1)
	assertTableCount(t, db, &entity.LedgerEntry{}, 1)
}

func TestPostgresHighFanoutTransfersWithSameIdempotencyKeyMutateOnce(t *testing.T) {
	db := testkit.NewPostgresTestDB(t).DB
	transactionUsecase, senderWallet, recipientWallet := newPostgresTransactionUsecase(t, db, 10_000, 1_000)

	fanout := 12
	rounds := 3
	hashValue, err := idempotency.HashPayload(map[string]interface{}{
		"recipient_wallet_id": recipientWallet.ID.String(),
		"amount":              int64(3_000),
	})
	require.NoError(t, err)

	totalReplayCount, totalNonReplayCount := runRepeatedIdempotentRequests(t, fanout, rounds, func() (*usecase.OperationResult, error) {
		return transactionUsecase.Transfer(context.Background(), usecase.TransferRequest{
			UserID:            senderWallet.UserID.String(),
			RecipientWalletID: recipientWallet.ID.String(),
			Amount:            3_000,
			IdempotencyKey:    "pg-high-fanout-transfer",
			RequestHash:       hashValue,
		})
	})

	require.Equal(t, 1, totalNonReplayCount)
	require.Equal(t, fanout*rounds-1, totalReplayCount)
	verifyWalletBalance(t, db, senderWallet.ID.String(), 7_000)
	verifyWalletBalance(t, db, recipientWallet.ID.String(), 4_000)
	assertTableCount(t, db, &entity.Transaction{}, 1)
	assertTableCount(t, db, &entity.LedgerEntry{}, 2)
}

func TestPostgresHighFanoutCompetingTransfersRemainConsistent(t *testing.T) {
	db := testkit.NewPostgresTestDB(t).DB
	transactionUsecase, senderWallet, _ := newPostgresTransactionUsecase(t, db, 9_000, 0)

	recipientCount := 6
	amount := int64(3_000)
	recipients := make([]*entity.Wallet, 0, recipientCount)
	for i := 0; i < recipientCount; i++ {
		_, recipientWallet := seedPostgresWalletOwner(t, db, fmt.Sprintf("fanout-recipient-%d@example.com", i), 0)
		recipients = append(recipients, recipientWallet)
	}

	requests := make([]func() error, 0, recipientCount)
	for i, recipientWallet := range recipients {
		i := i
		recipientWallet := recipientWallet
		requests = append(requests, func() error {
			hashValue, err := idempotency.HashPayload(map[string]interface{}{
				"recipient_wallet_id": recipientWallet.ID.String(),
				"amount":              amount,
			})
			if err != nil {
				return err
			}
			_, err = transactionUsecase.Transfer(context.Background(), usecase.TransferRequest{
				UserID:            senderWallet.UserID.String(),
				RecipientWalletID: recipientWallet.ID.String(),
				Amount:            amount,
				IdempotencyKey:    fmt.Sprintf("pg-fanout-competing-transfer-%d", i),
				RequestHash:       hashValue,
			})
			return err
		})
	}

	errs := runConcurrentWithResults(t, requests)

	successCount := 0
	for _, err := range errs {
		if err == nil {
			successCount++
			continue
		}
		require.Contains(t, err.Error(), "insufficient balance")
	}

	require.Equal(t, 3, successCount)
	verifyWalletBalance(t, db, senderWallet.ID.String(), 0)

	totalRecipientBalance := int64(0)
	for _, recipientWallet := range recipients {
		totalRecipientBalance += getWalletBalance(t, db, recipientWallet.ID.String())
	}
	require.Equal(t, int64(9_000), totalRecipientBalance)
	assertTableCount(t, db, &entity.Transaction{}, 3)
	assertTableCount(t, db, &entity.LedgerEntry{}, 6)
}

func runRepeatedIdempotentRequests(t *testing.T, fanout, rounds int, call func() (*usecase.OperationResult, error)) (int, int) {
	t.Helper()

	replayCount := 0
	nonReplayCount := 0
	for round := 0; round < rounds; round++ {
		results := make(chan *usecase.OperationResult, fanout)
		errs := runConcurrent(t, fanout, func() error {
			result, err := call()
			if err == nil {
				results <- result
			}
			return err
		})
		close(results)

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
	}

	return replayCount, nonReplayCount
}
