package model

import "time"

type CardStatus string
type CardType string
type CardBrand string

const (
	CardStatusActive      CardStatus = "Active"
	CardStatusBlocked     CardStatus = "Blocked"
	CardStatusDeactivated CardStatus = "Deactivated"
)

const (
	CardTypeDebit CardType = "Debit"
)

const (
	CardBrandVisa            CardBrand = "Visa"
	CardBrandMasterCard      CardBrand = "MasterCard"
	CardBrandDinaCard        CardBrand = "DinaCard"
)

const (
	MaxPersonalCardsPerAccount = 2
	MaxBusinessCardsPerPerson  = 1
)

const (
	BankCommissionRate          = 0.02
	MastercardConversionFeeRate = 0.005
)

type Card struct {
	CardID             uint       `gorm:"primaryKey"`
	CardNumber         string     `gorm:"uniqueIndex;not null;size:16"`
	CardType           CardType   `gorm:"not null;size:20"`
	CardBrand          CardBrand  `gorm:"not null;size:20"`
	Name               string     `gorm:"not null;size:50"`
	AccountNumber      string     `gorm:"not null;index"`
	CVV                string     `gorm:"not null;size:3"`
	Limit              float64    `gorm:"not null;default:0"`
	Status             CardStatus `gorm:"not null;size:20;default:'Active'"`
	AuthorizedPersonID *uint      `gorm:"index"`
	AuthorizedPerson   *AuthorizedPerson
	CreatedAt          time.Time `gorm:"autoCreateTime"`
	ExpiresAt          time.Time `gorm:"not null"`
}
