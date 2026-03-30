package testkit

import (
	"context"
	"net/http"
	"testing"
	"time"

	"go-hermes/internal/app"
	"go-hermes/internal/config"
	httpdelivery "go-hermes/internal/delivery/http"
	"go-hermes/internal/delivery/http/handler"
	"go-hermes/internal/entity"
	"go-hermes/internal/middleware"
	"go-hermes/internal/pkg/auth"
	"go-hermes/internal/pkg/logger"
	"go-hermes/internal/pkg/metrics"
	"go-hermes/internal/pkg/ratelimit"
	"go-hermes/internal/pkg/validator"
	"go-hermes/internal/usecase"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type TestHarness struct {
	App              *fiber.App
	Repos            *MemoryRepositories
	JWTManager       *auth.JWTManager
	AuthUsecase      *usecase.AuthUsecase
	UserUsecase      *usecase.UserUsecase
	WalletUsecase    *usecase.WalletUsecase
	TransactionUC    *usecase.TransactionUsecase
	AdminUsecase     *usecase.AdminUsecase
	ReconciliationUC *usecase.ReconciliationUsecase
	HealthUsecase    *usecase.HealthUsecase
	WebhookService   *usecase.WebhookService
	RequestValidate  *validator.Validator
}

type SeededUser struct {
	User     *entity.User
	Wallet   *entity.Wallet
	Password string
}

type HarnessOptions struct {
	LoginRateLimit     int
	TopUpRateLimit     int
	TransferRateLimit  int
	RateLimitWindow    time.Duration
	MetricsToken       string
	WebhookEnabled     bool
	WebhookTargetURL   string
	WebhookSecret      string
	WebhookMaxRetry    int
	WebhookRetryDelay  time.Duration
	WebhookHTTPClient  *http.Client
	StartWebhookWorker bool
}

type HarnessOption func(*HarnessOptions)

func WithRateLimit(login, topUp, transfer int, window time.Duration) HarnessOption {
	return func(options *HarnessOptions) {
		options.LoginRateLimit = login
		options.TopUpRateLimit = topUp
		options.TransferRateLimit = transfer
		options.RateLimitWindow = window
	}
}

func WithMetricsToken(token string) HarnessOption {
	return func(options *HarnessOptions) {
		options.MetricsToken = token
	}
}

func WithWebhook(targetURL, secret string, maxRetry int, retryDelay time.Duration, client *http.Client, startWorker bool) HarnessOption {
	return func(options *HarnessOptions) {
		options.WebhookEnabled = true
		options.WebhookTargetURL = targetURL
		options.WebhookSecret = secret
		options.WebhookMaxRetry = maxRetry
		options.WebhookRetryDelay = retryDelay
		options.WebhookHTTPClient = client
		options.StartWebhookWorker = startWorker
	}
}

func NewTestHarness(t *testing.T, opts ...HarnessOption) *TestHarness {
	t.Helper()

	options := HarnessOptions{
		LoginRateLimit:    0,
		TopUpRateLimit:    0,
		TransferRateLimit: 0,
		RateLimitWindow:   time.Minute,
		WebhookMaxRetry:   3,
		WebhookRetryDelay: 50 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(&options)
	}

	repos := NewMemoryRepositories()
	jwtManager := auth.NewJWTManager("test-secret", "go-hermes-test", 60)
	validate := validator.New()
	testLogger := logger.New("test")
	metricsCollector := metrics.NewCollector()

	webhookService := usecase.NewWebhookService(config.WebhookConfig{
		Enabled:              options.WebhookEnabled,
		TargetURL:            options.WebhookTargetURL,
		Secret:               options.WebhookSecret,
		MaxRetry:             options.WebhookMaxRetry,
		RetryIntervalSeconds: retryDelaySeconds(options.WebhookRetryDelay),
		WorkerBatchSize:      20,
	}, repos.Webhooks, options.WebhookHTTPClient, testLogger, metricsCollector)

	authUsecase := usecase.NewAuthUsecase(repos.TxManager, repos.Users, repos.Wallets, repos.Audits, jwtManager)
	userUsecase := usecase.NewUserUsecase(repos.Users)
	walletUsecase := usecase.NewWalletUsecase(repos.Wallets)
	transactionUsecase := usecase.NewTransactionUsecase(repos.TxManager, repos.Wallets, repos.Transactions, repos.Ledgers, repos.Idempotency, repos.Audits, webhookService)
	adminUsecase := usecase.NewAdminUsecase(repos.Audits, repos.Transactions, repos.Webhooks)
	reconciliationUsecase := usecase.NewReconciliationUsecase(repos.Reconciliation, repos.Audits)
	healthUsecase := usecase.NewHealthUsecase(repos.Health)

	appInstance := app.NewHTTPApp("go-hermes-test", jwtManager, middleware.RequestLogger(testLogger), httpdelivery.Handlers{
		Auth:   handler.NewAuthHandler(authUsecase, validate),
		User:   handler.NewUserHandler(userUsecase),
		Wallet: handler.NewWalletHandler(walletUsecase, transactionUsecase, validate),
		Tx:     handler.NewTransactionHandler(transactionUsecase, validate),
		Ledger: handler.NewLedgerHandler(transactionUsecase),
		Admin:  handler.NewAdminHandler(adminUsecase, reconciliationUsecase),
		Health: handler.NewHealthHandler(healthUsecase),
	}, httpdelivery.RouteMiddleware{
		Login:    middleware.RateLimit("login", ratelimit.NewMemoryLimiter(), options.LoginRateLimit, options.RateLimitWindow, metricsCollector, testLogger),
		TopUp:    middleware.RateLimit("topup", ratelimit.NewMemoryLimiter(), options.TopUpRateLimit, options.RateLimitWindow, metricsCollector, testLogger),
		Transfer: middleware.RateLimit("transfer", ratelimit.NewMemoryLimiter(), options.TransferRateLimit, options.RateLimitWindow, metricsCollector, testLogger),
	}, httpdelivery.Instrumentation{
		TraceContext:    middleware.TraceContext(),
		Metrics:         middleware.Metrics(metricsCollector),
		MetricsAuth:     middleware.ProtectMetrics(options.MetricsToken),
		MetricsEndpoint: metricsCollector.FiberHandler(),
	}, false)

	if options.WebhookEnabled && options.StartWebhookWorker {
		webhookService.Start(context.Background())
	}

	return &TestHarness{
		App:              appInstance,
		Repos:            repos,
		JWTManager:       jwtManager,
		AuthUsecase:      authUsecase,
		UserUsecase:      userUsecase,
		WalletUsecase:    walletUsecase,
		TransactionUC:    transactionUsecase,
		AdminUsecase:     adminUsecase,
		ReconciliationUC: reconciliationUsecase,
		HealthUsecase:    healthUsecase,
		WebhookService:   webhookService,
		RequestValidate:  validate,
	}
}

func (h *TestHarness) SeedUser(t *testing.T, builder *UserBuilder, balance int64) SeededUser {
	t.Helper()

	user, password := builder.Build(t)
	wallet := NewWalletBuilder().WithUserID(user.ID).WithBalance(balance).Build()

	require.NoError(t, h.Repos.Users.Create(context.Background(), user))
	require.NoError(t, h.Repos.Wallets.Create(context.Background(), wallet))

	return SeededUser{
		User:     user,
		Wallet:   wallet,
		Password: password,
	}
}

func (h *TestHarness) MustIssueToken(t *testing.T, user *entity.User) string {
	t.Helper()

	token, _, err := h.JWTManager.GenerateToken(user)
	require.NoError(t, err)
	return token
}

func MustParseUUID(t *testing.T, value string) uuid.UUID {
	t.Helper()

	id, err := uuid.Parse(value)
	require.NoError(t, err)
	return id
}

func retryDelaySeconds(delay time.Duration) int {
	if delay < time.Second {
		return 1
	}
	return int(delay / time.Second)
}
