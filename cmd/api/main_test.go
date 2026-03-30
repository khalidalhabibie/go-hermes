package main

import (
	"context"
	"testing"

	"go-hermes/internal/config"
	"go-hermes/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestSeedAdminSkipsOutsideDevelopment(t *testing.T) {
	repos := testkit.NewMemoryRepositories()
	cfg := config.Config{
		App: config.AppConfig{
			Env: "production",
		},
		Seed: config.SeedConfig{
			EnableAdminSeed: true,
			AdminName:       "System Admin",
			AdminEmail:      "admin@example.com",
			AdminPassword:   "Password123!",
		},
	}

	err := seedAdmin(context.Background(), cfg, repos.TxManager, repos.Users, repos.Wallets, repos.Audits)

	require.NoError(t, err)
	require.Equal(t, 0, repos.Store.UserCount())
	require.Equal(t, 0, repos.Store.WalletCount())
}

func TestSeedAdminCreatesAdminInDevelopment(t *testing.T) {
	repos := testkit.NewMemoryRepositories()
	cfg := config.Config{
		App: config.AppConfig{
			Env: "development",
		},
		Seed: config.SeedConfig{
			EnableAdminSeed: true,
			AdminName:       "System Admin",
			AdminEmail:      "admin@example.com",
			AdminPassword:   "Password123!",
		},
	}

	err := seedAdmin(context.Background(), cfg, repos.TxManager, repos.Users, repos.Wallets, repos.Audits)

	require.NoError(t, err)
	require.Equal(t, 1, repos.Store.UserCount())
	require.Equal(t, 1, repos.Store.WalletCount())
}
