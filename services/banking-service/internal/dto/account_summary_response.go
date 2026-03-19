package dto

import "banking-service/internal/model"

type RequestLimitsChangeResponse struct {
	Code string `json:"code"`
}

type AccountSummaryResponse struct {
	AccountNumber    string  `json:"account_number"`
	Name             string  `json:"name"`
	AccountType      string  `json:"account_type"`
	AccountKind      string  `json:"account_kind"`
	CurrencyCode     string  `json:"currency"`
	Balance          float64 `json:"balance"`
	AvailableBalance float64 `json:"available_balance"`
	ReservedFunds    float64 `json:"reserved_funds"`
}

func ToAccountSummaryResponse(a *model.Account) AccountSummaryResponse {
	return AccountSummaryResponse{
		AccountNumber:    a.AccountNumber,
		Name:             a.Name,
		AccountType:      string(a.AccountType),
		AccountKind:      string(a.AccountKind),
		CurrencyCode:     string(a.Currency.Code),
		Balance:          a.Balance,
		AvailableBalance: a.AvailableBalance,
		ReservedFunds:    a.Balance - a.AvailableBalance,
	}
}
