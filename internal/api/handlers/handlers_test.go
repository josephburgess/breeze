package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/josephburgess/breeze/internal/api/handlers"
	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/models"
	"github.com/josephburgess/breeze/internal/services/weather"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- mocks ---

type mockWeatherClient struct{ mock.Mock }

func (m *mockWeatherClient) GetCoordinates(city, customKey string) (*models.City, error) {
	args := m.Called(city, customKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.City), args.Error(1)
}
func (m *mockWeatherClient) GetWeather(lat, lon float64, units, customKey string) (*models.OneCallResponse, error) {
	args := m.Called(lat, lon, units, customKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OneCallResponse), args.Error(1)
}
func (m *mockWeatherClient) SearchCities(query string, limit int) ([]models.City, error) {
	args := m.Called(query, limit)
	return args.Get(0).([]models.City), args.Error(1)
}

type mockQuotaStore struct{ mock.Mock }

func (m *mockQuotaStore) GetAPIKeyQuota(apiKey string) (int, int, time.Time, error) {
	args := m.Called(apiKey)
	return args.Int(0), args.Int(1), args.Get(2).(time.Time), args.Error(3)
}

// --- WeatherHandler tests ---

func TestWeatherHandler_GetWeather_OK(t *testing.T) {
	city := &models.City{Name: "London", Country: "GB", Lat: 51.5074, Lon: -0.1278}
	weatherData := &models.OneCallResponse{Lat: 51.5074, Lon: -0.1278, Timezone: "Europe/London",
		Current: models.CurrentWeather{Temp: 15.5}}

	mc := new(mockWeatherClient)
	mc.On("GetCoordinates", "London", "").Return(city, nil)
	mc.On("GetWeather", city.Lat, city.Lon, "metric", "").Return(weatherData, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /weather/{city}", handlers.NewWeatherHandler(mc).GetWeather)

	req := httptest.NewRequest("GET", "/weather/London?units=metric", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp models.WeatherResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "London", resp.City.Name)
	assert.Equal(t, 15.5, resp.Weather.Current.Temp)
	mc.AssertExpectations(t)
}

func TestWeatherHandler_GetWeather_CityNotFound(t *testing.T) {
	mc := new(mockWeatherClient)
	mc.On("GetCoordinates", "Nowhere", "").Return(nil, errors.New("no coordinates found for Nowhere"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /weather/{city}", handlers.NewWeatherHandler(mc).GetWeather)

	req := httptest.NewRequest("GET", "/weather/Nowhere", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	mc.AssertExpectations(t)
}

func TestWeatherHandler_GetWeather_InvalidAPIKey(t *testing.T) {
	mc := new(mockWeatherClient)
	mc.On("GetCoordinates", "London", "badkey").Return(nil, &weather.InvalidAPIKeyError{})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /weather/{city}", handlers.NewWeatherHandler(mc).GetWeather)

	req := httptest.NewRequest("GET", "/weather/London", nil)
	ctx := context.WithValue(req.Context(), middleware.CustomApiContextKey, "badkey")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	mc.AssertExpectations(t)
}

func TestWeatherHandler_SearchCities_OK(t *testing.T) {
	cities := []models.City{{Name: "London", Country: "GB"}, {Name: "London", Country: "CA"}}

	mc := new(mockWeatherClient)
	mc.On("SearchCities", "lon", 5).Return(cities, nil)

	req := httptest.NewRequest("GET", "/cities?q=lon", nil)
	rr := httptest.NewRecorder()
	handlers.NewWeatherHandler(mc).SearchCities(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var result []models.City
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Len(t, result, 2)
	mc.AssertExpectations(t)
}

func TestWeatherHandler_SearchCities_MissingQuery(t *testing.T) {
	mc := new(mockWeatherClient)
	req := httptest.NewRequest("GET", "/cities", nil)
	rr := httptest.NewRecorder()
	handlers.NewWeatherHandler(mc).SearchCities(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- UserHandler tests ---

func TestUserHandler_GetUser_OK(t *testing.T) {
	user := &models.User{ID: 1, GithubID: 12345, Login: "testuser"}
	handler := handlers.NewUserHandler(nil)

	req := httptest.NewRequest("GET", "/user", nil)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
	rr := httptest.NewRecorder()
	handler.GetUser(rr, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, float64(12345), resp["github_id"])
}

func TestUserHandler_GetUser_Unauthorized(t *testing.T) {
	handler := handlers.NewUserHandler(nil)
	req := httptest.NewRequest("GET", "/user", nil)
	rr := httptest.NewRecorder()
	handler.GetUser(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestUserHandler_GetQuota_GustKey(t *testing.T) {
	resetAt := time.Now().Add(time.Hour)
	qs := new(mockQuotaStore)
	qs.On("GetAPIKeyQuota", "gust_abc").Return(50, 10, resetAt, nil)

	handler := handlers.NewUserHandler(qs)
	req := httptest.NewRequest("GET", "/quota?api_key=gust_abc", nil)
	rr := httptest.NewRecorder()
	handler.GetQuota(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, float64(50), resp["daily_limit"])
	assert.Equal(t, float64(10), resp["daily_used"])
	assert.Equal(t, float64(40), resp["remaining"])
	assert.Equal(t, false, resp["unlimited"])
	qs.AssertExpectations(t)
}

func TestUserHandler_GetQuota_CustomKey(t *testing.T) {
	handler := handlers.NewUserHandler(nil)
	req := httptest.NewRequest("GET", "/quota", nil)
	ctx := context.WithValue(req.Context(), middleware.CustomApiContextKey, "my_ow_key")
	rr := httptest.NewRecorder()
	handler.GetQuota(rr, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["unlimited"])
}

func stringPtr(s string) *string { return &s }
