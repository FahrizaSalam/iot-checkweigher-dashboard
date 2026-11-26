package models

import "time"

// DeviceStatus merepresentasikan tabel device_status untuk memantau kondisi sensor dan aktuator.
type DeviceStatus struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	DeviceName string     `gorm:"size:100;not null;uniqueIndex" json:"device_name"`
	DeviceType string     `gorm:"size:50;not null" json:"device_type"`
	Status     string     `gorm:"size:20;default:'OFFLINE'" json:"status"`
	LastValue  string     `gorm:"column:last_value;size:255" json:"last_value"`
	LastSeen   *time.Time `json:"last_seen"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// TableName menentukan nama tabel database untuk model DeviceStatus.
func (DeviceStatus) TableName() string {
	return "device_status"
}

