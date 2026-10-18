package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/josephburgess/breeze/internal/models"
)

const githubTimeout = 10 * time.Second

var httpClient = &http.Client{Timeout: githubTimeout}

// GitHub is the OAuth adapter. It holds no per-login state.
type GitHub struct {
	ClientID     string
	ClientSecret string
	WebURL       string
	APIURL       string
}

func NewGitHub(clientID, clientSecret string) *GitHub {
	return &GitHub{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		WebURL:       "https://github.com",
		APIURL:       "https://api.github.com",
	}
}

func (g *GitHub) AuthURL(redirectURI, state string) string {
	params := url.Values{
		"client_id":    {g.ClientID},
		"redirect_uri": {redirectURI},
		"state":        {state},
		"scope":        {"read:user"},
	}
	return g.WebURL + "/login/oauth/authorize?" + params.Encode()
}

func (g *GitHub) Exchange(ctx context.Context, code, redirectURI string) (string, error) {
	form := url.Values{
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.WebURL+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if token := values.Get("access_token"); token != "" {
		return token, nil
	}
	if msg := values.Get("error_description"); msg != "" {
		return "", fmt.Errorf("github oauth: %s", msg)
	}
	return "", fmt.Errorf("github oauth: no access token")
}

func (g *GitHub) User(ctx context.Context, token string) (*models.User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.APIURL+"/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github user: status %d", resp.StatusCode)
	}

	var body struct {
		ID    int64   `json:"id"`
		Login string  `json:"login"`
		Name  *string `json:"name"`
		Email *string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode github user: %w", err)
	}
	return &models.User{GithubID: body.ID, Login: body.Login, Name: body.Name, Email: body.Email}, nil
}
