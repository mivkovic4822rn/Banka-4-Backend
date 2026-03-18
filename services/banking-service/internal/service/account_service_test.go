package service

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"common/pkg/pb"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeAccountRepo struct {
	createErr           error
	accountNumberExists bool
	nameExists          bool
	nameExistsErr       error
}

func (f *fakeAccountRepo) Create(_ context.Context, _ *model.Account) error {
	return f.createErr
}

func (f *fakeAccountRepo) AccountNumberExists(_ context.Context, _ string) (bool, error) {
	return f.accountNumberExists, nil
}

func (f *fakeAccountRepo) FindByAccountNumber(_ context.Context, _ string) (*model.Account, error) {
	return nil, nil
}

func (f *fakeAccountRepo) UpdateBalance(_ context.Context, _ *model.Account) error {
	return nil
}

func (f *fakeAccountRepo) NameExistsForClient(_ context.Context, _ uint, _ string, _ string) (bool, error) {
	if f.nameExistsErr != nil {
		return false, f.nameExistsErr
	}
	return f.nameExists, nil
}

type fakeCurrencyRepo struct {
	currency *model.Currency
	findErr  error
}

func (f *fakeCurrencyRepo) FindByCode(_ context.Context, _ model.CurrencyCode) (*model.Currency, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.currency, nil
}

type fakeUserClient struct {
	clientErr   error
	employeeErr error
}

func (f *fakeUserClient) GetClientByID(_ context.Context, _ uint) (*pb.GetClientByIdResponse, error) {
	if f.clientErr != nil {
		return nil, f.clientErr
	}
	return &pb.GetClientByIdResponse{}, nil
}

func (f *fakeUserClient) GetEmployeeByID(_ context.Context, _ uint) (*pb.GetEmployeeByIdResponse, error) {
	if f.employeeErr != nil {
		return nil, f.employeeErr
	}
	return &pb.GetEmployeeByIdResponse{}, nil
}

type fakeCurrencyConverter struct {
	result     float64
	convertErr error
}

func (f *fakeCurrencyConverter) Convert(_ context.Context, amount float64, _ model.CurrencyCode, _ model.CurrencyCode) (float64, error) {
	if f.convertErr != nil {
		return 0, f.convertErr
	}
	if f.result != 0 {
		return f.result, nil
	}
	return amount, nil
}

func rsdCurrency() *model.Currency {
	return &model.Currency{
		CurrencyID: 1,
		Name:       "Serbian Dinar",
		Code:       model.RSD,
		Symbol:     "RSD",
		Country:    "Serbia",
		Status:     "Active",
	}
}

func eurCurrency() *model.Currency {
	return &model.Currency{
		CurrencyID: 2,
		Name:       "Euro",
		Code:       model.EUR,
		Symbol:     "€",
		Country:    "EU",
		Status:     "Active",
	}
}

func ptrUint(v uint) *uint { return &v }

func baseExpiresAt() time.Time {
	return time.Now().AddDate(5, 0, 0)
}

func newAccountService(
	accountRepo *fakeAccountRepo,
	currencyRepo *fakeCurrencyRepo,
	userClient *fakeUserClient,
	exchangeConverter *fakeCurrencyConverter,
) *AccountService {
	return NewAccountService(accountRepo, currencyRepo, userClient, nil, exchangeConverter)
}

func TestCreateAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		accountRepo       *fakeAccountRepo
		currencyRepo      *fakeCurrencyRepo
		userClient        *fakeUserClient
		exchangeConverter *fakeCurrencyConverter
		req               dto.CreateAccountRequest
		expectErr         bool
		errMsg            string
	}{
		{
			name:              "successful personal current account",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{currency: rsdCurrency()},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "My Account",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
		},
		{
			name:              "successful business current account",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{currency: rsdCurrency()},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "Business Account",
				ClientID:    1,
				EmployeeID:  1,
				CompanyID:   ptrUint(10),
				AccountType: model.AccountTypeBusiness,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeLLC,
				ExpiresAt:   baseExpiresAt(),
			},
		},
		{
			name:              "successful foreign account with converted limits",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{currency: eurCurrency()},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{result: 2500.0},
			req: dto.CreateAccountRequest{
				Name:         "EUR Account",
				ClientID:     1,
				EmployeeID:   1,
				AccountType:  model.AccountTypePersonal,
				AccountKind:  model.AccountKindForeign,
				CurrencyCode: model.EUR,
				ExpiresAt:    baseExpiresAt(),
			},
		},
		{
			name:              "client not found",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{clientErr: fmt.Errorf("not found")},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "My Account",
				ClientID:    999,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "client not found",
		},
		{
			name:              "employee not found",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{employeeErr: fmt.Errorf("not found")},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "My Account",
				ClientID:    1,
				EmployeeID:  999,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "employee not found",
		},
		{
			name:              "business account without company",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "Business No Company",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypeBusiness,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeLLC,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "business account requires a company",
		},
		{
			name:              "personal account with company",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "Personal With Company",
				ClientID:    1,
				EmployeeID:  1,
				CompanyID:   ptrUint(10),
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "personal account cannot have a company",
		},
		{
			name:              "foreign account without currency code",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "Foreign No Currency",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindForeign,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "currency code is required for foreign accounts",
		},
		{
			name:              "current account without subtype",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "No Subtype",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "subtype is required for current accounts",
		},
		{
			name:              "account name already exists for client",
			accountRepo:       &fakeAccountRepo{nameExists: true},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "Duplicate Name",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "account with this name already exists",
		},
		{
			name:              "name exists repo error",
			accountRepo:       &fakeAccountRepo{nameExistsErr: fmt.Errorf("db error")},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "My Account",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
		},
		{
			name:              "currency not found",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{findErr: fmt.Errorf("currency not found: RSD")},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "My Account",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "currency not found",
		},
		{
			name:              "exchange conversion fails",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{currency: eurCurrency()},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{convertErr: fmt.Errorf("exchange service unavailable")},
			req: dto.CreateAccountRequest{
				Name:         "EUR Account",
				ClientID:     1,
				EmployeeID:   1,
				AccountType:  model.AccountTypePersonal,
				AccountKind:  model.AccountKindForeign,
				CurrencyCode: model.EUR,
				ExpiresAt:    baseExpiresAt(),
			},
			expectErr: true,
		},
		{
			name:              "repo create fails",
			accountRepo:       &fakeAccountRepo{createErr: fmt.Errorf("db error")},
			currencyRepo:      &fakeCurrencyRepo{currency: rsdCurrency()},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "My Account",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newAccountService(tt.accountRepo, tt.currencyRepo, tt.userClient, tt.exchangeConverter)

			account, err := svc.Create(context.Background(), tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, account)
				require.NotEmpty(t, account.AccountNumber)
				require.Equal(t, tt.req.ClientID, account.ClientID)
				require.Equal(t, tt.req.EmployeeID, account.EmployeeID)
				require.Equal(t, tt.req.AccountType, account.AccountType)
				require.Equal(t, tt.req.AccountKind, account.AccountKind)
				require.Equal(t, tt.currencyRepo.currency.CurrencyID, account.CurrencyID)
			}
		})
	}
}
