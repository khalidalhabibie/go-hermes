package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const minJWTSecretLength = 32

type Config struct {
	App           AppConfig
	DB            DBConfig
	Redis         RedisConfig
	RateLimit     RateLimitConfig
	JWT           JWTConfig
	Seed          SeedConfig
	Docs          DocsConfig
	Observability ObservabilityConfig
	Webhook       WebhookConfig
}

type AppConfig struct {
	Name string
	Env  string
	Port string
}

type DBConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	TimeZone        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type JWTConfig struct {
	Secret        string
	Issuer        string
	ExpiryMinutes int
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type RateLimitConfig struct {
	WindowSeconds int
	Login         int
	TopUp         int
	Transfer      int
}

type SeedConfig struct {
	EnableAdminSeed bool
	AdminName       string
	AdminEmail      string
	AdminPassword   string
}

type DocsConfig struct {
	Enabled bool
}

type ObservabilityConfig struct {
	MetricsEnabled bool
	MetricsToken   string
}

type WebhookConfig struct {
	Enabled              bool
	TargetURL            string
	Secret               string
	MaxRetry             int
	RetryIntervalSeconds int
	WorkerBatchSize      int
}

func Load() Config {
	appEnv := getEnv("APP_ENV", "development")
	enableAdminSeedByDefault := isDevelopmentEnv(appEnv)

	return Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "go-hermes"),
			Env:  appEnv,
			Port: getEnv("APP_PORT", "8080"),
		},
		DB: DBConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5432"),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", "postgres"),
			Name:            getEnv("DB_NAME", "go_hermes"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			TimeZone:        getEnv("DB_TIMEZONE", "Asia/Jakarta"),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 25),
			ConnMaxLifetime: time.Duration(getEnvAsInt("DB_CONN_MAX_LIFETIME_MINUTES", 30)) * time.Minute,
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		RateLimit: RateLimitConfig{
			WindowSeconds: getEnvAsInt("RATE_LIMIT_WINDOW_SECONDS", 60),
			Login:         getEnvAsInt("RATE_LIMIT_LOGIN", 10),
			TopUp:         getEnvAsInt("RATE_LIMIT_TOPUP", 20),
			Transfer:      getEnvAsInt("RATE_LIMIT_TRANSFER", 20),
		},
		JWT: JWTConfig{
			Secret:        getEnv("JWT_SECRET", "change-me"),
			Issuer:        getEnv("JWT_ISSUER", "go-hermes"),
			ExpiryMinutes: getEnvAsInt("JWT_EXPIRY_MINUTES", 60),
		},
		Seed: SeedConfig{
			EnableAdminSeed: getEnvAsBool("SEED_ADMIN_ENABLED", enableAdminSeedByDefault),
			AdminName:       getEnv("SEED_ADMIN_NAME", "System Admin"),
			AdminEmail:      getEnv("SEED_ADMIN_EMAIL", "admin@gohermes.local"),
			AdminPassword:   getEnv("SEED_ADMIN_PASSWORD", "ChangeMe123!"),
		},
		Docs: DocsConfig{
			Enabled: getEnvAsBool("DOCS_ENABLED", true),
		},
		Observability: ObservabilityConfig{
			MetricsEnabled: getEnvAsBool("METRICS_ENABLED", true),
			MetricsToken:   getEnv("METRICS_TOKEN", ""),
		},
		Webhook: WebhookConfig{
			Enabled:              getEnvAsBool("WEBHOOK_ENABLED", false),
			TargetURL:            getEnv("WEBHOOK_TARGET_URL", ""),
			Secret:               getEnv("WEBHOOK_SECRET", ""),
			MaxRetry:             getEnvAsInt("WEBHOOK_MAX_RETRY", 3),
			RetryIntervalSeconds: getEnvAsInt("WEBHOOK_RETRY_INTERVAL_SECONDS", 30),
			WorkerBatchSize:      getEnvAsInt("WEBHOOK_WORKER_BATCH_SIZE", 20),
		},
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.JWT.Issuer) == "" {
		return fmt.Errorf("JWT_ISSUER must be set")
	}

	if !c.App.IsDevelopment() && isWeakJWTSecret(c.JWT.Secret) {
		return fmt.Errorf("JWT_SECRET must be set to a non-default value with at least %d characters outside development", minJWTSecretLength)
	}

	if c.Observability.MetricsEnabled && !c.App.IsDevelopment() && strings.TrimSpace(c.Observability.MetricsToken) == "" {
		return fmt.Errorf("METRICS_TOKEN must be set when metrics are enabled outside development")
	}

	return nil
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		c.Host,
		c.Port,
		c.User,
		c.Password,
		c.Name,
		c.SSLMode,
		c.TimeZone,
	)
}

func (c RedisConfig) Address() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

func (c AppConfig) IsDevelopment() bool {
	return isDevelopmentEnv(c.Env)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvAsBool(key string, fallback bool) bool {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func isDevelopmentEnv(env string) bool {
	return strings.EqualFold(env, "development") || strings.EqualFold(env, "dev")
}

func isWeakJWTSecret(secret string) bool {
	secret = strings.TrimSpace(secret)
	if len(secret) < minJWTSecretLength {
		return true
	}

	switch strings.ToLower(secret) {
	case "change-me", "super-secret-change-me", "dev-only-change-me", "jwt-secret", "secret", "test-secret":
		return true
	default:
		return false
	}
}
