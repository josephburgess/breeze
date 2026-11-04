package respond

import (
	"encoding/json"
	"net/http"
)

// Error codes are the contract with gust; messages are for people.
const (
	CodeBadRequest  = "bad_request"
	CodeMissingKey  = "missing_key"
	CodeInvalidKey  = "invalid_key"
	CodeRateLimited = "rate_limited"
	CodeNotFound    = "not_found"
	CodeUpstream    = "upstream_error"
	CodeAuthFailed  = "auth_failed"
	CodeInternal    = "internal_error"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, ErrorBody{Code: code, Message: message})
}
