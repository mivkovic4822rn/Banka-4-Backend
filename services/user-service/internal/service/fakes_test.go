package service

import (
	"common/pkg/auth"
	"common/pkg/permission"
	"context"
	"user-service/internal/config"
	"user-service/internal/model"

	"golang.org/x/crypto/bcrypt"
)

type fakeEmployeeRepo struct {
	byEmail    *model.Employee
	byUsername *model.Employee
	byID       *model.Employee
	allEmps    []model.Employee
	allTotal   int64

	findErr   error
	createErr error
	updateErr error
	getAllErr error

	createdEmployee *model.Employee
	updatedEmployee *model.Employee
}

func (f *fakeEmployeeRepo) FindByEmail(_ context.Context, _ string) (*model.Employee, error) {
	return f.byEmail, f.findErr
}

func (f *fakeEmployeeRepo) FindByUserName(_ context.Context, _ string) (*model.Employee, error) {
	return f.byUsername, f.findErr
}

func (f *fakeEmployeeRepo) FindByID(_ context.Context, _ uint) (*model.Employee, error) {
	return f.byID, f.findErr
}

func (f *fakeEmployeeRepo) Create(_ context.Context, emp *model.Employee) error {
	f.createdEmployee = emp
	return f.createErr
}

func (f *fakeEmployeeRepo) Update(_ context.Context, emp *model.Employee) error {
	f.updatedEmployee = emp
	return f.updateErr
}

func (f *fakeEmployeeRepo) GetAll(_ context.Context, _, _, _, _ string, _, _ int) ([]model.Employee, int64, error) {
	return f.allEmps, f.allTotal, f.getAllErr
}

type fakeActivationTokenRepo struct {
	token     *model.ActivationToken
	findErr   error
	createErr error
	deleteErr error
}

func (f *fakeActivationTokenRepo) Create(_ context.Context, _ *model.ActivationToken) error {
	return f.createErr
}

func (f *fakeActivationTokenRepo) FindByToken(_ context.Context, _ string) (*model.ActivationToken, error) {
	return f.token, f.findErr
}

func (f *fakeActivationTokenRepo) Delete(_ context.Context, _ *model.ActivationToken) error {
	return f.deleteErr
}

type fakeResetTokenRepo struct {
	token     *model.ResetToken
	findErr   error
	createErr error
	deleteErr error
}

func (f *fakeResetTokenRepo) Create(_ context.Context, _ *model.ResetToken) error {
	return f.createErr
}

func (f *fakeResetTokenRepo) FindByToken(_ context.Context, _ string) (*model.ResetToken, error) {
	return f.token, f.findErr
}

func (f *fakeResetTokenRepo) DeleteByEmployeeID(_ context.Context, _ uint) error {
	return f.deleteErr
}

type fakeRefreshTokenRepo struct {
	token     *model.RefreshToken
	findErr   error
	createErr error
	deleteErr error
}

func (f *fakeRefreshTokenRepo) Create(_ context.Context, _ *model.RefreshToken) error {
	return f.createErr
}

func (f *fakeRefreshTokenRepo) FindByToken(_ context.Context, _ string) (*model.RefreshToken, error) {
	return f.token, f.findErr
}

func (f *fakeRefreshTokenRepo) DeleteByEmployeeID(_ context.Context, _ uint) error {
	return f.deleteErr
}

type fakePositionRepo struct {
	exists    bool
	existsErr error
}

func (f *fakePositionRepo) Exists(_ context.Context, _ uint) (bool, error) {
	return f.exists, f.existsErr
}

type fakeMailer struct {
	sendErr error
	sent    bool
}

func (f *fakeMailer) Send(_, _, _ string) error {
	f.sent = true
	return f.sendErr
}

func testConfig() *config.Configuration {
	return &config.Configuration{
		JWTSecret:     "test-secret",
		JWTExpiry:     15,
		RefreshExpiry: 10080,
		URLs: config.URLConfig{
			FrontendBaseURL: "http://localhost:5173",
		},
	}
}

func hashedPassword(plain string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}

	return string(h)
}

func activeEmployee() *model.Employee {
	return &model.Employee{
		EmployeeID: 1,
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john@example.com",
		Username:   "johndoe",
		Password:   hashedPassword("Password12"),
		Active:     true,
		Permissions: []model.EmployeePermission{
			{EmployeeID: 1, Permission: permission.EmployeeView},
		},
	}
}

func withAuth(ctx context.Context, userID uint) context.Context {
	return auth.SetAuthOnContext(ctx, &auth.AuthContext{
		UserID:      userID,
		Permissions: []permission.Permission{permission.EmployeeView},
	})
}
