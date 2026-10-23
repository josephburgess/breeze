package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/josephburgess/breeze/internal/api/respond"
)

type window struct {
	start time.Time
	count int
}

// PerIP allows limit requests per client IP in each fixed window.
func PerIP(limit int, per time.Duration) func(http.Handler) http.Handler {
	var mu sync.Mutex
	windows := map[string]*window{}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			now := time.Now()

			mu.Lock()
			for k, win := range windows {
				if now.Sub(win.start) >= per {
					delete(windows, k)
				}
			}
			win, ok := windows[ip]
			if !ok {
				win = &window{start: now}
				windows[ip] = win
			}
			win.count++
			allowed := win.count <= limit
			mu.Unlock()

			if !allowed {
				respond.Error(w, http.StatusTooManyRequests, respond.CodeRateLimited, "too many requests, try again in a minute")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP takes the last X-Forwarded-For entry: the proxy in front of breeze
// appends it, so the client cannot forge it.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		entries := strings.Split(fwd, ",")
		return strings.TrimSpace(entries[len(entries)-1])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
