package main

import (
	"log"
	"net/http"

	"github.com/josephburgess/gust-api/internal/api"
	"github.com/josephburgess/gust-api/internal/config"
	"github.com/josephburgess/gust-api/internal/services/auth"
	"github.com/josephburgess/gust-api/internal/services/store"
	"github.com/josephburgess/gust-api/internal/services/weather"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize services
	weatherClient := weather.NewClient(cfg.OpenWeatherAPIKey)

	userStore, err := store.NewUserStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize user store: %v", err)
	}
	defer userStore.Close()

	githubOAuth := auth.NewGitHubOAuth(
		cfg.GithubClientID,
		cfg.GithubClientSecret,
		cfg.GithubRedirectURI,
	)

	// Create router with all routes configured
	router := api.NewRouter(weatherClient, userStore, githubOAuth)

	// Start server
	log.Printf("Starting server on port %s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, router))
}
