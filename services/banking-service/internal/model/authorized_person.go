package model

import "time"

type AuthorizedPerson struct {
	AuthorizedPersonID uint    `gorm:"primaryKey"`
	AccountNumber      string  `gorm:"not null;index"`
	FirstName          string  `gorm:"not null;size:20"`
	LastName           string  `gorm:"not null;size:100"`
	DateOfBirth        time.Time
	Gender             string `gorm:"size:10"`
	Email              string `gorm:"not null;size:100"`
	PhoneNumber        string `gorm:"size:20"`
	Address            string `gorm:"size:255"`

	Cards []Card `gorm:"foreignKey:AuthorizedPersonID"`
}
