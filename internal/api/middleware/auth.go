package middleware

import (
	"context"
	"net/http"

	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/services/store"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

func ApiKeyAuth(userStore *store.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.URL.Query().Get("api_key")

			if apiKey == "" {
				logging.Warn("API key is missing in request")
				http.Error(w, "API key is required", http.StatusUnauthorized)
				return
			}

			user, err := userStore.ValidateAPIKey(apiKey)
			if err != nil {
				logging.Warn("Invalid API key attempted: %s", apiKey)
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			logging.Info("Authenticated user: %s", user.Login)

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
