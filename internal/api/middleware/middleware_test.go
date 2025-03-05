package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type UserValidator interface {
	ValidateAPIKey(apiKey string) (*models.User, error)
}

type MockUserStore struct {
	mock.Mock
}

func (m *MockUserStore) ValidateAPIKey(apiKey string) (*models.User, error) {
	args := m.Called(apiKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func TestApiKeyAuth_ValidKey(t *testing.T) {
	mockStore := new(MockUserStore)

	// Test data
	testUser := &models.User{
		ID:       1,
		GithubID: 12345,
		Login:    "testuser",
	}
	testAPIKey := "gust_valid_api_key"

	// Set expectations
	mockStore.On("ValidateAPIKey", testAPIKey).Return(testUser, nil)

	// Create a custom middleware function that uses our mock
	apiKeyAuthMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.URL.Query().Get("api_key")

			if apiKey == "" {
				http.Error(w, "API key is required", http.StatusUnauthorized)
				return
			}

			user, err := mockStore.ValidateAPIKey(apiKey)
			if err != nil {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), middleware.UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Create a test handler that checks if user is in context
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if user is in context
		user, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
		assert.True(t, ok, "User should be in context")
		assert.Equal(t, testUser, user)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Create request with API key
	req, err := http.NewRequest("GET", "/api/test?api_key="+testAPIKey, nil)
	require.NoError(t, err)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute middleware with next handler
	handlerToTest := apiKeyAuthMiddleware(nextHandler)
	handlerToTest.ServeHTTP(rr, req)

	// Assertions
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "success", rr.Body.String())

	// Verify expectations
	mockStore.AssertExpectations(t)
}

func TestApiKeyAuth_MissingKey(t *testing.T) {
	// Setup
	mockStore := new(MockUserStore)

	// Create a custom middleware function that uses our mock
	apiKeyAuthMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.URL.Query().Get("api_key")

			if apiKey == "" {
				http.Error(w, "API key is required", http.StatusUnauthorized)
				return
			}

			user, err := mockStore.ValidateAPIKey(apiKey)
			if err != nil {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), middleware.UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Create a test handler that should not be called
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Next handler should not be called")
	})

	// Create request without API key
	req, err := http.NewRequest("GET", "/api/test", nil)
	require.NoError(t, err)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute middleware with next handler
	handlerToTest := apiKeyAuthMiddleware(nextHandler)
	handlerToTest.ServeHTTP(rr, req)

	// Assertions
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "API key is required")
}

func TestApiKeyAuth_InvalidKey(t *testing.T) {
	// Setup
	mockStore := new(MockUserStore)

	// Test data
	testAPIKey := "gust_invalid_api_key"
	testError := assert.AnError // Use testify's built-in error

	// Set expectations
	mockStore.On("ValidateAPIKey", testAPIKey).Return(nil, testError)

	// Create a custom middleware function that uses our mock
	apiKeyAuthMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.URL.Query().Get("api_key")

			if apiKey == "" {
				http.Error(w, "API key is required", http.StatusUnauthorized)
				return
			}

			user, err := mockStore.ValidateAPIKey(apiKey)
			if err != nil {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), middleware.UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Create a test handler that should not be called
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Next handler should not be called")
	})

	// Create request with invalid API key
	req, err := http.NewRequest("GET", "/api/test?api_key="+testAPIKey, nil)
	require.NoError(t, err)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute middleware with next handler
	handlerToTest := apiKeyAuthMiddleware(nextHandler)
	handlerToTest.ServeHTTP(rr, req)

	// Assertions
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid API key")

	// Verify expectations
	mockStore.AssertExpectations(t)
}
