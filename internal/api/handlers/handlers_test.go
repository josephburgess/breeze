package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/josephburgess/breeze/internal/api/handlers"
	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserHandler_GetUser(t *testing.T) {
	handler := handlers.NewUserHandler()

	testUser := &models.User{
		ID:       1,
		GithubID: 12345,
		Login:    "testuser",
		Name:     stringPtr("Test User"),
		Email:    stringPtr("test@example.com"),
	}

	req, err := http.NewRequest("GET", "/user", nil)
	require.NoError(t, err)

	ctx := context.WithValue(req.Context(), middleware.UserContextKey, testUser)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.GetUser(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	t.Logf("Response: %v", response)

	if val, ok := response["github_id"]; ok {
		assert.Equal(t, float64(12345), val, "github_id should be 12345")
	} else if val, ok := response["id"]; ok {
		assert.Equal(t, float64(1), val, "id should be 1")
	} else {
		t.Fatalf("Neither github_id nor id found in response: %v", response)
	}
}

func stringPtr(s string) *string {
	return &s
}
