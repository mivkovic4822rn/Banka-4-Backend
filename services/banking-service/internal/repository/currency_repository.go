package repository

import (
	"banking-service/internal/model"
	"context"
)

type CurrencyRepository interface {
	FindByCode(ctx context.Context, code model.CurrencyCode) (*model.Currency, error)
}
