package repository

import (
	"banking-service/internal/model"
	"context"
	"errors"

	"gorm.io/gorm"
)

type cardRepository struct {
	db *gorm.DB
}

func NewCardRepository(db *gorm.DB) CardRepository {
	return &cardRepository{db: db}
}

func (r *cardRepository) Create(ctx context.Context, card *model.Card) error {
	return r.db.WithContext(ctx).Create(card).Error
}

func (r *cardRepository) FindByID(ctx context.Context, id uint) (*model.Card, error) {
	var card model.Card

	err := r.db.WithContext(ctx).
		First(&card, "card_id = ?", id).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &card, nil
}

func (r *cardRepository) ListByAccountNumber(ctx context.Context, accountNumber string) ([]model.Card, error) {
	var cards []model.Card

	err := r.db.WithContext(ctx).
		Where("account_number = ?", accountNumber).
		Order("card_id ASC").
		Find(&cards).
		Error
	if err != nil {
		return nil, err
	}

	return cards, nil
}

func (r *cardRepository) CountByAccountNumber(ctx context.Context, accountNumber string) (int64, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&model.Card{}).
		Where("account_number = ?", accountNumber).
		Count(&count).
		Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *cardRepository) CountByAccountNumberAndAuthorizedPersonID(ctx context.Context, accountNumber string, authorizedPersonID *uint) (int64, error) {
	var count int64

	query := r.db.WithContext(ctx).
		Model(&model.Card{}).
		Where("account_number = ?", accountNumber)

	if authorizedPersonID == nil {
		query = query.Where("authorized_person_id IS NULL")
	} else {
		query = query.Where("authorized_person_id = ?", *authorizedPersonID)
	}

	err := query.Count(&count).Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *cardRepository) CountNonDeactivatedByAccountNumber(ctx context.Context, accountNumber string) (int64, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&model.Card{}).
		Where("account_number = ?", accountNumber).
		Where("status <> ?", model.CardStatusDeactivated).
		Count(&count).
		Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *cardRepository) CountNonDeactivatedByAccountNumberAndAuthorizedPersonID(ctx context.Context, accountNumber string, authorizedPersonID *uint) (int64, error) {
	var count int64

	query := r.db.WithContext(ctx).
		Model(&model.Card{}).
		Where("account_number = ?", accountNumber).
		Where("status <> ?", model.CardStatusDeactivated)

	if authorizedPersonID == nil {
		query = query.Where("authorized_person_id IS NULL")
	} else {
		query = query.Where("authorized_person_id = ?", *authorizedPersonID)
	}

	err := query.Count(&count).Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *cardRepository) CardNumberExists(ctx context.Context, cardNumber string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&model.Card{}).
		Where("card_number = ?", cardNumber).
		Count(&count).
		Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *cardRepository) Update(ctx context.Context, card *model.Card) error {
	return r.db.WithContext(ctx).Save(card).Error
}