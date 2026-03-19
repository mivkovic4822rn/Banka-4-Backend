package repository

import (
	"banking-service/internal/model"
	"context"
)

type AccountRepository interface {
	Create(ctx context.Context, account *model.Account) error
	AccountNumberExists(ctx context.Context, accountNumber string) (bool, error)
	FindAllByClientID(ctx context.Context, clientID uint) ([]model.Account, error)
	FindByAccountNumberAndClientID(ctx context.Context, accountNumber string, clientID uint) (*model.Account, error)
	UpdateName(ctx context.Context, accountNumber string, name string) error
	UpdateLimits(ctx context.Context, accountNumber string, daily float64, monthly float64) error
	NameExistsForClient(ctx context.Context, clientID uint, name string, excludeNumber string) (bool, error)
	FindByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error)
	UpdateBalance(ctx context.Context, account *model.Account) error
}
