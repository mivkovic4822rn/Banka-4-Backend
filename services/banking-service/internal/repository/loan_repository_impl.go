package repository

import (
	"banking-service/internal/model"
	"context"

	"gorm.io/gorm"
)

type loanRepository struct {
	db *gorm.DB
}

func NewLoanRepository(db *gorm.DB) LoanRepository {
	return &loanRepository{db: db}
}

func (r *loanRepository) FindByClientID(ctx context.Context, clientID uint, sortByAmountDesc bool) ([]model.LoanRequest, error) {
	var loans []model.LoanRequest

	query := r.db.WithContext(ctx).Where("client_id = ?", clientID).Preload("LoanType")

	if sortByAmountDesc {
		query = query.Order("amount DESC")
	} else {
		query = query.Order("amount ASC")
	}

	if err := query.Find(&loans).Error; err != nil {
		return nil, err
	}
	return loans, nil
}

func (r *loanRepository) FindByIDAndClientID(ctx context.Context, id uint, clientID uint) (*model.LoanRequest, error) {
	var loan model.LoanRequest
	if err := r.db.WithContext(ctx).Where("id = ? AND client_id = ?", id, clientID).Preload("LoanType").First(&loan).Error; err != nil {
		return nil, err
	}
	return &loan, nil
}

func (r *loanRepository) CreateRequest(ctx context.Context, request *model.LoanRequest) error {
	return r.db.WithContext(ctx).Create(request).Error
}
