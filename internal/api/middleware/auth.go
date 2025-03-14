package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/models"
	"github.com/josephburgess/breeze/internal/services/store"
)

type contextKey string

const (
	UserContextKey      contextKey = "user"
	CustomApiContextKey contextKey = "custom-api-user"
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

			if !strings.HasPrefix(apiKey, "gust_") {
				logging.Info("User has provided a custom API key")
				user := &models.User{
					ID:        0,
					GithubID:  0,
					Login:     "custom-api-user",
					CreatedAt: time.Now(),
					LastLogin: time.Now(),
				}

				ctx := context.WithValue(r.Context(), UserContextKey, user)
				ctx = context.WithValue(ctx, CustomApiContextKey, apiKey)

				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			user, dailyLimit, dailyUsed, resetTime, err := userStore.ValidateAPIKey(apiKey)

			if dailyLimit > 0 {
				remaining := max(dailyLimit-dailyUsed, 0)

				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", dailyLimit))
				w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
				w.Header().Set("X-RateLimit-Reset", resetTime.Format(time.RFC3339))
			}

			if err != nil {
				if rateLimitErr, ok := err.(*store.RateLimitError); ok {
					http.Error(w, rateLimitErr.Message, http.StatusTooManyRequests)
					return
				}
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
