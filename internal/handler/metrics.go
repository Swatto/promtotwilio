package handler

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// Metrics holds Prometheus counters for the service. Safe for concurrent use.
type Metrics struct {
	alertsProcessedTotal atomic.Uint64
	smsSentTotal         atomic.Uint64
	smsFailedTotal       atomic.Uint64
}

// NewMetrics returns a new Metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{}
}

// IncAlertsProcessed increments the alerts-processed counter.
func (m *Metrics) IncAlertsProcessed() {
	m.alertsProcessedTotal.Add(1)
}

// IncSMSSent increments the SMS sent counter.
func (m *Metrics) IncSMSSent() {
	m.smsSentTotal.Add(1)
}

// IncSMSFailed increments the SMS failed counter.
func (m *Metrics) IncSMSFailed() {
	m.smsFailedTotal.Add(1)
}

// Metrics serves GET /metrics in Prometheus text exposition format.
func (h *Handler) Metrics(w http.ResponseWriter, _ *http.Request) {
	processed := h.metrics.alertsProcessedTotal.Load()
	sent := h.metrics.smsSentTotal.Load()
	failed := h.metrics.smsFailedTotal.Load()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = fmt.Fprintf(w, "# HELP promtotwilio_alerts_processed_total Total number of alert batches processed via POST /send.\n")
	_, _ = fmt.Fprintf(w, "# TYPE promtotwilio_alerts_processed_total counter\n")
	_, _ = fmt.Fprintf(w, "promtotwilio_alerts_processed_total %d\n", processed)
	_, _ = fmt.Fprintf(w, "# HELP promtotwilio_sms_sent_total Total SMS messages sent successfully.\n")
	_, _ = fmt.Fprintf(w, "# TYPE promtotwilio_sms_sent_total counter\n")
	_, _ = fmt.Fprintf(w, "promtotwilio_sms_sent_total %d\n", sent)
	_, _ = fmt.Fprintf(w, "# HELP promtotwilio_sms_failed_total Total SMS messages that failed to send.\n")
	_, _ = fmt.Fprintf(w, "# TYPE promtotwilio_sms_failed_total counter\n")
	_, _ = fmt.Fprintf(w, "promtotwilio_sms_failed_total %d\n", failed)
}
