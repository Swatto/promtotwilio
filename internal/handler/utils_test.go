package handler

import (
	"reflect"
	"strings"
	"testing"
)

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

func TestFindAndReplaceLabels(t *testing.T) {
	alert := &Alert{
		Status: "firing",
		Labels: map[string]string{
			"alertname": "InstanceDown",
			"instance":  "http://test.com",
			"job":       "blackbox",
		},
		Annotations: map[string]string{
			"description": "Unable to scrape $labels.instance",
			"summary":     "Address $labels.instance appears to be down",
		},
		StartsAt:     "2017-01-06T19:34:52.887Z",
		EndsAt:       "0001-01-01T00:00:00Z",
		GeneratorURL: "http://test.com/graph?g0.expr=probe_success%7Bjob%3D%22blackbox%22%7D+%3D%3D+0&g0.tab=0",
	}

	input := "Address $labels.instance appears to be down with $labels.alertname"
	expected := "Address http://test.com appears to be down with InstanceDown"
	output := FindAndReplaceLabels(input, alert)

	if output != expected {
		t.Errorf("FindAndReplaceLabels(%q, alert) == %q, want %q", input, output, expected)
	}
}

func TestTruncateMessage_ShortMessage(t *testing.T) {
	msg := "Short message"
	result := TruncateMessage(msg, 150)
	if result != msg {
		t.Errorf("expected %q, got %q", msg, result)
	}
}

func TestTruncateMessage_LongMessage(t *testing.T) {
	msg := "This is a very long message that exceeds the maximum length and should be truncated"
	result := TruncateMessage(msg, 20)
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
	result := TruncateMessage(msg, 20)
	if result != msg {
		t.Errorf("expected %q, got %q", msg, result)
	}
}

func TestTruncateMessage_VeryShortMaxLen(t *testing.T) {
	msg := "This is a long message"
	result := TruncateMessage(msg, 3)
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
	result := TruncateMessage(msg, 150)
	if result != msg {
		t.Errorf("expected empty string, got %q", result)
	}
}
