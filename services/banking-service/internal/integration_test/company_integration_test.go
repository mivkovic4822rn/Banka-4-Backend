//go:build integration

package integration_test

import (
	"net/http"
	"testing"

	"banking-service/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestCreateCompany(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	wc := seedWorkCode(t, db)
	employeeAuth := authHeaderForEmployee(t, 1, 1)
	clientAuth := authHeaderForClient(t, 10, 100)

	testCases := []struct {
		name       string
		body       map[string]any
		auth       string
		wantStatus int
	}{
		{
			name: "happy path",
			body: map[string]any{
				"name":                uniqueValue(t, "company"),
				"registration_number": "12345678",
				"tax_number":          "123456789",
				"work_code_id":        wc.WorkCodeID,
				"address":             "Company Street 1",
				"owner_id":            100,
			},
			auth:       employeeAuth,
			wantStatus: http.StatusCreated,
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
			name:       "missing auth",
			body:       map[string]any{"name": "test"},
			auth:       "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "client cannot create company",
			body: map[string]any{
				"name":                uniqueValue(t, "company"),
				"registration_number": "11111111",
				"tax_number":          "111111111",
				"work_code_id":        wc.WorkCodeID,
				"owner_id":            100,
			},
			auth:       clientAuth,
			wantStatus: http.StatusForbidden,
		},
		{
			name: "work code not found",
			body: map[string]any{
				"name":                uniqueValue(t, "company"),
				"registration_number": "22222222",
				"tax_number":          "222222222",
				"work_code_id":        99999,
				"owner_id":            100,
			},
			auth:       employeeAuth,
			wantStatus: http.StatusNotFound,
		},
		{
			name: "duplicate registration number",
			body: map[string]any{
				"name":                uniqueValue(t, "company"),
				"registration_number": "12345678",
				"tax_number":          "333333333",
				"work_code_id":        wc.WorkCodeID,
				"owner_id":            100,
			},
			auth:       employeeAuth,
			wantStatus: http.StatusConflict,
		},
		{
			name: "duplicate tax number",
			body: map[string]any{
				"name":                uniqueValue(t, "company"),
				"registration_number": "44444444",
				"tax_number":          "123456789",
				"work_code_id":        wc.WorkCodeID,
				"owner_id":            100,
			},
			auth:       employeeAuth,
			wantStatus: http.StatusConflict,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodPost, "/api/companies", tc.body, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusCreated {
				resp := decodeResponse[map[string]any](t, recorder)
				assert.NotEmpty(t, resp["name"])
				assert.Equal(t, "12345678", resp["registration_number"])
			}
		})
	}

	var count int64
	db.Model(&model.Company{}).Count(&count)
	assert.Equal(t, int64(1), count)
}
