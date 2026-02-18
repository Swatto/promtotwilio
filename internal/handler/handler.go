package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// maxBodySize is the maximum allowed request body size (5 MB).
// This prevents denial-of-service attacks via large request bodies
// while allowing for large alerts or many receivers.
const maxBodySize = 5 << 20

// Config holds the configuration for the handler
//
//nolint:govet // fieldalignment: minor optimization not worth reduced readability
type Config struct {
	AccountSid       string
	AuthToken        string // Auth Token (used when API Key is not provided)
	APIKey           string // API Key SID (optional, takes precedence over AuthToken)
	APIKeySecret     string // API Key Secret (required if APIKey is set)
	Sender           string
	Receivers        []string
	TwilioBaseURL    string // Optional: override Twilio API base URL (for testing)
	SendResolved     bool   // Enable sending notifications for resolved alerts
	MaxMessageLength int    // Maximum message length before truncation (default: 150)
	MessagePrefix    string // Custom prefix to prepend to all messages (optional)
	RateLimit        int    // Max requests per minute on /send (0 = disabled)
	LogFormat        string // Access log format: "simple" (default) or "nginx"
	WebhookSecret    string // If set, POST /send requires Authorization: Bearer <secret>
	DryRun           bool   // If true, log messages instead of calling Twilio
}

// Validate checks that all required configuration fields are set and consistent.
func (c *Config) Validate() error {
	if c.AccountSid == "" {
		return fmt.Errorf("missing required configuration: AccountSid (env SID)")
	}
	if c.Sender == "" {
		return fmt.Errorf("missing required configuration: Sender (env SENDER)")
	}
	if c.RateLimit < 0 {
		return fmt.Errorf("RateLimit must be >= 0 (got %d)", c.RateLimit)
	}
	switch c.LogFormat {
	case "", "simple", "nginx":
	default:
		return fmt.Errorf("LogFormat must be \"simple\" or \"nginx\" (got %q)", c.LogFormat)
	}
	if c.APIKey != "" {
		if c.APIKeySecret == "" {
			return fmt.Errorf("APIKeySecret is required when APIKey is set")
		}
	} else if c.AuthToken == "" {
		return fmt.Errorf("missing required configuration: AuthToken (env TOKEN) or APIKey + APIKeySecret")
	}
	return nil
}

// Handler handles HTTP requests for the promtotwilio service
type Handler struct {
	Config      *Config
	Client      TwilioClient
	StartTime   time.Time
	Version     string
	rateLimiter *RateLimiter
	metrics     *Metrics
}

// New creates a new Handler with the given configuration
func New(cfg *Config, version string) *Handler {
	// Determine auth credentials: API Key takes precedence over Auth Token
	authUser := cfg.AccountSid
	authPassword := cfg.AuthToken
	if cfg.APIKey != "" {
		authUser = cfg.APIKey
		authPassword = cfg.APIKeySecret
	}

	client := NewTwilioClient(cfg.AccountSid, authUser, authPassword, cfg.TwilioBaseURL)
	h := &Handler{
		Config:    cfg,
		Client:    client,
		StartTime: time.Now(),
		Version:   version,
		metrics:   NewMetrics(),
	}
	if cfg.RateLimit > 0 {
		h.rateLimiter = NewRateLimiter(cfg.RateLimit)
	}
	return h
}

// NewWithClient creates a new Handler with a custom TwilioClient (useful for testing)
func NewWithClient(cfg *Config, client TwilioClient, version string) *Handler {
	return &Handler{
		Config:    cfg,
		Client:    client,
		StartTime: time.Now(),
		Version:   version,
		metrics:   NewMetrics(),
	}
}

// RegisterRoutes registers all HTTP routes on the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", h.Ping)
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /metrics", h.Metrics)

	var sendHandler http.Handler = http.HandlerFunc(h.SendRequest)
	if h.rateLimiter != nil {
		sendHandler = h.rateLimiter.Wrap(sendHandler)
	}
	if h.Config.WebhookSecret != "" {
		sendHandler = RequireWebhookAuth(h.Config.WebhookSecret, sendHandler)
	}
	mux.Handle("POST /send", sendHandler)
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

	var payload AlertManagerPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Error("send: failed to parse JSON", "error", err)
		http.Error(w, "send: invalid JSON in request body", http.StatusBadRequest)
		return
	}

	status := payload.Status

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

	// Process alerts if status is "firing" or if status is "resolved" and SendResolved is enabled
	shouldProcess := status == "firing" || (status == "resolved" && h.Config.SendResolved)

	if shouldProcess {
		h.metrics.IncAlertsProcessed()
		var wg sync.WaitGroup
		var mu sync.Mutex
		var sendErrors []string
		var sent, failed int

		for i := range payload.Alerts {
			alert := &payload.Alerts[i]
			for _, receiver := range receivers {
				rcv, a := receiver, alert
				wg.Go(func() {
					sendErr := h.sendMessage(rcv, a, status)
					mu.Lock()
					defer mu.Unlock()
					if sendErr != nil {
						failed++
						if !h.Config.DryRun {
							h.metrics.IncSMSFailed()
						}
						sendErrors = append(sendErrors, fmt.Sprintf("Failed to send to %s: %v", rcv, sendErr))
					} else {
						sent++
						if !h.Config.DryRun {
							h.metrics.IncSMSSent()
						}
					}
				})
			}
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

func (h *Handler) sendMessage(receiver string, alert *Alert, status string) error {
	body, err := FormatMessage(alert, status, h.Config)
	if err != nil {
		return err
	}

	if h.Config.DryRun {
		slog.Info("dry-run: would send SMS", "receiver", receiver, "body", body)
		return nil
	}

	if err := h.Client.SendMessage(receiver, h.Config.Sender, body); err != nil {
		slog.Error("twilio: failed to send SMS", "receiver", receiver, "error", err)
		return err
	}

	slog.Info("Message sent", "receiver", receiver)
	return nil
}
