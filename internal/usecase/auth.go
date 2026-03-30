package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/apperror"
	"go-hermes/internal/pkg/auth"
	"go-hermes/internal/pkg/hash"
	"go-hermes/internal/repository"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type AuthUsecase struct {
	txManager repository.TransactionManager
	users     repository.UserRepository
	wallets   repository.WalletRepository
	audits    repository.AuditLogRepository
	jwt       *auth.JWTManager
}

func NewAuthUsecase(
	txManager repository.TransactionManager,
	users repository.UserRepository,
	wallets repository.WalletRepository,
	audits repository.AuditLogRepository,
	jwt *auth.JWTManager,
) *AuthUsecase {
	return &AuthUsecase{
		txManager: txManager,
		users:     users,
		wallets:   wallets,
		audits:    audits,
		jwt:       jwt,
	}
}

func (u *AuthUsecase) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Name == "" || req.Email == "" || len(req.Password) < 8 {
		return nil, apperror.Validation([]map[string]string{
			{"field": "name/email/password", "message": "name, email, and password minimum 8 chars are required"},
		})
	}

	if existing, err := u.users.GetByEmail(ctx, req.Email); err == nil && existing != nil {
		return nil, apperror.Conflict("email already registered")
	} else if err != nil && !repository.IsNotFound(err) {
		return nil, apperror.Internal(err)
	}

	passwordHash, err := hash.Password(req.Password)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	user := &entity.User{
		ID:           uuid.New(),
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Role:         entity.RoleUser,
	}
	wallet := &entity.Wallet{
		ID:      uuid.New(),
		UserID:  user.ID,
		Balance: 0,
		Status:  entity.WalletStatusActive,
	}

	if err := u.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := u.users.Create(txCtx, user); err != nil {
			return err
		}
		if err := u.wallets.Create(txCtx, wallet); err != nil {
			return err
		}

		metadata, _ := json.Marshal(map[string]interface{}{
			"email": user.Email,
			"role":  user.Role,
		})
		return u.audits.Create(txCtx, &entity.AuditLog{
			ID:          uuid.New(),
			ActorUserID: &user.ID,
			Action:      "USER_REGISTERED",
			EntityType:  "user",
			EntityID:    &user.ID,
			Metadata:    datatypes.JSON(metadata),
			CreatedAt:   time.Now(),
		})
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, apperror.Conflict("email already registered")
		}
		return nil, apperror.Internal(err)
	}

	return &RegisterResponse{
		User:   toUserResponse(user),
		Wallet: toWalletResponse(wallet),
	}, nil
}

func (u *AuthUsecase) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		return nil, apperror.Validation([]map[string]string{
			{"field": "email/password", "message": "email and password are required"},
		})
	}

	user, err := u.users.GetByEmail(ctx, req.Email)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apperror.Unauthorized("invalid credentials")
		}
		return nil, apperror.Internal(err)
	}

	if err := hash.Compare(req.Password, user.PasswordHash); err != nil {
		return nil, apperror.Unauthorized("invalid credentials")
	}

	token, expiresAt, err := u.jwt.GenerateToken(user)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	metadata, _ := json.Marshal(map[string]interface{}{
		"email": user.Email,
	})
	if err := u.audits.Create(ctx, &entity.AuditLog{
		ID:          uuid.New(),
		ActorUserID: &user.ID,
		Action:      "USER_LOGGED_IN",
		EntityType:  "user",
		EntityID:    &user.ID,
		Metadata:    datatypes.JSON(metadata),
		CreatedAt:   time.Now(),
	}); err != nil {
		return nil, apperror.Internal(err)
	}

	return &LoginResponse{
		AccessToken: token,
		ExpiresAt:   expiresAt,
		User:        toUserResponse(user),
	}, nil
}
