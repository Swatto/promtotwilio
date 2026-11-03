package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/swatto/promtotwilio/internal/handler"
)

const (
	// AppName is the name of the application
	AppName = "promtotwilio"
	// AppDescription provides a brief description of the application
	AppDescription = "Prometheus Alertmanager webhook to Twilio SMS bridge"
)

// Version can be set at build time via ldflags
var Version = "1.0.0"

// envInt returns the env var as an int of at least minVal, or def if unset.
// A malformed or too-small value is an error, so a typo can't silently
// disable a setting such as the rate limit.
func envInt(key string, def, minVal int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < minVal {
		return 0, fmt.Errorf("%s must be an integer >= %d (got %q)", key, minVal, raw)
	}
	return v, nil
}

// loadConfig reads environment variables and returns the application config and port.
func loadConfig() (*handler.Config, string, error) {
	maxLen, err := envInt("MAX_MESSAGE_LENGTH", handler.DefaultMaxMessageLength, 1)
	if err != nil {
		return nil, "", err
	}
	rateLimit, err := envInt("RATE_LIMIT", 0, 0)
	if err != nil {
		return nil, "", err
	}

	cfg := &handler.Config{
		AccountSid:       os.Getenv("SID"),
		AuthToken:        os.Getenv("TOKEN"),
		APIKey:           os.Getenv("API_KEY"),
		APIKeySecret:     os.Getenv("API_KEY_SECRET"),
		Receivers:        handler.ParseReceivers(os.Getenv("RECEIVER")),
		Sender:           os.Getenv("SENDER"),
		TwilioBaseURL:    os.Getenv("TWILIO_BASE_URL"),
		SendResolved:     os.Getenv("SEND_RESOLVED") == "true",
		MaxMessageLength: maxLen,
		MessagePrefix:    os.Getenv("MESSAGE_PREFIX"),
		RateLimit:        rateLimit,
		LogFormat:        os.Getenv("LOG_FORMAT"),
		WebhookSecret:    os.Getenv("WEBHOOK_SECRET"),
		DryRun:           os.Getenv("DRY_RUN") == "true",
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	return cfg, port, nil
}

// run contains the application lifecycle. It returns an error instead of
// calling os.Exit so that the logic is testable.
func run(ctx context.Context) error {
	cfg, port, err := loadConfig()
	if err != nil {
		return fmt.Errorf("startup: invalid configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("startup: invalid configuration: %w", err)
	}

	h := handler.New(cfg, Version)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler.LogRequests(cfg.LogFormat, mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)

	printBanner(port, cfg)

	go func() {
		slog.Info("Server started successfully", "app", AppName, "version", Version, "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("startup: failed to start HTTP server: %w", err)
	case <-ctx.Done():
		slog.Info("Shutting down server...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: server forced to terminate: %w", err)
	}

	slog.Info("Server stopped gracefully")
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}
