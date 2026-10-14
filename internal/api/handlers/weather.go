package handlers

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/api/respond"
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

	respond.JSON(w, http.StatusOK, resp)
}

func (h *WeatherHandler) SearchCities(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		respond.Error(w, http.StatusBadRequest, respond.CodeBadRequest, "the q parameter is required")
		return
	}

	cities, err := h.weather.Search(r.Context(), query)
	if err != nil {
		writeWeatherError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, cities)
}

func writeWeatherError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, weather.ErrNotFound):
		respond.Error(w, http.StatusNotFound, respond.CodeNotFound, "city not found")
	case errors.Is(err, weather.ErrInvalidKey):
		respond.Error(w, http.StatusUnauthorized, respond.CodeInvalidKey, "OpenWeather rejected the API key")
	default:
		logging.Error("Error getting weather", err)
		respond.Error(w, http.StatusBadGateway, respond.CodeUpstream, "could not get weather from OpenWeather")
	}
}
