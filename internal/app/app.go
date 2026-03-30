package app

import (
	httpdelivery "go-hermes/internal/delivery/http"
	"go-hermes/internal/pkg/apiresponse"
	"go-hermes/internal/pkg/apperror"
	"go-hermes/internal/pkg/auth"

	"github.com/gofiber/fiber/v2"
)

func NewHTTPApp(appName string, jwtManager *auth.JWTManager, requestLogger fiber.Handler, handlers httpdelivery.Handlers, routeMiddleware httpdelivery.RouteMiddleware, instrumentation httpdelivery.Instrumentation, docsEnabled bool) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      appName,
		ErrorHandler: ErrorHandler,
	})

	httpdelivery.RegisterRoutes(app, handlers, jwtManager, requestLogger, routeMiddleware, instrumentation, docsEnabled)
	return app
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	if fiberErr, ok := err.(*fiber.Error); ok {
		return c.Status(fiberErr.Code).JSON(apiresponse.Error(fiberErr.Message, "HTTP_ERROR", nil))
	}

	appErr := apperror.As(err)
	return c.Status(appErr.Status).JSON(apiresponse.Error(appErr.Message, appErr.Code, appErr.Details))
}
