package handler

import (
	httpresponse "go-hermes/internal/delivery/http/response"
	"go-hermes/internal/middleware"
	"go-hermes/internal/pkg/apiresponse"
	"go-hermes/internal/pkg/pagination"
	"go-hermes/internal/repository"
	"go-hermes/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

type AdminHandler struct {
	admin          *usecase.AdminUsecase
	reconciliation *usecase.ReconciliationUsecase
}

func NewAdminHandler(admin *usecase.AdminUsecase, reconciliation *usecase.ReconciliationUsecase) *AdminHandler {
	return &AdminHandler{
		admin:          admin,
		reconciliation: reconciliation,
	}
}

func (h *AdminHandler) AuditLogs(c *fiber.Ctx) error {
	params := pagination.New(parseInt(c.Query("page"), 1), parseInt(c.Query("limit"), 10))
	result, meta, err := h.admin.ListAuditLogs(c.Context(), c.Locals(middleware.ContextUserID).(string), params)
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, meta)
}

func (h *AdminHandler) Transactions(c *fiber.Ctx) error {
	params := pagination.New(parseInt(c.Query("page"), 1), parseInt(c.Query("limit"), 10))
	result, meta, err := h.admin.ListTransactions(c.Context(), c.Locals(middleware.ContextUserID).(string), params)
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, meta)
}

func (h *AdminHandler) Webhooks(c *fiber.Ctx) error {
	params := pagination.New(parseInt(c.Query("page"), 1), parseInt(c.Query("limit"), 10))
	result, meta, err := h.admin.ListWebhooks(c.Context(), c.Locals(middleware.ContextUserID).(string), repository.WebhookDeliveryFilter{
		EventType:      c.Query("event_type"),
		Status:         c.Query("status"),
		TransactionRef: c.Query("transaction_ref"),
	}, params)
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, meta)
}

func (h *AdminHandler) Webhook(c *fiber.Ctx) error {
	result, err := h.admin.GetWebhook(c.Context(), c.Locals(middleware.ContextUserID).(string), c.Params("id"))
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, nil)
}

func (h *AdminHandler) Reconciliation(c *fiber.Ctx) error {
	result, err := h.reconciliation.Run(c.Context(), c.Locals(middleware.ContextUserID).(string))
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, nil)
}

type HealthHandler struct {
	health *usecase.HealthUsecase
}

func NewHealthHandler(health *usecase.HealthUsecase) *HealthHandler {
	return &HealthHandler{health: health}
}

func (h *HealthHandler) Check(c *fiber.Ctx) error {
	if err := h.health.Check(c.Context()); err != nil {
		return httpresponse.Error(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(apiresponse.Success(map[string]string{"status": "ok"}, nil))
}
