package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendRequest_MissingSummaryAnnotation(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	// Alert without summary annotation but with description - should use description as fallback
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

	// Should succeed now because description is used as fallback
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
		t.Errorf("expected 1 call to SendMessage, got %d", mockClient.CallCount())
	}

	// Verify the message contains the description
	call := mockClient.GetCall(0)
	if !strings.Contains(call.Body, "Some description without summary") {
		t.Errorf("expected message to contain description, got %q", call.Body)
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

func TestSendMessage_SummaryOnly(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"summary": "Test summary"},
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
	if !strings.Contains(call.Body, "Test summary") {
		t.Errorf("expected message to contain 'Test summary', got %q", call.Body)
	}
}

func TestSendMessage_DescriptionFallback(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {"description": "Test description"},
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
	if !strings.Contains(call.Body, "Test description") {
		t.Errorf("expected message to contain 'Test description', got %q", call.Body)
	}
}

func TestSendMessage_SummaryPreferred(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {
				"summary": "Test summary",
				"description": "Test description"
			},
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
	if !strings.Contains(call.Body, "Test summary") {
		t.Errorf("expected message to contain 'Test summary' (preferred), got %q", call.Body)
	}
	if strings.Contains(call.Body, "Test description") {
		t.Errorf("expected message to use summary, not description, got %q", call.Body)
	}
}

func TestSendMessage_BothMissing(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {},
			"startsAt": "2024-01-15T10:30:00Z"
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
	if len(resp.Errors) == 0 {
		t.Errorf("expected error message, got none")
	} else if !strings.Contains(resp.Errors[0], "summary and description") {
		t.Errorf("expected error to mention both summary and description, got %q", resp.Errors[0])
	}
	if mockClient.CallCount() != 0 {
		t.Errorf("expected 0 calls to SendMessage, got %d", mockClient.CallCount())
	}
}

func TestSendMessage_EmptySummaryUsesDescription(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {
				"summary": "",
				"description": "Test description"
			},
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
	if !strings.Contains(call.Body, "Test description") {
		t.Errorf("expected message to contain 'Test description' (fallback), got %q", call.Body)
	}
}

func TestSendMessage_WhitespaceOnlySummaryUsesDescription(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"annotations": {
				"summary": "   ",
				"description": "Test description"
			},
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
	if !strings.Contains(call.Body, "Test description") {
		t.Errorf("expected message to contain 'Test description' (fallback for whitespace-only summary), got %q", call.Body)
	}
}

func TestSendMessage_WithAlertName(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"labels": {"alertname": "NodeDown"},
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

	call := mockClient.GetCall(0)
	if !strings.HasPrefix(call.Body, "[NodeDown]") {
		t.Errorf("expected message to start with '[NodeDown]', got %q", call.Body)
	}
	if !strings.Contains(call.Body, "Test alert") {
		t.Errorf("expected message to contain 'Test alert', got %q", call.Body)
	}
}

func TestSendMessage_WithoutAlertName(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"labels": {},
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

	call := mockClient.GetCall(0)
	// Should not start with [ (no alert name prefix)
	if strings.HasPrefix(call.Body, "[") {
		t.Errorf("expected message to not start with '[', got %q", call.Body)
	}
	if !strings.Contains(call.Body, "Test alert") {
		t.Errorf("expected message to contain 'Test alert', got %q", call.Body)
	}
}

func TestSendMessage_EmptyAlertName(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"labels": {"alertname": ""},
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

	call := mockClient.GetCall(0)
	// Should not start with [ (empty alert name is skipped)
	if strings.HasPrefix(call.Body, "[") {
		t.Errorf("expected message to not start with '[' (empty alertname), got %q", call.Body)
	}
}

func TestSendMessage_AlertNameFormat(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"labels": {"alertname": "HighCPUUsage"},
			"annotations": {"summary": "CPU is high"},
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
	expectedPrefix := "[HighCPUUsage]"
	if !strings.HasPrefix(call.Body, expectedPrefix) {
		t.Errorf("expected message to start with %q, got %q", expectedPrefix, call.Body)
	}
	// Verify format: [AlertName] "summary" alert starts at...
	if !strings.Contains(call.Body, `"CPU is high"`) {
		t.Errorf("expected message to contain quoted summary, got %q", call.Body)
	}
}

