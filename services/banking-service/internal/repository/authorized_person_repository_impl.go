package repository

import (
	"banking-service/internal/model"
	"context"
	"errors"

	"gorm.io/gorm"
)

type authorizedPersonRepository struct {
	db *gorm.DB
}

func NewAuthorizedPersonRepository(db *gorm.DB) AuthorizedPersonRepository {
	return &authorizedPersonRepository{db: db}
}

func (r *authorizedPersonRepository) Create(ctx context.Context, person *model.AuthorizedPerson) error {
	return r.db.WithContext(ctx).Create(person).Error
}

func (r *authorizedPersonRepository) FindByID(ctx context.Context, id uint) (*model.AuthorizedPerson, error) {
	var person model.AuthorizedPerson

	err := r.db.WithContext(ctx).
		First(&person, "authorized_person_id = ?", id).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &person, nil
}

func (r *authorizedPersonRepository) ListByAccountNumber(ctx context.Context, accountNumber string) ([]model.AuthorizedPerson, error) {
	var people []model.AuthorizedPerson

	err := r.db.WithContext(ctx).
		Where("account_number = ?", accountNumber).
		Order("authorized_person_id ASC").
		Find(&people).
		Error
	if err != nil {
		return nil, err
	}

	return people, nil
}
