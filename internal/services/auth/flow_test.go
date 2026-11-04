package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/josephburgess/breeze/internal/services/auth"
	"github.com/josephburgess/breeze/internal/services/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlow_BeginThenCompleteIssuesKey(t *testing.T) {
	var redirectSent string
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/oauth/access_token":
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "the-code", r.FormValue("code"))
			assert.Equal(t, "secret", r.FormValue("client_secret"))
			redirectSent = r.FormValue("redirect_uri")
			w.Write([]byte("access_token=tok"))
		case "/user":
			assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
			w.Write([]byte(`{"id":42,"login":"joe"}`))
		}
	}))
	defer github.Close()
	users, err := store.NewUserStore(":memory:")
	require.NoError(t, err)
	defer users.Close()
	flow := auth.NewFlow(&auth.GitHub{ClientID: "id", ClientSecret: "secret", WebURL: github.URL, APIURL: github.URL}, users)

	authURL, state := flow.Begin(9876)
	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9876/callback", parsed.Query().Get("redirect_uri"))
	assert.Equal(t, state, parsed.Query().Get("state"))

	login, key, err := flow.Complete(context.Background(), "the-code", state, 0)

	require.NoError(t, err)
	assert.Equal(t, "joe", login)
	assert.Equal(t, "http://localhost:9876/callback", redirectSent)
	_, err = users.Spend(context.Background(), key)
	assert.NoError(t, err)
}

func TestFlow_StateWorksOnce(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			w.Write([]byte(`{"id":42,"login":"joe"}`))
			return
		}
		w.Write([]byte("access_token=tok"))
	}))
	defer github.Close()
	users, err := store.NewUserStore(":memory:")
	require.NoError(t, err)
	defer users.Close()
	flow := auth.NewFlow(&auth.GitHub{WebURL: github.URL, APIURL: github.URL}, users)
	_, state := flow.Begin(9876)

	_, _, err = flow.Complete(context.Background(), "code", state, 0)
	require.NoError(t, err)
	_, _, err = flow.Complete(context.Background(), "code", state, 0)

	assert.ErrorIs(t, err, auth.ErrInvalidState)
}

func TestFlow_UnknownStateIsRejected(t *testing.T) {
	flow := auth.NewFlow(&auth.GitHub{}, nil)

	_, _, err := flow.Complete(context.Background(), "code", "forged", 0)

	assert.ErrorIs(t, err, auth.ErrInvalidState)
}

func TestFlow_ExpiredStateIsRejected(t *testing.T) {
	flow := auth.NewFlow(&auth.GitHub{}, nil)
	_, state := flow.Begin(9876)
	flow.Now = func() time.Time { return time.Now().Add(time.Hour) }

	_, _, err := flow.Complete(context.Background(), "code", state, 0)

	assert.ErrorIs(t, err, auth.ErrInvalidState)
}

func TestFlow_ParallelLoginsKeepTheirOwnRedirect(t *testing.T) {
	flow := auth.NewFlow(&auth.GitHub{}, nil)

	_, stateA := flow.Begin(1111)
	_, stateB := flow.Begin(2222)

	redirectA, ok := flow.RedirectFor(stateA)
	require.True(t, ok)
	redirectB, ok := flow.RedirectFor(stateB)
	require.True(t, ok)
	assert.Equal(t, "http://localhost:1111/callback", redirectA)
	assert.Equal(t, "http://localhost:2222/callback", redirectB)
}
