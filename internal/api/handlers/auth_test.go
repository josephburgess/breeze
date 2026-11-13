package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/josephburgess/breeze/internal/api/handlers"
	"github.com/josephburgess/breeze/internal/services/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestAuth_ThenCallbackRedirectsToThatPort(t *testing.T) {
	h := handlers.NewAuthHandler(auth.NewFlow(auth.NewGitHub("id", "secret"), nil))
	rr := httptest.NewRecorder()
	h.RequestAuth(rr, httptest.NewRequest(http.MethodGet, "/api/auth/request?callback_port=5555", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Contains(t, body["url"], "localhost%3A5555")

	rr = httptest.NewRecorder()
	h.Callback(rr, httptest.NewRequest(http.MethodGet, "/api/auth/callback?code=c&state="+body["state"], nil))

	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Equal(t, "http://localhost:5555/callback?code=c&state="+body["state"], rr.Header().Get("Location"))
}

func TestRequestAuth_RejectsBadPort(t *testing.T) {
	h := handlers.NewAuthHandler(auth.NewFlow(auth.NewGitHub("id", "secret"), nil))
	rr := httptest.NewRecorder()

	h.RequestAuth(rr, httptest.NewRequest(http.MethodGet, "/api/auth/request?callback_port=abc", nil))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCallback_UnknownStateIsRejected(t *testing.T) {
	h := handlers.NewAuthHandler(auth.NewFlow(auth.NewGitHub("id", "secret"), nil))
	rr := httptest.NewRecorder()

	h.Callback(rr, httptest.NewRequest(http.MethodGet, "/api/auth/callback?code=c&state=forged", nil))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExchangeToken_ForgedStateFails(t *testing.T) {
	h := handlers.NewAuthHandler(auth.NewFlow(auth.NewGitHub("id", "secret"), nil))
	rr := httptest.NewRecorder()

	h.ExchangeToken(rr, httptest.NewRequest(http.MethodPost, "/api/auth/exchange", strings.NewReader(`{"code":"c","state":"forged","callback_port":9876}`)))

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), `"auth_failed"`)
}
