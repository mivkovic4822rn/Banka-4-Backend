package dto

type RequestCardRequest struct {
	AccountNumber    string                   `json:"account_number" binding:"required"`
	AuthorizedPerson *AuthorizedPersonRequest `json:"authorized_person,omitempty"`
}
