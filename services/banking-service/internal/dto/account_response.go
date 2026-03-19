package dto

import (
	"banking-service/internal/model"
	"time"
)

type AccountResponse struct {
	AccountNumber    string             `json:"account_number"`
	Name             string             `json:"name"`
	ClientID         uint               `json:"client_id"`
	CompanyID        *uint              `json:"company_id,omitempty"`
	EmployeeID       uint               `json:"employee_id"`
	Balance          float64            `json:"balance"`
	AvailableBalance float64            `json:"available_balance"`
	CreatedAt        time.Time          `json:"created_at"`
	ExpiresAt        time.Time          `json:"expires_at"`
	CurrencyCode     model.CurrencyCode `json:"currency"`
	Status           string             `json:"status"`
	AccountType      string             `json:"account_type"`
	AccountKind      string             `json:"account_kind"`
	Subtype          string             `json:"subtype,omitempty"`
	MaintenanceFee   float64            `json:"maintenance_fee,omitempty"`
	DailyLimit       float64            `json:"daily_limit"`
	MonthlyLimit     float64            `json:"monthly_limit"`
	DailySpending    float64            `json:"daily_spending"`
	MonthlySpending  float64            `json:"monthly_spending"`
	ReservedFunds    float64            `json:"reserved_funds"`
}

func ToAccountResponse(a *model.Account) AccountResponse {
	return AccountResponse{
		AccountNumber:    a.AccountNumber,
		Name:             a.Name,
		ClientID:         a.ClientID,
		CompanyID:        a.CompanyID,
		EmployeeID:       a.EmployeeID,
		Balance:          a.Balance,
		AvailableBalance: a.AvailableBalance,
		CreatedAt:        a.CreatedAt,
		ExpiresAt:        a.ExpiresAt,
		Status:           a.Status,
		AccountType:      string(a.AccountType),
		AccountKind:      string(a.AccountKind),
		Subtype:          string(a.Subtype),
		MaintenanceFee:   a.MaintenanceFee,
		DailyLimit:       a.DailyLimit,
		MonthlyLimit:     a.MonthlyLimit,
		DailySpending:    a.DailySpending,
		MonthlySpending:  a.MonthlySpending,
		ReservedFunds:    a.Balance - a.AvailableBalance,
	}
}
