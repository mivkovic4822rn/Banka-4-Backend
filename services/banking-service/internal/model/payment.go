package model

import "time"

type PaymentStatus string

const (
	PaymentProcessing PaymentStatus = "processing"
	PaymentCompleted  PaymentStatus = "completed"
	PaymentRejected   PaymentStatus = "rejected"
)

type Payment struct {
	ID               uint `gorm:"primaryKey"`
	RecipientName    string
	RecipientAccount string
	Amount           float64
	ReferenceNumber  string
	PaymentCode      string
	Purpose          string
	PayerAccount     string
	Currency         string
	Status           PaymentStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
