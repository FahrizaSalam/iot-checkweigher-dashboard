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

// NewApp menyiapkan aplikasi Fiber: view engine, middleware, static file (mirip bootstrap/app.php).
func NewApp() *fiber.App {
	// 1. View engine: folder resources/views, ekstensi .html
	engine := html.New("./resources/views", ".html")
	engine.Reload(config.Env("APP_ENV", "local") == "local") // auto reload saat development
	engine.AddFunc("tanggal", helpers.Tanggal)
	engine.AddFunc("formatWeight", helpers.FormatWeight)
	engine.AddFunc("hasPrefix", strings.HasPrefix)
	engine.AddFunc("add", func(a, b int) int { return a + b })

	// 2. Instance Fiber (percaya header proxy lokal/private seperti Nginx aaPanel untuk X-Forwarded-Proto/Host)
	app := fiber.New(fiber.Config{
		AppName:      config.Env("APP_NAME", "Checkweigher IoT"),
		Views:        engine,
		ViewsLayout:  "layouts/app", // layout default untuk semua c.Render()
		ErrorHandler: errorHandler,
		TrustProxy:   true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Loopback: true,
			Private:  true,
		},
	})

	// 3. Middleware global
	app.Use(middleware.MethodOverride()) // _method=PUT/DELETE
	app.Use(recover.New())               // cegah server panic
	app.Use(logger.New())                // log setiap request
	app.Use(csrf.New(csrf.Config{        // proteksi CSRF
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

	// 4. File statis: /public
	app.Use("/public", static.New("./public"))

	return app
}

// errorHandler menampilkan halaman error (404, 500, dst).
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
