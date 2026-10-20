package main

import (
	"context"
	"errors"
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

const shutdownTimeout = 15 * time.Second

func main() {
	cfg := config.Load()

	weatherClient := weather.NewClient(cfg.OpenWeatherAPIKey, weather.DefaultBaseURL)

	userStore, err := store.NewUserStore(cfg.DBPath)
	if err != nil {
		logging.Error("Failed to initialize user store", err)
		os.Exit(1)
	}
	defer userStore.Close()

	authFlow := auth.NewFlow(auth.NewGitHub(cfg.GithubClientID, cfg.GithubClientSecret), userStore)

	router := api.NewRouter(weatherClient, userStore, authFlow)
	router.Use(logging.Middleware)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logging.Info("Starting server on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.Error("Server encountered an error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logging.Info("Shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logging.Error("Graceful shutdown failed", err)
	}
}
