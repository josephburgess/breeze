package respond_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/josephburgess/breeze/internal/api/respond"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestError_WritesCodeAndMessageAsJSON(t *testing.T) {
	rr := httptest.NewRecorder()

	respond.Error(rr, http.StatusTooManyRequests, respond.CodeRateLimited, "slow down")

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	var body respond.ErrorBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, respond.ErrorBody{Code: "rate_limited", Message: "slow down"}, body)
}
