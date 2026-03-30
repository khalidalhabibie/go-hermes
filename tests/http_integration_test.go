package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"go-hermes/tests/testkit"

	"github.com/stretchr/testify/require"
)

type loginData struct {
	AccessToken string `json:"access_token"`
	User        struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

type walletData struct {
	ID      string `json:"id"`
	UserID  string `json:"user_id"`
	Balance int64  `json:"balance"`
	Status  string `json:"status"`
}

type registerData struct {
	User struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
	Wallet walletData `json:"wallet"`
}

type topUpData struct {
	Transaction struct {
		ID     string `json:"id"`
		Amount int64  `json:"amount"`
	} `json:"transaction"`
	Wallet walletData `json:"wallet"`
}

type transferData struct {
	Transaction struct {
		ID     string `json:"id"`
		Amount int64  `json:"amount"`
	} `json:"transaction"`
	SourceWallet      walletData `json:"source_wallet"`
	DestinationWallet walletData `json:"destination_wallet"`
}

type transactionItem struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Amount int64  `json:"amount"`
}

func TestRegisterLoginGetWalletFlow(t *testing.T) {
	h := testkit.NewTestHarness(t)

	registerResp, registerBody := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"name":     "Alice",
		"email":    "alice@example.com",
		"password": "Password123",
	}, nil)
	require.Equal(t, http.StatusCreated, registerResp.StatusCode)
	registerResult := testkit.DecodeSuccess[registerData](t, registerBody)

	loginResp, loginBody := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{
		"email":    "alice@example.com",
		"password": "Password123",
	}, nil)
	require.Equal(t, http.StatusOK, loginResp.StatusCode)
	loginResult := testkit.DecodeSuccess[loginData](t, loginBody)

	walletResp, walletBody := testkit.PerformJSONRequest(t, h.App, http.MethodGet, "/api/v1/wallets/me", nil, map[string]string{
		"Authorization": "Bearer " + loginResult.Data.AccessToken,
	})
	require.Equal(t, http.StatusOK, walletResp.StatusCode)
	walletResult := testkit.DecodeSuccess[walletData](t, walletBody)

	require.Equal(t, registerResult.Data.User.ID, loginResult.Data.User.ID)
	require.Equal(t, registerResult.Data.Wallet.ID, walletResult.Data.ID)
	require.Equal(t, int64(0), walletResult.Data.Balance)
}

func TestTopUpFlowEndToEndWithIdempotencyReplay(t *testing.T) {
	h := testkit.NewTestHarness(t)
	user := h.SeedUser(t, testkit.NewUserBuilder().WithEmail("topup@example.com"), 1000)
	token := h.MustIssueToken(t, user.User)

	firstResp, firstBody := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/wallets/me/top-up", map[string]interface{}{
		"amount":      5000,
		"description": "initial funding",
	}, map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": "topup-flow-1",
	})
	require.Equal(t, http.StatusOK, firstResp.StatusCode)
	firstResult := testkit.DecodeSuccess[topUpData](t, firstBody)

	secondResp, secondBody := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/wallets/me/top-up", map[string]interface{}{
		"amount":      5000,
		"description": "initial funding",
	}, map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": "topup-flow-1",
	})
	require.Equal(t, http.StatusOK, secondResp.StatusCode)
	secondResult := testkit.DecodeSuccess[topUpData](t, secondBody)

	require.Equal(t, firstResult.Data.Transaction.ID, secondResult.Data.Transaction.ID)
	require.Equal(t, int64(6000), secondResult.Data.Wallet.Balance)
	require.Equal(t, 1, h.Repos.Store.TransactionCount())
	require.Equal(t, 1, h.Repos.Store.LedgerCount())

	walletAfter, ok := h.Repos.Store.WalletByUserID(user.User.ID)
	require.True(t, ok)
	require.Equal(t, int64(6000), walletAfter.Balance)
}

