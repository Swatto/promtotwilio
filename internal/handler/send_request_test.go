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

	// The truncated body is invalid JSON, so we return 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// No messages should be sent since the JSON is invalid/truncated
	if mockClient.CallCount() != 0 {
		t.Errorf("expected 0 calls to SendMessage, got %d", mockClient.CallCount())
	}
}
