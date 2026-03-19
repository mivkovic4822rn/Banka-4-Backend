package dto

type UpdateAccountNameRequest struct {
	Name string `json:"name" binding:"required"`
}

type RequestLimitsChangeRequest struct {
	DailyLimit   float64 `json:"daily_limit"   binding:"required,gt=0"`
	MonthlyLimit float64 `json:"monthly_limit" binding:"required,gt=0"`
}

type ConfirmLimitsChangeRequest struct {
	Code string `json:"code" binding:"required,len=6"`
}
