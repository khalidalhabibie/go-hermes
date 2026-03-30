package handler

import (
	"strconv"

	"go-hermes/internal/delivery/http/dto"
	httpresponse "go-hermes/internal/delivery/http/response"
	"go-hermes/internal/middleware"
	"go-hermes/internal/pkg/apperror"
	"go-hermes/internal/pkg/idempotency"
	"go-hermes/internal/pkg/pagination"
	"go-hermes/internal/pkg/validator"
	"go-hermes/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

type TransactionHandler struct {
	transactions *usecase.TransactionUsecase
	validator    *validator.Validator
}

func NewTransactionHandler(transactions *usecase.TransactionUsecase, validator *validator.Validator) *TransactionHandler {
	return &TransactionHandler{
		transactions: transactions,
		validator:    validator,
	}
}

func (h *TransactionHandler) Transfer(c *fiber.Ctx) error {
	var req dto.TransferRequest
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
	result, err := h.transactions.Transfer(c.Context(), usecase.TransferRequest{
		UserID:            c.Locals(middleware.ContextUserID).(string),
		RecipientWalletID: req.RecipientWalletID,
		Amount:            req.Amount,
		Description:       req.Description,
		IdempotencyKey:    c.Get("Idempotency-Key"),
		RequestHash:       requestHash,
	})
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.RawJSON(c, result.StatusCode, result.Body)
}

func (h *TransactionHandler) ListMine(c *fiber.Ctx) error {
	params := pagination.New(parseInt(c.Query("page"), 1), parseInt(c.Query("limit"), 10))
	result, meta, err := h.transactions.ListMyTransactions(c.Context(), c.Locals(middleware.ContextUserID).(string), params)
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, meta)
}

func (h *TransactionHandler) GetOne(c *fiber.Ctx) error {
	result, err := h.transactions.GetMyTransaction(c.Context(), c.Locals(middleware.ContextUserID).(string), c.Params("id"))
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, nil)
}

type LedgerHandler struct {
	transactions *usecase.TransactionUsecase
}

func NewLedgerHandler(transactions *usecase.TransactionUsecase) *LedgerHandler {
	return &LedgerHandler{transactions: transactions}
}

func (h *LedgerHandler) ListMine(c *fiber.Ctx) error {
	params := pagination.New(parseInt(c.Query("page"), 1), parseInt(c.Query("limit"), 10))
	result, meta, err := h.transactions.ListMyLedgers(c.Context(), c.Locals(middleware.ContextUserID).(string), params)
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, meta)
}

func (h *LedgerHandler) ListByTransaction(c *fiber.Ctx) error {
	result, err := h.transactions.ListTransactionLedgers(c.Context(), c.Locals(middleware.ContextUserID).(string), c.Params("transactionId"))
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, nil)
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
