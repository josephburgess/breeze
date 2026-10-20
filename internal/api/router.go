package api

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/josephburgess/breeze/internal/api/handlers"
	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/services/auth"
	"github.com/josephburgess/breeze/internal/services/store"
	"github.com/josephburgess/breeze/internal/services/weather"
)

// search needs no key, so it gets a per-IP limit instead
const searchLimitPerMinute = 30

func NewRouter(weatherClient *weather.Client, userStore *store.UserStore, authFlow *auth.Flow) *mux.Router {
	router := mux.NewRouter()

	// create handlers
	authHandler := handlers.NewAuthHandler(authFlow)
	weatherHandler := handlers.NewWeatherHandler(weatherClient)

	// auth routes (public)
	router.HandleFunc("/api/auth/request", authHandler.RequestAuth).Methods("GET")
	router.HandleFunc("/api/auth/callback", authHandler.Callback).Methods("GET")
	router.HandleFunc("/api/auth/exchange", authHandler.ExchangeToken).Methods("POST")
	router.Handle("/api/cities/search", middleware.PerIP(searchLimitPerMinute, time.Minute)(http.HandlerFunc(weatherHandler.SearchCities))).Methods("GET")

	// auth'ed routes (needs key)
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(middleware.ApiKeyAuth(userStore))
	apiRouter.HandleFunc("/weather/{city}", weatherHandler.GetWeather).Methods("GET")

	return router
}
