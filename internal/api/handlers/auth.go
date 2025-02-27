package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/josephburgess/breeze/internal/models"
	"github.com/josephburgess/breeze/internal/services/auth"
	"github.com/josephburgess/breeze/internal/services/store"
	"github.com/josephburgess/breeze/internal/templates"
)

type AuthHandler struct {
	githubOAuth *auth.GitHubOAuth
	userStore   *store.UserStore
}

func NewAuthHandler(githubOAuth *auth.GitHubOAuth, userStore *store.UserStore) *AuthHandler {
	return &AuthHandler{
		githubOAuth: githubOAuth,
		userStore:   userStore,
	}
}

func (h *AuthHandler) RequestAuth(w http.ResponseWriter, r *http.Request) {
	callbackPort := r.URL.Query().Get("callback_port")
	if callbackPort == "" {
		callbackPort = "9876"
	}

	callbackURL := fmt.Sprintf("http://localhost:%s/callback", callbackPort)
	h.githubOAuth.RedirectURI = callbackURL

	authURL, state := h.githubOAuth.GetAuthURL()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url":   authURL,
		"state": state,
	})
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if strings.HasPrefix(h.githubOAuth.RedirectURI, "http://localhost:") {
		http.Redirect(w, r, fmt.Sprintf("%s?code=%s&state=%s",
			h.githubOAuth.RedirectURI, code, state), http.StatusFound)
		return
	}

	h.handleGitHubCallback(w, code, state)
}

func (h *AuthHandler) handleGitHubCallback(w http.ResponseWriter, code, state string) {
	user, apiKey, err := h.handleGitHubAuthCode(code, state)
	if err != nil {
		http.Error(w, fmt.Sprintf("Authentication failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	if err := templates.RenderSuccessTemplate(w, user.Login, apiKey); err != nil {
		http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
	}
}

func (h *AuthHandler) handleGitHubAuthCode(code, state string) (*models.User, string, error) {
	token, err := h.githubOAuth.ExchangeCodeForToken(code, state)
	if err != nil {
		return nil, "", fmt.Errorf("failed to exchange code for token: %w", err)
	}

	user, err := h.githubOAuth.GetUserInfo(token)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user info: %w", err)
	}

	if err := h.userStore.SaveUser(user); err != nil {
		return nil, "", fmt.Errorf("failed to save user: %w", err)
	}

	credential, err := h.userStore.CreateAPICredential(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create API credential: %w", err)
	}

	return user, credential.ApiKey, nil
}

func (h *AuthHandler) ExchangeToken(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Code         string `json:"code"`
		CallbackPort int    `json:"callback_port"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.githubOAuth.RedirectURI = fmt.Sprintf("http://localhost:%d/callback", request.CallbackPort)

	user, apiKey, err := h.handleGitHubAuthCode(request.Code, "")
	if err != nil {
		http.Error(w, fmt.Sprintf("Authentication failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"api_key":     apiKey,
		"github_user": user.Login,
	})
}
