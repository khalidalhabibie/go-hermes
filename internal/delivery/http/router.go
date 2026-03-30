package http

import (
	"fmt"

	"go-hermes/internal/delivery/http/handler"
	"go-hermes/internal/entity"
	"go-hermes/internal/middleware"
	"go-hermes/internal/pkg/auth"
	"go-hermes/internal/pkg/requestid"

	"github.com/gofiber/fiber/v2"
	fiberrecover "github.com/gofiber/fiber/v2/middleware/recover"
	fiberrequestid "github.com/gofiber/fiber/v2/middleware/requestid"
)

type Handlers struct {
	Auth   *handler.AuthHandler
	User   *handler.UserHandler
	Wallet *handler.WalletHandler
	Tx     *handler.TransactionHandler
	Ledger *handler.LedgerHandler
	Admin  *handler.AdminHandler
	Health *handler.HealthHandler
}

type RouteMiddleware struct {
	Login    fiber.Handler
	TopUp    fiber.Handler
	Transfer fiber.Handler
}

type Instrumentation struct {
	TraceContext    fiber.Handler
	Metrics         fiber.Handler
	MetricsAuth     fiber.Handler
	MetricsEndpoint fiber.Handler
}

func RegisterRoutes(app *fiber.App, handlers Handlers, jwtManager *auth.JWTManager, requestLogger fiber.Handler, routeMiddleware RouteMiddleware, instrumentation Instrumentation, docsEnabled bool) {
	app.Use(fiberrequestid.New(fiberrequestid.Config{
		Header:     requestid.HeaderName,
		ContextKey: "requestid",
	}))
	if instrumentation.TraceContext != nil {
		app.Use(instrumentation.TraceContext)
	}
	app.Use(fiberrecover.New())
	if instrumentation.Metrics != nil {
		app.Use(instrumentation.Metrics)
	}
	app.Use(requestLogger)

	app.Get("/health", handlers.Health.Check)
	if instrumentation.MetricsEndpoint != nil {
		if instrumentation.MetricsAuth != nil {
			app.Get("/metrics", instrumentation.MetricsAuth, instrumentation.MetricsEndpoint)
		} else {
			app.Get("/metrics", instrumentation.MetricsEndpoint)
		}
	}

	if docsEnabled {
		app.Get("/swagger", func(c *fiber.Ctx) error {
			return c.Type("html").SendString(`<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>go-hermes Swagger</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: '/swagger/openapi.yaml', dom_id: '#swagger-ui' });
  </script>
</body>
</html>`)
		})
		app.Get("/swagger/openapi.yaml", func(c *fiber.Ctx) error {
			return c.SendFile("./docs/openapi.yaml")
		})
	}

	v1 := app.Group("/api/v1")
	v1.Post("/auth/register", handlers.Auth.Register)
	if routeMiddleware.Login != nil {
		v1.Post("/auth/login", routeMiddleware.Login, handlers.Auth.Login)
	} else {
		v1.Post("/auth/login", handlers.Auth.Login)
	}

	protected := v1.Group("", middleware.Auth(jwtManager))
	protected.Get("/users/me", handlers.User.Me)
	protected.Get("/wallets/me", handlers.Wallet.Me)
	protected.Get("/wallets/me/balance", handlers.Wallet.Balance)
	if routeMiddleware.TopUp != nil {
		protected.Post("/wallets/me/top-up", routeMiddleware.TopUp, handlers.Wallet.TopUp)
	} else {
		protected.Post("/wallets/me/top-up", handlers.Wallet.TopUp)
	}
	if routeMiddleware.Transfer != nil {
		protected.Post("/transfers", routeMiddleware.Transfer, handlers.Tx.Transfer)
	} else {
		protected.Post("/transfers", handlers.Tx.Transfer)
	}
	protected.Get("/transactions/me", handlers.Tx.ListMine)
	protected.Get("/transactions/:id", handlers.Tx.GetOne)
	protected.Get("/ledgers/me", handlers.Ledger.ListMine)
	protected.Get("/ledgers/transactions/:transactionId", handlers.Ledger.ListByTransaction)

	admin := protected.Group("/admin", middleware.RequireRole(entity.RoleAdmin))
	admin.Get("/audit-logs", handlers.Admin.AuditLogs)
	admin.Get("/reconciliation", handlers.Admin.Reconciliation)
	admin.Get("/transactions", handlers.Admin.Transactions)
	admin.Get("/webhooks", handlers.Admin.Webhooks)
	admin.Get("/webhooks/:id", handlers.Admin.Webhook)

	app.Use(func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("route %s not found", c.Path()))
	})
}
