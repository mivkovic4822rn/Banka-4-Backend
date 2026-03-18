package service

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"common/pkg/pb"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ── Fakes ─────────────────────────────────────────────────────────────────────

type fakeAccountRepo struct {
	accNumExists    bool
	accNumExistsErr error
	createErr       error
	accounts        []model.Account
	account         *model.Account
	findErr         error
	nameExists      bool
	nameExistsErr   error
	updateNameErr   error
	updateLimitsErr error
	getByAccNumber      *model.Account
	getByAccNumberErr   error
	updateErr           error
}

func (r *fakeAccountRepo) Create(_ context.Context, _ *model.Account) error {
	return r.createErr
}

func (r *fakeAccountRepo) AccountNumberExists(_ context.Context, _ string) (bool, error) {
	return r.accNumExists, r.accNumExistsErr
}

func (r *fakeAccountRepo) FindAllByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return r.accounts, r.findErr
}

func (r *fakeAccountRepo) FindByAccountNumber(_ context.Context, _ string) (*model.Account, error) {
	return r.account, r.findErr
}

func (r *fakeAccountRepo) FindByAccountNumberAndClientID(_ context.Context, _ string, _ uint) (*model.Account, error) {
	return r.account, r.findErr
}

func (r *fakeAccountRepo) UpdateName(_ context.Context, _ string, _ string) error {
	return r.updateNameErr
}

func (r *fakeAccountRepo) UpdateLimits(_ context.Context, _ string, _ float64, _ float64) error {
	return r.updateLimitsErr
}

func (r *fakeAccountRepo) NameExistsForClient(_ context.Context, _ uint, _ string, _ string) (bool, error) {
	return r.nameExists, r.nameExistsErr
}

type fakeVerificationTokenRepo struct {
	token     *model.VerificationToken
	findErr   error
	createErr error
	deleteErr error
}

func (r *fakeVerificationTokenRepo) Create(_ context.Context, _ *model.VerificationToken) error {
	return r.createErr
}

func (r *fakeVerificationTokenRepo) FindByAccountAndClient(_ context.Context, _ string, _ uint) (*model.VerificationToken, error) {
	return r.token, r.findErr
}

func (r *fakeVerificationTokenRepo) DeleteByAccountAndClient(_ context.Context, _ string, _ uint) error {
	return r.deleteErr
}

func (r *fakeVerificationTokenRepo) MarkUsed(_ context.Context, _ uint) error {
	return nil
}
func (r *fakeAccountRepo) GetByAccountNumber(_ context.Context, _ string) (*model.Account, error) {
	return r.getByAccNumber, r.getByAccNumberErr
}

func (r *fakeAccountRepo) UpdateBalance(_ context.Context, _ *model.Account) error {
	return nil
}

type fakeAccountUserClient struct {
	clientErr   error
	employeeErr error
}

func (f *fakeAccountUserClient) GetClientByID(_ context.Context, _ uint) (*pb.GetClientByIdResponse, error) {
	return nil, f.clientErr
}

func (f *fakeAccountUserClient) GetEmployeeByID(_ context.Context, _ uint) (*pb.GetEmployeeByIdResponse, error) {
	return nil, f.employeeErr
}

// ── Constructor ───────────────────────────────────────────────────────────────

func newAccountService(
	repo *fakeAccountRepo,
	vr *fakeVerificationTokenRepo,
	uc *fakeAccountUserClient,
	db *gorm.DB,
) *AccountService {
	return &AccountService{
		repo:             repo,
		verificationRepo: vr,
		userClient:       uc,
		db:               db,
	}
}

// ── Fixtures ──────────────────────────────────────────────────────────────────

func validPersonalCurrentReq() dto.CreateAccountRequest {
	return dto.CreateAccountRequest{
		Name:           "Standard Personal Account",
		ClientID:       1,
		EmployeeID:     1,
		AccountType:    model.AccountTypePersonal,
		AccountKind:    model.AccountKindCurrent,
		Subtype:        model.SubtypeStandard,
		InitialBalance: 50000,
		ExpiresAt:      time.Now().AddDate(4, 0, 0),
	}
}

