package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"checkweigher/app/models"
	"checkweigher/config"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MQTTService struct {
	mu     sync.RWMutex
	client mqtt.Client
}

var MQTT *MQTTService

// InitMQTT menginisialisasi instance layanan MQTT dan memulai langganan topik jika sudah terhubung.
func InitMQTT(client mqtt.Client) *MQTTService {
	if MQTT == nil {
		MQTT = &MQTTService{client: client}
	} else {
		MQTT.SetClient(client)
	}
	if client != nil && client.IsConnected() {
		MQTT.Subscribe()
	}
	return MQTT
}

// SetClient memperbarui referensi client MQTT secara thread-safe.
func (s *MQTTService) SetClient(client mqtt.Client) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.client = client
	s.mu.Unlock()
}

// getClient membaca referensi client MQTT secara thread-safe.
func (s *MQTTService) getClient() mqtt.Client {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

// Subscribe mendaftarkan callback untuk topik penimbangan, telemetri RPM, dan status perangkat.
func (s *MQTTService) Subscribe() {
	client := s.getClient()
	if client == nil || !client.IsConnected() {
		return
	}

	client.Subscribe("checkweigher/weighing", 1, func(c mqtt.Client, m mqtt.Message) {
		s.handleWeighing(m.Payload())
	})

	client.Subscribe("checkweigher/conveyor/rpm", 0, func(c mqtt.Client, m mqtt.Message) {
		s.handleConveyorRPM(m.Payload())
	})

	client.Subscribe("checkweigher/device/status", 0, func(c mqtt.Client, m mqtt.Message) {
		s.handleDeviceStatus(m.Payload())
	})

	log.Println("[MQTT] Berlangganan topik penimbangan, telemetri RPM, dan status perangkat")
}

// handleWeighing memproses pesan hasil penimbangan, menyimpan ke database, dan menyiarkan ke dashboard.
func (s *MQTTService) handleWeighing(payload []byte) {
	var data struct {
		RfidUID       string  `json:"rfid_uid"`
		Weight        float64 `json:"weight"`
		ConveyorSpeed float64 `json:"conveyor_speed"`
		Timestamp     string  `json:"timestamp"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("[MQTT] Gagal parse JSON penimbangan: %v", err)
		return
	}

	var product models.Product
	if err := config.DB.Where("is_active = ?", true).First(&product).Error; err != nil {
		product = models.Product{ToleranceMin: 300.0, ToleranceMax: 1000.0}
	}

	category := "BERAT"
	if data.Weight < product.ToleranceMin {
		category = "RINGAN"
	} else if data.Weight > product.ToleranceMax {
		category = "ERROR"
	}

	measuredAt := time.Now()
	if data.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, data.Timestamp); err == nil {
			measuredAt = t
		}
	}

	var item models.Item
	var itemID *uint
	if data.RfidUID != "" {
		if err := config.DB.Where("rfid_uid = ?", data.RfidUID).First(&item).Error; err == nil {
			itemID = &item.ID
		}
	}

	weighing := models.Weighing{
		ItemID:        itemID,
		RfidUID:       data.RfidUID,
		Weight:        data.Weight,
		Category:      category,
		ConveyorSpeed: data.ConveyorSpeed,
		MeasuredAt:    measuredAt,
	}

	if err := config.DB.Create(&weighing).Error; err != nil {
		log.Printf("[MQTT] Gagal simpan penimbangan: %v", err)
		return
	}

	todayStr := measuredAt.Format("2006-01-02")
	ringanInc, beratInc, errorInc := 0, 0, 0
	switch category {
	case "RINGAN":
		ringanInc = 1
	case "BERAT":
		beratInc = 1
	case "ERROR":
		errorInc = 1
	}

	config.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "stat_date"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"total_weighed": gorm.Expr("total_weighed + 1"),
			"total_ringan":  gorm.Expr("total_ringan + ?", ringanInc),
			"total_berat":   gorm.Expr("total_berat + ?", beratInc),
			"total_error":   gorm.Expr("total_error + ?", errorInc),
			"avg_weight":    gorm.Expr("(avg_weight * total_weighed + ?) / (total_weighed + 1)", data.Weight),
			"min_weight":    gorm.Expr("LEAST(min_weight, ?)", data.Weight),
			"max_weight":    gorm.Expr("GREATEST(max_weight, ?)", data.Weight),
		}),
	}).Create(&models.DailyStats{
		StatDate:     todayStr,
		TotalWeighed: 1,
		TotalRingan:  ringanInc,
		TotalBerat:   beratInc,
		TotalError:   errorInc,
		AvgWeight:    data.Weight,
		MinWeight:    data.Weight,
		MaxWeight:    data.Weight,
	})

	if itemID != nil {
		weighing.Item = &item
	}

	Hub.Broadcast(map[string]interface{}{
		"type": "weighing",
		"data": weighing,
	})

	log.Printf("[MQTT] Data penimbangan tersimpan: ID=%d, Berat=%.1fg, Kategori=%s", weighing.ID, weighing.Weight, weighing.Category)
}

// handleConveyorRPM mencatat telemetri RPM conveyor dan menyiarkannya ke dashboard.
func (s *MQTTService) handleConveyorRPM(payload []byte) {
	var data struct {
		ActualRPM float64 `json:"actual_rpm"`
		PWMValue  int     `json:"pwm_value"`
		Timestamp string  `json:"timestamp"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return
	}

	recordedAt := time.Now()
	if data.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, data.Timestamp); err == nil {
			recordedAt = t
		}
	}

	var product models.Product
	setpoint := 60.0
	if err := config.DB.Where("is_active = ?", true).First(&product).Error; err == nil {
		setpoint = product.SetpointRPM
	}

	logEntry := models.ConveyorLog{
		ActualRPM:   data.ActualRPM,
		SetpointRPM: setpoint,
		PWMValue:    data.PWMValue,
		RecordedAt:  recordedAt,
	}

	config.DB.Create(&logEntry)

	Hub.Broadcast(map[string]interface{}{
		"type": "conveyor_rpm",
		"data": logEntry,
	})
}

