package main

import (
	"net/http"
	"os"

	"github.com/josephburgess/breeze/internal/api"
	"github.com/josephburgess/breeze/internal/config"
	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/services/auth"
	"github.com/josephburgess/breeze/internal/services/store"
	"github.com/josephburgess/breeze/internal/services/weather"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		logging.Error("Configuration error", err)
		os.Exit(1)
	}

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

	handler := logging.Middleware(api.NewRouter(weatherClient, userStore, githubOAuth))

	logging.Info("Starting server on port %s", cfg.Port)
	logging.Error("Server encountered an error", http.ListenAndServe(":"+cfg.Port, handler))
}
