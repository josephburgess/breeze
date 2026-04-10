package logging

import (
	"net/http"
	"time"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("HTTP Request",
			"method", r.Method,
			"uri", r.RequestURI,
			"remote_addr", r.RemoteAddr,
			"duration", time.Since(start),
		)
	})
}
