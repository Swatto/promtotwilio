package handler

import (
	"regexp"
	"strings"
)

// labelReg matches $labels.xxx placeholders in alert messages.
// Compiled once at package init for performance.
var labelReg = regexp.MustCompile(`\$labels\.[a-zA-Z_][a-zA-Z0-9_]*`)

// ParseReceivers splits a comma-separated string of phone numbers into a slice
func ParseReceivers(receivers string) []string {
	if receivers == "" {
		return nil
	}
	parts := strings.Split(receivers, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// FindAndReplaceLabels replaces $labels.xxx placeholders with actual label values
func FindAndReplaceLabels(body string, alert *Alert) string {
	matches := labelReg.FindAllString(body, -1)

	for _, match := range matches {
		labelName := strings.Split(match, ".")
		if len(labelName) == 2 {
			// GetLabel returns empty string if label doesn't exist (user-defined labels are not guaranteed)
			replaceWith := alert.GetLabel(labelName[1])
			body = strings.ReplaceAll(body, match, replaceWith)
		}
	}

	return body
}

// TruncateMessage truncates a message to the specified maximum length, adding "..." if truncated.
// If maxLen is <= 3, it truncates without the "..." suffix.
func TruncateMessage(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	if maxLen <= 3 {
		return msg[:maxLen]
	}
	return msg[:maxLen-3] + "..."
}
