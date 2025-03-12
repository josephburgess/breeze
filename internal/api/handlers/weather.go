package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/models"
	"github.com/josephburgess/breeze/internal/services/weather"
)

type WeatherHandler struct {
	weatherClient *weather.Client
}

func NewWeatherHandler(weatherClient *weather.Client) *WeatherHandler {
	return &WeatherHandler{
		weatherClient: weatherClient,
	}
}

func (h *WeatherHandler) GetWeather(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cityName := vars["city"]
	units := r.URL.Query().Get("units")
	var customApiKey string
	if key, ok := r.Context().Value(middleware.CustomApiContextKey).(string); ok {
		customApiKey = key
		logging.Info("Using direct OpenWeather API key")
	}

	logging.Info("Fetching weather for city: %s", cityName)
	if units != "" {
		logging.Info("Using units: %s", units)
	}

	city, err := h.weatherClient.GetCoordinates(cityName, customApiKey)
	if err != nil {
		if strings.Contains(err.Error(), "invalid_api_key") {
			logging.Error("Invalid API key provided", err)
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		logging.Error("Error finding city", err)
		http.Error(w, "Error finding city", http.StatusNotFound)
		return
	}

	logging.Info("Found city: %s (Lat: %f, Lon: %f)", city.Name, city.Lat, city.Lon)

	weather, err := h.weatherClient.GetWeather(city.Lat, city.Lon, units, customApiKey)
	if err != nil {
		logging.Error("Error getting weather", err)
		http.Error(w, "Error getting weather", http.StatusInternalServerError)
		return
	}

	logging.Info("Retrieved weather for city: %s", city.Name)

	response := models.WeatherResponse{
		City:    city,
		Weather: weather,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *WeatherHandler) SearchCities(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "City query parameter is required", http.StatusBadRequest)
		return
	}

	limit := 5

	logging.Info("Searching cities for query: %s", query)

	cities, err := h.weatherClient.SearchCities(query, limit)
	if err != nil {
		logging.Error("Error searching cities", err)
		http.Error(w, "Error searching cities", http.StatusInternalServerError)
		return
	}

	logging.Info("Found %d cities for query: %s", len(cities), query)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cities)
}
