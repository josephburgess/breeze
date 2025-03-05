package weather_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/josephburgess/breeze/internal/models"
	"github.com/josephburgess/breeze/internal/services/weather"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetCoordinates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/geo/1.0/direct", r.URL.Path)
		assert.Equal(t, "London", r.URL.Query().Get("q"))
		assert.Equal(t, "1", r.URL.Query().Get("limit"))
		assert.Equal(t, "test-api-key", r.URL.Query().Get("appid"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{
				"name": "London",
				"lat": 51.5074,
				"lon": -0.1278,
				"country": "GB",
				"state": "England"
			}
		]`))
	}))
	defer server.Close()

	client := weather.NewClient("test-api-key")

	client.BaseURL = server.URL + "/"

	city, err := client.GetCoordinates("London")

	require.NoError(t, err)
	require.NotNil(t, city)
	assert.Equal(t, "London", city.Name)
	assert.Equal(t, 51.5074, city.Lat)
	assert.Equal(t, -0.1278, city.Lon)
	assert.Equal(t, "GB", city.Country)
	assert.Equal(t, "England", city.State)
}

func TestClient_GetCoordinates_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := weather.NewClient("test-api-key")
	client.BaseURL = server.URL + "/"

	city, err := client.GetCoordinates("NonExistentCity")

	assert.Error(t, err)
	assert.Nil(t, city)
	assert.Contains(t, err.Error(), "no coordinates found")
}

func TestClient_GetWeather(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/data/3.0/onecall", r.URL.Path)

		latStr := r.URL.Query().Get("lat")
		lonStr := r.URL.Query().Get("lon")

		assert.Contains(t, latStr, "51.507")
		assert.Contains(t, lonStr, "-0.127")

		assert.Equal(t, "test-api-key", r.URL.Query().Get("appid"))
		assert.Equal(t, "metric", r.URL.Query().Get("units"))

		weather := models.OneCallResponse{
			Lat:            51.5074,
			Lon:            -0.1278,
			Timezone:       "Europe/London",
			TimezoneOffset: 0,
			Current: models.CurrentWeather{
				Temp:      15.5,
				FeelsLike: 14.8,
				Pressure:  1012,
				Humidity:  76,
				Weather: []models.WeatherCondition{
					{
						ID:          800,
						Main:        "Clear",
						Description: "clear sky",
						Icon:        "01d",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(weather)
	}))
	defer server.Close()

	client := weather.NewClient("test-api-key")
	client.BaseURL = server.URL + "/"

	weather, err := client.GetWeather(51.5074, -0.1278, "metric")

	require.NoError(t, err)
	require.NotNil(t, weather)
	assert.Equal(t, 51.5074, weather.Lat)
	assert.Equal(t, -0.1278, weather.Lon)
	assert.Equal(t, "Europe/London", weather.Timezone)
	assert.Equal(t, 15.5, weather.Current.Temp)
	assert.Equal(t, 14.8, weather.Current.FeelsLike)
	assert.Equal(t, 76, weather.Current.Humidity)
	assert.Equal(t, 1, len(weather.Current.Weather))
	assert.Equal(t, "Clear", weather.Current.Weather[0].Main)
}

func TestClient_GetWeather_DefaultUnits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "", r.URL.Query().Get("units"))

		weather := models.OneCallResponse{
			Lat: 51.5074,
			Lon: -0.1278,
			Current: models.CurrentWeather{
				Temp: 288.15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(weather)
	}))
	defer server.Close()

	client := weather.NewClient("test-api-key")
	client.BaseURL = server.URL + "/"

	weather, err := client.GetWeather(51.5074, -0.1278, "")

	require.NoError(t, err)
	require.NotNil(t, weather)
	assert.Equal(t, 288.15, weather.Current.Temp)
}

func TestClient_SearchCities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/geo/1.0/direct", r.URL.Path)
		assert.Equal(t, "Lon", r.URL.Query().Get("q"))
		assert.Equal(t, "5", r.URL.Query().Get("limit"))
		assert.Equal(t, "test-api-key", r.URL.Query().Get("appid"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{
				"name": "London",
				"lat": 51.5074,
				"lon": -0.1278,
				"country": "GB",
				"state": "England"
			},
			{
				"name": "Londonderry",
				"lat": 54.9966,
				"lon": -7.3086,
				"country": "GB",
				"state": "Northern Ireland"
			},
			{
				"name": "Londrina",
				"lat": -23.3045,
				"lon": -51.1696,
				"country": "BR",
				"state": "Paraná"
			}
		]`))
	}))
	defer server.Close()

	client := weather.NewClient("test-api-key")
	client.BaseURL = server.URL + "/"

	cities, err := client.SearchCities("Lon", 5)

	require.NoError(t, err)
	require.NotNil(t, cities)
	assert.Equal(t, 3, len(cities))

	assert.Equal(t, "London", cities[0].Name)
	assert.Equal(t, 51.5074, cities[0].Lat)
	assert.Equal(t, -0.1278, cities[0].Lon)
	assert.Equal(t, "GB", cities[0].Country)
	assert.Equal(t, "England", cities[0].State)

	assert.Equal(t, "Londonderry", cities[1].Name)

	assert.Equal(t, "Londrina", cities[2].Name)
	assert.Equal(t, "BR", cities[2].Country)
}

func TestClient_SearchCities_EmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := weather.NewClient("test-api-key")
	client.BaseURL = server.URL + "/"

	cities, err := client.SearchCities("XYZ123NonExistent", 5)

	require.NoError(t, err)
	assert.NotNil(t, cities)
	assert.Empty(t, cities)
}

func TestClient_SearchCities_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"cod":401, "message": "Invalid API key"}`))
	}))
	defer server.Close()

	client := weather.NewClient("invalid-api-key")
	client.BaseURL = server.URL + "/"

	cities, err := client.SearchCities("London", 5)

	assert.Error(t, err)
	assert.Nil(t, cities)
	assert.Contains(t, err.Error(), "API returned status 401")
}
