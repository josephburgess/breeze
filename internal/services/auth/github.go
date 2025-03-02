package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/models"
)

type GitHubOAuth struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	States       map[string]bool
}

func NewGitHubOAuth(clientID, clientSecret, redirectURI string) *GitHubOAuth {
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/api/auth/callback"
	}

	return &GitHubOAuth{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		States:       make(map[string]bool),
	}
}

func (g *GitHubOAuth) GetAuthURL() (string, string) {
	state := uuid.New().String()
	g.States[state] = true

	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=user:email,public_repo",
		g.ClientID,
		url.QueryEscape(g.RedirectURI),
		state,
	)

	return authURL, state
}

func (g *GitHubOAuth) ExchangeCodeForToken(code, state string) (string, error) {
	if state != "" && !g.States[state] {
		logging.Warn("Invalid state parameter received: %s", state)
		return "", fmt.Errorf("invalid state parameter")
	}

	if state != "" {
		delete(g.States, state)
	}

	logging.Info("Exchanging code for token with GitHub")
	tokenURL := "https://github.com/login/oauth/access_token"
	resp, err := http.PostForm(tokenURL, url.Values{
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"code":          {code},
		"redirect_uri":  {g.RedirectURI},
	})
	if err != nil {
		logging.Error("Token exchange request failed", err)
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.Error("Failed to read token response body", err)
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		logging.Error("Failed to parse token response", err)
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	token := values.Get("access_token")
	if token == "" {
		errorMsg := values.Get("error_description")
		if errorMsg == "" {
			errorMsg = "No access token received"
		}
		logging.Warn("GitHub OAuth error: %s", errorMsg)
		return "", fmt.Errorf("github oauth error: %s", errorMsg)
	}

	return token, nil
}

func (g *GitHubOAuth) GetUserInfo(token string) (*models.User, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		logging.Error("Failed to create request for user info", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logging.Error("User info request failed", err)
		return nil, fmt.Errorf("user info request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.Warn("GitHub API returned status: %d", resp.StatusCode)
		return nil, fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var githubResponse struct {
		ID    int64   `json:"id"`
		Login string  `json:"login"`
		Name  *string `json:"name"`
		Email *string `json:"email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&githubResponse); err != nil {
		logging.Error("Failed to decode user info", err)
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	user := &models.User{
		GithubID: githubResponse.ID,
		Login:    githubResponse.Login,
		Name:     githubResponse.Name,
		Email:    githubResponse.Email,
		Token:    token,
	}

	logging.Info("Successfully retrieved GitHub user: %s (ID: %d)", user.Login, user.GithubID)
	return user, nil
}
