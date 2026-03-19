package repository

import (
	"banking-service/internal/model"
	"context"
	"errors"

	"gorm.io/gorm"
)

type VerificationTokenRepository interface {
	Create(ctx context.Context, token *model.VerificationToken) error
	FindByAccountAndClient(ctx context.Context, accountNumber string, clientID uint) (*model.VerificationToken, error)
	DeleteByAccountAndClient(ctx context.Context, accountNumber string, clientID uint) error
}

type verificationTokenRepository struct {
	db *gorm.DB
}

func NewVerificationTokenRepository(db *gorm.DB) VerificationTokenRepository {
	return &verificationTokenRepository{db: db}
}

func (r *verificationTokenRepository) Create(ctx context.Context, token *model.VerificationToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *verificationTokenRepository) FindByAccountAndClient(ctx context.Context, accountNumber string, clientID uint) (*model.VerificationToken, error) {
	var token model.VerificationToken
	err := r.db.WithContext(ctx).
		Where("account_number = ? AND client_id = ?", accountNumber, clientID).
		First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &token, nil
}

func (r *verificationTokenRepository) DeleteByAccountAndClient(ctx context.Context, accountNumber string, clientID uint) error {
	return r.db.WithContext(ctx).
		Where("account_number = ? AND client_id = ?", accountNumber, clientID).
		Delete(&model.VerificationToken{}).Error
}
