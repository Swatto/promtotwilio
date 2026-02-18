package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/swatto/promtotwilio/internal/handler"
)

// setEnv is a helper that sets multiple env vars and returns a cleanup func.
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

// minimalEnv returns the minimum env vars required for a valid config.
func minimalEnv() map[string]string {
	return map[string]string{
		"SID":    "AC_test",
		"TOKEN":  "tok_test",
		"SENDER": "+15550000000",
	}
}

// ---------- loadConfig tests ----------

func TestLoadConfig_Defaults(t *testing.T) {
	setEnv(t, minimalEnv())

	cfg, port := loadConfig()

	if port != "9090" {
		t.Errorf("expected default port 9090, got %q", port)
	}
	if cfg.MaxMessageLength != 150 {
		t.Errorf("expected default max message length 150, got %d", cfg.MaxMessageLength)
	}
	if cfg.RateLimit != 0 {
		t.Errorf("expected default rate limit 0, got %d", cfg.RateLimit)
	}
	if cfg.SendResolved {
		t.Error("expected SendResolved to be false by default")
	}
}

func TestLoadConfig_CustomPort(t *testing.T) {
	env := minimalEnv()
	env["PORT"] = "8080"
	setEnv(t, env)

	_, port := loadConfig()
	if port != "8080" {
		t.Errorf("expected port 8080, got %q", port)
	}
}

func TestLoadConfig_MaxMessageLength(t *testing.T) {
	tests := []struct {
		name     string
		envVal   string
		expected int
	}{
		{"valid", "200", 200},
		{"invalid string", "abc", 150},
		{"zero", "0", 150},
		{"negative", "-5", 150},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := minimalEnv()
			env["MAX_MESSAGE_LENGTH"] = tt.envVal
			setEnv(t, env)

			cfg, _ := loadConfig()
			if cfg.MaxMessageLength != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, cfg.MaxMessageLength)
			}
		})
	}
}

func TestLoadConfig_RateLimit(t *testing.T) {
	tests := []struct {
		name     string
		envVal   string
		expected int
	}{
		{"valid", "60", 60},
		{"invalid string", "abc", 0},
		{"zero", "0", 0},
		{"negative", "-1", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := minimalEnv()
			env["RATE_LIMIT"] = tt.envVal
			setEnv(t, env)

			cfg, _ := loadConfig()
			if cfg.RateLimit != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, cfg.RateLimit)
			}
		})
	}
}

func TestLoadConfig_SendResolved(t *testing.T) {
	env := minimalEnv()
	env["SEND_RESOLVED"] = "true"
	setEnv(t, env)

	cfg, _ := loadConfig()
	if !cfg.SendResolved {
		t.Error("expected SendResolved to be true")
	}
}

func TestLoadConfig_Receivers(t *testing.T) {
	env := minimalEnv()
	env["RECEIVER"] = "+1111, +2222, +3333"
	setEnv(t, env)

	cfg, _ := loadConfig()
	if len(cfg.Receivers) != 3 {
		t.Fatalf("expected 3 receivers, got %d", len(cfg.Receivers))
	}
	if cfg.Receivers[1] != "+2222" {
		t.Errorf("expected second receiver +2222, got %q", cfg.Receivers[1])
	}
}

func TestLoadConfig_APIKeyFields(t *testing.T) {
	env := minimalEnv()
	env["API_KEY"] = "SK_key"
	env["API_KEY_SECRET"] = "secret"
	setEnv(t, env)

	cfg, _ := loadConfig()
	if cfg.APIKey != "SK_key" {
		t.Errorf("expected API_KEY SK_key, got %q", cfg.APIKey)
	}
	if cfg.APIKeySecret != "secret" {
		t.Errorf("expected API_KEY_SECRET secret, got %q", cfg.APIKeySecret)
	}
}

func TestLoadConfig_OptionalFields(t *testing.T) {
	env := minimalEnv()
	env["TWILIO_BASE_URL"] = "http://localhost:9999"
	env["MESSAGE_PREFIX"] = "[ALERT]"
	env["LOG_FORMAT"] = "nginx"
	setEnv(t, env)

	cfg, _ := loadConfig()
	if cfg.TwilioBaseURL != "http://localhost:9999" {
		t.Errorf("unexpected TwilioBaseURL %q", cfg.TwilioBaseURL)
	}
	if cfg.MessagePrefix != "[ALERT]" {
		t.Errorf("unexpected MessagePrefix %q", cfg.MessagePrefix)
	}
	if cfg.LogFormat != "nginx" {
		t.Errorf("unexpected LogFormat %q", cfg.LogFormat)
	}
}

