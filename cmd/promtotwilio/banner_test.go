package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/swatto/promtotwilio/internal/handler"
)

func TestPadCenter(t *testing.T) {
	tests := []struct {
		s      string
		width  int
		length int // expected total length
	}{
		{"ab", 5, 5},
		{"x", 3, 3},
		{"", 4, 4},
		{"hello", 5, 5},
		{"hello", 10, 10},
		{"promtotwilio", 64, 64},
	}
	for _, tt := range tests {
		t.Run(tt.s+"/"+fmt.Sprint(tt.width), func(t *testing.T) {
			got := padCenter(tt.s, tt.width)
			if len(got) != tt.length {
				t.Errorf("padCenter(%q, %d) length = %d, want %d", tt.s, tt.width, len(got), tt.length)
			}
			if len(tt.s) >= tt.width && tt.s[:tt.width] != got {
				t.Errorf("padCenter(%q, %d) should truncate to %q", tt.s, tt.width, tt.s[:tt.width])
			}
		})
	}
}

func TestConfigLine(t *testing.T) {
	// Value should start at column configValueAt (24).
	tests := []struct {
		label       string
		value       string
		wantValueAt int // column index where value should start (0-based)
	}{
		{"Port", "9090", 24},
		{"Sender", "+15550000000", 24},
		{"Auth method", "API Key (recommended)", 24},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := configLine(tt.label, tt.value)
			prefix := "    • " + tt.label + ":"
			if len(prefix) > tt.wantValueAt {
				return // long label; just ensure we don't panic
			}
			pad := tt.wantValueAt - len(prefix)
			expectedPad := strings.Repeat(" ", pad)
			if !strings.HasPrefix(got, prefix) {
				t.Errorf("configLine(%q, %q) should start with %q", tt.label, tt.value, prefix)
			}
			afterPrefix := got[len(prefix):]
			if !strings.HasPrefix(afterPrefix, expectedPad) || !strings.Contains(got, tt.value) {
				t.Errorf("configLine(%q, %q) = %q: value should start at column %d", tt.label, tt.value, got, tt.wantValueAt)
			}
		})
	}
}

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
		Sender:    "+1234",
		APIKey:    "SK123",
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

func TestPrintBanner_WebhookAuthAndDryRun(t *testing.T) {
	output := captureBanner("9090", &handler.Config{
		Sender:        "+1234",
		AuthToken:     "tok",
		WebhookSecret: "secret",
		DryRun:        true,
	})

	if !strings.Contains(output, "Webhook auth") {
		t.Errorf("expected webhook auth in banner, got:\n%s", output)
	}
	if !strings.Contains(output, "Dry-run") {
		t.Errorf("expected dry-run in banner, got:\n%s", output)
	}
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
