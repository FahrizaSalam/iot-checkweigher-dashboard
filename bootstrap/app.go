package bootstrap

import (
	"errors"
	"log"
	"strings"
	"time"

	"checkweigher/app/helpers"
	"checkweigher/app/middleware"
	"checkweigher/config"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/gofiber/template/html/v2"
)

// NewApp menginisialisasi instance Fiber beserta template HTML, middleware, dan file statis.
func NewApp() *fiber.App {
	engine := html.New("./resources/views", ".html")
	engine.Reload(config.Env("APP_ENV", "local") == "local")
	engine.AddFunc("tanggal", helpers.Tanggal)
	engine.AddFunc("formatWeight", helpers.FormatWeight)
	engine.AddFunc("hasPrefix", strings.HasPrefix)
	engine.AddFunc("add", func(a, b int) int { return a + b })

	app := fiber.New(fiber.Config{
		AppName:      config.Env("APP_NAME", "Checkweigher IoT"),
		Views:        engine,
		ViewsLayout:  "layouts/app",
		ErrorHandler: errorHandler,
		TrustProxy:   true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Loopback: true,
			Private:  true,
		},
	})

	app.Use(middleware.MethodOverride())
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(csrf.New(csrf.Config{
		Extractor:      extractors.Chain(extractors.FromHeader("X-CSRF-Token"), extractors.FromForm("_csrf")),
		CookieName:     "csrf_token",
		CookiePath:     "/",
		CookieSameSite: "Lax",
		IdleTimeout:    12 * time.Hour,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			log.Printf("[CSRF] Ditolak %s %s (scheme=%s, host=%s, origin=%s): %v", c.Method(), c.Path(), c.Scheme(), c.Host(), c.Get("Origin"), err)
			if strings.HasPrefix(c.Path(), "/api/") || strings.Contains(c.Get("Accept"), "application/json") {
				return c.Status(419).JSON(fiber.Map{
					"success": false,
					"error":   "Sesi halaman kedaluwarsa (token CSRF tidak valid). Silakan refresh halaman.",
				})
			}
			return c.Status(419).SendString("419 | Halaman kedaluwarsa (token CSRF tidak valid). Silakan muat ulang halaman.")
		},
	}))

	app.Use("/public", static.New("./public"))

	return app
}

// errorHandler menampilkan halaman error sesuai kode status HTTP.
func errorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
	}
	message := "Terjadi kesalahan pada server."
	if code == fiber.StatusNotFound {
		message = "Halaman atau data yang Anda cari tidak ditemukan."
	}
	c.Status(code)
	return helpers.View(c, "errors/error", fiber.Map{
		"Title":   "Error",
		"Code":    code,
		"Message": message,
	})
}
