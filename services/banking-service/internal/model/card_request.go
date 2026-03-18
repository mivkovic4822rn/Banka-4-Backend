package model

import "time"

type CardRequest struct {
	CardRequestID               uint      `gorm:"primaryKey"`
	AccountNumber               string    `gorm:"not null;index"`
	ConfirmationCode            string    `gorm:"not null;size:6;index"`
	ExpiresAt                   time.Time `gorm:"not null"`
	Used                        bool      `gorm:"not null;default:false"`
	ForAuthorizedPerson         bool      `gorm:"not null;default:false"`
	AuthorizedPersonFirstName   *string   `gorm:"size:20"`
	AuthorizedPersonLastName    *string   `gorm:"size:100"`
	AuthorizedPersonDateOfBirth *time.Time
	AuthorizedPersonGender      *string   `gorm:"size:10"`
	AuthorizedPersonEmail       *string   `gorm:"size:100"`
	AuthorizedPersonPhoneNumber *string   `gorm:"size:20"`
	AuthorizedPersonAddress     *string   `gorm:"size:255"`
	CreatedAt                   time.Time `gorm:"autoCreateTime"`
}
