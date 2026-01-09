package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/buger/jsonparser"
)

// labelReg matches $labels.xxx placeholders in alert messages.
// Compiled once at package init for performance.
var labelReg = regexp.MustCompile(`\$labels\.[a-zA-Z_][a-zA-Z0-9_]*`)

// maxBodySize is the maximum allowed request body size (5 MB).
// This prevents denial-of-service attacks via large request bodies
// while allowing for large alerts or many receivers.
const maxBodySize = 5 << 20

// Config holds the configuration for the handler
//
//nolint:govet // fieldalignment: minor optimization not worth reduced readability
type Config struct {
	AccountSid    string
	AuthToken     string
	Sender        string
	Receivers     []string
	TwilioBaseURL string // Optional: override Twilio API base URL (for testing)
}

// Handler handles HTTP requests for the promtotwilio service
type Handler struct {
	Config    *Config
	Client    TwilioClient
	StartTime time.Time
	Version   string
}

// New creates a new Handler with the given configuration
func New(cfg *Config, version string) *Handler {
	client := NewTwilioClient(cfg.AccountSid, cfg.AuthToken, cfg.TwilioBaseURL)
	return &Handler{
		Config:    cfg,
		Client:    client,
		StartTime: time.Now(),
		Version:   version,
	}
}

// NewWithClient creates a new Handler with a custom TwilioClient (useful for testing)
func NewWithClient(cfg *Config, client TwilioClient, version string) *Handler {
	return &Handler{
		Config:    cfg,
		Client:    client,
		StartTime: time.Now(),
		Version:   version,
	}
}

// RegisterRoutes registers all HTTP routes on the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", h.Ping)
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("POST /send", h.SendRequest)
}

// Ping handles the ping endpoint
func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	if _, err := io.WriteString(w, "ping"); err != nil {
		slog.Error("ping: failed to write response", "error", err)
	}
}

// Health handles the health check endpoint
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.StartTime).Round(time.Second)
	response := HealthResponse{
		Status:  "ok",
		Version: h.Version,
		Uptime:  uptime.String(),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("health: failed to encode JSON response", "error", err)
	}
}

// SendRequest handles the send SMS endpoint
func (h *Handler) SendRequest(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	// Handle Content-Type case-insensitively and allow charset parameters
	// e.g., "application/json", "Application/JSON", "application/json; charset=utf-8"
	if !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		slog.Error("send: invalid Content-Type", "content_type", contentType)
		http.Error(w, "send: Content-Type must be application/json", http.StatusNotAcceptable)
		return
	}

	defer func() {
		if err := r.Body.Close(); err != nil {
			slog.Error("send: failed to close request body", "error", err)
		}
	}()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		slog.Error("send: failed to read request body", "error", err)
		http.Error(w, "send: failed to read request body", http.StatusBadRequest)
		return
	}

	status, _ := jsonparser.GetString(body, "status") //nolint:errcheck // status is optional

	// Determine receivers: query param overrides default
	receivers := h.Config.Receivers
	if rcvParam := r.URL.Query().Get("receiver"); rcvParam != "" {
		receivers = ParseReceivers(rcvParam)
	}

	if len(receivers) == 0 {
		slog.Error("send: no receiver specified")
		http.Error(w, "send: receiver not specified", http.StatusBadRequest)
		return
	}

	response := SendResponse{
		Success: true,
		Errors:  []string{},
	}

	if status == "firing" {
		var wg sync.WaitGroup
		var mu sync.Mutex
		var sendErrors []string
		var sent, failed int

		_, err := jsonparser.ArrayEach(body, func(alert []byte, dataType jsonparser.ValueType, offset int, err error) {
			for _, receiver := range receivers {
				wg.Add(1)
				// Copy alert data to avoid race condition when passing to goroutine
				// This ensures each goroutine has its own independent copy of the alert data
				alertCopy := make([]byte, len(alert))
				copy(alertCopy, alert)
				go func(rcv string, alertData []byte) {
					defer wg.Done()
					sendErr := h.sendMessage(rcv, alertData)
					mu.Lock()
					defer mu.Unlock()
					if sendErr != nil {
						failed++
						sendErrors = append(sendErrors, fmt.Sprintf("Failed to send to %s: %v", rcv, sendErr))
					} else {
						sent++
					}
				}(receiver, alertCopy)
			}
		}, "alerts")

		if err != nil {
			slog.Error("send: failed to parse alerts array", "error", err)
			http.Error(w, "send: invalid alerts format in request body", http.StatusBadRequest)
			return
		}

		wg.Wait()

		response.Sent = sent
		response.Failed = failed
		response.Errors = sendErrors
		response.Success = failed == 0
	}

	w.Header().Set("Content-Type", "application/json")
	if !response.Success {
		w.WriteHeader(http.StatusInternalServerError)
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("send: failed to encode JSON response", "error", err)
	}
}

func (h *Handler) sendMessage(receiver string, alert []byte) error {
	body, err := jsonparser.GetString(alert, "annotations", "summary")
	if err != nil || body == "" {
		slog.Error("send: alert missing summary annotation")
		return fmt.Errorf("alert missing summary annotation")
	}

	body = FindAndReplaceLabels(body, alert)

	// startsAt is optional - only include timestamp if present and valid
	if startsAt, err := jsonparser.GetString(alert, "startsAt"); err == nil {
		if parsedStartsAt, err := time.Parse(time.RFC3339, startsAt); err == nil {
			body = "\"" + body + "\"" + " alert starts at " + parsedStartsAt.Format(time.RFC1123)
		}
	}

	if err := h.Client.SendMessage(receiver, h.Config.Sender, body); err != nil {
		slog.Error("twilio: failed to send SMS", "receiver", receiver, "error", err)
		return err
	}

	slog.Info("Message sent", "receiver", receiver)
	return nil
}

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
func FindAndReplaceLabels(body string, alert []byte) string {
	matches := labelReg.FindAllString(body, -1)

	for _, match := range matches {
		labelName := strings.Split(match, ".")
		if len(labelName) == 2 {
			replaceWith, _ := jsonparser.GetString(alert, "labels", labelName[1]) //nolint:errcheck // missing label replaced with empty string
			body = strings.ReplaceAll(body, match, replaceWith)
		}
	}

	return body
}
