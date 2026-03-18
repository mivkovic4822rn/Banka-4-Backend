package service

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeCompanyRepo struct {
	createdCompany        *model.Company
	createErr             error
	workCodeExists        bool
	workCodeErr           error
	registrationNumExists bool
	registrationNumErr    error
	taxNumExists          bool
	taxNumErr             error
}

func (f *fakeCompanyRepo) Create(_ context.Context, company *model.Company) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.createdCompany = company
	return nil
}

func (f *fakeCompanyRepo) WorkCodeExists(_ context.Context, _ uint) (bool, error) {
	if f.workCodeErr != nil {
		return false, f.workCodeErr
	}
	return f.workCodeExists, nil
}

func (f *fakeCompanyRepo) RegistrationNumberExists(_ context.Context, _ string) (bool, error) {
	if f.registrationNumErr != nil {
		return false, f.registrationNumErr
	}
	return f.registrationNumExists, nil
}

func (f *fakeCompanyRepo) TaxNumberExists(_ context.Context, _ string) (bool, error) {
	if f.taxNumErr != nil {
		return false, f.taxNumErr
	}
	return f.taxNumExists, nil
}

func TestCreateCompany(t *testing.T) {
	t.Parallel()

	req := dto.CreateCompanyRequest{
		Name:               "Acme Ltd",
		RegistrationNumber: "12345678",
		TaxNumber:          "123456789",
		WorkCodeID:         1,
		Address:            "123 Main St",
		OwnerID:            1,
	}

	tests := []struct {
		name       string
		repo       *fakeCompanyRepo
		userClient *fakeUserClient
		req        dto.CreateCompanyRequest
		expectErr  bool
		errMsg     string
	}{
		{
			name:       "successful creation",
			repo:       &fakeCompanyRepo{workCodeExists: true},
			userClient: &fakeUserClient{},
			req:        req,
		},
		{
			name:       "owner client not found",
			repo:       &fakeCompanyRepo{},
			userClient: &fakeUserClient{clientErr: fmt.Errorf("not found")},
			req:        req,
			expectErr:  true,
			errMsg:     "owner client not found",
		},
		{
			name:       "work code not found",
			repo:       &fakeCompanyRepo{workCodeExists: false},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
			errMsg:     "work code not found",
		},
		{
			name:       "work code repo error",
			repo:       &fakeCompanyRepo{workCodeErr: fmt.Errorf("db error")},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
		},
		{
			name:       "registration number already exists",
			repo:       &fakeCompanyRepo{workCodeExists: true, registrationNumExists: true},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
			errMsg:     "registration number already exists",
		},
		{
			name:       "registration number repo error",
			repo:       &fakeCompanyRepo{workCodeExists: true, registrationNumErr: fmt.Errorf("db error")},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
		},
		{
			name:       "tax number already exists",
			repo:       &fakeCompanyRepo{workCodeExists: true, taxNumExists: true},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
			errMsg:     "tax number already exists",
		},
		{
			name:       "tax number repo error",
			repo:       &fakeCompanyRepo{workCodeExists: true, taxNumErr: fmt.Errorf("db error")},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
		},
		{
			name:       "repo create fails",
			repo:       &fakeCompanyRepo{workCodeExists: true, createErr: fmt.Errorf("db error")},
			userClient: &fakeUserClient{},
			req:        req,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewCompanyService(tt.repo, tt.userClient, nil)

			company, err := svc.Create(context.Background(), tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, company)
				require.Equal(t, tt.req.Name, company.Name)
				require.Equal(t, tt.req.RegistrationNumber, company.RegistrationNumber)
				require.Equal(t, tt.req.TaxNumber, company.TaxNumber)
				require.Equal(t, tt.req.WorkCodeID, company.WorkCodeID)
				require.Equal(t, tt.req.OwnerID, company.OwnerID)
			}
		})
	}
}
