//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"common/pkg/auth"
	"common/pkg/permission"
	"user-service/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientRegister(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)
	empIdentity, _ := seedEmployee(t, db, position.PositionID)
	empAuth := authHeader(t, empIdentity.ID)

	email := fmt.Sprintf("client-%d@example.com", uniqueCounter.Add(1))
	username := fmt.Sprintf("client-%d", uniqueCounter.Add(1))

	recorder := performRequest(t, router, http.MethodPost, "/api/clients/register", map[string]any{
		"first_name":    "Mika",
		"last_name":     "Client",
		"date_of_birth": time.Now().UTC().AddDate(-28, 0, 0).Format(time.RFC3339),
		"gender":        "male",
		"email":         email,
		"username":      username,
		"phone_number":  "0601111111",
		"address":       "Client Street 1",
	}, empAuth)

	requireStatus(t, recorder, http.StatusCreated)

	var identity model.Identity
	require.NoError(t, db.Where("email = ?", email).First(&identity).Error)
	assert.Equal(t, auth.IdentityClient, identity.Type)
	assert.False(t, identity.Active)

	var client model.Client
	require.NoError(t, db.Where("identity_id = ?", identity.ID).First(&client).Error)
	assert.Equal(t, "Mika", client.FirstName)
	assert.Equal(t, identity.ID, client.IdentityID)

	var activationToken model.ActivationToken
	require.NoError(t, db.Where("identity_id = ?", identity.ID).First(&activationToken).Error)
	assert.NotEmpty(t, activationToken.Token)
}

func TestClientRegisterActivateAndLogin(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)
	empIdentity, _ := seedEmployee(t, db, position.PositionID)
	empAuth := authHeader(t, empIdentity.ID)

	email := fmt.Sprintf("clientauth-%d@example.com", uniqueCounter.Add(1))
	username := fmt.Sprintf("clientauth-%d", uniqueCounter.Add(1))

	register := performRequest(t, router, http.MethodPost, "/api/clients/register", map[string]any{
		"first_name":    "Jana",
		"last_name":     "Client",
		"date_of_birth": time.Now().UTC().AddDate(-31, 0, 0).Format(time.RFC3339),
		"gender":        "female",
		"email":         email,
		"username":      username,
		"phone_number":  "0602222222",
		"address":       "Client Street 2",
	}, empAuth)
	requireStatus(t, register, http.StatusCreated)

	var identity model.Identity
	require.NoError(t, db.Where("email = ?", email).First(&identity).Error)

	var client model.Client
	require.NoError(t, db.Where("identity_id = ?", identity.ID).First(&client).Error)

	var activationToken model.ActivationToken
	require.NoError(t, db.Where("identity_id = ?", identity.ID).First(&activationToken).Error)

	activate := performRequest(t, router, http.MethodPost, "/api/auth/activate", map[string]any{
		"token":    activationToken.Token,
		"password": "Password12",
	}, "")
	requireStatus(t, activate, http.StatusOK)

	login := performRequest(t, router, http.MethodPost, "/api/auth/login", map[string]any{
		"email":    email,
		"password": "Password12",
	}, "")
	requireStatus(t, login, http.StatusOK)

	response := decodeResponse[loginResponse](t, login)
	assert.Equal(t, client.ClientID, response.User.ID)

	claims := verifyAccessToken(t, response.Token)
	assert.Equal(t, identity.ID, claims.IdentityID)
	assert.Equal(t, string(auth.IdentityClient), claims.IdentityType)
	if assert.NotNil(t, claims.ClientID) {
		assert.Equal(t, client.ClientID, *claims.ClientID)
	}
	assert.Nil(t, claims.EmployeeID)
}

type clientResponse struct {
	ClientID    uint   `json:"ClientID"`
	FirstName   string `json:"FirstName"`
	LastName    string `json:"LastName"`
	PhoneNumber string `json:"PhoneNumber"`
	Address     string `json:"Address"`
}

type listClientsResponse struct {
	Data  []clientResponse `json:"data"`
	Total int64            `json:"total"`
}

