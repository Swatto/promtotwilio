package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendRequest(t *testing.T) {
	firingPayload := `{"status":"firing","alerts":[{"annotations":{"summary":"Test alert"},"startsAt":"2024-01-01T12:00:00Z"}]}`
	resolvedPayload := `{"status":"resolved","alerts":[{"annotations":{"summary":"Test alert"},"startsAt":"2024-01-15T10:30:00Z"}]}`

	tests := []struct {
		name          string
		cfg           Config
		mockErr       error
		payload       string
		url           string
		contentType   string
		wantStatus    int
		wantCallCount int
		wantSuccess   bool
		wantSent      int
		wantFailed    int
		wantErrCount  int
		checkCalls    func(t *testing.T, mock *MockTwilioClient)
	}{
		{
			name:        "invalid content type",
			cfg:         Config{Receivers: []string{"+1234567890"}, Sender: "+0987654321"},
			payload:     "{}",
			contentType: "text/plain",
			wantStatus:  http.StatusNotAcceptable,
		},
		{
			name:          "content type with charset",
			cfg:           Config{Receivers: []string{"+1234567890"}, Sender: "+0987654321"},
			payload:       firingPayload,
			contentType:   "application/json; charset=utf-8",
			wantStatus:    http.StatusOK,
			wantCallCount: 1,
			wantSuccess:   true,
			wantSent:      1,
		},
		{
			name:          "content type case insensitive",
			cfg:           Config{Receivers: []string{"+1234567890"}, Sender: "+0987654321"},
			payload:       firingPayload,
			contentType:   "Application/JSON",
			wantStatus:    http.StatusOK,
			wantCallCount: 1,
			wantSuccess:   true,
			wantSent:      1,
		},
		{
			name:       "no receiver",
			cfg:        Config{Sender: "+0987654321"},
			payload:    "{}",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:          "success",
			cfg:           Config{Receivers: []string{"+1234567890"}, Sender: "+0987654321"},
			payload:       firingPayload,
			wantStatus:    http.StatusOK,
			wantCallCount: 1,
			wantSuccess:   true,
			wantSent:      1,
		},
		{
			name:          "multiple receivers",
			cfg:           Config{Receivers: []string{"+1111111111", "+2222222222", "+3333333333"}, Sender: "+0987654321"},
			payload:       firingPayload,
			wantStatus:    http.StatusOK,
			wantCallCount: 3,
			wantSuccess:   true,
			wantSent:      3,
		},
		{
			name:          "receiver query param overrides default",
			cfg:           Config{Receivers: []string{"+1111111111"}, Sender: "+0987654321"},
			payload:       firingPayload,
			url:           "/send?receiver=%2B9999999999",
			wantStatus:    http.StatusOK,
			wantCallCount: 1,
			wantSuccess:   true,
			wantSent:      1,
			checkCalls: func(t *testing.T, mock *MockTwilioClient) {
				if mock.GetCall(0).To != "+9999999999" {
					t.Errorf("expected receiver '+9999999999', got %q", mock.GetCall(0).To)
				}
			},
		},
		{
			name:          "twilio error",
			cfg:           Config{Receivers: []string{"+1234567890"}, Sender: "+0987654321"},
			mockErr:       errors.New("twilio error"),
			payload:       firingPayload,
			wantStatus:    http.StatusInternalServerError,
			wantCallCount: 1,
			wantFailed:    1,
			wantErrCount:  1,
		},
		{
			name:       "invalid alerts format",
			cfg:        Config{Receivers: []string{"+1234567890"}, Sender: "+0987654321"},
			payload:    `{"status":"firing","alerts":"not-an-array"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "resolved alert ignored when disabled",
			cfg:         Config{Receivers: []string{"+1234567890"}, Sender: "+0987654321", SendResolved: false},
			payload:     resolvedPayload,
			wantStatus:  http.StatusOK,
			wantSuccess: true,
		},
		{
			name:          "resolved alert sent when enabled",
			cfg:           Config{Receivers: []string{"+1234567890"}, Sender: "+0987654321", SendResolved: true},
			payload:       resolvedPayload,
			wantStatus:    http.StatusOK,
			wantCallCount: 1,
			wantSuccess:   true,
			wantSent:      1,
			checkCalls: func(t *testing.T, mock *MockTwilioClient) {
				body := mock.GetCall(0).Body
				expected := `RESOLVED: "Test alert" alert starts at Mon, 15 Jan 2024 10:30:00 UTC`
				if body != expected {
					t.Errorf("expected body %q, got %q", expected, body)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockTwilioClient{}
			if tt.mockErr != nil {
				mock.SendMessageFunc = func(to, from, body string) error {
					return tt.mockErr
				}
			}
			h := NewWithClient(&tt.cfg, mock, "test")

			reqURL := "/send"
			if tt.url != "" {
				reqURL = tt.url
			}
			ct := "application/json"
			if tt.contentType != "" {
				ct = tt.contentType
			}

			req := httptest.NewRequest(http.MethodPost, reqURL, bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", ct)
			w := httptest.NewRecorder()

			h.SendRequest(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
			if mock.CallCount() != tt.wantCallCount {
				t.Fatalf("call count: got %d, want %d", mock.CallCount(), tt.wantCallCount)
			}

			if tt.wantStatus == http.StatusOK || tt.wantStatus == http.StatusInternalServerError {
				var resp SendResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Success != tt.wantSuccess {
					t.Errorf("success: got %v, want %v", resp.Success, tt.wantSuccess)
				}
				if resp.Sent != tt.wantSent {
					t.Errorf("sent: got %d, want %d", resp.Sent, tt.wantSent)
				}
				if resp.Failed != tt.wantFailed {
					t.Errorf("failed count: got %d, want %d", resp.Failed, tt.wantFailed)
				}
				if len(resp.Errors) != tt.wantErrCount {
					t.Errorf("error count: got %d, want %d", len(resp.Errors), tt.wantErrCount)
				}
			}

			if tt.checkCalls != nil {
				tt.checkCalls(t, mock)
			}
		})
	}
}

func TestSendRequest_MixedStatus(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers:    []string{"+1234567890"},
		Sender:       "+0987654321",
		SendResolved: true,
	}, mockClient, "test")

	// Send firing alert
	firingReq := httptest.NewRequest(http.MethodPost, "/send",
		bytes.NewBufferString(`{"status":"firing","alerts":[{"annotations":{"summary":"Firing alert"},"startsAt":"2024-01-15T10:30:00Z"}]}`))
	firingReq.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()

	h.SendRequest(w1, firingReq)

	if w1.Code != http.StatusOK {
		t.Fatalf("firing: status got %d, want %d", w1.Code, http.StatusOK)
	}
	if mockClient.CallCount() != 1 {
		t.Fatalf("firing: call count got %d, want 1", mockClient.CallCount())
	}
	if strings.Contains(mockClient.GetCall(0).Body, "RESOLVED:") {
		t.Errorf("firing alert should not have RESOLVED prefix, got %q", mockClient.GetCall(0).Body)
	}

	// Send resolved alert
	resolvedReq := httptest.NewRequest(http.MethodPost, "/send",
		bytes.NewBufferString(`{"status":"resolved","alerts":[{"annotations":{"summary":"Resolved alert"},"startsAt":"2024-01-15T10:30:00Z"}]}`))
	resolvedReq.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	h.SendRequest(w2, resolvedReq)

	if w2.Code != http.StatusOK {
		t.Fatalf("resolved: status got %d, want %d", w2.Code, http.StatusOK)
	}
	if mockClient.CallCount() != 2 {
		t.Fatalf("resolved: call count got %d, want 2", mockClient.CallCount())
	}
	if !strings.Contains(mockClient.GetCall(1).Body, "RESOLVED:") {
		t.Errorf("resolved alert should have RESOLVED prefix, got %q", mockClient.GetCall(1).Body)
	}
}

func TestSendRequest_BodySizeLimitEnforced(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	largePayload := make([]byte, maxBodySize+1000)
	for i := range largePayload {
		largePayload[i] = 'x'
	}

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(largePayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
	if mockClient.CallCount() != 0 {
		t.Errorf("call count: got %d, want 0", mockClient.CallCount())
	}
}
