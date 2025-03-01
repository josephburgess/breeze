package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
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

	logging.Info("Fetching weather for city: %s", cityName)
	if units != "" {
		logging.Info("Using units: %s", units)
	}

	city, err := h.weatherClient.GetCoordinates(cityName)
	if err != nil {
		logging.Error("Error finding city", err)
		http.Error(w, "Error finding city", http.StatusNotFound)
		return
	}

	logging.Info("Found city: %s (Lat: %f, Lon: %f)", city.Name, city.Lat, city.Lon)

	weather, err := h.weatherClient.GetWeather(city.Lat, city.Lon, units)
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
