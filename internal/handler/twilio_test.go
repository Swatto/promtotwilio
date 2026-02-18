package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewTwilioClient_DefaultBaseURL(t *testing.T) {
	client := NewTwilioClient("AC123", "authUser", "authPass", "")

	if client.baseURL != defaultTwilioBaseURL {
		t.Errorf("expected baseURL %q, got %q", defaultTwilioBaseURL, client.baseURL)
	}
	if client.accountSid != "AC123" {
		t.Errorf("expected accountSid %q, got %q", "AC123", client.accountSid)
	}
	if client.authUser != "authUser" {
		t.Errorf("expected authUser %q, got %q", "authUser", client.authUser)
	}
	if client.authPassword != "authPass" {
		t.Errorf("expected authPassword %q, got %q", "authPass", client.authPassword)
	}
}

func TestNewTwilioClient_CustomBaseURL(t *testing.T) {
	client := NewTwilioClient("AC123", "authUser", "authPass", "https://custom.twilio.com")

	if client.baseURL != "https://custom.twilio.com" {
		t.Errorf("expected baseURL %q, got %q", "https://custom.twilio.com", client.baseURL)
	}
}

func TestTwilioHTTPClient_SendMessage_AccountSIDAuth(t *testing.T) {
	// Create a test server to verify the request
	var receivedAuthUser, receivedAuthPass string
	var receivedURL string
	var receivedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.Path
		user, pass, _ := r.BasicAuth()
		receivedAuthUser = user
		receivedAuthPass = pass

		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sid":"SM123"}`))
	}))
	defer server.Close()

	// Test with Account SID auth (same as authUser)
	client := NewTwilioClient("AC123456", "AC123456", "authToken123", server.URL)
	err := client.SendMessage("+15551234567", "+15559876543", "Test message")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the request was made correctly
	expectedURL := "/2010-04-01/Accounts/AC123456/Messages.json"
	if receivedURL != expectedURL {
		t.Errorf("expected URL %q, got %q", expectedURL, receivedURL)
	}

	if receivedAuthUser != "AC123456" {
		t.Errorf("expected auth user %q, got %q", "AC123456", receivedAuthUser)
	}
	if receivedAuthPass != "authToken123" {
		t.Errorf("expected auth pass %q, got %q", "authToken123", receivedAuthPass)
	}

	if !strings.Contains(receivedBody, "To=%2B15551234567") {
		t.Errorf("expected body to contain To parameter, got %q", receivedBody)
	}
	if !strings.Contains(receivedBody, "From=%2B15559876543") {
		t.Errorf("expected body to contain From parameter, got %q", receivedBody)
	}
	if !strings.Contains(receivedBody, "Body=Test+message") {
		t.Errorf("expected body to contain Body parameter, got %q", receivedBody)
	}
}

func TestTwilioHTTPClient_SendMessage_APIKeyAuth(t *testing.T) {
	// Create a test server to verify the request
	var receivedAuthUser, receivedAuthPass string
	var receivedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.Path
		user, pass, _ := r.BasicAuth()
		receivedAuthUser = user
		receivedAuthPass = pass

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sid":"SM123"}`))
	}))
	defer server.Close()

	// Test with API Key auth (authUser differs from accountSid)
	client := NewTwilioClient("AC123456", "SK789abc", "apiKeySecret", server.URL)
	err := client.SendMessage("+15551234567", "+15559876543", "Test message")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// URL should still use Account SID
	expectedURL := "/2010-04-01/Accounts/AC123456/Messages.json"
	if receivedURL != expectedURL {
		t.Errorf("expected URL %q, got %q", expectedURL, receivedURL)
	}

	// But auth should use API Key credentials
	if receivedAuthUser != "SK789abc" {
		t.Errorf("expected auth user %q (API Key), got %q", "SK789abc", receivedAuthUser)
	}
	if receivedAuthPass != "apiKeySecret" {
		t.Errorf("expected auth pass %q (API Key Secret), got %q", "apiKeySecret", receivedAuthPass)
	}
}

func TestTwilioHTTPClient_SendMessage_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":20003,"message":"Authenticate"}`))
	}))
	defer server.Close()

	client := NewTwilioClient("AC123456", "AC123456", "badToken", server.URL)
	err := client.SendMessage("+15551234567", "+15559876543", "Test message")

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "API error (status 401)") {
		t.Errorf("expected error to contain status code, got %q", err.Error())
	}
}

func TestNew_WithAuthToken(t *testing.T) {
	cfg := &Config{
		AccountSid: "AC123456",
		AuthToken:  "authToken123",
		Sender:     "+15551234567",
	}

	h := New(cfg, "1.0.0")

	// Verify handler was created
	if h == nil {
		t.Fatal("expected handler, got nil")
		return
	}
	if h.Config != cfg {
		t.Error("expected config to be set")
	}
	if h.Client == nil {
		t.Error("expected client to be set")
	}
}

func TestNew_WithAPIKey(t *testing.T) {
	cfg := &Config{
		AccountSid:   "AC123456",
		APIKey:       "SK789abc",
		APIKeySecret: "apiKeySecret",
		Sender:       "+15551234567",
	}

	h := New(cfg, "1.0.0")

	// Verify handler was created
	if h == nil {
		t.Fatal("expected handler, got nil")
		return
	}
	if h.Config != cfg {
		t.Error("expected config to be set")
	}
	if h.Client == nil {
		t.Error("expected client to be set")
	}
}

func TestTwilioHTTPClient_SendMessage_RetriesOn5xx(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"overloaded"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sid":"SM123"}`))
	}))
	defer server.Close()

	client := NewTwilioClient("AC123456", "AC123456", "authToken", server.URL)
	err := client.SendMessage("+15551234567", "+15559876543", "Test")

	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if attempt != 3 {
		t.Errorf("expected 3 attempts, got %d", attempt)
	}
}

func TestNew_APIKeyTakesPrecedence(t *testing.T) {
	// Create a test server to verify which credentials are used
	var receivedAuthUser string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, _ := r.BasicAuth()
		receivedAuthUser = user
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sid":"SM123"}`))
	}))
	defer server.Close()

	cfg := &Config{
		AccountSid:    "AC123456",
		AuthToken:     "authToken123", // Both are provided
		APIKey:        "SK789abc",     // API Key should take precedence
		APIKeySecret:  "apiKeySecret",
		Sender:        "+15551234567",
		TwilioBaseURL: server.URL,
	}

	h := New(cfg, "1.0.0")
	err := h.Client.SendMessage("+15559876543", cfg.Sender, "Test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// API Key should be used, not Account SID
	if receivedAuthUser != "SK789abc" {
		t.Errorf("expected API Key %q to take precedence, but got %q", "SK789abc", receivedAuthUser)
	}
}
