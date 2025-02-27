package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/josephburgess/gust-api/internal/api/middleware"
	"github.com/josephburgess/gust-api/internal/models"
)

type UserHandler struct{}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(middleware.UserContextKey).(*models.User)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"github_id": user.ID,
		"login":     user.Login,
		"name":      user.Name,
		"email":     user.Email,
		"avatar":    user.AvatarURL,
	})
}
