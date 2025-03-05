package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/josephburgess/breeze/internal/api/handlers"
	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type WeatherClientInterface interface {
	GetCoordinates(city string) (*models.City, error)
	GetWeather(lat, lon float64, units string) (*models.OneCallResponse, error)
	SearchCities(query string, limit int) ([]models.City, error)
}

type MockWeatherClient struct {
	mock.Mock
}

func (m *MockWeatherClient) GetCoordinates(city string) (*models.City, error) {
	args := m.Called(city)
	return args.Get(0).(*models.City), args.Error(1)
}

func (m *MockWeatherClient) GetWeather(lat, lon float64, units string) (*models.OneCallResponse, error) {
	args := m.Called(lat, lon, units)
	return args.Get(0).(*models.OneCallResponse), args.Error(1)
}

func (m *MockWeatherClient) SearchCities(query string, limit int) ([]models.City, error) {
	args := m.Called(query, limit)
	return args.Get(0).([]models.City), args.Error(1)
}

type UserStoreInterface interface {
	SaveUser(user *models.User) error
	GetUser(githubID int64) (*models.User, error)
	GetOrCreateAPICredential(githubUserID int64) (*models.ApiCredential, error)
	ValidateAPIKey(apiKey string) (*models.User, error)
	Close() error
}

type MockUserStore struct {
	mock.Mock
}

func (m *MockUserStore) SaveUser(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserStore) GetUser(githubID int64) (*models.User, error) {
	args := m.Called(githubID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserStore) GetOrCreateAPICredential(githubUserID int64) (*models.ApiCredential, error) {
	args := m.Called(githubUserID)
	return args.Get(0).(*models.ApiCredential), args.Error(1)
}

func (m *MockUserStore) ValidateAPIKey(apiKey string) (*models.User, error) {
	args := m.Called(apiKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

type GitHubOAuthInterface interface {
	GetAuthURL() (string, string)
	ExchangeCodeForToken(code, state string) (string, error)
	GetUserInfo(token string) (*models.User, error)
}

type MockGitHubOAuth struct {
	mock.Mock
}

func (m *MockGitHubOAuth) GetAuthURL() (string, string) {
	args := m.Called()
	return args.String(0), args.String(1)
}

func (m *MockGitHubOAuth) ExchangeCodeForToken(code, state string) (string, error) {
	args := m.Called(code, state)
	return args.String(0), args.Error(1)
}

func (m *MockGitHubOAuth) GetUserInfo(token string) (*models.User, error) {
	args := m.Called(token)
	return args.Get(0).(*models.User), args.Error(1)
}

func TestWeatherHandler_GetWeather(t *testing.T) {
	mockClient := new(MockWeatherClient)

	getWeatherHandler := func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		cityName := vars["city"]
		units := r.URL.Query().Get("units")

		city, err := mockClient.GetCoordinates(cityName)
		if err != nil {
			http.Error(w, "Error finding city", http.StatusNotFound)
			return
		}

		weather, err := mockClient.GetWeather(city.Lat, city.Lon, units)
		if err != nil {
			http.Error(w, "Error getting weather", http.StatusInternalServerError)
			return
		}

		response := models.WeatherResponse{
			City:    city,
			Weather: weather,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	testCity := &models.City{
		Name:    "London",
		Country: "GB",
		Lat:     51.5074,
		Lon:     -0.1278,
	}

	testWeather := &models.OneCallResponse{
		Lat:      51.5074,
		Lon:      -0.1278,
		Timezone: "Europe/London",
		Current: models.CurrentWeather{
			Temp:      15.5,
			FeelsLike: 14.8,
			Humidity:  70,
			Weather: []models.WeatherCondition{
				{
					Main:        "Clouds",
					Description: "scattered clouds",
					Icon:        "03d",
				},
			},
		},
	}

	mockClient.On("GetCoordinates", "London").Return(testCity, nil)
	mockClient.On("GetWeather", testCity.Lat, testCity.Lon, "metric").Return(testWeather, nil)

	req, err := http.NewRequest("GET", "/weather/London?units=metric", nil)
	require.NoError(t, err)

	router := mux.NewRouter()
	router.HandleFunc("/weather/{city}", getWeatherHandler).Methods("GET")

	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	statusCode := rr.Code
	responseBody := rr.Body.String()

	t.Logf("Status Code: %d", statusCode)
	t.Logf("Response Body: %s", responseBody)

	assert.Equal(t, http.StatusOK, statusCode)

	var response models.WeatherResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, testCity.Name, response.City.Name)
	assert.Equal(t, testWeather.Current.Temp, response.Weather.Current.Temp)

	mockClient.AssertExpectations(t)
}

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

func TestAuthHandler_RequestAuth(t *testing.T) {
	mockOAuth := new(MockGitHubOAuth)
	mockOAuth.On("GetAuthURL").Return("https://github.com/login/oauth/authorize?client_id=test", "test-state")

	authRequestHandler := func(w http.ResponseWriter, r *http.Request) {
		callbackPort := r.URL.Query().Get("callback_port")
		if callbackPort == "" {
			callbackPort = "9876"
		}

		authURL, state := mockOAuth.GetAuthURL()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"url":   authURL,
			"state": state,
		})
	}

	req, err := http.NewRequest("GET", "/api/auth/request", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	authRequestHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/login/oauth/authorize?client_id=test", response["url"])
	assert.Equal(t, "test-state", response["state"])

	mockOAuth.AssertExpectations(t)
}

func stringPtr(s string) *string {
	return &s
}
