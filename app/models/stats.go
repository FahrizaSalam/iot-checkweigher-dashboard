package models

import "time"

// DailyStats merepresentasikan tabel daily_stats untuk ringkasan statistik penimbangan harian.
type DailyStats struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	StatDate     string    `gorm:"size:10;not null;uniqueIndex" json:"stat_date"`
	TotalWeighed int       `gorm:"default:0" json:"total_weighed"`
	TotalRingan  int       `gorm:"default:0" json:"total_ringan"`
	TotalBerat   int       `gorm:"default:0" json:"total_berat"`
	TotalError   int       `gorm:"default:0" json:"total_error"`
	AvgWeight    float64   `gorm:"type:float;default:0" json:"avg_weight"`
	MinWeight    float64   `gorm:"type:float;default:0" json:"min_weight"`
	MaxWeight    float64   `gorm:"type:float;default:0" json:"max_weight"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName menentukan nama tabel database untuk model DailyStats.
func (DailyStats) TableName() string {
	return "daily_stats"
}

