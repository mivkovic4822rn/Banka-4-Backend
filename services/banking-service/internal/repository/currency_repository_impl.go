package repository

import (
	"banking-service/internal/model"
	"common/pkg/errors"
	"context"

	"gorm.io/gorm"
)

type currencyRepository struct {
	db *gorm.DB
}

func NewCurrencyRepository(db *gorm.DB) CurrencyRepository {
	return &currencyRepository{db: db}
}

func (r *currencyRepository) FindByCode(ctx context.Context, code model.CurrencyCode) (*model.Currency, error) {
	var currency model.Currency
	result := r.db.WithContext(ctx).Where("code = ?", code).First(&currency)
	if result.Error != nil {
		return nil, errors.NotFoundErr("currency not found: " + string(code))
	}
	return &currency, nil
}
