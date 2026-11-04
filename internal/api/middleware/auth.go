package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/josephburgess/breeze/internal/api/respond"
	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/models"
	"github.com/josephburgess/breeze/internal/services/store"
)

type contextKey string

const (
	UserContextKey      contextKey = "user"
	CustomApiContextKey contextKey = "custom-api-user"
)

type KeySpender interface {
	Spend(ctx context.Context, apiKey string) (store.Usage, error)
}

func ApiKeyAuth(keys KeySpender) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := requestKey(r)
			if apiKey == "" {
				respond.Error(w, http.StatusUnauthorized, respond.CodeMissingKey, "an API key is required")
				return
			}

			// a key without the gust_ prefix is the user's own OpenWeather key
			if !strings.HasPrefix(apiKey, "gust_") {
				ctx := context.WithValue(r.Context(), UserContextKey, &models.User{Login: "custom-api-user"})
				ctx = context.WithValue(ctx, CustomApiContextKey, apiKey)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			usage, err := keys.Spend(r.Context(), apiKey)
			if usage.Limit > 0 {
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(usage.Limit))
				w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(usage.Remaining))
				w.Header().Set("X-RateLimit-Reset", usage.Reset.Format(time.RFC3339))
			}

			switch {
			case errors.Is(err, store.ErrRateLimited):
				respond.Error(w, http.StatusTooManyRequests, respond.CodeRateLimited,
					"rate limit exceeded: maximum of "+strconv.Itoa(usage.Limit)+" requests per day")
				return
			case errors.Is(err, store.ErrInvalidKey):
				respond.Error(w, http.StatusUnauthorized, respond.CodeInvalidKey, "invalid API key")
				return
			case err != nil:
				logging.Error("Failed to check API key", err)
				respond.Error(w, http.StatusInternalServerError, respond.CodeInternal, "could not check the API key")
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, usage.User)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// requestKey reads the Authorization header. The api_key query parameter
// stays for gust versions that do not send the header.
func requestKey(r *http.Request) string {
	if key, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return strings.TrimSpace(key)
	}
	return r.URL.Query().Get("api_key")
}
