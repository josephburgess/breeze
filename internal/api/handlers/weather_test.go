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
	"github.com/josephburgess/breeze/internal/services/weather"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetWeather_ReturnsCityAndWeather(t *testing.T) {
	owm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/geo/1.0/direct" {
			w.Write([]byte(`[{"name":"London","lat":51.5,"lon":-0.12}]`))
			return
		}
		w.Write([]byte(`{"current":{"temp":15.5}}`))
	}))
	defer owm.Close()
	router := mux.NewRouter()
	router.HandleFunc("/weather/{city}", handlers.NewWeatherHandler(weather.NewClient("k", owm.URL)).GetWeather)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/weather/London?units=metric", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp models.WeatherResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "London", resp.City.Name)
	assert.Equal(t, 15.5, resp.Weather.Current.Temp)
}

func TestGetWeather_PassesCustomKeyUpstream(t *testing.T) {
	var keys []string
	owm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.URL.Query().Get("appid"))
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer owm.Close()
	router := mux.NewRouter()
	router.HandleFunc("/weather/{city}", handlers.NewWeatherHandler(weather.NewClient("server", owm.URL)).GetWeather)

	req := httptest.NewRequest(http.MethodGet, "/weather/London", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.CustomApiContextKey, "mine"))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, []string{"mine"}, keys)
}

func TestGetWeather_UnknownCityIsNotFound(t *testing.T) {
	owm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer owm.Close()
	router := mux.NewRouter()
	router.HandleFunc("/weather/{city}", handlers.NewWeatherHandler(weather.NewClient("k", owm.URL)).GetWeather)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/weather/Nowhere", nil))

	assert.Equal(t, http.StatusNotFound, rr.Code)
}
