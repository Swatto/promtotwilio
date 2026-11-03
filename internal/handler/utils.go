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

// FindAndReplaceLabels replaces $labels.xxx placeholders with actual label values.
// Missing labels are replaced with an empty string.
func FindAndReplaceLabels(body string, alert *Alert) string {
	return labelReg.ReplaceAllStringFunc(body, func(match string) string {
		return alert.GetLabel(strings.TrimPrefix(match, "$labels."))
	})
}

// TruncateMessage truncates a message to at most maxLen characters (runes), adding "..." if truncated.
// If maxLen is <= 3, it truncates without the "..." suffix. Multi-byte characters are never split.
func TruncateMessage(msg string, maxLen int) string {
	runes := []rune(msg)
	if len(runes) <= maxLen {
		return msg
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
