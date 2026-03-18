package service

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"common/pkg/pb"
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ── Fakes ─────────────────────────────────────────────────────────────────────

type fakeCompanyRepo struct {
	workCodeExists    bool
	workCodeExistsErr error
	taxNumExists    bool
	taxNumExistsErr error
	regNumExists    bool
	regNumExistsErr error
	createErr error
}

func (r *fakeCompanyRepo) Create(_ context.Context, _ *model.Company) error {
	return r.createErr
}

func (r *fakeCompanyRepo) RegistrationNumberExists(_ context.Context, _ string) (bool, error) {
	return r.regNumExists, r.regNumExistsErr
}

func (r *fakeCompanyRepo) TaxNumberExists(_ context.Context, _ string) (bool, error) {
	return r.taxNumExists, r.taxNumExistsErr
}

func (r *fakeCompanyRepo) WorkCodeExists(_ context.Context, _ uint) (bool, error) {
	return r.workCodeExists, r.workCodeExistsErr
}

type fakeUserClient struct {
	clientErr   error
	employeeErr error
}

func (f *fakeUserClient) GetClientByID(_ context.Context, _ uint) (*pb.GetClientByIdResponse, error) {
	return nil, f.clientErr
}

func (f *fakeUserClient) GetEmployeeByID(_ context.Context, _ uint) (*pb.GetEmployeeByIdResponse, error) {
	return nil, f.employeeErr
}

// ── DB helper ─────────────────────────────────────────────────────────────────

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	dialector := postgres.New(postgres.Config{
		Conn:       sqlDB,
		DriverName: "postgres",
	})
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)

	t.Cleanup(func() { sqlDB.Close() })
	return db, mock
}

// ── Constructor ───────────────────────────────────────────────────────────────

func newCompanyService(
	repo *fakeCompanyRepo,
	uc *fakeUserClient,
	db *gorm.DB,
) *CompanyService {
	return &CompanyService{
		repo:       repo,
		userClient: uc,
		db:         db,
	}
}

// ── Fixture ───────────────────────────────────────────────────────────────────

func validCompanyReq() dto.CreateCompanyRequest {
	return dto.CreateCompanyRequest{
		Name:               "Tech DOO",
		RegistrationNumber: "12345678",
		TaxNumber:          "123456789",
		WorkCodeID:         1,
		Address:            "Knez Mihailova 10",
		OwnerID:            1,
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestCreateCompany(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeCompanyRepo
		uc        *fakeUserClient
		setupMock func(mock sqlmock.Sqlmock)
		req       dto.CreateCompanyRequest
		expectErr bool
		errMsg    string
	}{
		{
			name:      "success",
			repo:      &fakeCompanyRepo{workCodeExists: true},
			uc:        &fakeUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req:       validCompanyReq(),
		},
		{
			name:      "owner not found",
			repo:      &fakeCompanyRepo{workCodeExists: true},
			uc:        &fakeUserClient{clientErr: fmt.Errorf("not found")},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req:       validCompanyReq(),
			expectErr: true,
			errMsg:    "owner client not found",
		},
		{
			name: "work code not found",
			repo: &fakeCompanyRepo{},
			uc:   &fakeUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT count\(\*\) FROM "work_codes"`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
			req:       validCompanyReq(),
			expectErr: true,
			errMsg:    "work code not found",
		},
		{
			name: "work code db error",
			repo: &fakeCompanyRepo{},
			uc:   &fakeUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT count\(\*\) FROM "work_codes"`).
					WillReturnError(fmt.Errorf("db error"))
			},
			req:       validCompanyReq(),
			expectErr: true,
		},
		{
			name: "registration number already exists",
			repo: &fakeCompanyRepo{workCodeExists: true, regNumExists: true},
			uc:   &fakeUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT count\(\*\) FROM "work_codes"`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(`SELECT count\(\*\) FROM "companies"`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			},
			req:       validCompanyReq(),
			expectErr: true,
			errMsg:    "registration number already exists",
		},
		{
			name: "registration number db error",
			repo: &fakeCompanyRepo{regNumExists: true},
			uc:   &fakeUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT count\(\*\) FROM "work_codes"`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(`SELECT count\(\*\) FROM "companies"`).
					WillReturnError(fmt.Errorf("db error"))
			},
			req:       validCompanyReq(),
			expectErr: true,
		},
		{
			name: "tax number already exists",
			repo: &fakeCompanyRepo{workCodeExists: true, taxNumExists: true},
			uc:   &fakeUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT count\(\*\) FROM "work_codes"`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(`SELECT count\(\*\) FROM "companies"`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery(`SELECT count\(\*\) FROM "companies"`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			},
			req:       validCompanyReq(),
			expectErr: true,
			errMsg:    "tax number already exists",
		},
		{
			name: "tax number db error",
			repo: &fakeCompanyRepo{taxNumExists: true},
			uc:   &fakeUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT count\(\*\) FROM "work_codes"`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(`SELECT count\(\*\) FROM "companies"`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery(`SELECT count\(\*\) FROM "companies"`).
					WillReturnError(fmt.Errorf("db error"))
			},
			req:       validCompanyReq(),
			expectErr: true,
		},
		{
			name: "repo create fails",
			repo: &fakeCompanyRepo{createErr: fmt.Errorf("insert failed")},
			uc:   &fakeUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT count\(\*\) FROM "work_codes"`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(`SELECT count\(\*\) FROM "companies"`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery(`SELECT count\(\*\) FROM "companies"`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
			req:       validCompanyReq(),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, mock := newMockDB(t)
			tt.setupMock(mock)

			svc := newCompanyService(tt.repo, tt.uc, db)
			company, err := svc.Create(context.Background(), tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, company)
			require.Equal(t, tt.req.Name, company.Name)
			require.Equal(t, tt.req.OwnerID, company.OwnerID)
			require.Equal(t, tt.req.RegistrationNumber, company.RegistrationNumber)
			require.Equal(t, tt.req.TaxNumber, company.TaxNumber)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
