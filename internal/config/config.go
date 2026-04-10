package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/josephburgess/breeze/internal/logging"
)

type Config struct {
	Port               string
	DBPath             string
	OpenWeatherAPIKey  string
	GithubClientID     string
	GithubClientSecret string
	GithubRedirectURI  string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		logging.Warn(".env file not found, using environment variables")
	}

	port := getEnv("PORT", "8080")
	dbPath := getEnv("DB_PATH", "gust.db")
	openWeatherAPIKey := getEnv("OPENWEATHER_API_KEY", "")
	githubClientID := getEnv("GITHUB_CLIENT_ID", "")
	githubClientSecret := getEnv("GITHUB_CLIENT_SECRET", "")
	githubRedirectURI := getEnv("GITHUB_REDIRECT_URI", "http://localhost:8080/api/auth/callback")

	if openWeatherAPIKey == "" {
		return nil, fmt.Errorf("missing required environment variable: OPENWEATHER_API_KEY")
	}
	if githubClientID == "" || githubClientSecret == "" {
		return nil, fmt.Errorf("missing required environment variables: GITHUB_CLIENT_ID and/or GITHUB_CLIENT_SECRET")
	}

	logging.Info("Configuration loaded successfully")

	return &Config{
		Port:               port,
		DBPath:             dbPath,
		OpenWeatherAPIKey:  openWeatherAPIKey,
		GithubClientID:     githubClientID,
		GithubClientSecret: githubClientSecret,
		GithubRedirectURI:  githubRedirectURI,
	}, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
