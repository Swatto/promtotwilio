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

// Config holds the configuration for the handler
type Config struct {
	Receivers     []string // slice first for optimal alignment
	AccountSid    string
	AuthToken     string
	Sender        string
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
	fmt.Fprint(w, "ping")
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
		slog.Error("Failed to encode health response", "error", err)
	}
}

// SendRequest handles the send SMS endpoint
func (h *Handler) SendRequest(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusNotAcceptable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	status, _ := jsonparser.GetString(body, "status") //nolint:errcheck // status is optional

	// Determine receivers: query param overrides default
	receivers := h.Config.Receivers
	if rcvParam := r.URL.Query().Get("receiver"); rcvParam != "" {
		receivers = ParseReceivers(rcvParam)
	}

	if len(receivers) == 0 {
		slog.Error("Bad request: receiver not specified")
		http.Error(w, "receiver not specified", http.StatusBadRequest)
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
				}(receiver, alert)
			}
		}, "alerts")

		if err != nil {
			slog.Warn("Error parsing json", "error", err)
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
		slog.Error("Failed to encode send response", "error", err)
	}
}

func (h *Handler) sendMessage(receiver string, alert []byte) error {
	body, _ := jsonparser.GetString(alert, "annotations", "summary") //nolint:errcheck // empty string is acceptable

	if body == "" {
		slog.Error("Bad format: missing summary annotation")
		return fmt.Errorf("missing summary annotation")
	}

	body = FindAndReplaceLabels(body, alert)
	startsAt, _ := jsonparser.GetString(alert, "startsAt") //nolint:errcheck // startsAt is optional
	parsedStartsAt, err := time.Parse(time.RFC3339, startsAt)
	if err == nil {
		body = "\"" + body + "\"" + " alert starts at " + parsedStartsAt.Format(time.RFC1123)
	}

	err = h.Client.SendMessage(receiver, h.Config.Sender, body)
	if err != nil {
		slog.Error("Failed to send SMS", "receiver", receiver, "error", err)
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
	labelReg := regexp.MustCompile(`\$labels\.[a-zA-Z_][a-zA-Z0-9_]*`)
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
