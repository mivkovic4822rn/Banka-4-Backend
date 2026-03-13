//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	commonpermission "common/pkg/permission"
	"user-service/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogin(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)
	employee := seedEmployee(t, db, position.PositionID)

	testCases := []struct {
		name       string
		body       any
		rawBody    string
		wantStatus int
		assertBody func(t *testing.T, recorderBody loginResponse)
	}{
		{
			name: "correct credentials",
			body: map[string]any{
				"email":    employee.Email,
				"password": "Password12",
			},
			wantStatus: http.StatusOK,
			assertBody: func(t *testing.T, response loginResponse) {
				t.Helper()
				assert.NotEmpty(t, response.Token)
				assert.NotEmpty(t, response.RefreshToken)
				assert.Equal(t, employee.EmployeeID, response.User.ID)
			},
		},
		{
			name: "wrong password",
			body: map[string]any{
				"email":    employee.Email,
				"password": "WrongPassword12",
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "non existent email",
			body: map[string]any{
				"email":    "missing@example.com",
				"password": "Password12",
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid json",
			rawBody:    "{",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var recorder *httptest.ResponseRecorder
			if tc.rawBody != "" {
				recorder = performRawJSONRequest(t, router, http.MethodPost, "/api/employees/login", tc.rawBody, "")
			} else {
				recorder = performRequest(t, router, http.MethodPost, "/api/employees/login", tc.body, "")
			}

			requireStatus(t, recorder, tc.wantStatus)
			if tc.assertBody != nil {
				tc.assertBody(t, decodeResponse[loginResponse](t, recorder))
			}
		})
	}
}

func TestRegister(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)
	admin := seedEmployeeWithPermissions(t, db, position.PositionID, commonpermission.EmployeeCreate)
	noPermissionUser := seedEmployee(t, db, position.PositionID)

	validBody := map[string]any{
		"first_name":    "Ana",
		"last_name":     "Test",
		"gender":        "female",
		"date_of_birth": time.Now().UTC().AddDate(-25, 0, 0).Format(time.RFC3339),
		"email":         uniqueValue(t, "register") + "@example.com",
		"phone_number":  "0600000000",
		"address":       "Main 1",
		"username":      uniqueValue(t, "register-user"),
		"department":    "QA",
		"position_id":   position.PositionID,
		"active":        false,
		"permissions":   []string{string(commonpermission.EmployeeView)},
	}

	testCases := []struct {
		name       string
		body       map[string]any
		auth       string
		wantStatus int
		wantEmail  string
	}{
		{
			name:       "valid data with permission",
			body:       validBody,
			auth:       authHeader(t, admin.EmployeeID),
			wantStatus: http.StatusCreated,
			wantEmail:  validBody["email"].(string),
		},
		{
			name: "duplicate email",
			body: map[string]any{
				"first_name":    "Ana",
				"last_name":     "Dup",
				"gender":        "female",
				"date_of_birth": time.Now().UTC().AddDate(-24, 0, 0).Format(time.RFC3339),
				"email":         validBody["email"],
				"phone_number":  "0600000001",
				"address":       "Main 2",
				"username":      uniqueValue(t, "register-user"),
				"department":    "QA",
				"position_id":   position.PositionID,
				"active":        true,
			},
			auth:       authHeader(t, admin.EmployeeID),
			wantStatus: http.StatusConflict,
		},
		{
			name:       "missing auth header",
			body:       map[string]any{"email": uniqueValue(t, "missing-auth") + "@example.com"},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "without employee create permission",
			body: map[string]any{
				"first_name":    "Ana",
				"last_name":     "NoPerm",
				"gender":        "female",
				"date_of_birth": time.Now().UTC().AddDate(-24, 0, 0).Format(time.RFC3339),
				"email":         uniqueValue(t, "noperm") + "@example.com",
				"phone_number":  "0600000002",
				"address":       "Main 3",
				"username":      uniqueValue(t, "noperm-user"),
				"department":    "QA",
				"position_id":   position.PositionID,
				"active":        true,
			},
			auth:       authHeader(t, noPermissionUser.EmployeeID),
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodPost, "/api/employees/register", tc.body, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusCreated {
				response := decodeResponse[employeeResponse](t, recorder)
				assert.Equal(t, tc.wantEmail, response.Email)
				assert.NotZero(t, response.ID)
			}
		})
	}
}