func TestSendMessage_AlertNameWithResolved(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers:    []string{"+1234567890"},
		Sender:       "+0987654321",
		SendResolved: true,
	}, mockClient, "test")

	payload := `{
		"status": "resolved",
		"alerts": [{
			"labels": {"alertname": "NodeDown"},
			"annotations": {"summary": "Node is back online"},
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
	// Should be: RESOLVED: [NodeDown] "Node is back online" alert starts at...
	if !strings.HasPrefix(call.Body, "RESOLVED: [NodeDown]") {
		t.Errorf("expected message to start with 'RESOLVED: [NodeDown]', got %q", call.Body)
	}
}

func TestSendMessage_WithPrefix(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers:    []string{"+1234567890"},
		Sender:       "+0987654321",
		MessagePrefix: "[PROD]",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"labels": {"alertname": "NodeDown"},
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

	call := mockClient.GetCall(0)
	// Should be: [PROD] [NodeDown] "Test alert" alert starts at...
	if !strings.HasPrefix(call.Body, "[PROD]") {
		t.Errorf("expected message to start with '[PROD]', got %q", call.Body)
	}
	if !strings.Contains(call.Body, "[NodeDown]") {
		t.Errorf("expected message to contain '[NodeDown]', got %q", call.Body)
	}
}

func TestSendMessage_WithoutPrefix(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers: []string{"+1234567890"},
		Sender:    "+0987654321",
		// MessagePrefix not set
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"labels": {"alertname": "NodeDown"},
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

	call := mockClient.GetCall(0)
	// Should NOT start with a prefix like [PROD]
	if strings.HasPrefix(call.Body, "[PROD]") || strings.HasPrefix(call.Body, "[STAGING]") {
		t.Errorf("expected message to not have custom prefix, got %q", call.Body)
	}
	// Should still have alert name
	if !strings.Contains(call.Body, "[NodeDown]") {
		t.Errorf("expected message to contain '[NodeDown]', got %q", call.Body)
	}
}

func TestSendMessage_EmptyPrefix(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers:    []string{"+1234567890"},
		Sender:       "+0987654321",
		MessagePrefix: "",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"labels": {"alertname": "NodeDown"},
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

	call := mockClient.GetCall(0)
	// Empty prefix should be ignored - message should start with alert name or summary
	if strings.HasPrefix(call.Body, " ") {
		t.Errorf("expected message to not start with space (empty prefix), got %q", call.Body)
	}
}

func TestSendMessage_PrefixWithSpecialChars(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers:    []string{"+1234567890"},
		Sender:       "+0987654321",
		MessagePrefix: "[ENV:PROD-01]",
	}, mockClient, "test")

	payload := `{
		"status": "firing",
		"alerts": [{
			"labels": {"alertname": "NodeDown"},
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

	call := mockClient.GetCall(0)
	// Should include prefix with special characters
	if !strings.HasPrefix(call.Body, "[ENV:PROD-01]") {
		t.Errorf("expected message to start with '[ENV:PROD-01]', got %q", call.Body)
	}
}

func TestSendMessage_PrefixWithResolved(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers:    []string{"+1234567890"},
		Sender:       "+0987654321",
		SendResolved: true,
		MessagePrefix: "[PROD]",
	}, mockClient, "test")

	payload := `{
		"status": "resolved",
		"alerts": [{
			"labels": {"alertname": "NodeDown"},
			"annotations": {"summary": "Node is back online"},
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
	// Should be: [PROD] RESOLVED: [NodeDown] "Node is back online" alert starts at...
	if !strings.HasPrefix(call.Body, "[PROD]") {
		t.Errorf("expected message to start with '[PROD]', got %q", call.Body)
	}
	if !strings.Contains(call.Body, "RESOLVED:") {
		t.Errorf("expected message to contain 'RESOLVED:', got %q", call.Body)
	}
}

func TestSendMessage_PrefixTruncation(t *testing.T) {
	mockClient := &MockTwilioClient{}
	h := NewWithClient(&Config{
		Receivers:       []string{"+1234567890"},
		Sender:          "+0987654321",
		MessagePrefix:   "[VERY-LONG-PREFIX-THAT-TAKES-MANY-CHARACTERS]",
		MaxMessageLength: 50,
	}, mockClient, "test")

	longSummary := "This is a very long summary that will be truncated"
	payload := `{
		"status": "firing",
		"alerts": [{
			"labels": {"alertname": "NodeDown"},
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
	// Prefix should be included, and total message should be truncated to 50
	if len(call.Body) > 50 {
		t.Errorf("expected message length <= 50 (including prefix), got %d: %q", len(call.Body), call.Body)
	}
	// Prefix should still be present (at the start)
	if !strings.HasPrefix(call.Body, "[VERY-LONG-PREFIX") {
		t.Errorf("expected message to start with prefix, got %q", call.Body)
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
