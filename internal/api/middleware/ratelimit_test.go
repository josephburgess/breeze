package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/josephburgess/breeze/internal/api/middleware"
	"github.com/stretchr/testify/assert"
)

func TestPerIP_BlocksAnIPAfterTheLimit(t *testing.T) {
	limited := middleware.PerIP(2, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	var codes []int

	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/api/cities/search?q=x", nil)
		req.RemoteAddr = "10.0.0.1:5000"
		rr := httptest.NewRecorder()
		limited.ServeHTTP(rr, req)
		codes = append(codes, rr.Code)
	}

	assert.Equal(t, []int{200, 200, 429}, codes)
}

func TestPerIP_CountsEachIPApart(t *testing.T) {
	limited := middleware.PerIP(1, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	var codes []int

	for _, ip := range []string{"10.0.0.1", "10.0.0.2"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", ip+", 172.18.0.1")
		rr := httptest.NewRecorder()
		limited.ServeHTTP(rr, req)
		codes = append(codes, rr.Code)
	}

	assert.Equal(t, []int{200, 200}, codes)
}
