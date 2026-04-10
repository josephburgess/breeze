package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/models"
	"github.com/josephburgess/breeze/internal/templates"
)

type GitHubOAuthClient interface {
	GetAuthURL() (string, string)
	ExchangeCodeForToken(code, state string) (string, error)
	ExchangeCodeDirect(code string) (string, error)
	GetUserInfo(token string) (*models.User, error)
	SetRedirectURI(uri string)
	GetRedirectURI() string
}

type AuthUserStore interface {
	SaveUser(user *models.User) error
	GetOrCreateAPICredential(githubUserID int64) (*models.ApiCredential, error)
}

type AuthHandler struct {
	githubOAuth GitHubOAuthClient
	userStore   AuthUserStore
}

func NewAuthHandler(githubOAuth GitHubOAuthClient, userStore AuthUserStore) *AuthHandler {
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

	h.githubOAuth.SetRedirectURI(fmt.Sprintf("http://localhost:%s/callback", callbackPort))
	authURL, state := h.githubOAuth.GetAuthURL()
	logging.Info("Generated authentication URL: %s", authURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url":   authURL,
		"state": state,
	})
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if strings.HasPrefix(h.githubOAuth.GetRedirectURI(), "http://localhost:") {
		redirectURL := fmt.Sprintf("%s?code=%s&state=%s", h.githubOAuth.GetRedirectURI(), code, state)
		logging.Info("Redirecting to local callback: %s", redirectURL)
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	h.handleGitHubCallback(w, code, state)
}

func (h *AuthHandler) handleGitHubCallback(w http.ResponseWriter, code, state string) {
	token, err := h.githubOAuth.ExchangeCodeForToken(code, state)
	if err != nil {
		logging.Error("Failed to exchange code for token", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	user, apiKey, err := h.completeGitHubAuth(token)
	if err != nil {
		logging.Error("Authentication failed", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	logging.Info("User authenticated successfully: %s", user.Login)

	w.Header().Set("Content-Type", "text/html")
	if err := templates.RenderSuccessTemplate(w, user.Login, apiKey); err != nil {
		logging.Error("Failed to render template", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

func (h *AuthHandler) completeGitHubAuth(token string) (*models.User, string, error) {
	user, err := h.githubOAuth.GetUserInfo(token)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user info: %w", err)
	}

	if err := h.userStore.SaveUser(user); err != nil {
		return nil, "", fmt.Errorf("failed to save user: %w", err)
	}

	credential, err := h.userStore.GetOrCreateAPICredential(user.GithubID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create API credential: %w", err)
	}

	logging.Info("Successfully created API credential for user: %s", user.Login)
	return user, credential.ApiKey, nil
}

func (h *AuthHandler) ExchangeToken(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Code         string `json:"code"`
		CallbackPort int    `json:"callback_port"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logging.Error("Invalid request body", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// State was already consumed at the /callback redirect step; exchange code directly.
	token, err := h.githubOAuth.ExchangeCodeDirect(request.Code)
	if err != nil {
		logging.Error("Failed to exchange code for token", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	user, apiKey, err := h.completeGitHubAuth(token)
	if err != nil {
		logging.Error("Authentication failed", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	logging.Info("Token exchanged successfully for user: %s", user.Login)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"api_key":     apiKey,
		"github_user": user.Login,
	})
}
