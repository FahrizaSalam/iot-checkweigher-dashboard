package helpers

import (
	"checkweigher/config"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/csrf"
)

// View merender template HTML beserta data bersama seperti nama aplikasi, token CSRF, dan pesan flash.
func View(c fiber.Ctx, name string, data fiber.Map) error {
	if data == nil {
		data = fiber.Map{}
	}
	data["AppName"] = config.Env("APP_NAME", "Checkweigher IoT")
	data["CsrfToken"] = csrf.TokenFromContext(c)
	data["CurrentPath"] = c.Path()
	if _, ok := data["Errors"]; !ok {
		data["Errors"] = map[string]string{}
	}
	if msg := c.Redirect().Message("success"); msg.Value != "" {
		data["FlashSuccess"] = msg.Value
	}
	if msg := c.Redirect().Message("error"); msg.Value != "" {
		data["FlashError"] = msg.Value
	}
	return c.Render(name, data)
}

