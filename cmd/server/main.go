package main

import (
	"net/http"

	"github.com/josephburgess/breeze/internal/api"
	"github.com/josephburgess/breeze/internal/config"
	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/services/auth"
	"github.com/josephburgess/breeze/internal/services/store"
	"github.com/josephburgess/breeze/internal/services/weather"
)

func main() {
	cfg := config.Load()

	weatherClient := weather.NewClient(cfg.OpenWeatherAPIKey, weather.DefaultBaseURL)

	userStore, err := store.NewUserStore(cfg.DBPath)
	if err != nil {
		logging.Error("Failed to initialize user store", err)
		return
	}
	defer userStore.Close()

	authFlow := auth.NewFlow(auth.NewGitHub(cfg.GithubClientID, cfg.GithubClientSecret), userStore)

	router := api.NewRouter(weatherClient, userStore, authFlow)
	router.Use(logging.Middleware)

	logging.Info("Starting server on port %s", cfg.Port)
	logging.Error("Server encountered an error", http.ListenAndServe(":"+cfg.Port, router))
}
