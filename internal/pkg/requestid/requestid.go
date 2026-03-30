package requestid

import "github.com/gofiber/fiber/v2"

const HeaderName = "X-Request-ID"

func Get(c *fiber.Ctx) string {
	if requestID := c.GetRespHeader(HeaderName); requestID != "" {
		return requestID
	}
	return c.Get(HeaderName)
}
