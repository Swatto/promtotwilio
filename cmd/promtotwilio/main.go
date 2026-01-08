package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/swatto/promtotwilio/internal/handler"
)

// Version can be set at build time via ldflags
var Version = "1.0.0"

func main() {
	cfg := &handler.Config{
		AccountSid:    os.Getenv("SID"),
		AuthToken:     os.Getenv("TOKEN"),
		Receivers:     handler.ParseReceivers(os.Getenv("RECEIVER")),
		Sender:        os.Getenv("SENDER"),
		TwilioBaseURL: os.Getenv("TWILIO_BASE_URL"),
	}

	if cfg.AccountSid == "" || cfg.AuthToken == "" || cfg.Sender == "" {
		slog.Error("'SID', 'TOKEN' and 'SENDER' environment variables need to be set")
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	h := handler.New(cfg, Version)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("Starting server", "port", port, "version", Version)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("ListenAndServe failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	// Give outstanding requests 10 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server stopped gracefully")
}
