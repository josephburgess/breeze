package api

import (
	"net/http"

	"github.com/josephburgess/breeze/internal/api/handlers"
	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/services/auth"
	"github.com/josephburgess/breeze/internal/services/store"
	"github.com/josephburgess/breeze/internal/services/weather"
)

func NewRouter(weatherClient *weather.Client, userStore *store.UserStore, githubOAuth *auth.GitHubOAuth) http.Handler {
	mux := http.NewServeMux()

	authHandler := handlers.NewAuthHandler(githubOAuth, userStore)
	userHandler := handlers.NewUserHandler()
	weatherHandler := handlers.NewWeatherHandler(weatherClient)

	// public routes
	mux.HandleFunc("GET /api/auth/request", authHandler.RequestAuth)
	mux.HandleFunc("GET /api/auth/callback", authHandler.Callback)
	mux.HandleFunc("POST /api/auth/exchange", authHandler.ExchangeToken)
	mux.HandleFunc("GET /api/cities/search", weatherHandler.SearchCities)

	// authenticated routes
	authed := middleware.ApiKeyAuth(userStore)
	mux.Handle("GET /api/user", authed(http.HandlerFunc(userHandler.GetUser)))
	mux.Handle("GET /api/weather/{city}", authed(http.HandlerFunc(weatherHandler.GetWeather)))

	return mux
}
