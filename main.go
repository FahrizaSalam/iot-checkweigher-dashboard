package main

import (
	"log"
	"os"

	"checkweigher/app/services"
	"checkweigher/bootstrap"
	"checkweigher/config"
	"checkweigher/database/migrations"
	"checkweigher/database/seeders"
	"checkweigher/routes"

	"github.com/gofiber/fiber/v3"
)

// main menginisialisasi konfigurasi, database, koneksi MQTT, dan server web Fiber.
func main() {
	config.LoadEnv()
	config.ConnectDatabase()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			migrations.Migrate(config.DB)
		case "migrate:fresh":
			migrations.Fresh(config.DB)
		case "seed":
			seeders.Run(config.DB)
		default:
			log.Fatalf("Perintah tidak dikenal: %s (gunakan: migrate | migrate:fresh | seed)", os.Args[1])
		}
		return
	}

	go services.Hub.Run()

	services.InitMQTT(nil)
	mqttClient, _ := config.ConnectMQTT(func() {
		services.MQTT.SetClient(config.GetMQTTClient())
		services.MQTT.Subscribe()
	})
	services.MQTT.SetClient(mqttClient)

	app := bootstrap.NewApp()
	routes.Web(app)

	app.Use(func(c fiber.Ctx) error { return fiber.ErrNotFound })

	port := config.Env("APP_PORT", "3000")
	log.Println("[SERVER] Checkweigher Fullstack MVC berjalan di http://localhost:" + port)
	log.Fatal(app.Listen(":" + port))
}

