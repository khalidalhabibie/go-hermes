package handler

import (
	"go-hermes/internal/delivery/http/dto"
	httpresponse "go-hermes/internal/delivery/http/response"
	"go-hermes/internal/middleware"
	"go-hermes/internal/pkg/apperror"
	"go-hermes/internal/pkg/idempotency"
	"go-hermes/internal/pkg/validator"
	"go-hermes/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

type UserHandler struct {
	users *usecase.UserUsecase
}

func NewUserHandler(users *usecase.UserUsecase) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) Me(c *fiber.Ctx) error {
	result, err := h.users.GetMe(c.Context(), c.Locals(middleware.ContextUserID).(string))
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, nil)
}

type WalletHandler struct {
	wallets      *usecase.WalletUsecase
	transactions *usecase.TransactionUsecase
	validator    *validator.Validator
}

func NewWalletHandler(wallets *usecase.WalletUsecase, transactions *usecase.TransactionUsecase, validator *validator.Validator) *WalletHandler {
	return &WalletHandler{
		wallets:      wallets,
		transactions: transactions,
		validator:    validator,
	}
}

func (h *WalletHandler) Me(c *fiber.Ctx) error {
	result, err := h.wallets.GetMyWallet(c.Context(), c.Locals(middleware.ContextUserID).(string))
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, nil)
}

func (h *WalletHandler) Balance(c *fiber.Ctx) error {
	result, err := h.wallets.GetMyBalance(c.Context(), c.Locals(middleware.ContextUserID).(string))
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, nil)
}

func (h *WalletHandler) TopUp(c *fiber.Ctx) error {
	var req dto.TopUpRequest
	if err := c.BodyParser(&req); err != nil {
		return httpresponse.Error(c, apperror.Validation([]map[string]string{{"field": "body", "message": "invalid request body"}}))
	}
	if details := h.validator.Struct(req); len(details) > 0 {
		return httpresponse.Error(c, apperror.Validation(details))
	}

	requestHash, err := idempotency.HashPayload(req)
	if err != nil {
		return httpresponse.Error(c, apperror.Internal(err))
	}
	result, err := h.transactions.TopUp(c.Context(), usecase.TopUpRequest{
		UserID:         c.Locals(middleware.ContextUserID).(string),
		Amount:         req.Amount,
		Description:    req.Description,
		IdempotencyKey: c.Get("Idempotency-Key"),
		RequestHash:    requestHash,
	})
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.RawJSON(c, result.StatusCode, result.Body)
}
