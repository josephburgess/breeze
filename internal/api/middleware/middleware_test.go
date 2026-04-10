package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/models"
	"github.com/josephburgess/breeze/internal/services/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockValidator struct{ mock.Mock }

func (m *mockValidator) ValidateAPIKey(apiKey string) (*models.User, int, int, time.Time, error) {
	args := m.Called(apiKey)
	user, _ := args.Get(0).(*models.User)
	return user, args.Int(1), args.Int(2), args.Get(3).(time.Time), args.Error(4)
}

func okHandler(t *testing.T, expectedUser *models.User) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
		assert.True(t, ok)
		if expectedUser != nil {
			assert.Equal(t, expectedUser.Login, user.Login)
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestApiKeyAuth_MissingKey(t *testing.T) {
	v := new(mockValidator)
	handler := middleware.ApiKeyAuth(v)(okHandler(t, nil))

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "API key is required")
	v.AssertNotCalled(t, "ValidateAPIKey")
}

func TestApiKeyAuth_ValidGustKey(t *testing.T) {
	user := &models.User{GithubID: 1, Login: "testuser"}
	resetTime := time.Now().Add(time.Hour)

	v := new(mockValidator)
	v.On("ValidateAPIKey", "gust_abc123").Return(user, 50, 10, resetTime, nil)

	req := httptest.NewRequest("GET", "/api/test?api_key=gust_abc123", nil)
	rr := httptest.NewRecorder()
	middleware.ApiKeyAuth(v)(okHandler(t, user)).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "50", rr.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "40", rr.Header().Get("X-RateLimit-Remaining"))
	v.AssertExpectations(t)
}

func TestApiKeyAuth_InvalidGustKey(t *testing.T) {
	v := new(mockValidator)
	v.On("ValidateAPIKey", "gust_bad").Return((*models.User)(nil), 0, 0, time.Time{}, assert.AnError)

	req := httptest.NewRequest("GET", "/api/test?api_key=gust_bad", nil)
	rr := httptest.NewRecorder()
	middleware.ApiKeyAuth(v)(okHandler(t, nil)).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	v.AssertExpectations(t)
}

func TestApiKeyAuth_RateLimited(t *testing.T) {
	resetTime := time.Now().Add(time.Hour)
	rateLimitErr := &store.RateLimitError{
		Message:   "rate limit exceeded",
		ResetTime: resetTime,
		RateLimit: 50,
		Remaining: 0,
	}

	v := new(mockValidator)
	v.On("ValidateAPIKey", "gust_limited").Return((*models.User)(nil), 50, 50, resetTime, rateLimitErr)

	req := httptest.NewRequest("GET", "/api/test?api_key=gust_limited", nil)
	rr := httptest.NewRecorder()
	middleware.ApiKeyAuth(v)(okHandler(t, nil)).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "50", rr.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "0", rr.Header().Get("X-RateLimit-Remaining"))
	v.AssertExpectations(t)
}

func TestApiKeyAuth_CustomKey(t *testing.T) {
	v := new(mockValidator)
	// Custom keys don't start with "gust_" — ValidateAPIKey should never be called
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		customKey, ok := r.Context().Value(middleware.CustomApiContextKey).(string)
		assert.True(t, ok)
		assert.Equal(t, "my_openweather_key", customKey)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test?api_key=my_openweather_key", nil)
	rr := httptest.NewRecorder()
	middleware.ApiKeyAuth(v)(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called)
	v.AssertNotCalled(t, "ValidateAPIKey")
}