func TestTopUpFlowIdempotencyConflictDifferentPayload(t *testing.T) {
	h := testkit.NewTestHarness(t)
	user := h.SeedUser(t, testkit.NewUserBuilder().WithEmail("topup-conflict@example.com"), 1000)
	token := h.MustIssueToken(t, user.User)

	firstResp, _ := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/wallets/me/top-up", map[string]interface{}{
		"amount": 5000,
	}, map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": "topup-flow-2",
	})
	require.Equal(t, http.StatusOK, firstResp.StatusCode)

	secondResp, secondBody := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/wallets/me/top-up", map[string]interface{}{
		"amount": 7000,
	}, map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": "topup-flow-2",
	})
	require.Equal(t, http.StatusConflict, secondResp.StatusCode)
	errorResult := testkit.DecodeError(t, secondBody)
	require.Equal(t, "CONFLICT", errorResult.ErrorCode)
	require.Equal(t, 1, h.Repos.Store.TransactionCount())
	require.Equal(t, 1, h.Repos.Store.LedgerCount())

	walletAfter, ok := h.Repos.Store.WalletByUserID(user.User.ID)
	require.True(t, ok)
	require.Equal(t, int64(6000), walletAfter.Balance)
}

func TestTransferFlowEndToEnd(t *testing.T) {
	h := testkit.NewTestHarness(t)
	sender := h.SeedUser(t, testkit.NewUserBuilder().WithEmail("sender@example.com"), 10000)
	recipient := h.SeedUser(t, testkit.NewUserBuilder().WithEmail("recipient@example.com"), 1000)
	token := h.MustIssueToken(t, sender.User)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/transfers", map[string]interface{}{
		"recipient_wallet_id": recipient.Wallet.ID.String(),
		"amount":              3000,
		"description":         "peer transfer",
	}, map[string]string{
		"Authorization":   "Bearer " + token,
		"Idempotency-Key": "transfer-flow-1",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	result := testkit.DecodeSuccess[transferData](t, body)

	require.Equal(t, int64(7000), result.Data.SourceWallet.Balance)
	require.Equal(t, int64(4000), result.Data.DestinationWallet.Balance)
	require.Equal(t, 1, h.Repos.Store.TransactionCount())
	require.Equal(t, 2, h.Repos.Store.LedgerCount())
}

func TestTransactionListingFlow(t *testing.T) {
	h := testkit.NewTestHarness(t)
	user := h.SeedUser(t, testkit.NewUserBuilder().WithEmail("transactions@example.com"), 0)
	token := h.MustIssueToken(t, user.User)

	for _, key := range []string{"txn-list-1", "txn-list-2"} {
		resp, _ := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/wallets/me/top-up", map[string]interface{}{
			"amount": 1000,
		}, map[string]string{
			"Authorization":   "Bearer " + token,
			"Idempotency-Key": key,
		})
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodGet, "/api/v1/transactions/me?page=1&limit=10", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Message string            `json:"message"`
		Data    []transactionItem `json:"data"`
		Meta    json.RawMessage   `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.Len(t, envelope.Data, 2)
}

func TestUserCannotAccessAnotherUsersTransaction(t *testing.T) {
	h := testkit.NewTestHarness(t)
	owner := h.SeedUser(t, testkit.NewUserBuilder().WithEmail("owner@example.com"), 0)
	other := h.SeedUser(t, testkit.NewUserBuilder().WithEmail("other@example.com"), 0)

	ownerToken := h.MustIssueToken(t, owner.User)
	otherToken := h.MustIssueToken(t, other.User)

	topupResp, topupBody := testkit.PerformJSONRequest(t, h.App, http.MethodPost, "/api/v1/wallets/me/top-up", map[string]interface{}{
		"amount": 2500,
	}, map[string]string{
		"Authorization":   "Bearer " + ownerToken,
		"Idempotency-Key": "owner-txn-1",
	})
	require.Equal(t, http.StatusOK, topupResp.StatusCode)
	topupResult := testkit.DecodeSuccess[topUpData](t, topupBody)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodGet, "/api/v1/transactions/"+topupResult.Data.Transaction.ID, nil, map[string]string{
		"Authorization": "Bearer " + otherToken,
	})
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	errorResult := testkit.DecodeError(t, body)
	require.Equal(t, "NOT_FOUND", errorResult.ErrorCode)
}
