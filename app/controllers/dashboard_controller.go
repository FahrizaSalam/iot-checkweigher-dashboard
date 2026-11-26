package controllers

import (
	"encoding/json"
	"time"

	"checkweigher/app/helpers"
	"checkweigher/app/models"
	"checkweigher/config"

	"github.com/gofiber/fiber/v3"
)

type DashboardController struct{}

// Index menyiapkan data ringkasan penimbangan, grafik RPM, dan status perangkat untuk halaman dashboard.
func (ctl *DashboardController) Index(c fiber.Ctx) error {
	var latestWeighing models.Weighing
	config.DB.Preload("Item").Order("measured_at desc").First(&latestWeighing)

	todayStr := time.Now().Format("2006-01-02")
	var todayStats models.DailyStats
	config.DB.Where("stat_date = ?", todayStr).First(&todayStats)

	var devices []models.DeviceStatus
	config.DB.Find(&devices)

	allOnline := len(devices) > 0
	esp32Online := false
	for _, d := range devices {
		if d.Status != "ONLINE" {
			allOnline = false
		}
		if d.DeviceType == "loadcell" && d.Status == "ONLINE" {
			esp32Online = true
		}
	}

	var recentWeighings []models.Weighing
	config.DB.Order("measured_at desc").Limit(30).Find(&recentWeighings)
	if recentWeighings == nil {
		recentWeighings = []models.Weighing{}
	}

	var recentConveyorLogs []models.ConveyorLog
	config.DB.Order("recorded_at desc").Limit(30).Find(&recentConveyorLogs)
	if recentConveyorLogs == nil {
		recentConveyorLogs = []models.ConveyorLog{}
	}

	var activeProduct models.Product
	config.DB.Where("is_active = ?", true).First(&activeProduct)
	if activeProduct.ID == 0 {
		activeProduct = models.Product{
			Name:         "Default Config",
			TargetWeight: 300.0,
			ToleranceMin: 300.0,
			ToleranceMax: 1000.0,
			SetpointRPM:  60.0,
			IsActive:     true,
		}
	}

	weightChartJSON, _ := json.Marshal(recentWeighings)
	rpmChartJSON, _ := json.Marshal(recentConveyorLogs)

	mqttClient := config.GetMQTTClient()

	return helpers.View(c, "dashboard/index", fiber.Map{
		"Title":           "Dashboard",
		"LatestWeighing":  latestWeighing,
		"TodayStats":      todayStats,
		"Devices":         devices,
		"AllOnline":       allOnline,
		"Esp32Online":     esp32Online,
		"ActiveProduct":   activeProduct,
		"WeightChartJSON": string(weightChartJSON),
		"RPMChartJSON":    string(rpmChartJSON),
		"BrokerConnected": mqttClient != nil && mqttClient.IsConnected(),
	})
}

