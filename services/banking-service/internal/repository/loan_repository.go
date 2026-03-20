package repository

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"context"
	"time"
)

type LoanRepository interface {
	CreateRequest(ctx context.Context, request *model.LoanRequest) error
	FindByClientID(ctx context.Context, clientID uint, sortByAmountDesc bool) ([]model.LoanRequest, error)
	FindByIDAndClientID(ctx context.Context, id uint, clientID uint) (*model.LoanRequest, error)
	FindAll(ctx context.Context, query *dto.ListLoanRequestsQuery) ([]model.LoanRequest, int64, error)
	FindByID(ctx context.Context, id uint) (*model.LoanRequest, error)
	Update(ctx context.Context, request *model.LoanRequest) error

	// Metode za aktivne kredite
	CreateLoan(ctx context.Context, loan *model.Loan) error
	FindLoanByRequestID(ctx context.Context, requestID uint) (*model.Loan, error)
	UpdateLoan(ctx context.Context, loan *model.Loan) error

	// Metode za rate
	CreateInstallments(ctx context.Context, installments []model.LoanInstallment) error
	FindDueInstallments(ctx context.Context, date time.Time) ([]model.LoanInstallment, error)
	FindRetryInstallments(ctx context.Context, now time.Time) ([]model.LoanInstallment, error)
	UpdateInstallment(ctx context.Context, installment *model.LoanInstallment) error

	// Vraca sve aktivne kredite sa varijabilnom kamatnom stopom
	FindActiveVariableRateLoans(ctx context.Context) ([]model.Loan, error)
}
