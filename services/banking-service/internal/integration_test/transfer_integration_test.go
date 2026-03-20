//go:build integration

package integration_test

import (
	"net/http"
	"testing"

	"banking-service/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteTransfer(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	seedBankAccounts(t, db, rsd.CurrencyID)
	from := seedAccount(t, db, 100, rsd.CurrencyID, 50000)
	to := seedAccount(t, db, 100, rsd.CurrencyID, 10000)

	clientAuth := authHeaderForClient(t, 10, 100)

	recorder := performRequest(t, router, http.MethodPost, "/api/clients/100/transfers", map[string]any{
		"from_account": from.AccountNumber,
		"to_account":   to.AccountNumber,
		"amount":       5000.0,
	}, clientAuth)
	requireStatus(t, recorder, http.StatusCreated)

	resp := decodeResponse[map[string]any](t, recorder)
	assert.NotZero(t, resp["transfer_id"])
	assert.NotZero(t, resp["transaction_id"])

	var fromAccount model.Account
	require.NoError(t, db.Where("account_number = ?", from.AccountNumber).First(&fromAccount).Error)
	assert.Equal(t, 45000.0, fromAccount.Balance)

	var toAccount model.Account
	require.NoError(t, db.Where("account_number = ?", to.AccountNumber).First(&toAccount).Error)
	assert.Equal(t, 15000.0, toAccount.Balance)
}

func TestExecuteTransferValidation(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	seedBankAccounts(t, db, rsd.CurrencyID)
	from := seedAccount(t, db, 100, rsd.CurrencyID, 50000)
	to := seedAccount(t, db, 100, rsd.CurrencyID, 10000)
	otherAccount := seedAccount(t, db, 200, rsd.CurrencyID, 5000)

	clientAuth := authHeaderForClient(t, 10, 100)
	otherClientAuth := authHeaderForClient(t, 20, 200)

	testCases := []struct {
		name       string
		body       map[string]any
		auth       string
		wantStatus int
	}{
		{
			name: "same source and destination",
			body: map[string]any{
				"from_account": from.AccountNumber,
				"to_account":   from.AccountNumber,
				"amount":       100.0,
			},
			auth:       clientAuth,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "insufficient funds",
			body: map[string]any{
				"from_account": from.AccountNumber,
				"to_account":   to.AccountNumber,
				"amount":       999999999.0,
			},
			auth:       clientAuth,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "zero amount",
			body: map[string]any{
				"from_account": from.AccountNumber,
				"to_account":   to.AccountNumber,
				"amount":       0,
			},
			auth:       clientAuth,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "account not owned by client",
			body: map[string]any{
				"from_account": otherAccount.AccountNumber,
				"to_account":   to.AccountNumber,
				"amount":       100.0,
			},
			auth:       clientAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name: "destination not owned by client",
			body: map[string]any{
				"from_account": from.AccountNumber,
				"to_account":   otherAccount.AccountNumber,
				"amount":       100.0,
			},
			auth:       clientAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "missing auth",
			body:       map[string]any{},
			auth:       "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "wrong client id in path",
			body: map[string]any{
				"from_account": from.AccountNumber,
				"to_account":   to.AccountNumber,
				"amount":       100.0,
			},
			auth:       otherClientAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name: "missing required fields",
			body: map[string]any{
				"amount": 100.0,
			},
			auth:       clientAuth,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodPost, "/api/clients/100/transfers", tc.body, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)
		})
	}
}

func TestGetTransferHistory(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	seedBankAccounts(t, db, rsd.CurrencyID)
	from := seedAccount(t, db, 100, rsd.CurrencyID, 50000)
	to := seedAccount(t, db, 100, rsd.CurrencyID, 10000)

	clientAuth := authHeaderForClient(t, 10, 100)
	otherClientAuth := authHeaderForClient(t, 20, 200)

	performRequest(t, router, http.MethodPost, "/api/clients/100/transfers", map[string]any{
		"from_account": from.AccountNumber,
		"to_account":   to.AccountNumber,
		"amount":       1000.0,
	}, clientAuth)

	performRequest(t, router, http.MethodPost, "/api/clients/100/transfers", map[string]any{
		"from_account": from.AccountNumber,
		"to_account":   to.AccountNumber,
		"amount":       2000.0,
	}, clientAuth)

	testCases := []struct {
		name       string
		path       string
		auth       string
		wantStatus int
	}{
		{
			name:       "happy path",
			path:       "/api/clients/100/transfers?page=1&page_size=10",
			auth:       clientAuth,
			wantStatus: http.StatusOK,
		},
		{
			name:       "other client forbidden",
			path:       "/api/clients/100/transfers",
			auth:       otherClientAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "missing auth",
			path:       "/api/clients/100/transfers",
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
				data, ok := resp["data"].([]any)
				require.True(t, ok)
				assert.GreaterOrEqual(t, len(data), 2)
				assert.NotZero(t, resp["total"])
			}
		})
	}
}
