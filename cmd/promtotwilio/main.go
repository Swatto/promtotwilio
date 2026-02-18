package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
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

// printBanner prints startup information about the application
func printBanner(port string, cfg *handler.Config) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                         promtotwilio                         ║")
	fmt.Println("║          Prometheus Alertmanager → Twilio SMS Bridge         ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  Version:        %s\n", Version)
	fmt.Printf("  Go version:     %s\n", runtime.Version())
	fmt.Printf("  OS/Arch:        %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()
	fmt.Println("  Configuration:")
	fmt.Printf("    • Port:              %s\n", port)
	fmt.Printf("    • Sender:            %s\n", cfg.Sender)
	fmt.Printf("    • Receivers:         %d configured\n", len(cfg.Receivers))
	fmt.Printf("    • Max message len:   %d chars\n", cfg.MaxMessageLength)
	fmt.Printf("    • Send resolved:     %t\n", cfg.SendResolved)
	if cfg.APIKey != "" {
		fmt.Println("    • Auth method:       API Key (recommended)")
	} else {
		fmt.Println("    • Auth method:       Account SID/Token")
	}
	logFmt := cfg.LogFormat
	if logFmt == "" {
		logFmt = "simple"
	}
	fmt.Printf("    • Log format:        %s\n", logFmt)
	if cfg.RateLimit > 0 {
		fmt.Printf("    • Rate limit:        %d req/min\n", cfg.RateLimit)
	}
	if cfg.MessagePrefix != "" {
		fmt.Printf("    • Message prefix:    %q\n", cfg.MessagePrefix)
	}
	if cfg.TwilioBaseURL != "" {
		fmt.Printf("    • Twilio base URL:   %s (custom)\n", cfg.TwilioBaseURL)
	}
	if cfg.WebhookSecret != "" {
		fmt.Println("    • Webhook auth:       enabled (Bearer)")
	}
	if cfg.DryRun {
		fmt.Println("    • Dry-run:             enabled (no SMS sent)")
	}
	fmt.Println()
	fmt.Printf("  Server listening on http://0.0.0.0:%s\n", port)
	fmt.Println()
}

// loadConfig reads environment variables and returns the application config and port.
func loadConfig() (*handler.Config, string) {
	maxMessageLength := 150
	if maxLenStr := os.Getenv("MAX_MESSAGE_LENGTH"); maxLenStr != "" {
		if parsedLen, err := strconv.Atoi(maxLenStr); err == nil && parsedLen > 0 {
			maxMessageLength = parsedLen
		}
	}

	var rateLimit int
	if rlStr := os.Getenv("RATE_LIMIT"); rlStr != "" {
		if parsed, err := strconv.Atoi(rlStr); err == nil && parsed > 0 {
			rateLimit = parsed
		}
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
		MaxMessageLength: maxMessageLength,
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

	return cfg, port
}

// run contains the application lifecycle. It returns an error instead of
// calling os.Exit so that the logic is testable.
func run(ctx context.Context) error {
	cfg, port := loadConfig()

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
