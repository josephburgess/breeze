package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/services/weather"
)

type WeatherHandler struct {
	weather *weather.Client
}

func NewWeatherHandler(weatherClient *weather.Client) *WeatherHandler {
	return &WeatherHandler{weather: weatherClient}
}

func (h *WeatherHandler) GetWeather(w http.ResponseWriter, r *http.Request) {
	customKey, _ := r.Context().Value(middleware.CustomApiContextKey).(string)

	resp, err := h.weather.Forecast(r.Context(), mux.Vars(r)["city"], r.URL.Query().Get("units"), customKey)
	if err != nil {
		writeWeatherError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *WeatherHandler) SearchCities(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "City query parameter is required", http.StatusBadRequest)
		return
	}

	cities, err := h.weather.Search(r.Context(), query)
	if err != nil {
		writeWeatherError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cities)
}

func writeWeatherError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, weather.ErrNotFound):
		http.Error(w, "Error finding city", http.StatusNotFound)
	case errors.Is(err, weather.ErrInvalidKey):
		http.Error(w, "Invalid API key", http.StatusUnauthorized)
	default:
		logging.Error("Error getting weather", err)
		http.Error(w, "Error getting weather", http.StatusBadGateway)
	}
}
