package repository

import (
	"banking-service/internal/model"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type cardRequestRepository struct {
	db *gorm.DB
}

func NewCardRequestRepository(db *gorm.DB) CardRequestRepository {
	return &cardRequestRepository{db: db}
}

func (r *cardRequestRepository) Create(ctx context.Context, request *model.CardRequest) error {
	return r.db.WithContext(ctx).Create(request).Error
}

func (r *cardRequestRepository) FindByAccountNumberAndCode(ctx context.Context, accountNumber string, code string) (*model.CardRequest, error) {
	var request model.CardRequest

	err := r.db.WithContext(ctx).
		Where("account_number = ?", accountNumber).
		Where("confirmation_code = ?", code).
		First(&request).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func (r *cardRequestRepository) FindLatestPendingByAccountNumber(ctx context.Context, accountNumber string) (*model.CardRequest, error) {
	var request model.CardRequest

	err := r.db.WithContext(ctx).
		Where("account_number = ?", accountNumber).
		Where("used = ?", false).
		Where("expires_at > ?", time.Now()).
		Order("card_request_id DESC").
		First(&request).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func (r *cardRequestRepository) Update(ctx context.Context, request *model.CardRequest) error {
	return r.db.WithContext(ctx).Save(request).Error
}
