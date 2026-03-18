package dto

import (
	"banking-service/internal/model"
	"time"
)

type CreateAccountRequest struct {
	Name           string             `json:"name"            binding:"required"`
	ClientID       uint               `json:"client_id"       binding:"required"`
	EmployeeID     uint               `json:"employee_id"     binding:"required"`
	CompanyID      *uint              `json:"company_id"`
	AccountType    model.AccountType  `json:"account_type"    binding:"required,account_type"`
	AccountKind    model.AccountKind  `json:"account_kind"    binding:"required,account_kind"`
	Subtype        model.Subtype      `json:"subtype"`                                            // required if AccountKind=Current
	CurrencyCode   model.CurrencyCode `json:"currency_code"    binding:"omitempty,currency_code"` // required if AccountKind=Foreign
	InitialBalance float64            `json:"initial_balance"  binding:"min=0"`
	ExpiresAt      time.Time          `json:"expires_at"       binding:"required"`
	CreateCard     bool               `json:"create_card"`
	GenerateCard   bool               `json:"generate_card"`
}
