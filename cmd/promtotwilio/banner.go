package main

import (
	"fmt"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/swatto/promtotwilio/internal/handler"
)

const (
	boxInnerWidth = 64
	configValueAt = 24 // column where config values start
)

// padCenter returns s centered in a string of length width, padded with spaces.
func padCenter(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	pad := width - len(s)
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// boxLine returns a box line with s centered between the vertical borders.
func boxLine(s string) string {
	return "║" + padCenter(s, boxInnerWidth) + "║"
}

// configLine returns a config line with label and value, value aligned at configValueAt.
// There is always at least one space between the colon and the value.
// Uses rune count for padding so multi-byte characters (e.g. •) don't break alignment.
func configLine(label, value string) string {
	prefix := "    • " + label + ":"
	prefixWidth := utf8.RuneCountInString(prefix)
	pad := max(1, configValueAt-prefixWidth)
	return prefix + strings.Repeat(" ", pad) + value
}

// printBanner prints startup information about the application
func printBanner(port string, cfg *handler.Config) {
	border := "╔" + strings.Repeat("═", boxInnerWidth) + "╗"
	fmt.Println()
	fmt.Println(border)
	fmt.Println(boxLine(AppName))
	fmt.Println(boxLine(AppDescription))
	fmt.Println("╚" + strings.Repeat("═", boxInnerWidth) + "╝")
	fmt.Println()
	fmt.Printf("  Version:        %s\n", Version)
	fmt.Printf("  Go version:     %s\n", runtime.Version())
	fmt.Printf("  OS/Arch:        %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()
	fmt.Println("  Configuration:")
	fmt.Println(configLine("Port", port))
	fmt.Println(configLine("Sender", cfg.Sender))
	fmt.Println(configLine("Receivers", fmt.Sprintf("%d configured", len(cfg.Receivers))))
	fmt.Println(configLine("Max message len", fmt.Sprintf("%d chars", cfg.MaxMessageLength)))
	fmt.Println(configLine("Send resolved", fmt.Sprintf("%t", cfg.SendResolved)))
	if cfg.APIKey != "" {
		fmt.Println(configLine("Auth method", "API Key (recommended)"))
	} else {
		fmt.Println(configLine("Auth method", "Account SID/Token"))
	}
	logFmt := cfg.LogFormat
	if logFmt == "" {
		logFmt = "simple"
	}
	fmt.Println(configLine("Log format", logFmt))
	if cfg.RateLimit > 0 {
		fmt.Println(configLine("Rate limit", fmt.Sprintf("%d req/min", cfg.RateLimit)))
	}
	if cfg.MessagePrefix != "" {
		fmt.Println(configLine("Message prefix", fmt.Sprintf("%q", cfg.MessagePrefix)))
	}
	if cfg.TwilioBaseURL != "" {
		fmt.Println(configLine("Twilio base URL", cfg.TwilioBaseURL+" (custom)"))
	}
	if cfg.WebhookSecret != "" {
		fmt.Println(configLine("Webhook auth", "enabled (Bearer)"))
	}
	if cfg.DryRun {
		fmt.Println(configLine("Dry-run", "enabled (no SMS sent)"))
	}
	fmt.Println()
	fmt.Printf("  Server listening on http://0.0.0.0:%s\n", port)
	fmt.Println()
}
