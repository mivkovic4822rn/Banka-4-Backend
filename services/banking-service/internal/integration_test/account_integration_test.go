//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"banking-service/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAccount(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	seedCurrency(t, db, model.RSD)
	seedCurrency(t, db, model.EUR)

	employeeAuth := authHeaderForEmployee(t, 1, 1)
	clientAuth := authHeaderForClient(t, 10, 100)

	testCases := []struct {
		name       string
		body       map[string]any
		auth       string
		wantStatus int
	}{
		{
			name: "happy path current personal account",
			body: map[string]any{
				"name":         uniqueValue(t, "acct"),
				"client_id":    100,
				"employee_id":  1,
				"account_type": "Personal",
				"account_kind": "Current",
				"subtype":      "Standard",
				"expires_at":   time.Now().AddDate(5, 0, 0).Format(time.RFC3339),
			},
			auth:       employeeAuth,
			wantStatus: http.StatusCreated,
		},
		{
			name: "happy path foreign personal account",
			body: map[string]any{
				"name":          uniqueValue(t, "acct"),
				"client_id":     100,
				"employee_id":   1,
				"account_type":  "Personal",
				"account_kind":  "Foreign",
				"currency_code": "EUR",
				"expires_at":    time.Now().AddDate(5, 0, 0).Format(time.RFC3339),
			},
			auth:       employeeAuth,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "missing auth header",
			body:       map[string]any{"name": "test"},
			auth:       "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "client cannot create account",
			body: map[string]any{
				"name":         uniqueValue(t, "acct"),
				"client_id":    100,
				"employee_id":  1,
				"account_type": "Personal",
				"account_kind": "Current",
				"subtype":      "Standard",
				"expires_at":   time.Now().AddDate(5, 0, 0).Format(time.RFC3339),
			},
			auth:       clientAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name: "missing required fields",
			body: map[string]any{
				"name": "incomplete",
			},
			auth:       employeeAuth,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "current account without subtype",
			body: map[string]any{
				"name":         uniqueValue(t, "acct"),
				"client_id":    100,
				"employee_id":  1,
				"account_type": "Personal",
				"account_kind": "Current",
				"expires_at":   time.Now().AddDate(5, 0, 0).Format(time.RFC3339),
			},
			auth:       employeeAuth,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodPost, "/api/accounts", tc.body, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusCreated {
				resp := decodeResponse[map[string]any](t, recorder)
				assert.NotEmpty(t, resp["account_number"])
			}
		})
	}
}

func TestListAccounts(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	seedAccount(t, db, 100, rsd.CurrencyID, 5000)
	seedAccount(t, db, 200, rsd.CurrencyID, 3000)

	employeeAuth := authHeaderForEmployee(t, 1, 1)
	clientAuth := authHeaderForClient(t, 10, 100)

	testCases := []struct {
		name       string
		path       string
		auth       string
		wantStatus int
	}{
		{
			name:       "employee can list all accounts",
			path:       "/api/accounts?page=1&page_size=10",
			auth:       employeeAuth,
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing auth header",
			path:       "/api/accounts",
			auth:       "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "client cannot list all accounts",
			path:       "/api/accounts",
			auth:       clientAuth,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodGet, tc.path, nil, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusOK {
				resp := decodeResponse[map[string]any](t, recorder)
				data, ok := resp["data"].([]any)
				require.True(t, ok)
				assert.GreaterOrEqual(t, len(data), 2)
			}
		})
	}
}

func TestGetClientAccounts(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	seedAccount(t, db, 100, rsd.CurrencyID, 5000)
	seedAccount(t, db, 100, rsd.CurrencyID, 3000)

	clientAuth := authHeaderForClient(t, 10, 100)
	otherClientAuth := authHeaderForClient(t, 20, 200)
	employeeAuth := authHeaderForEmployee(t, 1, 1)

	testCases := []struct {
		name       string
		path       string
		auth       string
		wantStatus int
	}{
		{
			name:       "client gets own accounts",
			path:       "/api/clients/100/accounts",
			auth:       clientAuth,
			wantStatus: http.StatusOK,
		},
		{
			name:       "employee can view client accounts",
			path:       "/api/clients/100/accounts",
			auth:       employeeAuth,
			wantStatus: http.StatusOK,
		},
		{
			name:       "client cannot view other clients accounts",
			path:       "/api/clients/100/accounts",
			auth:       otherClientAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "missing auth header",
			path:       "/api/clients/100/accounts",
			auth:       "",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodGet, tc.path, nil, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusOK {
				resp := decodeResponse[[]map[string]any](t, recorder)
				assert.Len(t, resp, 2)
			}
		})
	}
}

