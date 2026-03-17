package dto

type CreatePaymentResponse struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
}

type VerifyPaymentResponse struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
}
