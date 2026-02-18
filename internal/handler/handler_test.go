package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "missing AccountSid",
			cfg:     Config{Sender: "+1234567890", AuthToken: "tok"},
			wantErr: "AccountSid",
		},
		{
			name:    "missing Sender",
			cfg:     Config{AccountSid: "AC123", AuthToken: "tok"},
			wantErr: "Sender",
		},
		{
			name:    "missing auth entirely",
			cfg:     Config{AccountSid: "AC123", Sender: "+1234567890"},
			wantErr: "AuthToken",
		},
		{
			name:    "API key without secret",
			cfg:     Config{AccountSid: "AC123", Sender: "+1234567890", APIKey: "SK123"},
			wantErr: "APIKeySecret",
		},
		{
			name:    "negative rate limit",
			cfg:     Config{AccountSid: "AC123", Sender: "+1234567890", AuthToken: "tok", RateLimit: -1},
			wantErr: "RateLimit",
		},
		{
			name:    "invalid log format",
			cfg:     Config{AccountSid: "AC123", Sender: "+1234567890", AuthToken: "tok", LogFormat: "xml"},
			wantErr: "LogFormat",
		},
		{
			name: "valid with auth token",
			cfg:  Config{AccountSid: "AC123", Sender: "+1234567890", AuthToken: "tok"},
		},
		{
			name: "valid with API key",
			cfg:  Config{AccountSid: "AC123", Sender: "+1234567890", APIKey: "SK123", APIKeySecret: "sec"},
		},
		{
			name: "valid with nginx log format",
			cfg:  Config{AccountSid: "AC123", Sender: "+1234567890", AuthToken: "tok", LogFormat: "nginx"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestPing(t *testing.T) {
	h := NewWithClient(&Config{}, &MockTwilioClient{}, "test")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	h.Ping(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "ping" {
		t.Errorf("expected body 'ping', got %q", w.Body.String())
	}
}

func TestHealth(t *testing.T) {
	h := NewWithClient(&Config{}, &MockTwilioClient{}, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", resp.Version)
	}
}
