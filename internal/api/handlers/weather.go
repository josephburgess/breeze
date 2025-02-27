package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/josephburgess/gust-api/internal/models"
	"github.com/josephburgess/gust-api/internal/services/weather"
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

	city, err := h.weatherClient.GetCoordinates(cityName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error finding city: %v", err), http.StatusNotFound)
		return
	}

	weather, err := h.weatherClient.GetWeather(city.Lat, city.Lon)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting weather: %v", err), http.StatusInternalServerError)
		return
	}

	response := models.WeatherResponse{
		City:    city,
		Weather: weather,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
