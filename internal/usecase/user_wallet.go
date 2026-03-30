package usecase

import (
	"context"
	"time"

	"go-hermes/internal/pkg/apperror"
	"go-hermes/internal/repository"

	"github.com/google/uuid"
)

type UserUsecase struct {
	users repository.UserRepository
}

func NewUserUsecase(users repository.UserRepository) *UserUsecase {
	return &UserUsecase{users: users}
}

func (u *UserUsecase) GetMe(ctx context.Context, userID string) (*UserResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, apperror.BadRequest("invalid user id")
	}

	user, err := u.users.GetByID(ctx, parsedUserID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apperror.NotFound("user not found")
		}
		return nil, apperror.Internal(err)
	}
	response := toUserResponse(user)
	return &response, nil
}

type WalletUsecase struct {
	wallets repository.WalletRepository
}

func NewWalletUsecase(wallets repository.WalletRepository) *WalletUsecase {
	return &WalletUsecase{wallets: wallets}
}

func (u *WalletUsecase) GetMyWallet(ctx context.Context, userID string) (*WalletResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, apperror.BadRequest("invalid user id")
	}

	wallet, err := u.wallets.GetByUserID(ctx, parsedUserID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apperror.NotFound("wallet not found")
		}
		return nil, apperror.Internal(err)
	}
	response := toWalletResponse(wallet)
	return &response, nil
}

func (u *WalletUsecase) GetMyBalance(ctx context.Context, userID string) (*BalanceResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, apperror.BadRequest("invalid user id")
	}

	wallet, err := u.wallets.GetByUserID(ctx, parsedUserID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apperror.NotFound("wallet not found")
		}
		return nil, apperror.Internal(err)
	}

	return &BalanceResponse{
		WalletID:  wallet.ID.String(),
		Balance:   wallet.Balance,
		Currency:  "IDR",
		UpdatedAt: wallet.UpdatedAt.Format(time.RFC3339),
	}, nil
}
