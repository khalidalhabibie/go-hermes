package tests

import (
	"net/http"
	"testing"

	"go-hermes/internal/entity"
	"go-hermes/tests/testkit"

	"github.com/stretchr/testify/require"
)

type reconciliationData struct {
	CheckedAt string `json:"checked_at"`
	Healthy   bool   `json:"healthy"`
	Summary   struct {
		WalletsChecked      int `json:"wallets_checked"`
		TransactionsChecked int `json:"transactions_checked"`
		IssueCount          int `json:"issue_count"`
	} `json:"summary"`
}

func TestAdminReconciliationEndpointReturnsReport(t *testing.T) {
	h := testkit.NewTestHarness(t)
	admin := h.SeedUser(t, testkit.NewUserBuilder().WithRole(entity.RoleAdmin).WithEmail("admin-http-reconciliation@example.com"), 0)
	token := h.MustIssueToken(t, admin.User)

	resp, body := testkit.PerformJSONRequest(t, h.App, http.MethodGet, "/api/v1/admin/reconciliation", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	result := testkit.DecodeSuccess[reconciliationData](t, body)
	require.True(t, result.Data.Healthy)
	require.Equal(t, 1, result.Data.Summary.WalletsChecked)
	require.Equal(t, 0, result.Data.Summary.TransactionsChecked)
	require.Equal(t, 0, result.Data.Summary.IssueCount)
}
