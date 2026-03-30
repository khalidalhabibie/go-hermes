package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

const ContextTraceID = "trace_id"

func TraceContext() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if traceID := parseTraceParent(c.Get("traceparent")); traceID != "" {
			c.Locals(ContextTraceID, traceID)
		}
		return c.Next()
	}
}

func parseTraceParent(header string) string {
	parts := strings.Split(header, "-")
	if len(parts) != 4 {
		return ""
	}

	traceID := parts[1]
	if len(traceID) != 32 {
		return ""
	}
	if strings.Trim(traceID, "0") == "" {
		return ""
	}
	return traceID
}
