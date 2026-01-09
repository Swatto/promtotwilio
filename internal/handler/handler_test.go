package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// MockTwilioClient is a mock implementation of TwilioClient for testing
type MockTwilioClient struct {
	SendMessageFunc func(to, from, body string) error
	Calls           []MockCall
	mu              sync.Mutex
}

type MockCall struct {
	To   string
	From string
	Body string
}

func (m *MockTwilioClient) SendMessage(to, from, body string) error {
	m.mu.Lock()
	m.Calls = append(m.Calls, MockCall{To: to, From: from, Body: body})
	m.mu.Unlock()
	if m.SendMessageFunc != nil {
		return m.SendMessageFunc(to, from, body)
	}
	return nil
}

func (m *MockTwilioClient) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Calls)
}

func (m *MockTwilioClient) GetCall(index int) MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Calls[index]
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

func TestSendRequest_InvalidContentType(t *testing.T) {
	h := NewWithClient(&Config{Receivers: []string{"+1234567890"}, Sender: "+0987654321"}, &MockTwilioClient{}, "test")

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusNotAcceptable {
		t.Errorf("expected status %d, got %d", http.StatusNotAcceptable, w.Code)
	}
}

func TestSendRequest_ContentTypeWithCharset(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "Test alert"},
			"startsAt": "2024-01-01T12:00:00Z"
		}]
	}`

	// Test with charset parameter
	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if mockClient.CallCount() != 1 {
		t.Errorf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}
}

func TestSendRequest_ContentTypeCaseInsensitive(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "Test alert"},
			"startsAt": "2024-01-01T12:00:00Z"
		}]
	}`

	// Test with uppercase Content-Type
	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "Application/JSON")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if mockClient.CallCount() != 1 {
		t.Errorf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}
}

func TestSendRequest_NoReceiver(t *testing.T) {
	h := NewWithClient(&Config{Sender: "+0987654321"}, &MockTwilioClient{}, "test")

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestSendRequest_Success(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "Test alert"},
			"startsAt": "2024-01-01T12:00:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success true, got false")
	}
	if resp.Sent != 1 {
		t.Errorf("expected sent 1, got %d", resp.Sent)
	}
	if resp.Failed != 0 {
		t.Errorf("expected failed 0, got %d", resp.Failed)
	}
	if mockClient.CallCount() != 1 {
		t.Errorf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}
}

func TestSendRequest_MultipleReceivers(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1111111111", "+2222222222", "+3333333333"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "Test alert"},
			"startsAt": "2024-01-01T12:00:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Sent != 3 {
		t.Errorf("expected sent 3, got %d", resp.Sent)
	}
	if mockClient.CallCount() != 3 {
		t.Errorf("expected 3 calls to SendMessage, got %d", mockClient.CallCount())
	}
}

func TestSendRequest_ReceiverQueryParam(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1111111111"}, // Default receiver
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "Test alert"},
			"startsAt": "2024-01-01T12:00:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send?receiver=%2B9999999999", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if mockClient.CallCount() != 1 {
		t.Fatalf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}
	if mockClient.GetCall(0).To != "+9999999999" {
		t.Errorf("expected receiver '+9999999999', got %q", mockClient.GetCall(0).To)
	}
}

func TestSendRequest_TwilioError(t *testing.T) {
	mockClient := &MockTwilioClient{
		SendMessageFunc: func(to, from, body string) error {
			return errors.New("twilio error")
		},
	}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "Test alert"},
			"startsAt": "2024-01-01T12:00:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Errorf("expected success false, got true")
	}
	if resp.Failed != 1 {
		t.Errorf("expected failed 1, got %d", resp.Failed)
	}
	if len(resp.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(resp.Errors))
	}
}

