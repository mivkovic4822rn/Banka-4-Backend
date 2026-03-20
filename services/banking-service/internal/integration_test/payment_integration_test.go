//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"banking-service/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePayment(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	payer := seedAccount(t, db, 100, rsd.CurrencyID, 50000)
	recipient := seedAccount(t, db, 200, rsd.CurrencyID, 10000)

	clientAuth := authHeaderForClient(t, 10, 100)
	otherClientAuth := authHeaderForClient(t, 20, 200)

	testCases := []struct {
		name       string
		body       map[string]any
		auth       string
		wantStatus int
	}{
		{
			name: "happy path",
			body: map[string]any{
				"recipient_name":           "Test Recipient",
				"recipient_account_number": recipient.AccountNumber,
				"amount":                   1000.0,
				"payer_account_number":     payer.AccountNumber,
				"reference_number":         "REF001",
				"payment_code":             "289",
				"purpose":                  "Test payment",
			},
			auth:       clientAuth,
			wantStatus: http.StatusOK,
		},
		{
			name: "insufficient funds",
			body: map[string]any{
				"recipient_name":           "Test Recipient",
				"recipient_account_number": recipient.AccountNumber,
				"amount":                   999999999.0,
				"payer_account_number":     payer.AccountNumber,
			},
			auth:       clientAuth,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "same client accounts",
			body: map[string]any{
				"recipient_name":           "Self",
				"recipient_account_number": payer.AccountNumber,
				"amount":                   100.0,
				"payer_account_number":     payer.AccountNumber,
			},
			auth:       clientAuth,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing required fields",
			body: map[string]any{
				"amount": 100.0,
			},
			auth:       clientAuth,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing auth",
			body:       map[string]any{},
			auth:       "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "wrong client",
			body: map[string]any{
				"recipient_name":           "Test",
				"recipient_account_number": recipient.AccountNumber,
				"amount":                   100.0,
				"payer_account_number":     payer.AccountNumber,
			},
			auth:       otherClientAuth,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodPost, "/api/clients/100/payments", tc.body, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusOK {
				resp := decodeResponse[map[string]any](t, recorder)
				assert.NotZero(t, resp["id"])
			}
		})
	}
}

func TestVerifyPayment(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	seedBankAccounts(t, db, rsd.CurrencyID)
	payer := seedAccount(t, db, 100, rsd.CurrencyID, 50000)
	recipient := seedAccount(t, db, 200, rsd.CurrencyID, 10000)

	clientAuth := authHeaderForClient(t, 10, 100)

	createResp := performRequest(t, router, http.MethodPost, "/api/clients/100/payments", map[string]any{
		"recipient_name":           "Verify Test",
		"recipient_account_number": recipient.AccountNumber,
		"amount":                   1000.0,
		"payer_account_number":     payer.AccountNumber,
		"reference_number":         "REF002",
		"payment_code":             "289",
		"purpose":                  "Verify test",
	}, clientAuth)
	requireStatus(t, createResp, http.StatusOK)

	paymentResp := decodeResponse[map[string]any](t, createResp)
	paymentID := uint(paymentResp["id"].(float64))

	code := generateTOTPCode(t)
	verifyPath := fmt.Sprintf("/api/clients/100/payments/%d/verify", paymentID)
	recorder := performRequest(t, router, http.MethodPost, verifyPath, map[string]any{
		"code": code,
	}, clientAuth)
	requireStatus(t, recorder, http.StatusOK)

	var payerAccount model.Account
	require.NoError(t, db.Where("account_number = ?", payer.AccountNumber).First(&payerAccount).Error)
	assert.Equal(t, 49000.0, payerAccount.Balance)

	var recipientAccount model.Account
	require.NoError(t, db.Where("account_number = ?", recipient.AccountNumber).First(&recipientAccount).Error)
	assert.Equal(t, 11000.0, recipientAccount.Balance)

	var tx model.Transaction
	require.NoError(t, db.Where("payer_account_number = ? AND recipient_account_number = ?",
		payer.AccountNumber, recipient.AccountNumber).First(&tx).Error)
	assert.Equal(t, model.TransactionCompleted, tx.Status)
}

