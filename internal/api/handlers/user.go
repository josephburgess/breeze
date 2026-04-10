package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/models"
)

type QuotaStore interface {
	GetAPIKeyQuota(apiKey string) (limit, used int, resetAt time.Time, err error)
}

type UserHandler struct {
	userStore QuotaStore
}

func NewUserHandler(userStore QuotaStore) *UserHandler {
	return &UserHandler{userStore: userStore}
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) GetQuota(w http.ResponseWriter, r *http.Request) {
	// Custom API key users are not rate-limited by breeze
	if _, ok := r.Context().Value(middleware.CustomApiContextKey).(string); ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"daily_limit": 0,
			"daily_used":  0,
			"remaining":   -1,
			"reset_at":    (*time.Time)(nil),
			"unlimited":   true,
		})
		return
	}

	apiKey := r.URL.Query().Get("api_key")
	if strings.HasPrefix(apiKey, "gust_") {
		limit, used, resetAt, err := h.userStore.GetAPIKeyQuota(apiKey)
		if err != nil {
			logging.Error("Failed to get quota", err)
			http.Error(w, "Failed to get quota", http.StatusInternalServerError)
			return
		}

		remaining := limit - used
		if remaining < 0 {
			remaining = 0
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"daily_limit": limit,
			"daily_used":  used,
			"remaining":   remaining,
			"reset_at":    resetAt,
			"unlimited":   false,
		})
		return
	}

	http.Error(w, "Unable to determine quota", http.StatusBadRequest)
}
