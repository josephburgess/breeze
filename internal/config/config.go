package config

import (
	"os"
	"strings"

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
	JWTSecret          string
	BaseServerURL      string
	IsProd             bool // Production mode flag
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
	githubRedirectURI := getEnv("GITHUB_REDIRECT_URI", "http://localhost:8080/api/auth/callback")
	jwtSecret := getEnv("JWT_SECRET", "")
	baseServerURL := getEnv("BASE_SERVER_URL", "")
	environment := getEnv("ENVIRONMENT", "development")

	isProd := false

	if strings.ToLower(environment) == "production" {
		isProd = true

		if baseServerURL == "" {
			logging.Error("Production mode requires BASE_SERVER_URL to be set", nil)
			logging.Warn("Falling back to development mode due to missing BASE_SERVER_URL")
			isProd = false
		}
	}

	if openWeatherAPIKey == "" {
		logging.Error("Missing required environment variable: OPENWEATHER_API_KEY", nil)
		os.Exit(1)
	}

	if githubClientID == "" || githubClientSecret == "" {
		logging.Error("Missing required environment variables: GITHUB_CLIENT_ID and/or GITHUB_CLIENT_SECRET", nil)
		os.Exit(1)
	}

	if jwtSecret == "" {
		logging.Error("Missing required environment variable: JWT_SECRET", nil)
		os.Exit(1)
	}

	if isProd {
		logging.Info("Running in PRODUCTION mode with base URL: %s", baseServerURL)
	} else {
		logging.Info("Running in DEVELOPMENT mode")
	}

	return &Config{
		Port:               port,
		DBPath:             dbPath,
		OpenWeatherAPIKey:  openWeatherAPIKey,
		GithubClientID:     githubClientID,
		GithubClientSecret: githubClientSecret,
		GithubRedirectURI:  githubRedirectURI,
		JWTSecret:          jwtSecret,
		BaseServerURL:      baseServerURL,
		IsProd:             isProd,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
