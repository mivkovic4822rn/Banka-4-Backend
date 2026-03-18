package model

import "time"

type VerificationToken struct {
	ID              uint      `gorm:"primaryKey"`
	ClientID        uint      `gorm:"not null;index"`
	AccountNumber   string    `gorm:"not null"`
	Code            string    `gorm:"not null"`
	NewDailyLimit   float64   `gorm:"not null"`
	NewMonthlyLimit float64   `gorm:"not null"`
	ExpiresAt       time.Time `gorm:"not null"`
}
