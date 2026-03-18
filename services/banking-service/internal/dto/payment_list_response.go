package dto

import "time"

type PaymentFilters struct {
	Status    string    `form:"status"`
	StartDate time.Time `form:"start_date" time_format:"2006-01-02"`
	EndDate   time.Time `form:"end_date"   time_format:"2006-01-02"`
	MinAmount float64   `form:"min_amount"`
	MaxAmount float64   `form:"max_amount"`
	Page      int       `form:"page"      binding:"min=1"`
	PageSize  int       `form:"page_size" binding:"min=1,max=100"`
}

type PaymentSummaryResponse struct {
	ID               uint      `json:"id"`
	RecipientName    string    `json:"recipient_name"`
	RecipientAccount string    `json:"recipient_account"`
	PayerAccount     string    `json:"payer_account"`
	Amount           float64   `json:"amount"`
	Currency         string    `json:"currency"`
	Status           string    `json:"status"`
	Purpose          string    `json:"purpose"`
	PaymentCode      string    `json:"payment_code"`
	CreatedAt        time.Time `json:"created_at"`
}

type ListPaymentsResponse struct {
	Data       []PaymentSummaryResponse `json:"data"`
	Total      int64                    `json:"total"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
	TotalPages int                      `json:"total_pages"`
}
