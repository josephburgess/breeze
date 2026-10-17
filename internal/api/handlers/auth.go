package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/josephburgess/breeze/internal/api/respond"
	"github.com/josephburgess/breeze/internal/logging"
	"github.com/josephburgess/breeze/internal/services/auth"
)

const defaultCallbackPort = 9876

type AuthHandler struct {
	flow *auth.Flow
}

func NewAuthHandler(flow *auth.Flow) *AuthHandler {
	return &AuthHandler{flow: flow}
}

func (h *AuthHandler) RequestAuth(w http.ResponseWriter, r *http.Request) {
	port := defaultCallbackPort
	if raw := r.URL.Query().Get("callback_port"); raw != "" {
		p, err := strconv.Atoi(raw)
		if err != nil || p < 1 || p > 65535 {
			respond.Error(w, http.StatusBadRequest, respond.CodeBadRequest, "callback_port must be a port number")
			return
		}
		port = p
	}

	authURL, state := h.flow.Begin(port)
	respond.JSON(w, http.StatusOK, map[string]string{"url": authURL, "state": state})
}

// Callback forwards a GitHub redirect to the gust that started the login.
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	redirectURI, ok := h.flow.RedirectFor(state)
	if !ok {
		respond.Error(w, http.StatusBadRequest, respond.CodeAuthFailed, "unknown or expired sign in")
		return
	}
	params := url.Values{"code": {r.URL.Query().Get("code")}, "state": {state}}
	http.Redirect(w, r, redirectURI+"?"+params.Encode(), http.StatusFound)
}

func (h *AuthHandler) ExchangeToken(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Code         string `json:"code"`
		State        string `json:"state"`
		CallbackPort int    `json:"callback_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respond.Error(w, http.StatusBadRequest, respond.CodeBadRequest, "invalid request body")
		return
	}

	login, apiKey, err := h.flow.Complete(r.Context(), request.Code, request.State, request.CallbackPort)
	if err != nil {
		logging.Error("GitHub sign in failed", err)
		respond.Error(w, http.StatusUnauthorized, respond.CodeAuthFailed, "GitHub sign in failed")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{"api_key": apiKey, "github_user": login})
}
