package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaultsDisableAdminSeedingOutsideDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SEED_ADMIN_ENABLED", "")

	cfg := Load()

	require.False(t, cfg.Seed.EnableAdminSeed)
}

func TestLoadDefaultsEnableAdminSeedingInDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("SEED_ADMIN_ENABLED", "")

	cfg := Load()

	require.True(t, cfg.Seed.EnableAdminSeed)
}

func TestValidateRejectsWeakJWTSecretOutsideDevelopment(t *testing.T) {
	cfg := Config{
		App: AppConfig{
			Env: "production",
		},
		JWT: JWTConfig{
			Secret: "change-me",
			Issuer: "go-hermes",
		},
	}

	err := cfg.Validate()

	require.Error(t, err)
	require.Contains(t, err.Error(), "JWT_SECRET")
}

func TestValidateRequiresMetricsTokenOutsideDevelopment(t *testing.T) {
	cfg := Config{
		App: AppConfig{
			Env: "production",
		},
		JWT: JWTConfig{
			Secret: "01234567890123456789012345678901",
			Issuer: "go-hermes",
		},
		Observability: ObservabilityConfig{
			MetricsEnabled: true,
		},
	}

	err := cfg.Validate()

	require.Error(t, err)
	require.Contains(t, err.Error(), "METRICS_TOKEN")
}

func TestValidateAllowsDevelopmentConvenienceDefaults(t *testing.T) {
	cfg := Config{
		App: AppConfig{
			Env: "development",
		},
		JWT: JWTConfig{
			Secret: "change-me",
			Issuer: "go-hermes",
		},
		Observability: ObservabilityConfig{
			MetricsEnabled: true,
		},
	}

	require.NoError(t, cfg.Validate())
}
