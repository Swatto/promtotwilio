package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(3)

	for i := range 3 {
		if !rl.Allow() {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	if rl.Allow() {
		t.Fatal("4th request should be rejected")
	}
}

func TestRateLimiter_WindowRefill(t *testing.T) {
	rl := &RateLimiter{
		tokens:   0,
		max:      2,
		lastFill: time.Now().Add(-2 * time.Minute), // window already expired
		interval: time.Minute,
	}

	if !rl.Allow() {
		t.Fatal("request after window expiry should be allowed")
	}
	if !rl.Allow() {
		t.Fatal("second request in new window should be allowed")
	}
	if rl.Allow() {
		t.Fatal("third request should be rejected (limit is 2)")
	}
}

func TestRateLimiter_Wrap_AllowsWithinLimit(t *testing.T) {
	rl := NewRateLimiter(2)
	called := 0
	handler := rl.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))

	for range 2 {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/send", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	}

	if called != 2 {
		t.Fatalf("handler should have been called 2 times, got %d", called)
	}
}

func TestRateLimiter_Wrap_RejectsOverLimit(t *testing.T) {
	rl := NewRateLimiter(1)
	handler := rl.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request: allowed
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, httptest.NewRequest(http.MethodPost, "/send", nil))
	if w1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", w1.Code)
	}

	// Second request: rejected
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/send", nil))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", w2.Code)
	}
}

func TestLogRequests_NginxFormat(t *testing.T) {
	h := LogRequests("nginx", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("hello"))
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if w.Body.String() != "hello" {
		t.Fatalf("expected body %q, got %q", "hello", w.Body.String())
	}
}

func TestLogRequests_SimpleFormat(t *testing.T) {
	h := LogRequests("simple", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/send", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("expected body %q, got %q", "ok", w.Body.String())
	}
}

func TestLogRequests_DefaultIsSimple(t *testing.T) {
	h := LogRequests("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestResponseRecorder_TracksBytes(t *testing.T) {
	w := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: w}

	rec.Write([]byte("abc"))
	rec.Write([]byte("de"))

	if rec.bytes != 5 {
		t.Fatalf("expected 5 bytes, got %d", rec.bytes)
	}
	if rec.status != http.StatusOK {
		t.Fatalf("expected implicit 200, got %d", rec.status)
	}
}

func TestResponseRecorder_ExplicitStatus(t *testing.T) {
	w := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: w}

	rec.WriteHeader(http.StatusNotFound)
	rec.WriteHeader(http.StatusOK) // second call should be ignored by recorder

	if rec.status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.status)
	}
}
