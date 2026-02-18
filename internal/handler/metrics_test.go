package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const (
	metricAlertsProcessed = "promtotwilio_alerts_processed_total"
	metricSMSSent         = "promtotwilio_sms_sent_total"
	metricSMSFailed       = "promtotwilio_sms_failed_total"
)

func TestMetrics_Endpoint(t *testing.T) {
	h := NewWithClient(&Config{Sender: "+1", AuthToken: "x"}, &MockTwilioClient{}, "test")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	h.Metrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type: got %q", ct)
	}
	body := w.Body.String()
	for _, name := range []string{
		metricAlertsProcessed,
		metricSMSSent,
		metricSMSFailed,
	} {
		if !strings.Contains(body, name) {
			t.Errorf("metrics body missing %q", name)
		}
	}
}

func TestMetrics_CountersIncrement(t *testing.T) {
	mock := &MockTwilioClient{}
	cfg := Config{Receivers: []string{"+1234567890"}, Sender: "+0987654321"}
	h := NewWithClient(&cfg, mock, "test")

	// No /send yet: all counters 0
	req0 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w0 := httptest.NewRecorder()
	h.Metrics(w0, req0)
	if v := parseCounter(w0.Body.Bytes(), metricAlertsProcessed); v != 0 {
		t.Errorf("initial alerts_processed_total: got %d, want 0", v)
	}

	// One successful POST /send
	payload := `{"status":"firing","alerts":[{"annotations":{"summary":"M"},"startsAt":"2024-01-01T12:00:00Z"}]}`
	postReq := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	postReq.Header.Set("Content-Type", "application/json")
	postW := httptest.NewRecorder()
	h.SendRequest(postW, postReq)
	if postW.Code != http.StatusOK {
		t.Fatalf("POST /send: got %d", postW.Code)
	}

	req1 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w1 := httptest.NewRecorder()
	h.Metrics(w1, req1)
	if v := parseCounter(w1.Body.Bytes(), metricAlertsProcessed); v != 1 {
		t.Errorf("after one send: alerts_processed_total got %d, want 1", v)
	}
	if v := parseCounter(w1.Body.Bytes(), metricSMSSent); v != 1 {
		t.Errorf("after one send: sms_sent_total got %d, want 1", v)
	}
	if v := parseCounter(w1.Body.Bytes(), metricSMSFailed); v != 0 {
		t.Errorf("sms_failed_total: got %d, want 0", v)
	}
}

func parseCounter(body []byte, metricName string) uint64 {
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if strings.HasPrefix(line, metricName+" ") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) != 2 {
				return 0
			}
			v, _ := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
			return v
		}
	}
	return 0
}
