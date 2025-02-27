package api

import (
	"github.com/gorilla/mux"
	"github.com/josephburgess/gust-api/internal/api/handlers"
	"github.com/josephburgess/gust-api/internal/api/middleware"
	"github.com/josephburgess/gust-api/internal/services/auth"
	"github.com/josephburgess/gust-api/internal/services/store"
	"github.com/josephburgess/gust-api/internal/services/weather"
)

func NewRouter(weatherClient *weather.Client, userStore *store.UserStore, githubOAuth *auth.GitHubOAuth) *mux.Router {
	router := mux.NewRouter()

	// Create handlers
	authHandler := handlers.NewAuthHandler(githubOAuth, userStore)
	userHandler := handlers.NewUserHandler()
	weatherHandler := handlers.NewWeatherHandler(weatherClient)

	// Auth routes (public)
	router.HandleFunc("/api/auth/request", authHandler.RequestAuth).Methods("GET")
	router.HandleFunc("/api/auth/callback", authHandler.Callback).Methods("GET")
	router.HandleFunc("/api/auth/exchange", authHandler.ExchangeToken).Methods("POST")

	// Protected API routes
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(middleware.ApiKeyAuth(userStore))

	// User routes
	apiRouter.HandleFunc("/user", userHandler.GetUser).Methods("GET")

	// Weather routes
	apiRouter.HandleFunc("/weather/{city}", weatherHandler.GetWeather).Methods("GET")

	return router
}
