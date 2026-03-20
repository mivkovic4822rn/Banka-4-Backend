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

func TestCreatePayee(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	clientAuth := authHeaderForClient(t, 10, 100)
	employeeAuth := authHeaderForEmployee(t, 1, 1)

	testCases := []struct {
		name       string
		body       map[string]any
		auth       string
		wantStatus int
	}{
		{
			name: "happy path",
			body: map[string]any{
				"name":           "Test Payee",
				"account_number": "111000000000000001",
			},
			auth:       clientAuth,
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing name",
			body: map[string]any{
				"account_number": "111000000000000002",
			},
			auth:       clientAuth,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing account number",
			body: map[string]any{
				"name": "Missing Account",
			},
			auth:       clientAuth,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing auth",
			body:       map[string]any{"name": "test", "account_number": "111"},
			auth:       "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "employee cannot create payee",
			body: map[string]any{
				"name":           "Employee Payee",
				"account_number": "111000000000000003",
			},
			auth:       employeeAuth,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodPost, "/api/payees", tc.body, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusCreated {
				resp := decodeResponse[map[string]any](t, recorder)
				assert.NotZero(t, resp["payee_id"])
				assert.Equal(t, "Test Payee", resp["name"])
			}
		})
	}
}

func TestGetAllPayees(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	clientAuth := authHeaderForClient(t, 10, 100)

	performRequest(t, router, http.MethodPost, "/api/payees", map[string]any{
		"name":           "Payee 1",
		"account_number": "111000000000000001",
	}, clientAuth)

	performRequest(t, router, http.MethodPost, "/api/payees", map[string]any{
		"name":           "Payee 2",
		"account_number": "111000000000000002",
	}, clientAuth)

	recorder := performRequest(t, router, http.MethodGet, "/api/payees", nil, clientAuth)
	requireStatus(t, recorder, http.StatusOK)

	resp := decodeResponse[[]map[string]any](t, recorder)
	assert.GreaterOrEqual(t, len(resp), 2)

	otherClientAuth := authHeaderForClient(t, 20, 200)
	recorder = performRequest(t, router, http.MethodGet, "/api/payees", nil, otherClientAuth)
	requireStatus(t, recorder, http.StatusOK)

	resp = decodeResponse[[]map[string]any](t, recorder)
	assert.Empty(t, resp)
}

func TestUpdatePayee(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	clientAuth := authHeaderForClient(t, 10, 100)
	otherClientAuth := authHeaderForClient(t, 20, 200)

	createResp := performRequest(t, router, http.MethodPost, "/api/payees", map[string]any{
		"name":           "Original Name",
		"account_number": "111000000000000001",
	}, clientAuth)
	requireStatus(t, createResp, http.StatusCreated)

	resp := decodeResponse[map[string]any](t, createResp)
	payeeID := uint(resp["payee_id"].(float64))

	testCases := []struct {
		name       string
		payeeID    uint
		body       map[string]any
		auth       string
		wantStatus int
	}{
		{
			name:       "update name",
			payeeID:    payeeID,
			body:       map[string]any{"name": "Updated Name"},
			auth:       clientAuth,
			wantStatus: http.StatusOK,
		},
		{
			name:       "update account number",
			payeeID:    payeeID,
			body:       map[string]any{"account_number": "222000000000000001"},
			auth:       clientAuth,
			wantStatus: http.StatusOK,
		},
		{
			name:       "other client forbidden",
			payeeID:    payeeID,
			body:       map[string]any{"name": "Hacked"},
			auth:       otherClientAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "payee not found",
			payeeID:    999999,
			body:       map[string]any{"name": "No such"},
			auth:       clientAuth,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing auth",
			payeeID:    payeeID,
			body:       map[string]any{"name": "x"},
			auth:       "",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/payees/%d", tc.payeeID)
			recorder := performRequest(t, router, http.MethodPatch, path, tc.body, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)
		})
	}

	var payee model.Payee
	require.NoError(t, db.First(&payee, payeeID).Error)
	assert.Equal(t, "Updated Name", payee.Name)
	assert.Equal(t, "222000000000000001", payee.AccountNumber)
}

func TestDeletePayee(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	clientAuth := authHeaderForClient(t, 10, 100)
	otherClientAuth := authHeaderForClient(t, 20, 200)

	createResp := performRequest(t, router, http.MethodPost, "/api/payees", map[string]any{
		"name":           "To Delete",
		"account_number": "111000000000000001",
	}, clientAuth)
	requireStatus(t, createResp, http.StatusCreated)

	resp := decodeResponse[map[string]any](t, createResp)
	payeeID := uint(resp["payee_id"].(float64))

	deletePath := fmt.Sprintf("/api/payees/%d", payeeID)

	recorder := performRequest(t, router, http.MethodDelete, deletePath, nil, otherClientAuth)
	requireStatus(t, recorder, http.StatusForbidden)

	recorder = performRequest(t, router, http.MethodDelete, deletePath, nil, clientAuth)
	requireStatus(t, recorder, http.StatusNoContent)

	var count int64
	db.Model(&model.Payee{}).Where("payee_id = ?", payeeID).Count(&count)
	assert.Zero(t, count)

	recorder = performRequest(t, router, http.MethodDelete, deletePath, nil, clientAuth)
	requireStatus(t, recorder, http.StatusNotFound)
}
