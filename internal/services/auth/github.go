package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/models"
)

var githubHTTPClient = &http.Client{Timeout: 15 * time.Second}

type GitHubOAuth struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	states       sync.Map
}

func NewGitHubOAuth(clientID, clientSecret, redirectURI string) *GitHubOAuth {
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/api/auth/callback"
	}

	return &GitHubOAuth{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
	}
}

func (g *GitHubOAuth) GetAuthURL() (string, string) {
	state := generateToken()
	g.states.Store(state, true)

	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=user:email,public_repo",
		g.ClientID,
		url.QueryEscape(g.RedirectURI),
		state,
	)

	return authURL, state
}

func (g *GitHubOAuth) ExchangeCodeForToken(code, state string) (string, error) {
	if state == "" {
		return "", fmt.Errorf("state parameter is required")
	}
	if _, loaded := g.states.LoadAndDelete(state); !loaded {
		logging.Warn("Invalid or expired state parameter: %s", state)
		return "", fmt.Errorf("invalid or expired state parameter")
	}

	logging.Info("Exchanging code for token with GitHub")
	tokenURL := "https://github.com/login/oauth/access_token"
	resp, err := githubHTTPClient.PostForm(tokenURL, url.Values{
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

	resp, err := githubHTTPClient.Do(req)
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

func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
