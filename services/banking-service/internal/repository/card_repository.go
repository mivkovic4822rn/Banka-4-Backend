package repository

import (
	"banking-service/internal/model"
	"context"
)

type CardRepository interface {
	Create(ctx context.Context, card *model.Card) error
	FindByID(ctx context.Context, id uint) (*model.Card, error)
	ListByAccountNumber(ctx context.Context, accountNumber string) ([]model.Card, error)
	CountByAccountNumber(ctx context.Context, accountNumber string) (int64, error)
	CountByAccountNumberAndAuthorizedPersonID(ctx context.Context, accountNumber string, authorizedPersonID *uint) (int64, error)
	CountNonDeactivatedByAccountNumber(ctx context.Context, accountNumber string) (int64, error)
	CountNonDeactivatedByAccountNumberAndAuthorizedPersonID(ctx context.Context, accountNumber string, authorizedPersonID *uint) (int64, error)
	CardNumberExists(ctx context.Context, cardNumber string) (bool, error)
	Update(ctx context.Context, card *model.Card) error
}