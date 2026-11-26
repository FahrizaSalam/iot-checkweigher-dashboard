package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// MethodOverride mengubah metode HTTP POST menjadi PUT, PATCH, atau DELETE berdasarkan field _method.
func MethodOverride() fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Method() == fiber.MethodPost {
			method := strings.ToUpper(c.FormValue("_method"))
			switch method {
			case fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete:
				c.Method(method)
				return c.RestartRouting()
			}
		}
		return c.Next()
	}
}

