package models

import "time"

// ConveyorLog merepresentasikan tabel conveyor_logs untuk menyimpan telemetri RPM dan PWM motor.
type ConveyorLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ActualRPM   float64   `gorm:"type:float;default:0" json:"actual_rpm"`
	SetpointRPM float64   `gorm:"type:float;default:0" json:"setpoint_rpm"`
	PWMValue    int       `gorm:"default:0" json:"pwm_value"`
	RecordedAt  time.Time `gorm:"not null;index" json:"recorded_at"`
}

