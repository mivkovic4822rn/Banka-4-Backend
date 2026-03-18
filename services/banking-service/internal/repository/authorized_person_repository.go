package repository

import (
	"banking-service/internal/model"
	"context"
)

type AuthorizedPersonRepository interface {
	Create(ctx context.Context, person *model.AuthorizedPerson) error
	FindByID(ctx context.Context, id uint) (*model.AuthorizedPerson, error)
	ListByAccountNumber(ctx context.Context, accountNumber string) ([]model.AuthorizedPerson, error)
}
