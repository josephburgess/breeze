package logging

import (
	"net/http"
	"time"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)

		Logger.Infow(
			"HTTP Request",
			"remote_addr", r.RemoteAddr,
			"method", r.Method,
			"uri", r.RequestURI,
			"duration", duration,
		)
	})
}
