package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
)

// ── Fake Loan Repository ─────────────────────────────────────────────

type fakeLoanRepo struct {
	request    *model.LoanRequest
	requests   []model.LoanRequest
	total      int64
	createErr  error
	findErr    error
	findAllErr error
	updateErr  error
	updated    *model.LoanRequest
	loan       *model.Loan
	loans      []model.Loan
}

func (f *fakeLoanRepo) CreateRequest(_ context.Context, r *model.LoanRequest) error {
	if f.createErr != nil {
		return f.createErr
	}
	r.ID = 1
	return nil
}

func (f *fakeLoanRepo) FindByClientID(_ context.Context, _ uint, _ bool) ([]model.LoanRequest, error) {
	return f.requests, f.findErr
}

func (f *fakeLoanRepo) FindByIDAndClientID(_ context.Context, _ uint, _ uint) (*model.LoanRequest, error) {
	return f.request, f.findErr
}

func (f *fakeLoanRepo) FindAll(_ context.Context, _ *dto.ListLoanRequestsQuery) ([]model.LoanRequest, int64, error) {
	return f.requests, f.total, f.findAllErr
}

func (f *fakeLoanRepo) FindByID(_ context.Context, _ uint) (*model.LoanRequest, error) {
	return f.request, f.findErr
}

func (f *fakeLoanRepo) Update(_ context.Context, r *model.LoanRequest) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = r
	return nil
}

func (f *fakeLoanRepo) CreateLoan(_ context.Context, _ *model.Loan) error {
	return f.createErr
}

func (f *fakeLoanRepo) FindLoanByRequestID(_ context.Context, _ uint) (*model.Loan, error) {
	return f.loan, f.findErr
}

func (f *fakeLoanRepo) UpdateLoan(_ context.Context, _ *model.Loan) error {
	return f.updateErr
}

func (f *fakeLoanRepo) CreateInstallments(_ context.Context, _ []model.LoanInstallment) error {
	return f.createErr
}

func (f *fakeLoanRepo) FindDueInstallments(_ context.Context, _ time.Time) ([]model.LoanInstallment, error) {
	return nil, f.findErr
}

func (f *fakeLoanRepo) FindRetryInstallments(_ context.Context, _ time.Time) ([]model.LoanInstallment, error) {
	return nil, f.findErr
}

func (f *fakeLoanRepo) UpdateInstallment(_ context.Context, _ *model.LoanInstallment) error {
	return f.updateErr
}

func (f *fakeLoanRepo) FindActiveVariableRateLoans(_ context.Context) ([]model.Loan, error) {
	return f.loans, f.findErr
}

// ── Fake Loan Type Repository ────────────────────────────────────────

type fakeLoanTypeRepo struct {
	loanType *model.LoanType
	findErr  error
}

func (f *fakeLoanTypeRepo) FindByID(_ context.Context, _ uint) (*model.LoanType, error) {
	return f.loanType, f.findErr
}

// ── Fake Account Repository for Loan Tests ───────────────────────────

type fakeLoanAccountRepo struct {
	account *model.Account
	findErr error
}

func (f *fakeLoanAccountRepo) Create(_ context.Context, _ *model.Account) error { return nil }
func (f *fakeLoanAccountRepo) AccountNumberExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (f *fakeLoanAccountRepo) FindByAccountNumber(_ context.Context, _ string) (*model.Account, error) {
	return f.account, f.findErr
}
func (f *fakeLoanAccountRepo) GetByAccountNumber(_ context.Context, _ string) (*model.Account, error) {
	return f.account, f.findErr
}
func (f *fakeLoanAccountRepo) Update(_ context.Context, _ *model.Account) error { return nil }
func (f *fakeLoanAccountRepo) FindAllByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return nil, nil
}
func (f *fakeLoanAccountRepo) FindByAccountNumberAndClientID(_ context.Context, _ string, _ uint) (*model.Account, error) {
	return nil, nil
}
func (f *fakeLoanAccountRepo) NameExistsForClient(_ context.Context, _ uint, _ string, _ string) (bool, error) {
	return false, nil
}
func (f *fakeLoanAccountRepo) UpdateName(_ context.Context, _ string, _ string) error { return nil }
func (f *fakeLoanAccountRepo) UpdateLimits(_ context.Context, _ string, _ float64, _ float64) error {
	return nil
}
func (f *fakeLoanAccountRepo) UpdateBalance(_ context.Context, _ *model.Account) error { return nil }
func (f *fakeLoanAccountRepo) FindAll(_ context.Context, _ *dto.ListAccountsQuery) ([]*model.Account, int64, error) {
	return nil, 0, nil
}