func TestGetAccountDetails(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	account := seedAccount(t, db, 100, rsd.CurrencyID, 7500)

	clientAuth := authHeaderForClient(t, 10, 100)
	otherClientAuth := authHeaderForClient(t, 20, 200)

	testCases := []struct {
		name       string
		path       string
		auth       string
		wantStatus int
	}{
		{
			name:       "happy path",
			path:       fmt.Sprintf("/api/clients/100/accounts/%s", account.AccountNumber),
			auth:       clientAuth,
			wantStatus: http.StatusOK,
		},
		{
			name:       "wrong client id",
			path:       fmt.Sprintf("/api/clients/100/accounts/%s", account.AccountNumber),
			auth:       otherClientAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "account not found",
			path:       "/api/clients/100/accounts/999999999999999999",
			auth:       clientAuth,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing auth",
			path:       fmt.Sprintf("/api/clients/100/accounts/%s", account.AccountNumber),
			auth:       "",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodGet, tc.path, nil, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusOK {
				resp := decodeResponse[map[string]any](t, recorder)
				assert.Equal(t, account.AccountNumber, resp["account_number"])
				assert.Equal(t, 7500.0, resp["balance"])
			}
		})
	}
}

func TestUpdateAccountName(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	account := seedAccount(t, db, 100, rsd.CurrencyID, 1000)

	clientAuth := authHeaderForClient(t, 10, 100)
	otherClientAuth := authHeaderForClient(t, 20, 200)

	newName := uniqueValue(t, "newname")

	testCases := []struct {
		name       string
		path       string
		body       map[string]any
		auth       string
		wantStatus int
	}{
		{
			name:       "happy path",
			path:       fmt.Sprintf("/api/clients/100/accounts/%s/name", account.AccountNumber),
			body:       map[string]any{"name": newName},
			auth:       clientAuth,
			wantStatus: http.StatusOK,
		},
		{
			name:       "same name as current",
			path:       fmt.Sprintf("/api/clients/100/accounts/%s/name", account.AccountNumber),
			body:       map[string]any{"name": newName},
			auth:       clientAuth,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong client",
			path:       fmt.Sprintf("/api/clients/100/accounts/%s/name", account.AccountNumber),
			body:       map[string]any{"name": "other"},
			auth:       otherClientAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "missing name field",
			path:       fmt.Sprintf("/api/clients/100/accounts/%s/name", account.AccountNumber),
			body:       map[string]any{},
			auth:       clientAuth,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "account not found",
			path:       "/api/clients/100/accounts/000000000000000000/name",
			body:       map[string]any{"name": "x"},
			auth:       clientAuth,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodPut, tc.path, tc.body, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)
		})
	}

	var updated model.Account
	require.NoError(t, db.Where("account_number = ?", account.AccountNumber).First(&updated).Error)
	assert.Equal(t, newName, updated.Name)
}

func TestRequestAndConfirmLimitsChange(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	account := seedAccount(t, db, 100, rsd.CurrencyID, 1000)

	clientAuth := authHeaderForClient(t, 10, 100)

	reqPath := fmt.Sprintf("/api/clients/100/accounts/%s/limits/request", account.AccountNumber)
	recorder := performRequest(t, router, http.MethodPost, reqPath, map[string]any{
		"daily_limit":   500000.0,
		"monthly_limit": 2000000.0,
	}, clientAuth)
	requireStatus(t, recorder, http.StatusOK)

	var token model.VerificationToken
	require.NoError(t, db.Where("account_number = ? AND client_id = ?", account.AccountNumber, 100).First(&token).Error)
	assert.Equal(t, 500000.0, token.NewDailyLimit)

	confirmPath := fmt.Sprintf("/api/clients/100/accounts/%s/limits", account.AccountNumber)
	code := generateTOTPCode(t)
	recorder = performRequest(t, router, http.MethodPut, confirmPath, map[string]any{
		"code": code,
	}, clientAuth)
	requireStatus(t, recorder, http.StatusOK)

	var updated model.Account
	require.NoError(t, db.Where("account_number = ?", account.AccountNumber).First(&updated).Error)
	assert.Equal(t, 500000.0, updated.DailyLimit)
	assert.Equal(t, 2000000.0, updated.MonthlyLimit)
}
