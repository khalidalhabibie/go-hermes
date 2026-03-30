package usecase_test

import (
	"context"
	"testing"

	"go-hermes/internal/usecase"
	"go-hermes/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestGetWalletDetailsSuccess(t *testing.T) {
	repos := testkit.NewMemoryRepositories()
	user, _ := testkit.NewUserBuilder().Build(t)
	wallet := testkit.NewWalletBuilder().WithUserID(user.ID).WithBalance(15000).Build()
	require.NoError(t, repos.Users.Create(context.Background(), user))
	require.NoError(t, repos.Wallets.Create(context.Background(), wallet))

	svc := usecase.NewWalletUsecase(repos.Wallets)
	result, err := svc.GetMyWallet(context.Background(), user.ID.String())

	require.NoError(t, err)
	require.Equal(t, wallet.ID.String(), result.ID)
	require.Equal(t, int64(15000), result.Balance)
}
