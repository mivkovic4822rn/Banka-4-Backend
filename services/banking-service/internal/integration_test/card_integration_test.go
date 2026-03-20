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

func TestRequestAndConfirmCard(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	account := seedAccount(t, db, 100, rsd.CurrencyID, 5000)

	clientAuth := authHeaderForClient(t, 10, 100)

	recorder := performRequest(t, router, http.MethodPost, "/api/cards/request", map[string]any{
		"account_number": account.AccountNumber,
	}, clientAuth)
	requireStatus(t, recorder, http.StatusOK)

	var cardRequest model.CardRequest
	require.NoError(t, db.Where("account_number = ?", account.AccountNumber).
		Order("created_at DESC").First(&cardRequest).Error)
	assert.False(t, cardRequest.Used)

	recorder = performRequest(t, router, http.MethodPost, "/api/cards/request/confirm", map[string]any{
		"account_number":    account.AccountNumber,
		"confirmation_code": cardRequest.ConfirmationCode,
	}, clientAuth)
	requireStatus(t, recorder, http.StatusCreated)

	resp := decodeResponse[map[string]any](t, recorder)
	assert.NotEmpty(t, resp["card_number"])
	assert.Equal(t, "Active", resp["status"])

	var updatedRequest model.CardRequest
	require.NoError(t, db.First(&updatedRequest, cardRequest.CardRequestID).Error)
	assert.True(t, updatedRequest.Used)

	var cardCount int64
	db.Model(&model.Card{}).Where("account_number = ?", account.AccountNumber).Count(&cardCount)
	assert.Equal(t, int64(1), cardCount)
}

func TestRequestCardValidation(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	account := seedAccount(t, db, 100, rsd.CurrencyID, 5000)

	clientAuth := authHeaderForClient(t, 10, 100)
	otherClientAuth := authHeaderForClient(t, 20, 200)
	employeeAuth := authHeaderForEmployee(t, 1, 1)

	testCases := []struct {
		name       string
		body       map[string]any
		auth       string
		wantStatus int
	}{
		{
			name:       "missing account number",
			body:       map[string]any{},
			auth:       clientAuth,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "account not owned by client",
			body:       map[string]any{"account_number": account.AccountNumber},
			auth:       otherClientAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "employee cannot request card",
			body:       map[string]any{"account_number": account.AccountNumber},
			auth:       employeeAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "missing auth",
			body:       map[string]any{"account_number": account.AccountNumber},
			auth:       "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "account not found",
			body:       map[string]any{"account_number": "000000000000000000"},
			auth:       clientAuth,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodPost, "/api/cards/request", tc.body, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)
		})
	}
}

func TestMaxPersonalCards(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	account := seedAccount(t, db, 100, rsd.CurrencyID, 5000)

	seedCard(t, db, account.AccountNumber)
	seedCard(t, db, account.AccountNumber)

	clientAuth := authHeaderForClient(t, 10, 100)

	recorder := performRequest(t, router, http.MethodPost, "/api/cards/request", map[string]any{
		"account_number": account.AccountNumber,
	}, clientAuth)
	requireStatus(t, recorder, http.StatusConflict)
}

func TestListCardsByAccount(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	account := seedAccount(t, db, 100, rsd.CurrencyID, 5000)
	seedCard(t, db, account.AccountNumber)

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
			name:       "client sees own cards",
			path:       fmt.Sprintf("/api/clients/100/accounts/%s/cards", account.AccountNumber),
			auth:       clientAuth,
			wantStatus: http.StatusOK,
		},
		{
			name:       "employee sees cards",
			path:       fmt.Sprintf("/api/clients/100/accounts/%s/cards", account.AccountNumber),
			auth:       employeeAuth,
			wantStatus: http.StatusOK,
		},
		{
			name:       "other client forbidden",
			path:       fmt.Sprintf("/api/clients/100/accounts/%s/cards", account.AccountNumber),
			auth:       otherClientAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "missing auth",
			path:       fmt.Sprintf("/api/clients/100/accounts/%s/cards", account.AccountNumber),
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
				cards, ok := resp["cards"].([]any)
				require.True(t, ok)
				assert.GreaterOrEqual(t, len(cards), 1)
			}
		})
	}
}