func TestGetPaymentByID(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	payer := seedAccount(t, db, 100, rsd.CurrencyID, 50000)
	recipient := seedAccount(t, db, 200, rsd.CurrencyID, 10000)

	clientAuth := authHeaderForClient(t, 10, 100)

	createResp := performRequest(t, router, http.MethodPost, "/api/clients/100/payments", map[string]any{
		"recipient_name":           "Get Test",
		"recipient_account_number": recipient.AccountNumber,
		"amount":                   500.0,
		"payer_account_number":     payer.AccountNumber,
	}, clientAuth)
	requireStatus(t, createResp, http.StatusOK)

	paymentResp := decodeResponse[map[string]any](t, createResp)
	paymentID := uint(paymentResp["id"].(float64))

	testCases := []struct {
		name       string
		path       string
		auth       string
		wantStatus int
	}{
		{
			name:       "happy path",
			path:       fmt.Sprintf("/api/clients/100/payments/%d", paymentID),
			auth:       clientAuth,
			wantStatus: http.StatusOK,
		},
		{
			name:       "payment not found",
			path:       "/api/clients/100/payments/999999",
			auth:       clientAuth,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing auth",
			path:       fmt.Sprintf("/api/clients/100/payments/%d", paymentID),
			auth:       "",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodGet, tc.path, nil, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)
		})
	}
}

func TestGetAccountPayments(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	payer := seedAccount(t, db, 100, rsd.CurrencyID, 50000)
	recipient := seedAccount(t, db, 200, rsd.CurrencyID, 10000)

	clientAuth := authHeaderForClient(t, 10, 100)

	performRequest(t, router, http.MethodPost, "/api/clients/100/payments", map[string]any{
		"recipient_name":           "ListTest1",
		"recipient_account_number": recipient.AccountNumber,
		"amount":                   100.0,
		"payer_account_number":     payer.AccountNumber,
	}, clientAuth)

	performRequest(t, router, http.MethodPost, "/api/clients/100/payments", map[string]any{
		"recipient_name":           "ListTest2",
		"recipient_account_number": recipient.AccountNumber,
		"amount":                   200.0,
		"payer_account_number":     payer.AccountNumber,
	}, clientAuth)

	path := fmt.Sprintf("/api/clients/100/accounts/%s/payments?page=1&page_size=10", payer.AccountNumber)
	recorder := performRequest(t, router, http.MethodGet, path, nil, clientAuth)
	requireStatus(t, recorder, http.StatusOK)

	resp := decodeResponse[map[string]any](t, recorder)
	data, ok := resp["data"].([]any)
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(data), 2)
	assert.NotZero(t, resp["total"])
}

func TestGetClientPayments(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	payer := seedAccount(t, db, 100, rsd.CurrencyID, 50000)
	recipient := seedAccount(t, db, 200, rsd.CurrencyID, 10000)

	clientAuth := authHeaderForClient(t, 10, 100)

	performRequest(t, router, http.MethodPost, "/api/clients/100/payments", map[string]any{
		"recipient_name":           "ClientPay1",
		"recipient_account_number": recipient.AccountNumber,
		"amount":                   300.0,
		"payer_account_number":     payer.AccountNumber,
	}, clientAuth)

	recorder := performRequest(t, router, http.MethodGet, "/api/clients/100/payments?page=1&page_size=10", nil, clientAuth)
	requireStatus(t, recorder, http.StatusOK)

	resp := decodeResponse[map[string]any](t, recorder)
	data, ok := resp["data"].([]any)
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(data), 1)
}

func TestGetReceipt(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	payer := seedAccount(t, db, 100, rsd.CurrencyID, 50000)
	recipient := seedAccount(t, db, 200, rsd.CurrencyID, 10000)

	clientAuth := authHeaderForClient(t, 10, 100)

	createResp := performRequest(t, router, http.MethodPost, "/api/clients/100/payments", map[string]any{
		"recipient_name":           "Receipt Test",
		"recipient_account_number": recipient.AccountNumber,
		"amount":                   750.0,
		"payer_account_number":     payer.AccountNumber,
		"purpose":                  "Receipt test",
	}, clientAuth)
	requireStatus(t, createResp, http.StatusOK)

	paymentResp := decodeResponse[map[string]any](t, createResp)
	paymentID := uint(paymentResp["id"].(float64))

	receiptPath := fmt.Sprintf("/api/clients/100/payments/%d/receipt", paymentID)
	recorder := performRequest(t, router, http.MethodGet, receiptPath, nil, clientAuth)
	requireStatus(t, recorder, http.StatusOK)
	assert.Equal(t, "application/pdf", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Header().Get("Content-Disposition"), "receipt")
}
