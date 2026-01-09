package handler

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/buger/jsonparser"
)

// FormatMessage formats an alert into a message string ready to be sent via SMS.
// It extracts the summary/description, replaces label placeholders, adds timestamps,
// alert names, resolved prefixes, and custom prefixes, then truncates if needed.
func FormatMessage(alert []byte, status string, config *Config) (string, error) {
	// Try to get summary first
	body, err := jsonparser.GetString(alert, "annotations", "summary")

	// If summary is missing or empty (including whitespace-only), try description as fallback
	if err != nil || strings.TrimSpace(body) == "" {
		body, err = jsonparser.GetString(alert, "annotations", "description")
		if err != nil || strings.TrimSpace(body) == "" {
			slog.Error("send: alert missing summary and description annotations")
			return "", fmt.Errorf("alert missing summary and description annotations")
		}
	}

	body = FindAndReplaceLabels(body, alert)

	// startsAt is optional - only include timestamp if present and valid
	if startsAt, err := jsonparser.GetString(alert, "startsAt"); err == nil {
		if parsedStartsAt, err := time.Parse(time.RFC3339, startsAt); err == nil {
			body = "\"" + body + "\"" + " alert starts at " + parsedStartsAt.Format(time.RFC1123)
		}
	}

	// Extract alert name from labels.alertname (always present per AlertManager spec, but handle gracefully)
	alertName, _ := jsonparser.GetString(alert, "labels", "alertname")
	if strings.TrimSpace(alertName) != "" {
		body = "[" + alertName + "] " + body
	}

	// Add "RESOLVED: " prefix for resolved alerts
	if status == "resolved" {
		body = "RESOLVED: " + body
	}

	// Add custom message prefix if configured (added last so it appears first in final message)
	if config.MessagePrefix != "" {
		body = config.MessagePrefix + " " + body
	}

	// Truncate message if it exceeds maximum length
	maxLen := config.MaxMessageLength
	if maxLen <= 0 {
		maxLen = 150 // Default to 150 if not set or invalid
	}
	body = TruncateMessage(body, maxLen)

	return body, nil
}