func TestBlockUnblockDeactivateCard(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	account := seedAccount(t, db, 100, rsd.CurrencyID, 5000)
	card := seedCard(t, db, account.AccountNumber)

	clientAuth := authHeaderForClient(t, 10, 100)
	employeeAuth := authHeaderForEmployee(t, 1, 1)

	blockPath := fmt.Sprintf("/api/cards/%d/block", card.CardID)
	recorder := performRequest(t, router, http.MethodPut, blockPath, nil, clientAuth)
	requireStatus(t, recorder, http.StatusOK)

	resp := decodeResponse[map[string]any](t, recorder)
	assert.Equal(t, "Blocked", resp["status"])

	recorder = performRequest(t, router, http.MethodPut, blockPath, nil, clientAuth)
	requireStatus(t, recorder, http.StatusBadRequest)

	unblockPath := fmt.Sprintf("/api/cards/%d/unblock", card.CardID)
	recorder = performRequest(t, router, http.MethodPut, unblockPath, nil, employeeAuth)
	requireStatus(t, recorder, http.StatusOK)

	resp = decodeResponse[map[string]any](t, recorder)
	assert.Equal(t, "Active", resp["status"])

	deactivatePath := fmt.Sprintf("/api/cards/%d/deactivate", card.CardID)
	recorder = performRequest(t, router, http.MethodPut, deactivatePath, nil, employeeAuth)
	requireStatus(t, recorder, http.StatusOK)

	resp = decodeResponse[map[string]any](t, recorder)
	assert.Equal(t, "Deactivated", resp["status"])

	recorder = performRequest(t, router, http.MethodPut, blockPath, nil, employeeAuth)
	requireStatus(t, recorder, http.StatusBadRequest)

	recorder = performRequest(t, router, http.MethodPut, unblockPath, nil, employeeAuth)
	requireStatus(t, recorder, http.StatusBadRequest)

	recorder = performRequest(t, router, http.MethodPut, deactivatePath, nil, employeeAuth)
	requireStatus(t, recorder, http.StatusBadRequest)
}

func TestCardNotFound(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	employeeAuth := authHeaderForEmployee(t, 1, 1)

	recorder := performRequest(t, router, http.MethodPut, "/api/cards/999999/block", nil, employeeAuth)
	requireStatus(t, recorder, http.StatusNotFound)

	recorder = performRequest(t, router, http.MethodPut, "/api/cards/999999/unblock", nil, employeeAuth)
	requireStatus(t, recorder, http.StatusNotFound)

	recorder = performRequest(t, router, http.MethodPut, "/api/cards/999999/deactivate", nil, employeeAuth)
	requireStatus(t, recorder, http.StatusNotFound)
}

func TestUnblockRequiresEmployee(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	account := seedAccount(t, db, 100, rsd.CurrencyID, 5000)
	card := seedCard(t, db, account.AccountNumber)

	clientAuth := authHeaderForClient(t, 10, 100)
	employeeAuth := authHeaderForEmployee(t, 1, 1)

	blockPath := fmt.Sprintf("/api/cards/%d/block", card.CardID)
	performRequest(t, router, http.MethodPut, blockPath, nil, employeeAuth)

	unblockPath := fmt.Sprintf("/api/cards/%d/unblock", card.CardID)
	recorder := performRequest(t, router, http.MethodPut, unblockPath, nil, clientAuth)
	requireStatus(t, recorder, http.StatusForbidden)
}

func TestDeactivateRequiresEmployee(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	rsd := seedCurrency(t, db, model.RSD)
	account := seedAccount(t, db, 100, rsd.CurrencyID, 5000)
	card := seedCard(t, db, account.AccountNumber)

	clientAuth := authHeaderForClient(t, 10, 100)

	deactivatePath := fmt.Sprintf("/api/cards/%d/deactivate", card.CardID)
	recorder := performRequest(t, router, http.MethodPut, deactivatePath, nil, clientAuth)
	requireStatus(t, recorder, http.StatusForbidden)
}
