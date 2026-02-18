package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

type responseRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	bytes       int
}

func (r *responseRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.status = http.StatusOK
		r.wroteHeader = true
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// RateLimiter implements a fixed-window rate limiter. Tokens refill to max at
// the start of each window (interval). Safe for concurrent use.
type RateLimiter struct {
	mu       sync.Mutex
	tokens   int
	max      int
	lastFill time.Time
	interval time.Duration
}

// NewRateLimiter creates a rate limiter allowing requestsPerMinute requests
// per one-minute window.
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		tokens:   requestsPerMinute,
		max:      requestsPerMinute,
		lastFill: time.Now(),
		interval: time.Minute,
	}
}

// Allow consumes one token and returns true, or returns false if the limit
// has been reached for the current window.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if time.Since(rl.lastFill) >= rl.interval {
		rl.tokens = rl.max
		rl.lastFill = time.Now()
	}

	if rl.tokens <= 0 {
		return false
	}
	rl.tokens--
	return true
}

// Wrap returns an http.Handler that rejects requests with 429 when the rate
// limit is exceeded.
func (rl *RateLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow() {
			slog.Warn("rate limit exceeded", "method", r.Method, "path", r.URL.Path)
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LogRequests returns middleware that logs each HTTP request.
// format selects the output style:
//   - "simple" (or ""): structured slog line with method, path, status, bytes, duration
//   - "nginx": nginx combined log format
func LogRequests(format string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		if format == "nginx" {
			orDash := func(s string) string {
				if s == "" {
					return "-"
				}
				return s
			}
			if _, err := fmt.Fprintf(os.Stdout, "%s - - [%s] \"%s %s %s\" %d %d \"%s\" \"%s\" \"%s\"\n",
				r.RemoteAddr,
				start.Format("02/Jan/2006:15:04:05 -0700"),
				r.Method,
				r.RequestURI,
				r.Proto,
				rec.status,
				rec.bytes,
				orDash(r.Referer()),
				orDash(r.UserAgent()),
				orDash(r.Header.Get("X-Forwarded-For")),
			); err != nil {
				slog.Error("failed to write access log", "error", err)
			}
		} else {
			slog.Info("http request",
				"method", r.Method,
				"path", r.RequestURI,
				"status", rec.status,
				"bytes", rec.bytes,
				"duration", time.Since(start),
			)
		}
	})
}
