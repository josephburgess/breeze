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

	weatherClient := weather.NewClient(cfg.OpenWeatherAPIKey)

	userStore, err := store.NewUserStore(cfg.DBPath)
	if err != nil {
		logging.Error("Failed to initialize user store", err)
		return
	}
	defer userStore.Close()

	githubOAuth := auth.NewGitHubOAuth(
		cfg.GithubClientID,
		cfg.GithubClientSecret,
		cfg.GithubRedirectURI,
	)

	router := api.NewRouter(weatherClient, userStore, githubOAuth, cfg.BaseServerURL, cfg.IsProd)
	router.Use(logging.Middleware)

	logging.Info("Starting server on port %s", cfg.Port)
	logging.Error("Server encountered an error", http.ListenAndServe(":"+cfg.Port, router))
}
