package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/models"
	"github.com/josephburgess/breeze/internal/services/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockKeys struct {
	mock.Mock
}

func (m *MockKeys) Spend(ctx context.Context, apiKey string) (store.Usage, error) {
	args := m.Called(apiKey)
	return args.Get(0).(store.Usage), args.Error(1)
}

var reached = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(middleware.UserContextKey).(*models.User)
	custom, _ := r.Context().Value(middleware.CustomApiContextKey).(string)
	w.Write([]byte(user.Login + "|" + custom))
})

func TestApiKeyAuth_ValidKeyReachesHandlerWithRateLimitHeaders(t *testing.T) {
	keys := new(MockKeys)
	reset := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	keys.On("Spend", "gust_ok").Return(store.Usage{User: &models.User{Login: "joe"}, Limit: 50, Remaining: 49, Reset: reset}, nil)
	rr := httptest.NewRecorder()

	middleware.ApiKeyAuth(keys)(reached).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/?api_key=gust_ok", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "joe|", rr.Body.String())
	assert.Equal(t, "50", rr.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "49", rr.Header().Get("X-RateLimit-Remaining"))
	assert.Equal(t, "2026-11-05T00:00:00Z", rr.Header().Get("X-RateLimit-Reset"))
}

func TestApiKeyAuth_MissingKeyIsUnauthorized(t *testing.T) {
	rr := httptest.NewRecorder()

	middleware.ApiKeyAuth(new(MockKeys))(reached).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.JSONEq(t, `{"code":"missing_key","message":"an API key is required"}`, rr.Body.String())
}

func TestApiKeyAuth_UnknownKeyIsUnauthorized(t *testing.T) {
	keys := new(MockKeys)
	keys.On("Spend", "gust_bad").Return(store.Usage{}, store.ErrInvalidKey)
	rr := httptest.NewRecorder()

	middleware.ApiKeyAuth(keys)(reached).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/?api_key=gust_bad", nil))

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), `"invalid_key"`)
}

func TestApiKeyAuth_SpentKeyIsRateLimited(t *testing.T) {
	keys := new(MockKeys)
	keys.On("Spend", "gust_spent").Return(store.Usage{Limit: 50, Remaining: 0}, store.ErrRateLimited)
	rr := httptest.NewRecorder()

	middleware.ApiKeyAuth(keys)(reached).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/?api_key=gust_spent", nil))

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "0", rr.Header().Get("X-RateLimit-Remaining"))
	assert.Contains(t, rr.Body.String(), `"rate_limited"`)
}

func TestApiKeyAuth_CustomKeySkipsStore(t *testing.T) {
	keys := new(MockKeys)
	rr := httptest.NewRecorder()

	middleware.ApiKeyAuth(keys)(reached).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/?api_key=owm123", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "custom-api-user|owm123", rr.Body.String())
	keys.AssertNotCalled(t, "Spend", mock.Anything)
}
