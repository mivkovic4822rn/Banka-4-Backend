package repository

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"context"
	"errors"
	"time"

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

func (r *loanRepository) FindAll(ctx context.Context, query *dto.ListLoanRequestsQuery) ([]model.LoanRequest, int64, error) {
	var loans []model.LoanRequest
	var count int64
	db := r.db.WithContext(ctx).Model(&model.LoanRequest{})

	if query.ClientID != 0 {
		db = db.Where("client_id = ?", query.ClientID)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	err := db.Preload("LoanType").
		Limit(query.PageSize).Offset(offset).Find(&loans).Error

	return loans, count, err
}

func (r *loanRepository) FindByID(ctx context.Context, id uint) (*model.LoanRequest, error) {
	var loan model.LoanRequest
	result := r.db.WithContext(ctx).Preload("LoanType").First(&loan, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &loan, nil
}

func (r *loanRepository) Update(ctx context.Context, request *model.LoanRequest) error {
	return r.db.WithContext(ctx).Save(request).Error
}

func (r *loanRepository) CreateLoan(ctx context.Context, loan *model.Loan) error {
	return r.db.WithContext(ctx).Create(loan).Error
}

func (r *loanRepository) FindLoanByRequestID(ctx context.Context, requestID uint) (*model.Loan, error) {
	var loan model.Loan
	err := r.db.WithContext(ctx).
		Preload("Installments").
		Where("loan_request_id = ?", requestID).
		First(&loan).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &loan, err
}

func (r *loanRepository) UpdateLoan(ctx context.Context, loan *model.Loan) error {
	return r.db.WithContext(ctx).Save(loan).Error
}

func (r *loanRepository) CreateInstallments(ctx context.Context, installments []model.LoanInstallment) error {
	return r.db.WithContext(ctx).Create(&installments).Error
}

func (r *loanRepository) FindDueInstallments(ctx context.Context, date time.Time) ([]model.LoanInstallment, error) {
	var installments []model.LoanInstallment
	err := r.db.WithContext(ctx).
		Preload("Loan.LoanRequest").
		Where("status = ? AND due_date <= ?", model.InstallmentStatusPending, date).
		Find(&installments).Error
	return installments, err
}

func (r *loanRepository) FindRetryInstallments(ctx context.Context, now time.Time) ([]model.LoanInstallment, error) {
	var installments []model.LoanInstallment
	err := r.db.WithContext(ctx).
		Preload("Loan.LoanRequest").
		Where("status = ? AND retry_at <= ?", model.InstallmentStatusRetrying, now).
		Find(&installments).Error
	return installments, err
}

func (r *loanRepository) UpdateInstallment(ctx context.Context, installment *model.LoanInstallment) error {
	return r.db.WithContext(ctx).Save(installment).Error
}

func (r *loanRepository) FindActiveVariableRateLoans(ctx context.Context) ([]model.Loan, error) {
	var loans []model.Loan
	err := r.db.WithContext(ctx).
		Preload("Installments").
		Where("status = ? AND is_variable_rate = ?", model.LoanStatusActive, true).
		Find(&loans).Error
	return loans, err
}