func TestListEmployees(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	developer := seedPosition(t, db)
	developer.Title = "Developer"
	require.NoError(t, db.Save(developer).Error)

	manager := seedPosition(t, db)
	manager.Title = "Manager"
	require.NoError(t, db.Save(manager).Error)

	viewer := seedEmployeeWithPermissions(t, db, developer.PositionID, commonpermission.EmployeeView)
	devOne := seedEmployee(t, db, developer.PositionID)
	devTwo := seedEmployee(t, db, developer.PositionID)
	managerEmployee := seedEmployee(t, db, manager.PositionID)

	testCases := []struct {
		name         string
		path         string
		auth         string
		wantStatus   int
		wantTotal    int64
		wantCount    int
		assertResult func(t *testing.T, response listEmployeesResponse)
	}{
		{
			name:       "list all employees",
			path:       "/api/employees?page=1&page_size=10",
			auth:       authHeader(t, viewer.EmployeeID),
			wantStatus: http.StatusOK,
			wantTotal:  4,
			wantCount:  4,
		},
		{
			name:       "filter by email",
			path:       "/api/employees?email=" + url.QueryEscape(devOne.Email) + "&page=1&page_size=10",
			auth:       authHeader(t, viewer.EmployeeID),
			wantStatus: http.StatusOK,
			wantTotal:  1,
			wantCount:  1,
			assertResult: func(t *testing.T, response listEmployeesResponse) {
				t.Helper()
				assert.Equal(t, devOne.Email, response.Data[0].Email)
			},
		},
		{
			name:       "filter by position title",
			path:       "/api/employees?position=Developer&page=1&page_size=10",
			auth:       authHeader(t, viewer.EmployeeID),
			wantStatus: http.StatusOK,
			wantTotal:  3,
			wantCount:  3,
			assertResult: func(t *testing.T, response listEmployeesResponse) {
				t.Helper()
				emails := []string{response.Data[0].Email, response.Data[1].Email, response.Data[2].Email}
				assert.Contains(t, emails, viewer.Email)
				assert.Contains(t, emails, devOne.Email)
				assert.Contains(t, emails, devTwo.Email)
				assert.NotContains(t, emails, managerEmployee.Email)
			},
		},
		{
			name:       "missing auth header",
			path:       "/api/employees",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodGet, tc.path, nil, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusOK {
				response := decodeResponse[listEmployeesResponse](t, recorder)
				assert.Equal(t, tc.wantTotal, response.Total)
				assert.Len(t, response.Data, tc.wantCount)
				if tc.assertResult != nil {
					tc.assertResult(t, response)
				}
			}
		})
	}
}

func TestGetEmployee(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)
	viewer := seedEmployeeWithPermissions(t, db, position.PositionID, commonpermission.EmployeeView)
	employee := seedEmployee(t, db, position.PositionID)
	auth := authHeader(t, viewer.EmployeeID)

	testCases := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "valid id", path: "/api/employees/" + itoa(employee.EmployeeID), wantStatus: http.StatusOK},
		{name: "missing employee", path: "/api/employees/999", wantStatus: http.StatusNotFound},
		{name: "invalid id format", path: "/api/employees/abc", wantStatus: http.StatusBadRequest},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodGet, tc.path, nil, auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusOK {
				response := decodeResponse[employeeResponse](t, recorder)
				assert.Equal(t, employee.Email, response.Email)
				assert.Equal(t, employee.EmployeeID, response.ID)
			}
		})
	}
}