// handleDeviceStatus memperbarui status dan nilai terakhir sensor di database serta menyiarkannya.
func (s *MQTTService) handleDeviceStatus(payload []byte) {
	var devices []models.DeviceStatus
	if err := json.Unmarshal(payload, &devices); err != nil {
		var single models.DeviceStatus
		if err2 := json.Unmarshal(payload, &single); err2 != nil {
			return
		}
		devices = []models.DeviceStatus{single}
	}

	now := time.Now()
	for _, d := range devices {
		config.DB.Model(&models.DeviceStatus{}).
			Where("device_name = ?", d.DeviceName).
			Updates(map[string]interface{}{
				"status":     d.Status,
				"last_value": d.LastValue,
				"last_seen":  now,
			})
	}

	Hub.Broadcast(map[string]interface{}{
		"type": "device_status",
		"data": devices,
	})
}

// publishJSON mengonversi data ke JSON dan mempublikasikannya ke topik MQTT tertentu.
func (s *MQTTService) publishJSON(topic string, payloadData interface{}) error {
	client := s.getClient()
	if client == nil || !client.IsConnected() {
		return fmt.Errorf("MQTT broker tidak terhubung")
	}

	payload, err := json.Marshal(payloadData)
	if err != nil {
		return err
	}

	token := client.Publish(topic, 1, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("timeout mengirim pesan ke broker MQTT")
	}
	return token.Error()
}

// SendCommand mengirim perintah START/STOP/TARE ke ESP32.
func (s *MQTTService) SendCommand(command string) error {
	return s.publishJSON("checkweigher/command", map[string]string{
		"command":   command,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// SendThreshold mengirim update batas toleransi ke ESP32.
func (s *MQTTService) SendThreshold(min, max float64) error {
	return s.publishJSON("checkweigher/config/threshold", map[string]interface{}{
		"tolerance_min": min,
		"tolerance_max": max,
		"timestamp":     time.Now().Format(time.RFC3339),
	})
}

// SendSetpoint mengirim update setpoint RPM ke ESP32.
func (s *MQTTService) SendSetpoint(rpm float64) error {
	return s.publishJSON("checkweigher/config/setpoint", map[string]interface{}{
		"setpoint_rpm": rpm,
		"timestamp":    time.Now().Format(time.RFC3339),
	})
}

