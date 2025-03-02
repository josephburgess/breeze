package api

import (
	"github.com/gorilla/mux"
	"github.com/josephburgess/breeze/internal/api/handlers"
	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/services/auth"
	"github.com/josephburgess/breeze/internal/services/store"
	"github.com/josephburgess/breeze/internal/services/weather"
)

func NewRouter(weatherClient *weather.Client, userStore *store.UserStore, githubOAuth *auth.GitHubOAuth, baseServerURL string) *mux.Router {
	router := mux.NewRouter()

	// create handlers
	authHandler := handlers.NewAuthHandler(githubOAuth, userStore, baseServerURL)
	userHandler := handlers.NewUserHandler()
	weatherHandler := handlers.NewWeatherHandler(weatherClient)

	// auth routes (public)
	router.HandleFunc("/api/auth/request", authHandler.RequestAuth).Methods("GET")
	router.HandleFunc("/api/auth/callback", authHandler.Callback).Methods("GET")
	router.HandleFunc("/api/auth/exchange", authHandler.ExchangeToken).Methods("POST")

	// auth'ed routes (needs key)
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(middleware.ApiKeyAuth(userStore))
	apiRouter.HandleFunc("/user", userHandler.GetUser).Methods("GET")
	apiRouter.HandleFunc("/weather/{city}", weatherHandler.GetWeather).Methods("GET")

	return router
}
