package service

import (
	"common/pkg/permission"
	"context"
	"fmt"
	"testing"

	"user-service/internal/dto"
	"user-service/internal/model"

	"github.com/stretchr/testify/require"
)

func newEmployeeService(
	employeeRepo *fakeEmployeeRepo,
	identityRepo *fakeIdentityRepo,
	activationTokenRepo *fakeActivationTokenRepo,
	positionRepo *fakePositionRepo,
	mailer *fakeMailer,
) *EmployeeService {
	return NewEmployeeService(
		employeeRepo,
		identityRepo,
		activationTokenRepo,
		positionRepo,
		mailer,
		testConfig(),
	)
}

func TestRegister(t *testing.T) {
	t.Parallel()

	req := &dto.CreateEmployeeRequest{
		FirstName:  "Jane",
		LastName:   "Doe",
		Email:      "jane@example.com",
		Username:   "janedoe",
		PositionID: 1,
	}

	tests := []struct {
		name         string
		empRepo      *fakeEmployeeRepo
		identityRepo *fakeIdentityRepo
		positionRepo *fakePositionRepo
		mailer       *fakeMailer
		expectErr    bool
		errMsg       string
	}{
		{
			name:         "successful registration",
			empRepo:      &fakeEmployeeRepo{},
			identityRepo: &fakeIdentityRepo{},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{},
		},
		{
			name:         "email already in use",
			empRepo:      &fakeEmployeeRepo{},
			identityRepo: &fakeIdentityRepo{emailExists: true},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{},
			expectErr:    true,
			errMsg:       "email already in use",
		},
		{
			name:         "username already in use",
			empRepo:      &fakeEmployeeRepo{},
			identityRepo: &fakeIdentityRepo{usernameExists: true},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{},
			expectErr:    true,
			errMsg:       "username already in use",
		},
		{
			name:         "repo create fails",
			empRepo:      &fakeEmployeeRepo{createErr: fmt.Errorf("db error")},
			identityRepo: &fakeIdentityRepo{},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{},
			expectErr:    true,
		},
		{
			name:         "email send fails",
			empRepo:      &fakeEmployeeRepo{},
			identityRepo: &fakeIdentityRepo{},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{sendErr: fmt.Errorf("smtp down")},
			expectErr:    true,
		},
		{
			name:         "invalid position",
			empRepo:      &fakeEmployeeRepo{},
			identityRepo: &fakeIdentityRepo{},
			positionRepo: &fakePositionRepo{exists: false},
			mailer:       &fakeMailer{},
			expectErr:    true,
			errMsg:       "invalid position",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newEmployeeService(tt.empRepo, tt.identityRepo, &fakeActivationTokenRepo{}, tt.positionRepo, tt.mailer)

			emp, err := svc.Register(context.Background(), req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, emp)
				require.Equal(t, "Jane", emp.FirstName)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestUpdateEmployee(t *testing.T) {
	t.Parallel()

	existing := activeEmployee()
	existing.PositionID = 1

	existingAdmin := activeEmployee()
	existingAdmin.Permissions = mapPermissions(existing.PositionID, permission.All)

	identity := activeIdentity()

	req := &dto.UpdateEmployeeRequest{
		FirstName:  ptr("John"),
		LastName:   ptr("Updated"),
		Email:      ptr("john@example.com"),
		Username:   ptr("johndoe"),
		PositionID: ptr(uint(1)),
	}

	tests := []struct {
		name         string
		empRepo      *fakeEmployeeRepo
		identityRepo *fakeIdentityRepo
		positionRepo *fakePositionRepo
		id           uint
		req          *dto.UpdateEmployeeRequest
		expectErr    bool
		errMsg       string
	}{
		{
			name:         "successful update same email/username",
			empRepo:      &fakeEmployeeRepo{byID: existing},
			identityRepo: &fakeIdentityRepo{byID: identity},
			positionRepo: &fakePositionRepo{exists: true},
			id:           1,
			req:          req,
		},
		{
			name:         "employee not found",
			empRepo:      &fakeEmployeeRepo{byID: nil},
			identityRepo: &fakeIdentityRepo{},
			positionRepo: &fakePositionRepo{},
			id:           999,
			req:          req,
			expectErr:    true,
			errMsg:       "employee not found",
		},
		{
			// TODO: Test for ability for other admins, or the admin themselves, to update their details?
			name:         "employee is admin",
			empRepo:      &fakeEmployeeRepo{byID: existingAdmin},
			identityRepo: &fakeIdentityRepo{byID: identity},
			positionRepo: &fakePositionRepo{},
			id:           2,
			req:          req,
			expectErr:    true,
			errMsg:       "cannot modify admin",
		},
		{
			name:         "email conflict",
			empRepo:      &fakeEmployeeRepo{byID: existing},
			identityRepo: &fakeIdentityRepo{byID: identity, emailExists: true},
			positionRepo: &fakePositionRepo{},
			id:           1,
			req: &dto.UpdateEmployeeRequest{
				Email:    ptr("taken@example.com"),
				Username: ptr("johndoe"),
			},
			expectErr: true,
			errMsg:    "email already in use",
		},
		{
			name:         "username conflict",
			empRepo:      &fakeEmployeeRepo{byID: existing},
			identityRepo: &fakeIdentityRepo{byID: identity, usernameExists: true},
			positionRepo: &fakePositionRepo{},
			id:           1,
			req: &dto.UpdateEmployeeRequest{
				Email:    ptr("john@example.com"),
				Username: ptr("taken"),
			},
			expectErr: true,
			errMsg:    "username already in use",
		},
		{
			name:         "invalid position",
			empRepo:      &fakeEmployeeRepo{byID: existing},
			identityRepo: &fakeIdentityRepo{byID: identity},
			positionRepo: &fakePositionRepo{exists: false},
			id:           1,
			req: &dto.UpdateEmployeeRequest{
				Email:      ptr("john@example.com"),
				Username:   ptr("johndoe"),
				PositionID: ptr(uint(999)),
			},
			expectErr: true,
			errMsg:    "invalid position_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newEmployeeService(tt.empRepo, tt.identityRepo, &fakeActivationTokenRepo{}, tt.positionRepo, &fakeMailer{})

			result, err := svc.UpdateEmployee(context.Background(), tt.id, tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, "Updated", result.LastName)
			}
		})
	}
}

func TestGetAllEmployees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeEmployeeRepo
		expectErr bool
	}{
		{
			name: "success",
			repo: &fakeEmployeeRepo{
				allEmps:  []model.Employee{*activeEmployee()},
				allTotal: 1,
			},
		},
		{
			name:      "repo error",
			repo:      &fakeEmployeeRepo{getAllErr: fmt.Errorf("db error")},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newEmployeeService(tt.repo, &fakeIdentityRepo{}, &fakeActivationTokenRepo{}, &fakePositionRepo{}, &fakeMailer{})

			res, err := svc.GetAllEmployees(context.Background(), &dto.ListEmployeesQuery{Page: 1, PageSize: 10})

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(1), res.Total)
				require.Len(t, res.Data, 1)
			}
		})
	}
}
