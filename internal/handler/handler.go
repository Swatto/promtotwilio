package handler

import (
	"context"
	"encoding/json"
	"errors"
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

const (
	// sendTimeout bounds all Twilio calls for one request. It must stay below
	// the server's WriteTimeout so the caller still gets the response.
	sendTimeout = 25 * time.Second
	// maxConcurrentSends bounds parallel Twilio calls for one request.
	maxConcurrentSends = 10
)

// DefaultMaxMessageLength is the SMS length used when MaxMessageLength is unset.
const DefaultMaxMessageLength = 150

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
	MaxMessageLength int    // Maximum message length before truncation (default: DefaultMaxMessageLength)
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

	return NewWithClient(cfg, NewTwilioClient(cfg.AccountSid, authUser, authPassword, cfg.TwilioBaseURL), version)
}

// NewWithClient creates a new Handler with a custom TwilioClient (useful for testing)
func NewWithClient(cfg *Config, client TwilioClient, version string) *Handler {
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

// RegisterRoutes registers all HTTP routes on the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.Ping)
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
	// Start the budget on arrival: the server's WriteTimeout is already running.
	ctx, cancel := context.WithTimeout(r.Context(), sendTimeout)
	defer cancel()

	contentType := r.Header.Get("Content-Type")
	// Handle Content-Type case-insensitively and allow charset parameters
	// e.g., "application/json", "Application/JSON", "application/json; charset=utf-8"
	if !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		slog.Error("send: invalid Content-Type", "content_type", contentType)
		http.Error(w, "send: Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	// The server closes r.Body itself; MaxBytesReader reports oversized bodies
	// explicitly instead of silently truncating them.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodySize))
	if err != nil {
		slog.Error("send: failed to read request body", "error", err)
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			http.Error(w, "send: request body too large", http.StatusRequestEntityTooLarge)
			return
		}
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
		sem := make(chan struct{}, maxConcurrentSends)
		var wg sync.WaitGroup
		var mu sync.Mutex
		sendErrors := []string{}
		var sent, failed int

		for i := range payload.Alerts {
			// Format once per alert; the body is identical for every receiver.
			body, fmtErr := FormatMessage(&payload.Alerts[i], status, h.Config)
			for _, rcv := range receivers {
				sem <- struct{}{}
				wg.Go(func() {
					defer func() { <-sem }()
					sendErr := fmtErr
					if sendErr == nil {
						sendErr = h.sendMessage(ctx, rcv, body)
					}
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

func (h *Handler) sendMessage(ctx context.Context, receiver, body string) error {
	if h.Config.DryRun {
		slog.Info("dry-run: would send SMS", "receiver", receiver, "body", body)
		return nil
	}

	if err := h.Client.SendMessage(ctx, receiver, h.Config.Sender, body); err != nil {
		slog.Error("twilio: failed to send SMS", "receiver", receiver, "error", err)
		return err
	}

	slog.Info("Message sent", "receiver", receiver)
	return nil
}
