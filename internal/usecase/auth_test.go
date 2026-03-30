package usecase_test

import (
	"context"
	"testing"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/auth"
	"go-hermes/internal/usecase"
	"go-hermes/tests/testkit"

	"github.com/stretchr/testify/require"
)

func newAuthUsecaseForTest() (*usecase.AuthUsecase, *testkit.MemoryRepositories) {
	repos := testkit.NewMemoryRepositories()
	jwtManager := auth.NewJWTManager("secret", "test", 60)
	return usecase.NewAuthUsecase(repos.TxManager, repos.Users, repos.Wallets, repos.Audits, jwtManager), repos
}

func TestRegisterSuccess(t *testing.T) {
	svc, repos := newAuthUsecaseForTest()

	result, err := svc.Register(context.Background(), usecase.RegisterRequest{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "Password123",
	})

	require.NoError(t, err)
	require.Equal(t, "Alice", result.User.Name)
	require.Equal(t, int64(0), result.Wallet.Balance)
	require.Equal(t, 1, repos.Store.UserCount())
	require.Equal(t, 1, repos.Store.WalletCount())
}

func TestRegisterDuplicateEmail(t *testing.T) {
	svc, _ := newAuthUsecaseForTest()

	_, err := svc.Register(context.Background(), usecase.RegisterRequest{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "Password123",
	})
	require.NoError(t, err)

	result, err := svc.Register(context.Background(), usecase.RegisterRequest{
		Name:     "Alice 2",
		Email:    "alice@example.com",
		Password: "Password123",
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "email already registered")
}

func TestLoginSuccess(t *testing.T) {
	svc, _ := newAuthUsecaseForTest()

	registerResult, err := svc.Register(context.Background(), usecase.RegisterRequest{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "Password123",
	})
	require.NoError(t, err)

	loginResult, err := svc.Login(context.Background(), usecase.LoginRequest{
		Email:    registerResult.User.Email,
		Password: "Password123",
	})

	require.NoError(t, err)
	require.NotEmpty(t, loginResult.AccessToken)
	require.Equal(t, registerResult.User.Email, loginResult.User.Email)
}

func TestLoginInvalidPassword(t *testing.T) {
	svc, repos := newAuthUsecaseForTest()
	user, password := testkit.NewUserBuilder().WithEmail("alice@example.com").Build(t)
	require.NotEmpty(t, password)
	wallet := testkit.NewWalletBuilder().WithUserID(user.ID).Build()
	require.NoError(t, repos.Users.Create(context.Background(), user))
	require.NoError(t, repos.Wallets.Create(context.Background(), wallet))

	result, err := svc.Login(context.Background(), usecase.LoginRequest{
		Email:    user.Email,
		Password: "WrongPassword123",
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid credentials")
}

func TestGetCurrentUserSuccess(t *testing.T) {
	repos := testkit.NewMemoryRepositories()
	user, _ := testkit.NewUserBuilder().WithRole(entity.RoleUser).Build(t)
	require.NoError(t, repos.Users.Create(context.Background(), user))

	svc := usecase.NewUserUsecase(repos.Users)
	result, err := svc.GetMe(context.Background(), user.ID.String())

	require.NoError(t, err)
	require.Equal(t, user.Email, result.Email)
}
