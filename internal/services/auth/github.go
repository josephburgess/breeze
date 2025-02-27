package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/google/uuid"
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

	return fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=user:email,public_repo",
		g.ClientID,
		url.QueryEscape(g.RedirectURI),
		state,
	), state
}

func (g *GitHubOAuth) ExchangeCodeForToken(code, state string) (string, error) {
	if state != "" && !g.States[state] {
		return "", fmt.Errorf("invalid state parameter")
	}

	if state != "" {
		delete(g.States, state)
	}

	tokenURL := "https://github.com/login/oauth/access_token"
	resp, err := http.PostForm(tokenURL, url.Values{
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"code":          {code},
		"redirect_uri":  {g.RedirectURI},
	})
	if err != nil {
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	token := values.Get("access_token")
	if token == "" {
		errorMsg := values.Get("error_description")
		if errorMsg == "" {
			errorMsg = "No access token received"
		}
		return "", fmt.Errorf("github oauth error: %s", errorMsg)
	}

	return token, nil
}

func (g *GitHubOAuth) GetUserInfo(token string) (*models.User, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user info request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var user models.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	user.Token = token
	return &user, nil
}
