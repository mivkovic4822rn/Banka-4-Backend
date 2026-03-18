package dto

type ConfirmCardRequest struct {
	AccountNumber     string `json:"account_number" binding:"required"`
	ConfirmationCode  string `json:"confirmation_code" binding:"required,len=6"`
}
