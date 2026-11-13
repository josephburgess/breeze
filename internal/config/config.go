package config

import (
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
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		logging.Warn(".env file not found")
	}

	port := getEnv("PORT", "8080")
	dbPath := getEnv("DB_PATH", "gust.db")
	openWeatherAPIKey := getEnv("OPENWEATHER_API_KEY", "")
	githubClientID := getEnv("GITHUB_CLIENT_ID", "")
	githubClientSecret := getEnv("GITHUB_CLIENT_SECRET", "")

	if openWeatherAPIKey == "" {
		logging.Error("Missing required environment variable: OPENWEATHER_API_KEY", nil)
		os.Exit(1)
	}

	if githubClientID == "" || githubClientSecret == "" {
		logging.Error("Missing required environment variables: GITHUB_CLIENT_ID and/or GITHUB_CLIENT_SECRET", nil)
		os.Exit(1)
	}

	logging.Info("Configuration loaded successfully")

	return &Config{
		Port:               port,
		DBPath:             dbPath,
		OpenWeatherAPIKey:  openWeatherAPIKey,
		GithubClientID:     githubClientID,
		GithubClientSecret: githubClientSecret,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
