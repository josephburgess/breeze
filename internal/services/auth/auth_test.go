package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubOAuth_GetAuthURL(t *testing.T) {
	clientID := "test-client-id"
	clientSecret := "test-client-secret"
	redirectURI := "http://localhost:8080/callback"

	githubOAuth := NewGitHubOAuth(clientID, clientSecret, redirectURI)

	url, state := githubOAuth.GetAuthURL()

	assert.Contains(t, url, "https://github.com/login/oauth/authorize")
	assert.Contains(t, url, "client_id="+clientID)
	assert.Contains(t, url, "redirect_uri=")
	assert.Contains(t, url, "state="+state)
	assert.Contains(t, url, "scope=user:email,public_repo")
	assert.True(t, githubOAuth.States[state])
}

func TestGitHubOAuth_ExchangeCodeForToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

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

	clientID := "test-client-id"
	clientSecret := "test-client-secret"
	redirectURI := "http://localhost:8080/callback"

	githubOAuth := NewGitHubOAuth(clientID, clientSecret, redirectURI)

	testState := "test-state"
	githubOAuth.States[testState] = true
}

func TestGitHubOAuth_GetUserInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": 12345,
			"login": "testuser",
			"name": "Test User",
			"email": "test@example.com"
		}`))
	}))
	server.Close()
}

func TestNewGitHubOAuth(t *testing.T) {
	clientID := "client-id"
	clientSecret := "client-secret"
	redirectURI := "http://custom-redirect.com/callback"

	oauth := NewGitHubOAuth(clientID, clientSecret, redirectURI)

	assert.Equal(t, clientID, oauth.ClientID)
	assert.Equal(t, clientSecret, oauth.ClientSecret)
	assert.Equal(t, redirectURI, oauth.RedirectURI)
	assert.NotNil(t, oauth.States)

	oauth = NewGitHubOAuth(clientID, clientSecret, "")

	assert.Equal(t, clientID, oauth.ClientID)
	assert.Equal(t, clientSecret, oauth.ClientSecret)
	assert.Equal(t, "http://localhost:8080/api/auth/callback", oauth.RedirectURI)
	assert.NotNil(t, oauth.States)
}