func TestUpdateEmployee(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)
	otherPosition := seedPosition(t, db)
	updater := seedEmployeeWithPermissions(t, db, position.PositionID, commonpermission.EmployeeUpdate)
	employee := seedEmployee(t, db, position.PositionID)
	otherEmployee := seedEmployee(t, db, position.PositionID)
	auth := authHeader(t, updater.EmployeeID)

	newDepartment := "Operations"
	newPositionID := otherPosition.PositionID

	testCases := []struct {
		name       string
		body       map[string]any
		wantStatus int
	}{
		{
			name: "valid partial update",
			body: map[string]any{
				"department":  newDepartment,
				"position_id": newPositionID,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "duplicate email",
			body: map[string]any{
				"email": otherEmployee.Email,
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "invalid position id",
			body: map[string]any{
				"position_id": uint(9999),
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodPatch, "/api/employees/"+itoa(employee.EmployeeID), tc.body, auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusOK {
				response := decodeResponse[employeeResponse](t, recorder)
				assert.Equal(t, newDepartment, response.Department)
				assert.Equal(t, newPositionID, response.PositionID)
			}
		})
	}
}

func TestActivateAccount(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)
	employee := seedEmployee(t, db, position.PositionID)
	employee.Active = false
	employee.Password = ""
	require.NoError(t, db.Save(employee).Error)

	validToken := &model.ActivationToken{
		EmployeeID: employee.EmployeeID,
		Token:      uniqueValue(t, "activation-token"),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
	}
	expiredToken := &model.ActivationToken{
		EmployeeID: employee.EmployeeID,
		Token:      uniqueValue(t, "activation-token"),
		ExpiresAt:  time.Now().Add(-30 * time.Minute),
	}
	require.NoError(t, db.Create(validToken).Error)
	require.NoError(t, db.Create(expiredToken).Error)

	testCases := []struct {
		name       string
		body       map[string]any
		wantStatus int
	}{
		{
			name: "valid token and password",
			body: map[string]any{
				"token":    validToken.Token,
				"password": "Password12",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "expired token",
			body: map[string]any{
				"token":    expiredToken.Token,
				"password": "Password12",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "weak password",
			body: map[string]any{
				"token":    validToken.Token,
				"password": "weak",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodPost, "/api/employees/activate", tc.body, "")
			requireStatus(t, recorder, tc.wantStatus)
		})
	}

	refreshedEmployee := &model.Employee{}
	require.NoError(t, db.First(refreshedEmployee, employee.EmployeeID).Error)
	assert.NotEmpty(t, refreshedEmployee.Password)
}

func TestPasswordResetFlow(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)
	employee := seedEmployee(t, db, position.PositionID)

	forgotPassword := performRequest(t, router, http.MethodPost, "/api/employees/forgot-password", map[string]any{
		"email": employee.Email,
	}, "")
	requireStatus(t, forgotPassword, http.StatusOK)

	var resetToken model.ResetToken
	require.NoError(t, db.WithContext(context.Background()).Where("employee_id = ?", employee.EmployeeID).First(&resetToken).Error)

	resetPassword := performRequest(t, router, http.MethodPost, "/api/employees/reset-password", map[string]any{
		"token":        resetToken.Token,
		"new_password": "NewPassword12",
	}, "")
	requireStatus(t, resetPassword, http.StatusOK)

	login := performRequest(t, router, http.MethodPost, "/api/employees/login", map[string]any{
		"email":    employee.Email,
		"password": "NewPassword12",
	}, "")
	requireStatus(t, login, http.StatusOK)
}

func TestRefreshToken(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)
	employee := seedEmployee(t, db, position.PositionID)

	login := loginEmployee(t, router, employee.Email, "Password12")

	validRefresh := performRequest(t, router, http.MethodPost, "/api/employees/refresh", map[string]any{
		"refresh_token": login.RefreshToken,
	}, "")
	requireStatus(t, validRefresh, http.StatusOK)

	refreshResponse := decodeResponse[loginResponse](t, validRefresh)
	assert.NotEmpty(t, refreshResponse.Token)
	assert.NotEmpty(t, refreshResponse.RefreshToken)
	assert.NotEqual(t, login.RefreshToken, refreshResponse.RefreshToken)

	invalidRefresh := performRequest(t, router, http.MethodPost, "/api/employees/refresh", map[string]any{
		"refresh_token": "invalid-token",
	}, "")
	requireStatus(t, invalidRefresh, http.StatusUnauthorized)
}

func itoa(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}
