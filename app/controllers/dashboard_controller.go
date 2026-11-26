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

// Index menyiapkan data ringkasan penimbangan dan grafik RPM untuk halaman dashboard.
func (ctl *DashboardController) Index(c fiber.Ctx) error {
	var latestWeighing models.Weighing
	config.DB.Preload("Item").Order("measured_at desc").First(&latestWeighing)

	todayStr := time.Now().Format("2006-01-02")
	var todayStats models.DailyStats
	config.DB.Where("stat_date = ?", todayStr).First(&todayStats)

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

	esp32Online := (len(recentConveyorLogs) > 0 && time.Since(recentConveyorLogs[0].RecordedAt) <= 15*time.Second) ||
		(len(recentWeighings) > 0 && time.Since(recentWeighings[0].MeasuredAt) <= 15*time.Second)
	allOnline := esp32Online

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
		"AllOnline":       allOnline,
		"Esp32Online":     esp32Online,
		"ActiveProduct":   activeProduct,
		"WeightChartJSON": string(weightChartJSON),
		"RPMChartJSON":    string(rpmChartJSON),
		"BrokerConnected": mqttClient != nil && mqttClient.IsConnected(),
	})
}