// ---------- run tests ----------

func TestRun_InvalidConfig(t *testing.T) {
	// No env vars set → validation must fail
	err := run(context.Background())
	if err == nil {
		t.Fatal("expected error from invalid config, got nil")
	}
	if !strings.Contains(err.Error(), "invalid configuration") {
		t.Errorf("expected 'invalid configuration' in error, got %q", err)
	}
}

func TestRun_StartsAndStops(t *testing.T) {
	port := freePort(t)
	env := minimalEnv()
	env["PORT"] = port
	setEnv(t, env)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx)
	}()

	addr := "http://127.0.0.1:" + port
	if err := waitForServer(addr, 3*time.Second); err != nil {
		cancel()
		t.Fatalf("server did not start: %v", err)
	}

	resp, err := http.Get(addr + "/")
	if err != nil {
		cancel()
		t.Fatalf("GET / failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "ping" {
		t.Errorf("expected 'ping', got %q", body)
	}

	resp, err = http.Get(addr + "/health")
	if err != nil {
		cancel()
		t.Fatalf("GET /health failed: %v", err)
	}
	var health handler.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		cancel()
		t.Fatalf("failed to decode health response: %v", err)
	}
	_ = resp.Body.Close()
	if health.Status != "ok" {
		t.Errorf("expected health status 'ok', got %q", health.Status)
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return after context cancellation")
	}
}

func TestRun_PortConflict(t *testing.T) {
	port := freePort(t)
	env := minimalEnv()
	env["PORT"] = port
	setEnv(t, env)

	// Occupy the port so run() will fail to bind
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		t.Fatalf("failed to occupy port %s: %v", port, err)
	}
	defer func() { _ = ln.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runErr := run(ctx)
	if runErr == nil {
		t.Fatal("expected error from port conflict, got nil")
	}
	if !strings.Contains(runErr.Error(), "failed to start HTTP server") {
		t.Errorf("expected 'failed to start HTTP server' in error, got %q", runErr)
	}
}

// ---------- printBanner tests ----------

func TestPrintBanner_AuthToken(t *testing.T) {
	output := captureBanner("8080", &handler.Config{
		Sender:           "+1234",
		Receivers:        []string{"+5678"},
		MaxMessageLength: 150,
		AuthToken:        "tok",
	})

	for _, want := range []string{"8080", "+1234", "1 configured", "150 chars", "Account SID/Token"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected banner to contain %q, got:\n%s", want, output)
		}
	}
}

func TestPrintBanner_APIKey(t *testing.T) {
	output := captureBanner("9090", &handler.Config{
		Sender:   "+1234",
		APIKey:   "SK123",
		LogFormat: "nginx",
	})

	if !strings.Contains(output, "API Key (recommended)") {
		t.Errorf("expected 'API Key (recommended)' in banner, got:\n%s", output)
	}
	if !strings.Contains(output, "nginx") {
		t.Errorf("expected 'nginx' log format in banner, got:\n%s", output)
	}
}

func TestPrintBanner_OptionalFields(t *testing.T) {
	output := captureBanner("9090", &handler.Config{
		Sender:        "+1234",
		AuthToken:     "tok",
		RateLimit:     100,
		MessagePrefix: "[PRE]",
		TwilioBaseURL: "http://custom",
	})

	for _, want := range []string{"100 req/min", `"[PRE]"`, "http://custom (custom)"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected banner to contain %q, got:\n%s", want, output)
		}
	}
}

func TestPrintBanner_DefaultLogFormat(t *testing.T) {
	output := captureBanner("9090", &handler.Config{
		Sender:    "+1234",
		AuthToken: "tok",
	})

	if !strings.Contains(output, "simple") {
		t.Errorf("expected default log format 'simple' in banner, got:\n%s", output)
	}
}

// ---------- helpers ----------

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return fmt.Sprintf("%d", port)
}

func waitForServer(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(addr + "/")
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("server at %s not ready after %v", addr, timeout)
}

func captureBanner(port string, cfg *handler.Config) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printBanner(port, cfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}
