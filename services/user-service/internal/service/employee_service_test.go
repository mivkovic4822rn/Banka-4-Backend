package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"user-service/internal/dto"
	"user-service/internal/model"

	"github.com/stretchr/testify/require"
)

func TestLogin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeEmployeeRepo
		req       *dto.LoginRequest
		expectErr bool
		errMsg    string
	}{
		{
			name: "successful login",
			repo: &fakeEmployeeRepo{byEmail: activeEmployee()},
			req:  &dto.LoginRequest{Email: "john@example.com", Password: "Password12"},
		},
		{
			name:      "user not found",
			repo:      &fakeEmployeeRepo{byEmail: nil},
			req:       &dto.LoginRequest{Email: "nobody@example.com", Password: "Password12"},
			expectErr: true,
			errMsg:    "invalid credentials",
		},
		{
			name:      "wrong password",
			repo:      &fakeEmployeeRepo{byEmail: activeEmployee()},
			req:       &dto.LoginRequest{Email: "john@example.com", Password: "WrongPass99"},
			expectErr: true,
			errMsg:    "invalid credentials",
		},
		{
			name: "inactive account",
			repo: &fakeEmployeeRepo{byEmail: func() *model.Employee {
				e := activeEmployee()
				e.Active = false
				return e
			}()},
			req:       &dto.LoginRequest{Email: "john@example.com", Password: "Password12"},
			expectErr: true,
			errMsg:    "account is disabled",
		},
		{
			name:      "repo error",
			repo:      &fakeEmployeeRepo{findErr: fmt.Errorf("db down")},
			req:       &dto.LoginRequest{Email: "john@example.com", Password: "Password12"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewEmployeeService(tt.repo, &fakeActivationTokenRepo{}, &fakeResetTokenRepo{}, &fakeRefreshTokenRepo{}, &fakePositionRepo{}, &fakeMailer{}, testConfig())

			res, err := svc.Login(context.Background(), tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				require.Nil(t, res)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, res.Token)
				require.NotEmpty(t, res.RefreshToken)
				require.NotNil(t, res.User)
				require.Equal(t, uint(1), res.User.ID)
			}
		})
	}
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
		repo         *fakeEmployeeRepo
		positionRepo *fakePositionRepo
		mailer       *fakeMailer
		expectErr    bool
		errMsg       string
	}{
		{
			name:         "successful registration",
			repo:         &fakeEmployeeRepo{},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{},
		},
		{
			name:         "email already in use",
			repo:         &fakeEmployeeRepo{byEmail: activeEmployee()},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{},
			expectErr:    true,
			errMsg:       "email already in use",
		},
		{
			name:         "username already in use",
			repo:         &fakeEmployeeRepo{byUsername: activeEmployee()},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{},
			expectErr:    true,
			errMsg:       "username already in use",
		},
		{
			name:         "repo create fails",
			repo:         &fakeEmployeeRepo{createErr: fmt.Errorf("db error")},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{},
			expectErr:    true,
		},
		{
			name:         "email send fails",
			repo:         &fakeEmployeeRepo{},
			positionRepo: &fakePositionRepo{exists: true},
			mailer:       &fakeMailer{sendErr: fmt.Errorf("smtp down")},
			expectErr:    true,
		},
		{
			name:         "invalid position",
			repo:         &fakeEmployeeRepo{},
			positionRepo: &fakePositionRepo{exists: false},
			mailer:       &fakeMailer{},
			expectErr:    true,
			errMsg:       "invalid position",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewEmployeeService(tt.repo, &fakeActivationTokenRepo{}, &fakeResetTokenRepo{}, &fakeRefreshTokenRepo{}, tt.positionRepo, tt.mailer, testConfig())

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

func TestActivateAccount(t *testing.T) {
	t.Parallel()

	validToken := &model.ActivationToken{
		EmployeeID: 1,
		Token:      "valid-token",
		ExpiresAt:  time.Now().Add(1 * time.Hour),
	}

	expiredToken := &model.ActivationToken{
		EmployeeID: 1,
		Token:      "expired-token",
		ExpiresAt:  time.Now().Add(-1 * time.Hour),
	}

	tests := []struct {
		name      string
		tokenRepo *fakeActivationTokenRepo
		empRepo   *fakeEmployeeRepo
		token     string
		password  string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "successful activation",
			tokenRepo: &fakeActivationTokenRepo{token: validToken},
			empRepo:   &fakeEmployeeRepo{byID: activeEmployee()},
			token:     "valid-token",
			password:  "NewPass12",
		},
		{
			name:      "invalid token",
			tokenRepo: &fakeActivationTokenRepo{token: nil},
			empRepo:   &fakeEmployeeRepo{},
			token:     "bad-token",
			expectErr: true,
			errMsg:    "invalid or expired token",
		},
		{
			name:      "expired token",
			tokenRepo: &fakeActivationTokenRepo{token: expiredToken},
			empRepo:   &fakeEmployeeRepo{},
			token:     "expired-token",
			expectErr: true,
			errMsg:    "token expired",
		},
		{
			name:      "employee not found",
			tokenRepo: &fakeActivationTokenRepo{token: validToken},
			empRepo:   &fakeEmployeeRepo{byID: nil},
			token:     "valid-token",
			expectErr: true,
			errMsg:    "employee not found",
		},
		{
			name:      "repo update fails",
			tokenRepo: &fakeActivationTokenRepo{token: validToken},
			empRepo:   &fakeEmployeeRepo{byID: activeEmployee(), updateErr: fmt.Errorf("db error")},
			token:     "valid-token",
			password:  "NewPass12",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewEmployeeService(tt.empRepo, tt.tokenRepo, &fakeResetTokenRepo{}, &fakeRefreshTokenRepo{}, &fakePositionRepo{}, &fakeMailer{}, testConfig())

			err := svc.ActivateAccount(context.Background(), tt.token, tt.password)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, tt.empRepo.updatedEmployee)
				require.NotEqual(t, tt.password, tt.empRepo.updatedEmployee.Password)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestUpdateEmployee(t *testing.T) {
	t.Parallel()

	existing := activeEmployee()
	existing.PositionID = 1

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
		positionRepo *fakePositionRepo
		id           uint
		req          *dto.UpdateEmployeeRequest
		expectErr    bool
		errMsg       string
	}{
		{
			name:         "successful update same email/username",
			empRepo:      &fakeEmployeeRepo{byID: existing},
			positionRepo: &fakePositionRepo{exists: true},
			id:           1,
			req:          req,
		},
		{
			name:         "employee not found",
			empRepo:      &fakeEmployeeRepo{byID: nil},
			positionRepo: &fakePositionRepo{},
			id:           999,
			req:          req,
			expectErr:    true,
			errMsg:       "employee not found",
		},
		{
			name: "email conflict",
			empRepo: &fakeEmployeeRepo{
				byID:    existing,
				byEmail: &model.Employee{EmployeeID: 2},
			},
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
			name: "username conflict",
			empRepo: &fakeEmployeeRepo{
				byID:       existing,
				byUsername: &model.Employee{EmployeeID: 2},
			},
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
			svc := NewEmployeeService(tt.empRepo, &fakeActivationTokenRepo{}, &fakeResetTokenRepo{}, &fakeRefreshTokenRepo{}, tt.positionRepo, &fakeMailer{}, testConfig())

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
			svc := NewEmployeeService(tt.repo, &fakeActivationTokenRepo{}, &fakeResetTokenRepo{}, &fakeRefreshTokenRepo{}, &fakePositionRepo{}, &fakeMailer{}, testConfig())

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

func TestRefreshToken(t *testing.T) {
	t.Parallel()

	validRefresh := &model.RefreshToken{
		EmployeeID: 1,
		Token:      "valid-refresh",
		ExpiresAt:  time.Now().Add(1 * time.Hour),
	}

	expiredRefresh := &model.RefreshToken{
		EmployeeID: 1,
		Token:      "expired-refresh",
		ExpiresAt:  time.Now().Add(-1 * time.Hour),
	}

	tests := []struct {
		name        string
		refreshRepo *fakeRefreshTokenRepo
		empRepo     *fakeEmployeeRepo
		token       string
		expectErr   bool
		errMsg      string
	}{
		{
			name:        "successful refresh",
			refreshRepo: &fakeRefreshTokenRepo{token: validRefresh},
			empRepo:     &fakeEmployeeRepo{byID: activeEmployee()},
			token:       "valid-refresh",
		},
		{
			name:        "token not found",
			refreshRepo: &fakeRefreshTokenRepo{token: nil},
			empRepo:     &fakeEmployeeRepo{},
			token:       "bad-token",
			expectErr:   true,
			errMsg:      "invalid or expired refresh token",
		},
		{
			name:        "expired refresh token",
			refreshRepo: &fakeRefreshTokenRepo{token: expiredRefresh},
			empRepo:     &fakeEmployeeRepo{},
			token:       "expired-refresh",
			expectErr:   true,
			errMsg:      "refresh token expired",
		},
		{
			name:        "employee not found",
			refreshRepo: &fakeRefreshTokenRepo{token: validRefresh},
			empRepo:     &fakeEmployeeRepo{byID: nil},
			token:       "valid-refresh",
			expectErr:   true,
			errMsg:      "user not found",
		},
		{
			name:        "inactive account",
			refreshRepo: &fakeRefreshTokenRepo{token: validRefresh},
			empRepo: &fakeEmployeeRepo{byID: func() *model.Employee {
				e := activeEmployee()
				e.Active = false
				return e
			}()},
			token:     "valid-refresh",
			expectErr: true,
			errMsg:    "account is disabled",
		},
		{
			name:        "repo error on find",
			refreshRepo: &fakeRefreshTokenRepo{findErr: fmt.Errorf("db down")},
			empRepo:     &fakeEmployeeRepo{},
			token:       "any",
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewEmployeeService(tt.empRepo, &fakeActivationTokenRepo{}, &fakeResetTokenRepo{}, tt.refreshRepo, &fakePositionRepo{}, &fakeMailer{}, testConfig())

			res, err := svc.RefreshToken(context.Background(), tt.token)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, res.Token)
				require.NotEmpty(t, res.RefreshToken)
				require.NotNil(t, res.User)
			}
		})
	}
}

func TestConfirmPasswordReset(t *testing.T) {
	t.Parallel()

	validReset := &model.ResetToken{
		EmployeeID: 1,
		Token:      "valid-reset",
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}

	expiredReset := &model.ResetToken{
		EmployeeID: 1,
		Token:      "expired-reset",
		ExpiresAt:  time.Now().Add(-10 * time.Minute),
	}

	tests := []struct {
		name      string
		resetRepo *fakeResetTokenRepo
		empRepo   *fakeEmployeeRepo
		token     string
		password  string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "successful reset",
			resetRepo: &fakeResetTokenRepo{token: validReset},
			empRepo:   &fakeEmployeeRepo{byID: activeEmployee()},
			token:     "valid-reset",
			password:  "NewPass12",
		},
		{
			name:      "token not found",
			resetRepo: &fakeResetTokenRepo{token: nil},
			empRepo:   &fakeEmployeeRepo{},
			token:     "bad",
			expectErr: true,
			errMsg:    "invalid or expired token",
		},
		{
			name:      "expired token",
			resetRepo: &fakeResetTokenRepo{token: expiredReset},
			empRepo:   &fakeEmployeeRepo{},
			token:     "expired-reset",
			expectErr: true,
			errMsg:    "token has expired",
		},
		{
			name:      "employee not found",
			resetRepo: &fakeResetTokenRepo{token: validReset},
			empRepo:   &fakeEmployeeRepo{byID: nil},
			token:     "valid-reset",
			expectErr: true,
			errMsg:    "employee not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewEmployeeService(tt.empRepo, &fakeActivationTokenRepo{}, tt.resetRepo, &fakeRefreshTokenRepo{}, &fakePositionRepo{}, &fakeMailer{}, testConfig())

			err := svc.ConfirmPasswordReset(context.Background(), tt.token, tt.password)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRequestPasswordReset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		empRepo   *fakeEmployeeRepo
		resetRepo *fakeResetTokenRepo
		email     string
		expectErr bool
	}{
		{
			name:      "employee found sends reset",
			empRepo:   &fakeEmployeeRepo{byEmail: activeEmployee()},
			resetRepo: &fakeResetTokenRepo{},
			email:     "john@example.com",
		},
		{
			name:      "employee not found returns nil",
			empRepo:   &fakeEmployeeRepo{byEmail: nil},
			resetRepo: &fakeResetTokenRepo{},
			email:     "nobody@example.com",
		},
		{
			name:      "repo error",
			empRepo:   &fakeEmployeeRepo{findErr: fmt.Errorf("db down")},
			resetRepo: &fakeResetTokenRepo{},
			email:     "john@example.com",
			expectErr: true,
		},
		{
			name:      "delete old token fails",
			empRepo:   &fakeEmployeeRepo{byEmail: activeEmployee()},
			resetRepo: &fakeResetTokenRepo{deleteErr: fmt.Errorf("db error")},
			email:     "john@example.com",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewEmployeeService(tt.empRepo, &fakeActivationTokenRepo{}, tt.resetRepo, &fakeRefreshTokenRepo{}, &fakePositionRepo{}, &fakeMailer{}, testConfig())

			err := svc.RequestPasswordReset(context.Background(), tt.email)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfirmChangePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		empRepo     *fakeEmployeeRepo
		ctx         context.Context
		oldPassword string
		newPassword string
		expectErr   bool
		errMsg      string
	}{
		{
			name:        "successful change",
			empRepo:     &fakeEmployeeRepo{byID: activeEmployee()},
			ctx:         withAuth(context.Background(), 1),
			oldPassword: "Password12",
			newPassword: "NewPass99",
		},
		{
			name:        "no auth context",
			empRepo:     &fakeEmployeeRepo{},
			ctx:         context.Background(),
			oldPassword: "Password12",
			newPassword: "NewPass99",
			expectErr:   true,
			errMsg:      "invalid credentials",
		},
		{
			name:        "same old and new password",
			empRepo:     &fakeEmployeeRepo{byID: activeEmployee()},
			ctx:         withAuth(context.Background(), 1),
			oldPassword: "Password12",
			newPassword: "Password12",
			expectErr:   true,
			errMsg:      "new password cannot be the same as the old one",
		},
		{
			name:        "wrong old password",
			empRepo:     &fakeEmployeeRepo{byID: activeEmployee()},
			ctx:         withAuth(context.Background(), 1),
			oldPassword: "WrongPass99",
			newPassword: "NewPass99",
			expectErr:   true,
			errMsg:      "invalid credentials",
		},
		{
			name:        "employee not found",
			empRepo:     &fakeEmployeeRepo{byID: nil},
			ctx:         withAuth(context.Background(), 1),
			oldPassword: "Password12",
			newPassword: "NewPass99",
			expectErr:   true,
			errMsg:      "invalid credentials",
		},
		{
			name:        "repo update fails",
			empRepo:     &fakeEmployeeRepo{byID: activeEmployee(), updateErr: fmt.Errorf("db error")},
			ctx:         withAuth(context.Background(), 1),
			oldPassword: "Password12",
			newPassword: "NewPass99",
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewEmployeeService(tt.empRepo, &fakeActivationTokenRepo{}, &fakeResetTokenRepo{}, &fakeRefreshTokenRepo{}, &fakePositionRepo{}, &fakeMailer{}, testConfig())

			err := svc.ConfirmChangePassword(tt.ctx, tt.oldPassword, tt.newPassword)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
