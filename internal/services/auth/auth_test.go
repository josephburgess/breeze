package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubOAuth_GetAuthURL(t *testing.T) {
	githubOAuth := NewGitHubOAuth("test-client-id", "test-client-secret", "http://localhost:8080/callback")

	url, state := githubOAuth.GetAuthURL()

	assert.Contains(t, url, "https://github.com/login/oauth/authorize")
	assert.Contains(t, url, "client_id=test-client-id")
	assert.Contains(t, url, "redirect_uri=")
	assert.Contains(t, url, "state="+state)
	assert.Contains(t, url, "scope=user:email,public_repo")

	_, stateStored := githubOAuth.states.Load(state)
	assert.True(t, stateStored)
}

func TestGitHubOAuth_ExchangeCodeForToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		err := r.ParseForm()
		require.NoError(t, err)
		assert.Equal(t, "test-client-id", r.FormValue("client_id"))
		assert.Equal(t, "test-client-secret", r.FormValue("client_secret"))
		assert.Equal(t, "test-code", r.FormValue("code"))
		assert.Equal(t, "http://localhost:8080/callback", r.FormValue("redirect_uri"))

		w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		w.Write([]byte("access_token=test-access-token&token_type=bearer&scope=user"))
	}))
	defer server.Close()

	// Point the package-level HTTP client at the test server
	origClient := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = origClient }()

	githubOAuth := NewGitHubOAuth("test-client-id", "test-client-secret", "http://localhost:8080/callback")
	githubOAuth.states.Store("test-state", true)

	// Override the token URL used internally
	origTokenURL := tokenURL
	tokenURL = server.URL
	defer func() { tokenURL = origTokenURL }()

	token, err := githubOAuth.ExchangeCodeForToken("test-code", "test-state")

	require.NoError(t, err)
	assert.Equal(t, "test-access-token", token)

	// State should be consumed
	_, stateStored := githubOAuth.states.Load("test-state")
	assert.False(t, stateStored)
}

func TestGitHubOAuth_ExchangeCodeForToken_InvalidState(t *testing.T) {
	githubOAuth := NewGitHubOAuth("test-client-id", "test-client-secret", "http://localhost:8080/callback")

	_, err := githubOAuth.ExchangeCodeForToken("test-code", "bad-state")
	assert.ErrorContains(t, err, "invalid or expired state")
}

func TestGitHubOAuth_ExchangeCodeForToken_EmptyState(t *testing.T) {
	githubOAuth := NewGitHubOAuth("test-client-id", "test-client-secret", "http://localhost:8080/callback")

	_, err := githubOAuth.ExchangeCodeForToken("test-code", "")
	assert.ErrorContains(t, err, "state parameter is required")
}

func TestGitHubOAuth_GetUserInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":12345,"login":"testuser","name":"Test User","email":"test@example.com"}`))
	}))
	defer server.Close()

	origClient := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = origClient }()

	origUserURL := userInfoURL
	userInfoURL = server.URL
	defer func() { userInfoURL = origUserURL }()

	githubOAuth := NewGitHubOAuth("id", "secret", "http://localhost/callback")
	user, err := githubOAuth.GetUserInfo("test-token")

	require.NoError(t, err)
	assert.Equal(t, int64(12345), user.GithubID)
	assert.Equal(t, "testuser", user.Login)
}

func TestNewGitHubOAuth(t *testing.T) {
	oauth := NewGitHubOAuth("client-id", "client-secret", "http://custom-redirect.com/callback")

	assert.Equal(t, "client-id", oauth.ClientID)
	assert.Equal(t, "client-secret", oauth.ClientSecret)
	assert.Equal(t, "http://custom-redirect.com/callback", oauth.RedirectURI)

	// Default redirect URI when empty
	oauth = NewGitHubOAuth("client-id", "client-secret", "")
	assert.Equal(t, "http://localhost:8080/api/auth/callback", oauth.RedirectURI)
}
