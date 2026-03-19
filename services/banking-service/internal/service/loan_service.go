package service

import (
	"context"
	"math"

	"banking-service/internal/dto"
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"common/pkg/errors"
)

type LoanService struct {
	accountRepo  repository.AccountRepository
	loanTypeRepo repository.LoanTypeRepository
	loanRepo     repository.LoanRepository
}

func NewLoanService(
	accountRepo repository.AccountRepository,
	loanTypeRepo repository.LoanTypeRepository,
	loanRepo repository.LoanRepository,
) *LoanService {
	return &LoanService{
		accountRepo:  accountRepo,
		loanTypeRepo: loanTypeRepo,
		loanRepo:     loanRepo,
	}
}

func (s *LoanService) CalculateMonthlyInstallment(amount float64, annualRatePercent float64, months int) float64 {
	if annualRatePercent <= 0 {
		if months == 0 {
			return 0
		}
		return amount / float64(months)
	}

	monthlyRate := (annualRatePercent / 100.0) / 12.0
	compoundFactor := math.Pow(1.0+monthlyRate, float64(months))
	installment := amount * (monthlyRate * compoundFactor) / (compoundFactor - 1.0)

	return math.Round(installment*100) / 100
}

func (s *LoanService) SubmitLoanRequest(ctx context.Context, req *dto.CreateLoanRequest, clientID uint) (*dto.CreateLoanResponse, error) {
	account, err := s.accountRepo.FindByAccountNumber(ctx, req.AccountNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if account == nil {
		return nil, errors.BadRequestErr("account not found")
	}

	loanType, err := s.loanTypeRepo.FindByID(ctx, req.LoanTypeID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if loanType == nil {
		return nil, errors.BadRequestErr("credit type not found")
	}

	if req.RepaymentPeriod < loanType.MinRepaymentPeriod || req.RepaymentPeriod > loanType.MaxRepaymentPeriod {
		return nil, errors.BadRequestErr("repayment perion is not valid for loan type")
	}

	// RAČUNANJE KAMATE I RATE
	totalInterestRate := loanType.BaseInterestRate + loanType.BankMargin
	monthlyInstallment := s.CalculateMonthlyInstallment(req.Amount, totalInterestRate, req.RepaymentPeriod)

	newRequest := &model.LoanRequest{
		ClientID:           clientID,
		AccountNumber:      req.AccountNumber,
		LoanTypeID:         req.LoanTypeID,
		Amount:             req.Amount,
		RepaymentPeriod:    req.RepaymentPeriod,
		CalculatedRate:     totalInterestRate,
		MonthlyInstallment: monthlyInstallment,
		Status:             model.LoanRequestPending, // Kreira se sa statusom PENDING, kako piše u tasku
	}

	if err := s.loanRepo.CreateRequest(ctx, newRequest); err != nil {
		return nil, errors.InternalErr(err)
	}

	return &dto.CreateLoanResponse{
		RequestID:          newRequest.ID,
		Status:             newRequest.Status,
		MonthlyInstallment: monthlyInstallment,
	}, nil
}

func (s *LoanService) GetClientLoans(ctx context.Context, clientID uint, sortByAmountDesc bool) ([]dto.LoanResponse, error) {
	loans, err := s.loanRepo.FindByClientID(ctx, clientID, sortByAmountDesc)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	

	var response []dto.LoanResponse
	for _, l := range loans {
		account, err := s.accountRepo.FindByAccountNumber(ctx, l.AccountNumber)
		if err != nil {
			return nil, errors.InternalErr(err)
		}

		response = append(response, dto.LoanResponse{
			ID:                 l.ID,
			LoanType:           l.LoanType.Name,
			Amount:             l.Amount,
			Currency:           account.Currency.Code,
			MonthlyInstallment: l.MonthlyInstallment,
			Status:             l.Status,
		})
	}
	return response, nil
}

func (s *LoanService) GetLoanDetails(ctx context.Context, clientID uint, loanID uint) (*dto.LoanDetailsResponse, error) {
	loan, err := s.loanRepo.FindByIDAndClientID(ctx, loanID, clientID)
	if err != nil {
		return nil, errors.NotFoundErr("loan not found")
	}

	// Generišemo plan otplate (Installments)
	var installments []dto.Installment
	for i := 1; i <= loan.RepaymentPeriod; i++ {
		installments = append(installments, dto.Installment{
			Number: i,
			Amount: loan.MonthlyInstallment,
			Status: "UPCOMING", // Svi su upcoming dok se ne napravi payment sistem
		})
	}

	account, err := s.accountRepo.FindByAccountNumber(ctx, loan.AccountNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return &dto.LoanDetailsResponse{
		LoanResponse: dto.LoanResponse{
			ID:                 loan.ID,
			LoanType:           loan.LoanType.Name,
			Amount:             loan.Amount,
			Currency:           account.Currency.Code,
			MonthlyInstallment: loan.MonthlyInstallment,
			Status:             loan.Status,
		},
		RepaymentPeriod: loan.RepaymentPeriod,
		InterestRate:    loan.CalculatedRate,
		Installments:    installments,
	}, nil
}
