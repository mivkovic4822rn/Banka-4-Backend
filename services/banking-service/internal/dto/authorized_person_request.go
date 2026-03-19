package dto

import "time"

type AuthorizedPersonRequest struct {
	FirstName   string    `json:"first_name" binding:"required,max=20"`
	LastName    string    `json:"last_name" binding:"required,max=100"`
	DateOfBirth time.Time `json:"date_of_birth"`
	Gender      string    `json:"gender"`
	Email       string    `json:"email" binding:"required,email"`
	PhoneNumber string    `json:"phone_number"`
	Address     string    `json:"address"`
}