// ── Helpers ──────────────────────────────────────────────────────────

func newLoanService(
	accountRepo repository.AccountRepository,
	loanTypeRepo repository.LoanTypeRepository,
	loanRepo repository.LoanRepository,
) *LoanService {
	return NewLoanService(accountRepo, loanTypeRepo, loanRepo, nil)
}

func testLoanType() *model.LoanType {
	return &model.LoanType{
		LoanTypeID:         1,
		Name:               "Cash Loan",
		BaseInterestRate:   3.0,
		BankMargin:         2.5,
		MinRepaymentPeriod: 6,
		MaxRepaymentPeriod: 60,
	}
}

func loanTestAccount() *model.Account {
	return &model.Account{
		AccountNumber: "4440001100000001",
		ClientID:      1,
		Currency: model.Currency{
			Code: model.RSD,
		},
	}
}

// ── CalculateMonthlyInstallment Tests ────────────────────────────────

func TestCalculateMonthlyInstallment(t *testing.T) {
	t.Parallel()

	svc := newLoanService(nil, nil, nil)

	tests := []struct {
		name     string
		amount   float64
		rate     float64
		months   int
		expected float64
	}{
		{
			name:     "zero rate divides evenly",
			amount:   12000,
			rate:     0,
			months:   12,
			expected: 1000,
		},
		{
			name:     "zero rate and zero months returns zero",
			amount:   12000,
			rate:     0,
			months:   0,
			expected: 0,
		},
		{
			name:     "standard interest rate calculation",
			amount:   100000,
			rate:     5.5,
			months:   24,
			expected: 4409.57,
		},
		{
			name:     "single month with interest",
			amount:   10000,
			rate:     12,
			months:   1,
			expected: 10100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.CalculateMonthlyInstallment(tt.amount, tt.rate, tt.months)
			require.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

// ── SubmitLoanRequest Tests ──────────────────────────────────────────

func TestSubmitLoanRequest(t *testing.T) {
	t.Parallel()

	lt := testLoanType()

	tests := []struct {
		name         string
		accountRepo  *fakeLoanAccountRepo
		loanTypeRepo *fakeLoanTypeRepo
		loanRepo     *fakeLoanRepo
		req          *dto.CreateLoanRequest
		expectErr    bool
		errMsg       string
	}{
		{
			name:         "success",
			accountRepo:  &fakeLoanAccountRepo{account: loanTestAccount()},
			loanTypeRepo: &fakeLoanTypeRepo{loanType: lt},
			loanRepo:     &fakeLoanRepo{},
			req: &dto.CreateLoanRequest{
				AccountNumber:   "4440001100000001",
				LoanTypeID:      1,
				Amount:          100000,
				RepaymentPeriod: 24,
			},
		},
		{
			name:         "account not found",
			accountRepo:  &fakeLoanAccountRepo{account: nil},
			loanTypeRepo: &fakeLoanTypeRepo{loanType: lt},
			loanRepo:     &fakeLoanRepo{},
			req: &dto.CreateLoanRequest{
				AccountNumber:   "nonexistent",
				LoanTypeID:      1,
				Amount:          100000,
				RepaymentPeriod: 24,
			},
			expectErr: true,
			errMsg:    "account not found",
		},
		{
			name:         "account repo error",
			accountRepo:  &fakeLoanAccountRepo{findErr: fmt.Errorf("db error")},
			loanTypeRepo: &fakeLoanTypeRepo{loanType: lt},
			loanRepo:     &fakeLoanRepo{},
			req: &dto.CreateLoanRequest{
				AccountNumber:   "4440001100000001",
				LoanTypeID:      1,
				Amount:          100000,
				RepaymentPeriod: 24,
			},
			expectErr: true,
		},
		{
			name:         "loan type not found",
			accountRepo:  &fakeLoanAccountRepo{account: loanTestAccount()},
			loanTypeRepo: &fakeLoanTypeRepo{loanType: nil},
			loanRepo:     &fakeLoanRepo{},
			req: &dto.CreateLoanRequest{
				AccountNumber:   "4440001100000001",
				LoanTypeID:      999,
				Amount:          100000,
				RepaymentPeriod: 24,
			},
			expectErr: true,
			errMsg:    "credit type not found",
		},
		{
			name:         "repayment period below minimum",
			accountRepo:  &fakeLoanAccountRepo{account: loanTestAccount()},
			loanTypeRepo: &fakeLoanTypeRepo{loanType: lt},
			loanRepo:     &fakeLoanRepo{},
			req: &dto.CreateLoanRequest{
				AccountNumber:   "4440001100000001",
				LoanTypeID:      1,
				Amount:          100000,
				RepaymentPeriod: 3,
			},
			expectErr: true,
			errMsg:    "repayment perion is not valid",
		},
		{
			name:         "repayment period above maximum",
			accountRepo:  &fakeLoanAccountRepo{account: loanTestAccount()},
			loanTypeRepo: &fakeLoanTypeRepo{loanType: lt},
			loanRepo:     &fakeLoanRepo{},
			req: &dto.CreateLoanRequest{
				AccountNumber:   "4440001100000001",
				LoanTypeID:      1,
				Amount:          100000,
				RepaymentPeriod: 120,
			},
			expectErr: true,
			errMsg:    "repayment perion is not valid",
		},
		{
			name:         "repo create fails",
			accountRepo:  &fakeLoanAccountRepo{account: loanTestAccount()},
			loanTypeRepo: &fakeLoanTypeRepo{loanType: lt},
			loanRepo:     &fakeLoanRepo{createErr: fmt.Errorf("db error")},
			req: &dto.CreateLoanRequest{
				AccountNumber:   "4440001100000001",
				LoanTypeID:      1,
				Amount:          100000,
				RepaymentPeriod: 24,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newLoanService(tt.accountRepo, tt.loanTypeRepo, tt.loanRepo)

			resp, err := svc.SubmitLoanRequest(context.Background(), tt.req, 1)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, model.LoanRequestPending, resp.Status)
			require.Greater(t, resp.MonthlyInstallment, 0.0)
		})
	}
}

// ── ApproveLoanRequest Tests ─────────────────────────────────────────

func TestApproveLoanRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		loanRepo  *fakeLoanRepo
		id        uint
		expectErr bool
		errMsg    string
	}{
		{
			name:      "request not found",
			loanRepo:  &fakeLoanRepo{request: nil},
			id:        99,
			expectErr: true,
			errMsg:    "loan request not found",
		},
		{
			name: "already approved",
			loanRepo: &fakeLoanRepo{
				request: &model.LoanRequest{ID: 1, Status: model.LoanRequestApproved},
			},
			id:        1,
			expectErr: true,
			errMsg:    "loan request is not pending",
		},
		{
			name:      "repo find error",
			loanRepo:  &fakeLoanRepo{findErr: fmt.Errorf("db error")},
			id:        1,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newLoanService(nil, nil, tt.loanRepo)

			err := svc.ApproveLoanRequest(context.Background(), tt.id)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			require.Equal(t, model.LoanRequestApproved, tt.loanRepo.updated.Status)
		})
	}
}

// ── RejectLoanRequest Tests ──────────────────────────────────────────

func TestRejectLoanRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		loanRepo  *fakeLoanRepo
		id        uint
		expectErr bool
		errMsg    string
	}{
		{
			name: "success",
			loanRepo: &fakeLoanRepo{
				request: &model.LoanRequest{ID: 1, Status: model.LoanRequestPending},
			},
			id: 1,
		},
		{
			name:      "request not found",
			loanRepo:  &fakeLoanRepo{request: nil},
			id:        99,
			expectErr: true,
			errMsg:    "loan request not found",
		},
		{
			name: "already rejected",
			loanRepo: &fakeLoanRepo{
				request: &model.LoanRequest{ID: 1, Status: model.LoanRequestRejected},
			},
			id:        1,
			expectErr: true,
			errMsg:    "loan request is not pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newLoanService(nil, nil, tt.loanRepo)

			err := svc.RejectLoanRequest(context.Background(), tt.id)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			require.Equal(t, model.LoanRequestRejected, tt.loanRepo.updated.Status)
		})
	}
}
