package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-hermes/internal/app"
	"go-hermes/internal/config"
	httpdelivery "go-hermes/internal/delivery/http"
	"go-hermes/internal/delivery/http/handler"
	"go-hermes/internal/entity"
	"go-hermes/internal/middleware"
	"go-hermes/internal/pkg/auth"
	"go-hermes/internal/pkg/database"
	"go-hermes/internal/pkg/hash"
	"go-hermes/internal/pkg/logger"
	"go-hermes/internal/pkg/ratelimit"
	redisclient "go-hermes/internal/pkg/redis"
	"go-hermes/internal/pkg/validator"
	"go-hermes/internal/repository"
	"go-hermes/internal/usecase"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.App.Env)

	db, err := database.Open(cfg.DB, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect database")
	}

	txManager := repository.NewTransactionManager(db)
	userRepo := repository.NewUserRepository(db)
	walletRepo := repository.NewWalletRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	ledgerRepo := repository.NewLedgerRepository(db)
	idempotencyRepo := repository.NewIdempotencyRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	healthRepo := repository.NewHealthRepository(db)
	webhookRepo := repository.NewWebhookDeliveryRepository(db)

	redis, err := redisclient.NewClient(cfg.Redis, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect redis")
	}

	redisLimiter := ratelimit.NewRedisLimiter(redis)

	jwtManager := auth.NewJWTManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.ExpiryMinutes)
	requestValidator := validator.New()
	webhookService := usecase.NewWebhookService(cfg.Webhook, webhookRepo, nil, log)

	authUsecase := usecase.NewAuthUsecase(txManager, userRepo, walletRepo, auditRepo, jwtManager)
	userUsecase := usecase.NewUserUsecase(userRepo)
	walletUsecase := usecase.NewWalletUsecase(walletRepo)
	transactionUsecase := usecase.NewTransactionUsecase(txManager, walletRepo, transactionRepo, ledgerRepo, idempotencyRepo, auditRepo, webhookService)
	adminUsecase := usecase.NewAdminUsecase(auditRepo, transactionRepo, webhookRepo)
	healthUsecase := usecase.NewHealthUsecase(healthRepo)

	if err := seedAdmin(context.Background(), cfg, userRepo, walletRepo, auditRepo); err != nil {
		log.Fatal().Err(err).Msg("failed to seed admin user")
	}

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	webhookService.Start(workerCtx)

	server := app.NewHTTPApp(cfg.App.Name, jwtManager, middleware.RequestLogger(log), httpdelivery.Handlers{
		Auth:   handler.NewAuthHandler(authUsecase, requestValidator),
		User:   handler.NewUserHandler(userUsecase),
		Wallet: handler.NewWalletHandler(walletUsecase, transactionUsecase, requestValidator),
		Tx:     handler.NewTransactionHandler(transactionUsecase, requestValidator),
		Ledger: handler.NewLedgerHandler(transactionUsecase),
		Admin:  handler.NewAdminHandler(adminUsecase),
		Health: handler.NewHealthHandler(healthUsecase),
	}, httpdelivery.RouteMiddleware{
		Login:    middleware.RateLimit("login", redisLimiter, cfg.RateLimit.Login, time.Duration(cfg.RateLimit.WindowSeconds)*time.Second),
		TopUp:    middleware.RateLimit("topup", redisLimiter, cfg.RateLimit.TopUp, time.Duration(cfg.RateLimit.WindowSeconds)*time.Second),
		Transfer: middleware.RateLimit("transfer", redisLimiter, cfg.RateLimit.Transfer, time.Duration(cfg.RateLimit.WindowSeconds)*time.Second),
	}, cfg.Docs.Enabled)

	go func() {
		address := ":" + cfg.App.Port
		log.Info().Str("address", address).Msg("starting api server")
		if err := server.Listen(address); err != nil {
			log.Fatal().Err(err).Msg("fiber server stopped")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down server")
	_ = server.ShutdownWithTimeout(10 * time.Second)
}

func seedAdmin(ctx context.Context, cfg config.Config, users repository.UserRepository, wallets repository.WalletRepository, audits repository.AuditLogRepository) error {
	if !cfg.Seed.EnableAdminSeed || cfg.Seed.AdminEmail == "" || cfg.Seed.AdminPassword == "" {
		return nil
	}

	existing, err := users.GetByEmail(ctx, cfg.Seed.AdminEmail)
	if err == nil && existing != nil {
		return nil
	}
	if err != nil && !repository.IsNotFound(err) {
		return err
	}

	passwordHash, err := hash.Password(cfg.Seed.AdminPassword)
	if err != nil {
		return err
	}

	adminID := uuid.New()
	admin := &entity.User{
		ID:           adminID,
		Name:         cfg.Seed.AdminName,
		Email:        cfg.Seed.AdminEmail,
		PasswordHash: passwordHash,
		Role:         entity.RoleAdmin,
	}
	wallet := &entity.Wallet{
		ID:      uuid.New(),
		UserID:  adminID,
		Balance: 0,
		Status:  entity.WalletStatusActive,
	}
	if err := users.Create(ctx, admin); err != nil {
		return err
	}
	if err := wallets.Create(ctx, wallet); err != nil {
		return err
	}

	metadata, _ := json.Marshal(map[string]string{"email": admin.Email})
	return audits.Create(ctx, &entity.AuditLog{
		ID:          uuid.New(),
		ActorUserID: &adminID,
		Action:      "ADMIN_SEEDED",
		EntityType:  "user",
		EntityID:    &adminID,
		Metadata:    datatypes.JSON(metadata),
		CreatedAt:   time.Now(),
	})
}
