package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		os.Exit(1)
	}
	defer userStore.Close()

	githubOAuth := auth.NewGitHubOAuth(
		cfg.GithubClientID,
		cfg.GithubClientSecret,
		cfg.GithubRedirectURI,
	)

	handler := logging.Middleware(api.NewRouter(weatherClient, userStore, githubOAuth))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logging.Info("Starting server on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Error("Server error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logging.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logging.Error("Server forced to shutdown", err)
	}

	logging.Info("Server exited")
}