func TestSendRequest_InvalidAlertsFormat(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	// alerts is not an array - should return 400
	payload := `{
		"status": "firing",
		"alerts": "not-an-array"
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// No messages should be sent
	if mockClient.CallCount() != 0 {
		t.Errorf("expected 0 calls to SendMessage, got %d", mockClient.CallCount())
	}
}

func TestSendRequest_NotFiring(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "resolved",
		"alerts": [{
			"annotations": {"summary": "Test alert"},
			"startsAt": "2024-01-01T12:00:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Should not send any messages for non-firing alerts when SendResolved is false
	if mockClient.CallCount() != 0 {
		t.Errorf("expected 0 calls to SendMessage, got %d", mockClient.CallCount())
	}
}

func TestSendRequest_ResolvedAlert_Disabled(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers:    []string{"+1234567890"},
		Sender:       "+0987654321",
		SendResolved: false,
	}, mockClient, "test")

	payload := `{
		"status": "resolved",
		"alerts": [{
			"annotations": {"summary": "Test alert"},
			"startsAt": "2024-01-01T12:00:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Should not send any messages when SendResolved is false
	if mockClient.CallCount() != 0 {
		t.Errorf("expected 0 calls to SendMessage, got %d", mockClient.CallCount())
	}
}

func TestSendRequest_ResolvedAlert_Enabled(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers:    []string{"+1234567890"},
		Sender:       "+0987654321",
		SendResolved: true,
	}, mockClient, "test")

	payload := `{
		"status": "resolved",
		"alerts": [{
			"annotations": {"summary": "Test alert"},
			"startsAt": "2024-01-15T10:30:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success true, got false")
	}
	if resp.Sent != 1 {
		t.Errorf("expected sent 1, got %d", resp.Sent)
	}
	if mockClient.CallCount() != 1 {
		t.Fatalf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}

	// Message should contain "RESOLVED: " prefix
	call := mockClient.GetCall(0)
	expectedBody := `RESOLVED: "Test alert" alert starts at Mon, 15 Jan 2024 10:30:00 UTC`
	if call.Body != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, call.Body)
	}
}

func TestSendRequest_MixedStatus(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers:    []string{"+1234567890"},
		Sender:       "+0987654321",
		SendResolved: true,
	}, mockClient, "test")

	// First send a firing alert
	firingPayload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "Firing alert"},
			"startsAt": "2024-01-15T10:30:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(firingPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if mockClient.CallCount() != 1 {
		t.Fatalf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}

	// Firing alert should not have RESOLVED prefix
	call := mockClient.GetCall(0)
	if strings.Contains(call.Body, "RESOLVED:") {
		t.Errorf("firing alert should not have RESOLVED prefix, got %q", call.Body)
	}

	// Now send a resolved alert
	resolvedPayload := `{
		"status": "resolved",
		"alerts": [{
			"annotations": {"summary": "Resolved alert"},
			"startsAt": "2024-01-15T10:30:00Z"
		}]
	}`

	req2 := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(resolvedPayload))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	h.SendRequest(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w2.Code)
	}

	if mockClient.CallCount() != 2 {
		t.Fatalf("expected 2 calls to SendMessage, got %d", mockClient.CallCount())
	}

	// Resolved alert should have RESOLVED prefix
	call2 := mockClient.GetCall(1)
	if !strings.Contains(call2.Body, "RESOLVED:") {
		t.Errorf("resolved alert should have RESOLVED prefix, got %q", call2.Body)
	}
}

func TestParseReceivers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single receiver",
			input:    "+1234567890",
			expected: []string{"+1234567890"},
		},
		{
			name:     "multiple receivers",
			input:    "+1234567890,+0987654321",
			expected: []string{"+1234567890", "+0987654321"},
		},
		{
			name:     "multiple receivers with spaces",
			input:    "+1234567890, +0987654321, +1122334455",
			expected: []string{"+1234567890", "+0987654321", "+1122334455"},
		},
		{
			name:     "receivers with extra whitespace",
			input:    "  +1234567890  ,  +0987654321  ",
			expected: []string{"+1234567890", "+0987654321"},
		},
		{
			name:     "empty parts are ignored",
			input:    "+1234567890,,+0987654321",
			expected: []string{"+1234567890", "+0987654321"},
		},
		{
			name:     "only commas",
			input:    ",,,",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseReceivers(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseReceivers(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSendRequest_BodySizeLimitEnforced(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	// Create a payload larger than maxBodySize (5 MB)
	// The body will be truncated, causing JSON parsing to fail
	largePayload := make([]byte, maxBodySize+1000)
	for i := range largePayload {
		largePayload[i] = 'x'
	}

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(largePayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	// The request should succeed (200 OK) but with 0 messages sent
	// because the truncated body is invalid JSON
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// No messages should be sent since the JSON is invalid/truncated
	if resp.Sent != 0 {
		t.Errorf("expected sent 0, got %d", resp.Sent)
	}
	if mockClient.CallCount() != 0 {
		t.Errorf("expected 0 calls to SendMessage, got %d", mockClient.CallCount())
	}
}

func TestFindAndReplaceLabels(t *testing.T) {
	alert := []byte(`
      {
        "status": "firing",
        "labels": {
          "alertname": "InstanceDown",
          "instance": "http://test.com",
          "job": "blackbox"
        },
        "annotations": {
          "description": "Unable to scrape $labels.instance",
          "summary": "Address $labels.instance appears to be down"
        },
        "startsAt": "2017-01-06T19:34:52.887Z",
        "endsAt": "0001-01-01T00:00:00Z",
        "generatorURL": "http://test.com/graph?g0.expr=probe_success%7Bjob%3D%22blackbox%22%7D+%3D%3D+0&g0.tab=0"
      }
    `)

	input := "Address $labels.instance appears to be down with $labels.alertname"
	expected := "Address http://test.com appears to be down with InstanceDown"
	output := FindAndReplaceLabels(input, alert)

	if output != expected {
		t.Errorf("FindAndReplaceLabels(%q, alert) == %q, want %q", input, output, expected)
	}
}

func TestSendRequest_MissingSummaryAnnotation(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	// Alert without summary annotation
	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"description": "Some description without summary"},
			"startsAt": "2024-01-01T12:00:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Errorf("expected success false, got true")
	}
	if resp.Failed != 1 {
		t.Errorf("expected failed 1, got %d", resp.Failed)
	}
	if mockClient.CallCount() != 0 {
		t.Errorf("expected 0 calls to SendMessage, got %d", mockClient.CallCount())
	}
}

func TestSendRequest_EmptySummaryAnnotation(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	// Alert with empty summary annotation
	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": ""},
			"startsAt": "2024-01-01T12:00:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Errorf("expected success false, got true")
	}
	if resp.Failed != 1 {
		t.Errorf("expected failed 1, got %d", resp.Failed)
	}
	if mockClient.CallCount() != 0 {
		t.Errorf("expected 0 calls to SendMessage, got %d", mockClient.CallCount())
	}
}

func TestSendRequest_MissingStartsAt(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	// Alert without startsAt field - should still succeed
	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "Test alert without timestamp"}
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success true, got false")
	}
	if resp.Sent != 1 {
		t.Errorf("expected sent 1, got %d", resp.Sent)
	}
	if mockClient.CallCount() != 1 {
		t.Fatalf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}

	// Message should not contain timestamp formatting
	call := mockClient.GetCall(0)
	if strings.Contains(call.Body, "alert starts at") {
		t.Errorf("expected message without timestamp, got %q", call.Body)
	}
	if call.Body != "Test alert without timestamp" {
		t.Errorf("expected body 'Test alert without timestamp', got %q", call.Body)
	}
}

func TestSendRequest_InvalidStartsAtFormat(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	// Alert with invalid startsAt format - should still succeed but without timestamp
	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "Test alert with bad timestamp"},
			"startsAt": "not-a-valid-timestamp"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success true, got false")
	}
	if resp.Sent != 1 {
		t.Errorf("expected sent 1, got %d", resp.Sent)
	}
	if mockClient.CallCount() != 1 {
		t.Fatalf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}

	// Message should not contain timestamp formatting due to invalid format
	call := mockClient.GetCall(0)
	if strings.Contains(call.Body, "alert starts at") {
		t.Errorf("expected message without timestamp, got %q", call.Body)
	}
	if call.Body != "Test alert with bad timestamp" {
		t.Errorf("expected body 'Test alert with bad timestamp', got %q", call.Body)
	}
}

func TestSendRequest_ValidStartsAtFormat(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "Test alert"},
			"startsAt": "2024-01-15T10:30:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if mockClient.CallCount() != 1 {
		t.Fatalf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}

	// Message should contain properly formatted timestamp
	call := mockClient.GetCall(0)
	expectedBody := `"Test alert" alert starts at Mon, 15 Jan 2024 10:30:00 UTC`
	if call.Body != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, call.Body)
	}
}

func TestSendRequest_MissingAnnotationsField(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	// Alert without annotations field at all
	payload := `{
		"status": "firing",
		"alerts": [{
			"labels": {"alertname": "TestAlert"},
			"startsAt": "2024-01-01T12:00:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Errorf("expected success false, got true")
	}
	if resp.Failed != 1 {
		t.Errorf("expected failed 1, got %d", resp.Failed)
	}
	if mockClient.CallCount() != 0 {
		t.Errorf("expected 0 calls to SendMessage, got %d", mockClient.CallCount())
	}
}

func TestTruncateMessage_ShortMessage(t *testing.T) {
	msg := "Short message"
	result := truncateMessage(msg, 150)
	if result != msg {
		t.Errorf("expected %q, got %q", msg, result)
	}
}

func TestTruncateMessage_LongMessage(t *testing.T) {
	msg := "This is a very long message that exceeds the maximum length and should be truncated"
	result := truncateMessage(msg, 20)
	expected := "This is a very lo..."
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
	if len(result) != 20 {
		t.Errorf("expected length 20, got %d", len(result))
	}
}

func TestTruncateMessage_ExactLength(t *testing.T) {
	msg := "Exactly 20 chars!!"
	result := truncateMessage(msg, 20)
	if result != msg {
		t.Errorf("expected %q, got %q", msg, result)
	}
}

func TestTruncateMessage_VeryShortMaxLen(t *testing.T) {
	msg := "This is a long message"
	result := truncateMessage(msg, 3)
	if len(result) != 3 {
		t.Errorf("expected length 3, got %d", len(result))
	}
	if result != "Thi" {
		t.Errorf("expected %q, got %q", "Thi", result)
	}
	// Should not have "..." suffix when maxLen <= 3
	if strings.Contains(result, "...") {
		t.Errorf("should not have ... suffix when maxLen <= 3, got %q", result)
	}
}

func TestTruncateMessage_EmptyMessage(t *testing.T) {
	msg := ""
	result := truncateMessage(msg, 150)
	if result != msg {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestSendMessage_Truncation(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers:       []string{"+1234567890"},
		Sender:          "+0987654321",
		MaxMessageLength: 50,
	}, mockClient, "test")

	// Create a message that will exceed 50 characters
	longSummary := "This is a very long summary that will definitely exceed the maximum message length of 50 characters"
	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "` + longSummary + `"},
			"startsAt": "2024-01-15T10:30:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if mockClient.CallCount() != 1 {
		t.Fatalf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}

	call := mockClient.GetCall(0)
	if len(call.Body) > 50 {
		t.Errorf("expected message length <= 50, got %d: %q", len(call.Body), call.Body)
	}
	if !strings.HasSuffix(call.Body, "...") {
		t.Errorf("expected message to end with '...', got %q", call.Body)
	}
}

func TestSendMessage_CustomMaxLength(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers:       []string{"+1234567890"},
		Sender:          "+0987654321",
		MaxMessageLength: 100,
	}, mockClient, "test")

	longSummary := "This is a summary that will be truncated at 100 characters"
	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "` + longSummary + `"},
			"startsAt": "2024-01-15T10:30:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if mockClient.CallCount() != 1 {
		t.Fatalf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}

	call := mockClient.GetCall(0)
	if len(call.Body) > 100 {
		t.Errorf("expected message length <= 100, got %d: %q", len(call.Body), call.Body)
	}
}

func TestSendMessage_DefaultMaxLength(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
		// MaxMessageLength not set, should default to 150
	}, mockClient, "test")

	// Create a message that exceeds 150 characters
	longSummary := strings.Repeat("A", 200)
	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "` + longSummary + `"},
			"startsAt": "2024-01-15T10:30:00Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if mockClient.CallCount() != 1 {
		t.Fatalf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}

	call := mockClient.GetCall(0)
	// Should default to 150
	if len(call.Body) > 150 {
		t.Errorf("expected message length <= 150 (default), got %d: %q", len(call.Body), call.Body)
	}
}