func TestListClients(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)

	viewerIdentity, _ := seedEmployeeWithPermissions(t, db, position.PositionID, permission.ClientView)
	noPermIdentity, _ := seedEmployee(t, db, position.PositionID)
	registerAuth := authHeader(t, viewerIdentity.ID)

	email1 := fmt.Sprintf("list-client-%d@example.com", uniqueCounter.Add(1))
	username1 := fmt.Sprintf("list-client-%d", uniqueCounter.Add(1))
	email2 := fmt.Sprintf("list-client-%d@example.com", uniqueCounter.Add(1))
	username2 := fmt.Sprintf("list-client-%d", uniqueCounter.Add(1))

	performRequest(t, router, http.MethodPost, "/api/clients/register", map[string]any{
		"first_name":    "Marko",
		"last_name":     "Markovic",
		"date_of_birth": time.Now().UTC().AddDate(-25, 0, 0).Format(time.RFC3339),
		"gender":        "male",
		"email":         email1,
		"username":      username1,
		"phone_number":  "0601111111",
		"address":       "Street 1",
	}, registerAuth)

	performRequest(t, router, http.MethodPost, "/api/clients/register", map[string]any{
		"first_name":    "Ana",
		"last_name":     "Anic",
		"date_of_birth": time.Now().UTC().AddDate(-22, 0, 0).Format(time.RFC3339),
		"gender":        "female",
		"email":         email2,
		"username":      username2,
		"phone_number":  "0602222222",
		"address":       "Street 2",
	}, registerAuth)

	testCases := []struct {
		name         string
		path         string
		auth         string
		wantStatus   int
		wantTotal    int64
		wantCount    int
		assertResult func(t *testing.T, response listClientsResponse)
	}{
		{
			name:       "list all clients",
			path:       "/api/clients?page=1&page_size=10",
			auth:       authHeader(t, viewerIdentity.ID),
			wantStatus: http.StatusOK,
			wantTotal:  2,
			wantCount:  2,
		},
		{
			name:       "filter by first name",
			path:       "/api/clients?first_name=Marko&page=1&page_size=10",
			auth:       authHeader(t, viewerIdentity.ID),
			wantStatus: http.StatusOK,
			wantTotal:  1,
			wantCount:  1,
			assertResult: func(t *testing.T, response listClientsResponse) {
				t.Helper()
				assert.Equal(t, "Marko", response.Data[0].FirstName)
			},
		},
		{
			name:       "filter by last name",
			path:       "/api/clients?last_name=Anic&page=1&page_size=10",
			auth:       authHeader(t, viewerIdentity.ID),
			wantStatus: http.StatusOK,
			wantTotal:  1,
			wantCount:  1,
			assertResult: func(t *testing.T, response listClientsResponse) {
				t.Helper()
				assert.Equal(t, "Anic", response.Data[0].LastName)
			},
		},
		{
			name:       "filter by email",
			path:       "/api/clients?email=" + email1 + "&page=1&page_size=10",
			auth:       authHeader(t, viewerIdentity.ID),
			wantStatus: http.StatusOK,
			wantTotal:  1,
			wantCount:  1,
		},
		{
			name:       "missing auth header",
			path:       "/api/clients",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "without client view permission",
			path:       "/api/clients",
			auth:       authHeader(t, noPermIdentity.ID),
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(t, router, http.MethodGet, tc.path, nil, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusOK {
				response := decodeResponse[listClientsResponse](t, recorder)
				assert.Equal(t, tc.wantTotal, response.Total)
				assert.Len(t, response.Data, tc.wantCount)
				if tc.assertResult != nil {
					tc.assertResult(t, response)
				}
			}
		})
	}
}

func TestUpdateClient(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)

	updaterIdentity, _ := seedEmployeeWithPermissions(t, db, position.PositionID, permission.ClientUpdate)
	noPermIdentity, _ := seedEmployee(t, db, position.PositionID)
	registerAuth := authHeader(t, updaterIdentity.ID)

	email := fmt.Sprintf("update-client-%d@example.com", uniqueCounter.Add(1))
	username := fmt.Sprintf("update-client-%d", uniqueCounter.Add(1))

	performRequest(t, router, http.MethodPost, "/api/clients/register", map[string]any{
		"first_name":    "Staro",
		"last_name":     "Ime",
		"date_of_birth": time.Now().UTC().AddDate(-25, 0, 0).Format(time.RFC3339),
		"gender":        "male",
		"email":         email,
		"username":      username,
		"phone_number":  "0601111111",
		"address":       "Street 1",
	}, registerAuth)

	var client model.Client
	require.NoError(t, db.
		Joins("JOIN identities ON identities.id = clients.identity_id").
		Where("identities.email = ?", email).
		First(&client).Error)

	testCases := []struct {
		name         string
		clientID     uint
		body         map[string]any
		auth         string
		wantStatus   int
		assertResult func(t *testing.T, response clientResponse)
	}{
		{
			name:       "valid partial update",
			clientID:   client.ClientID,
			body:       map[string]any{"first_name": "Novo", "phone_number": "0609999999"},
			auth:       authHeader(t, updaterIdentity.ID),
			wantStatus: http.StatusOK,
			assertResult: func(t *testing.T, response clientResponse) {
				t.Helper()
				assert.Equal(t, "Novo", response.FirstName)
				assert.Equal(t, "0609999999", response.PhoneNumber)
			},
		},
		{
			name:       "client not found",
			clientID:   99999,
			body:       map[string]any{"first_name": "Novo"},
			auth:       authHeader(t, updaterIdentity.ID),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing auth header",
			clientID:   client.ClientID,
			body:       map[string]any{"first_name": "Novo"},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "without client update permission",
			clientID:   client.ClientID,
			body:       map[string]any{"first_name": "Novo"},
			auth:       authHeader(t, noPermIdentity.ID),
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/clients/%s", itoa(tc.clientID))
			recorder := performRequest(t, router, http.MethodPatch, path, tc.body, tc.auth)
			requireStatus(t, recorder, tc.wantStatus)

			if tc.wantStatus == http.StatusOK && tc.assertResult != nil {
				tc.assertResult(t, decodeResponse[clientResponse](t, recorder))
			}
		})
	}
}
