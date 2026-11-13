package logging

import (
	"net/http"
	"time"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)

		Logger.Infow(
			"HTTP Request",
			"remote_addr", r.RemoteAddr,
			"method", r.Method,
			"uri", RedactedURI(r),
			"duration", time.Since(start),
		)
	})
}

// RedactedURI is the request URI without the api_key value.
func RedactedURI(r *http.Request) string {
	query := r.URL.Query()
	if !query.Has("api_key") {
		return r.URL.RequestURI()
	}
	query.Set("api_key", "REDACTED")
	u := *r.URL
	u.RawQuery = query.Encode()
	return u.RequestURI()
}
