package handler

import (
	"go-hermes/internal/delivery/http/dto"
	httpresponse "go-hermes/internal/delivery/http/response"
	"go-hermes/internal/pkg/apperror"
	"go-hermes/internal/pkg/validator"
	"go-hermes/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	auth      *usecase.AuthUsecase
	validator *validator.Validator
}

func NewAuthHandler(auth *usecase.AuthUsecase, validator *validator.Validator) *AuthHandler {
	return &AuthHandler{
		auth:      auth,
		validator: validator,
	}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return httpresponse.Error(c, apperror.Validation([]map[string]string{{"field": "body", "message": "invalid request body"}}))
	}
	if details := h.validator.Struct(req); len(details) > 0 {
		return httpresponse.Error(c, apperror.Validation(details))
	}

	result, err := h.auth.Register(c.Context(), usecase.RegisterRequest{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusCreated, result, nil)
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return httpresponse.Error(c, apperror.Validation([]map[string]string{{"field": "body", "message": "invalid request body"}}))
	}
	if details := h.validator.Struct(req); len(details) > 0 {
		return httpresponse.Error(c, apperror.Validation(details))
	}

	result, err := h.auth.Login(c.Context(), usecase.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return httpresponse.Error(c, err)
	}
	return httpresponse.Success(c, fiber.StatusOK, result, nil)
}