func validForeignReq() dto.CreateAccountRequest {
	return dto.CreateAccountRequest{
		Name:           "EUR Foreign Account",
		ClientID:       1,
		EmployeeID:     1,
		AccountType:    model.AccountTypePersonal,
		AccountKind:    model.AccountKindForeign,
		CurrencyCode:   "EUR",
		InitialBalance: 1000,
		ExpiresAt:      time.Now().AddDate(4, 0, 0),
	}
}

func ptrUint(v uint) *uint { return &v }

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestCreateAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		uc        *fakeAccountUserClient
		setupMock func(mock sqlmock.Sqlmock)
		req       dto.CreateAccountRequest
		expectErr bool
		errMsg    string
		check     func(t *testing.T, account *model.Account)
	}{
		{
			name: "success personal current",
			repo: &fakeAccountRepo{},
			uc:   &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "currencies"`).
					WillReturnRows(sqlmock.NewRows([]string{"currency_id", "code"}).AddRow(1, "RSD"))
			},
			req: validPersonalCurrentReq(),
			check: func(t *testing.T, a *model.Account) {
				require.Equal(t, model.AccountTypePersonal, a.AccountType)
				require.Equal(t, model.AccountKindCurrent, a.AccountKind)
				require.Equal(t, model.SubtypeStandard, a.Subtype)
				require.Equal(t, 250000.0, a.DailyLimit)
				require.Equal(t, 1000000.0, a.MonthlyLimit)
				require.NotEmpty(t, a.AccountNumber)
			},
		},
		{
			name: "success personal foreign EUR",
			repo: &fakeAccountRepo{},
			uc:   &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "currencies"`).
					WillReturnRows(sqlmock.NewRows([]string{"currency_id", "code"}).AddRow(2, "EUR"))
			},
			req: validForeignReq(),
			check: func(t *testing.T, a *model.Account) {
				require.Equal(t, model.AccountKindForeign, a.AccountKind)
				require.Equal(t, 5000.0, a.DailyLimit)
				require.Equal(t, 20000.0, a.MonthlyLimit)
			},
		},
		{
			name: "success business current LLC",
			repo: &fakeAccountRepo{},
			uc:   &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "currencies"`).
					WillReturnRows(sqlmock.NewRows([]string{"currency_id", "code"}).AddRow(1, "RSD"))
			},
			req: dto.CreateAccountRequest{
				Name:        "LLC Business Account",
				ClientID:    1,
				EmployeeID:  1,
				CompanyID:   ptrUint(1),
				AccountType: model.AccountTypeBusiness,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeLLC,
				ExpiresAt:   time.Now().AddDate(4, 0, 0),
			},
			check: func(t *testing.T, a *model.Account) {
				require.Equal(t, model.AccountTypeBusiness, a.AccountType)
				require.Equal(t, model.SubtypeLLC, a.Subtype)
			},
		},
		{
			name:      "client not found",
			repo:      &fakeAccountRepo{},
			uc:        &fakeAccountUserClient{clientErr: fmt.Errorf("not found")},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req:       validPersonalCurrentReq(),
			expectErr: true,
			errMsg:    "client not found",
		},
		{
			name:      "employee not found",
			repo:      &fakeAccountRepo{},
			uc:        &fakeAccountUserClient{employeeErr: fmt.Errorf("not found")},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req:       validPersonalCurrentReq(),
			expectErr: true,
			errMsg:    "employee not found",
		},
		{
			name:      "business account without company",
			repo:      &fakeAccountRepo{},
			uc:        &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req: dto.CreateAccountRequest{
				Name:        "Test",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypeBusiness,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeLLC,
				ExpiresAt:   time.Now().AddDate(4, 0, 0),
			},
			expectErr: true,
			errMsg:    "business account requires a company",
		},
		{
			name:      "personal account with company",
			repo:      &fakeAccountRepo{},
			uc:        &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req: dto.CreateAccountRequest{
				Name:        "Test",
				ClientID:    1,
				EmployeeID:  1,
				CompanyID:   ptrUint(1),
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   time.Now().AddDate(4, 0, 0),
			},
			expectErr: true,
			errMsg:    "personal account cannot have a company",
		},
		{
			name:      "foreign account missing currency code",
			repo:      &fakeAccountRepo{},
			uc:        &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req: dto.CreateAccountRequest{
				Name:        "Test",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindForeign,
				ExpiresAt:   time.Now().AddDate(4, 0, 0),
			},
			expectErr: true,
			errMsg:    "currency code is required for foreign accounts",
		},
		{
			name:      "current account missing subtype",
			repo:      &fakeAccountRepo{},
			uc:        &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req: dto.CreateAccountRequest{
				Name:        "Test",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				ExpiresAt:   time.Now().AddDate(4, 0, 0),
			},
			expectErr: true,
			errMsg:    "subtype is required for current accounts",
		},
		{
			name: "currency not found in db",
			repo: &fakeAccountRepo{},
			uc:   &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "currencies"`).
					WillReturnError(fmt.Errorf("record not found"))
			},
			req:       validPersonalCurrentReq(),
			expectErr: true,
			errMsg:    "currency not found",
		},
		{
			name: "repo create fails",
			repo: &fakeAccountRepo{createErr: fmt.Errorf("insert failed")},
			uc:   &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "currencies"`).
					WillReturnRows(sqlmock.NewRows([]string{"currency_id", "code"}).AddRow(1, "RSD"))
			},
			req:       validPersonalCurrentReq(),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, mock := newMockDB(t)
			tt.setupMock(mock)

			svc := newAccountService(tt.repo, &fakeVerificationTokenRepo{}, tt.uc, db)
			account, err := svc.Create(context.Background(), tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, account)
			if tt.check != nil {
				tt.check(t, account)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetClientAccounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		expectErr bool
		check     func(t *testing.T, accounts []model.Account)
	}{
		{
			name: "success returns accounts",
			repo: &fakeAccountRepo{
				accounts: []model.Account{
					{AccountNumber: "444000112345678911", Name: "Standard Personal Account"},
					{AccountNumber: "444000112345678921", Name: "Personal EUR Account"},
				},
			},
			check: func(t *testing.T, accounts []model.Account) {
				require.Len(t, accounts, 2)
				require.Equal(t, "444000112345678911", accounts[0].AccountNumber)
				require.Equal(t, "444000112345678921", accounts[1].AccountNumber)
			},
		},
		{
			name:      "repo error returns internal error",
			repo:      &fakeAccountRepo{findErr: fmt.Errorf("db failure")},
			expectErr: true,
		},
		{
			name: "returns empty slice when client has no accounts",
			repo: &fakeAccountRepo{accounts: []model.Account{}},
			check: func(t *testing.T, accounts []model.Account) {
				require.Empty(t, accounts)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newAccountService(tt.repo, &fakeVerificationTokenRepo{}, &fakeAccountUserClient{}, nil)
			accounts, err := svc.GetClientAccounts(context.Background(), 1)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, accounts)
			}
		})
	}
}

func TestGetAccountDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		expectErr bool
		errMsg    string
	}{
		{
			name: "success",
			repo: &fakeAccountRepo{
				account: &model.Account{AccountNumber: "444000112345678911", Name: "My Account"},
			},
		},
		{
			name:      "account not found",
			repo:      &fakeAccountRepo{findErr: fmt.Errorf("record not found")},
			expectErr: true,
			errMsg:    "account not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newAccountService(tt.repo, &fakeVerificationTokenRepo{}, &fakeAccountUserClient{}, nil)
			account, err := svc.GetAccountDetails(context.Background(), "444000112345678911", 1)
			if tt.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, account)
			require.Equal(t, "444000112345678911", account.AccountNumber)
		})
	}
}

func TestUpdateAccountName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		newName   string
		expectErr bool
		errMsg    string
	}{
		{
			name: "success",
			repo: &fakeAccountRepo{
				account: &model.Account{AccountNumber: "444000112345678911", Name: "Old Name"},
			},
			newName: "New Name",
		},
		{
			name:      "account not found",
			repo:      &fakeAccountRepo{findErr: fmt.Errorf("record not found")},
			newName:   "New Name",
			expectErr: true,
			errMsg:    "account not found",
		},
		{
			name: "same name as current",
			repo: &fakeAccountRepo{
				account: &model.Account{AccountNumber: "444000112345678911", Name: "Same Name"},
			},
			newName:   "Same Name",
			expectErr: true,
			errMsg:    "same as the current name",
		},
		{
			name: "name already used by another account",
			repo: &fakeAccountRepo{
				account:    &model.Account{AccountNumber: "444000112345678911", Name: "Old Name"},
				nameExists: true,
			},
			newName:   "Taken Name",
			expectErr: true,
			errMsg:    "already exists",
		},
		{
			name: "update fails",
			repo: &fakeAccountRepo{
				account:       &model.Account{AccountNumber: "444000112345678911", Name: "Old Name"},
				updateNameErr: fmt.Errorf("db failure"),
			},
			newName:   "New Name",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newAccountService(tt.repo, &fakeVerificationTokenRepo{}, &fakeAccountUserClient{}, nil)
			err := svc.UpdateAccountName(context.Background(), "444000112345678911", 1, tt.newName)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRequestLimitsChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		vr        *fakeVerificationTokenRepo
		expectErr bool
		errMsg    string
	}{
		{
			name: "success",
			repo: &fakeAccountRepo{account: &model.Account{}},
			vr:   &fakeVerificationTokenRepo{},
		},
		{
			name:      "account not found",
			repo:      &fakeAccountRepo{findErr: fmt.Errorf("not found")},
			vr:        &fakeVerificationTokenRepo{},
			expectErr: true,
			errMsg:    "account not found",
		},
		{
			name:      "delete existing token fails",
			repo:      &fakeAccountRepo{account: &model.Account{}},
			vr:        &fakeVerificationTokenRepo{deleteErr: fmt.Errorf("db failure")},
			expectErr: true,
		},
		{
			name:      "create token fails",
			repo:      &fakeAccountRepo{account: &model.Account{}},
			vr:        &fakeVerificationTokenRepo{createErr: fmt.Errorf("db failure")},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newAccountService(tt.repo, tt.vr, &fakeAccountUserClient{}, nil)
			code, err := svc.RequestLimitsChange(context.Background(), "444000112345678911", 1, 500000, 2000000)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			require.NotEmpty(t, code)
			require.Len(t, code, 6)
		})
	}
}

func TestConfirmLimitsChange(t *testing.T) {
	t.Parallel()

	validToken := &model.VerificationToken{
		ID:              1,
		ClientID:        1,
		AccountNumber:   "444000112345678911",
		Code:            "123456",
		NewDailyLimit:   500000,
		NewMonthlyLimit: 2000000,
		ExpiresAt:       time.Now().Add(5 * time.Minute),
		Used:            false,
	}

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		vr        *fakeVerificationTokenRepo
		code      string
		expectErr bool
		errMsg    string
	}{
		{
			name: "success with correct code",
			repo: &fakeAccountRepo{},
			vr:   &fakeVerificationTokenRepo{token: validToken},
			code: "123456",
		},
		{
			name: "success with cheat code 1234",
			repo: &fakeAccountRepo{},
			vr:   &fakeVerificationTokenRepo{token: validToken},
			code: "1234",
		},
		{
			name:      "token not found",
			repo:      &fakeAccountRepo{},
			vr:        &fakeVerificationTokenRepo{findErr: fmt.Errorf("not found")},
			code:      "123456",
			expectErr: true,
			errMsg:    "no pending limits change",
		},
		{
			name: "token already used",
			repo: &fakeAccountRepo{},
			vr: &fakeVerificationTokenRepo{token: &model.VerificationToken{
				Code: "123456", ExpiresAt: time.Now().Add(5 * time.Minute), Used: true,
			}},
			code:      "123456",
			expectErr: true,
			errMsg:    "already been used",
		},
		{
			name: "token expired",
			repo: &fakeAccountRepo{},
			vr: &fakeVerificationTokenRepo{token: &model.VerificationToken{
				Code: "123456", ExpiresAt: time.Now().Add(-1 * time.Minute), Used: false,
			}},
			code:      "123456",
			expectErr: true,
			errMsg:    "expired",
		},
		{
			name:      "wrong code",
			repo:      &fakeAccountRepo{},
			vr:        &fakeVerificationTokenRepo{token: validToken},
			code:      "000000",
			expectErr: true,
			errMsg:    "invalid verification code",
		},
		{
			name:      "update limits fails",
			repo:      &fakeAccountRepo{updateLimitsErr: fmt.Errorf("db failure")},
			vr:        &fakeVerificationTokenRepo{token: validToken},
			code:      "123456",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newAccountService(tt.repo, tt.vr, &fakeAccountUserClient{}, nil)
			err := svc.ConfirmLimitsChange(context.Background(), "444000112345678911", 1, tt.code)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}
