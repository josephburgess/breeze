package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DBPath             string
	OpenWeatherAPIKey  string
	GithubClientID     string
	GithubClientSecret string
	GithubRedirectURI  string
	JWTSecret          string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	port := getEnv("PORT", "8080")
	dbPath := getEnv("DB_PATH", "gust.db")
	openWeatherAPIKey := getEnv("OPENWEATHER_API_KEY", "")
	githubClientID := getEnv("GITHUB_CLIENT_ID", "")
	githubClientSecret := getEnv("GITHUB_CLIENT_SECRET", "")
	githubRedirectURI := getEnv("GITHUB_REDIRECT_URI", "http://localhost:8080/api/auth/callback")
	jwtSecret := getEnv("JWT_SECRET", "")

	if openWeatherAPIKey == "" {
		log.Fatal("OPENWEATHER_API_KEY not set")
	}

	if githubClientID == "" || githubClientSecret == "" {
		log.Fatal("GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET must be set")
	}

	if jwtSecret == "" {
		log.Fatal("JWT_SECRET not set")
	}

	return &Config{
		Port:               port,
		DBPath:             dbPath,
		OpenWeatherAPIKey:  openWeatherAPIKey,
		GithubClientID:     githubClientID,
		GithubClientSecret: githubClientSecret,
		GithubRedirectURI:  githubRedirectURI,
		JWTSecret:          jwtSecret,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
