package response

import (
	"go-hermes/internal/pkg/apiresponse"
	"go-hermes/internal/pkg/apperror"

	"github.com/gofiber/fiber/v2"
)

func Success(c *fiber.Ctx, status int, data interface{}, meta interface{}) error {
	return c.Status(status).JSON(apiresponse.Success(data, meta))
}

func Error(c *fiber.Ctx, err error) error {
	appErr := apperror.As(err)
	return c.Status(appErr.Status).JSON(apiresponse.Error(appErr.Message, appErr.Code, appErr.Details))
}

func RawJSON(c *fiber.Ctx, status int, body []byte) error {
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	return c.Status(status).Send(body)
}
