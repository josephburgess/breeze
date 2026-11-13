package logging_test

import (
	"net/http/httptest"
	"testing"

	"github.com/josephburgess/breeze/internal/logging"
	"github.com/stretchr/testify/assert"
)

func TestRedactedURI_HidesAPIKey(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/weather/London?api_key=gust_secret&units=metric", nil)

	assert.Equal(t, "/api/weather/London?api_key=REDACTED&units=metric", logging.RedactedURI(r))
}

func TestRedactedURI_KeepsURIWithoutKey(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/cities/search?q=New+York", nil)

	assert.Equal(t, "/api/cities/search?q=New+York", logging.RedactedURI(r))
}
