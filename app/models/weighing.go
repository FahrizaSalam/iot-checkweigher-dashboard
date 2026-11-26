package models

import "time"

// Weighing merepresentasikan tabel weighings untuk menyimpan catatan hasil penimbangan.
type Weighing struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	ItemID        *uint     `gorm:"index" json:"item_id"`
	Item          *Item     `gorm:"foreignKey:ItemID" json:"item,omitempty"`
	RfidUID       string    `gorm:"size:50;index" json:"rfid_uid"`
	Weight        float64   `gorm:"type:float;not null" json:"weight"`
	Category      string    `gorm:"size:20;not null;index" json:"category"`
	ConveyorSpeed float64   `gorm:"type:float;default:0" json:"conveyor_speed"`
	MeasuredAt    time.Time `gorm:"not null;index" json:"measured_at"`
	CreatedAt     time.Time `json:"created_at"`
}

