package handler

// AlertManagerPayload represents the top-level webhook payload from Alertmanager.
// Based on Alertmanager's schema, the following fields are guaranteed:
// - status: "firing" or "resolved"
// - alerts: array (can be empty in theory, assume ≥1)
// Other fields like groupLabels, commonLabels, etc. exist but may be empty.
type AlertManagerPayload struct {
	Status string  `json:"status"`
	Alerts []Alert `json:"alerts"`
}

// Alert represents a single alert in the Alertmanager payload.
// Guaranteed fields per alert:
// - status
// - labels (never null, at least contains alertname)
// - annotations (may be empty {})
// - startsAt
// - endsAt (zero time if alert is firing)
// - generatorURL
type Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
}

// GetLabel returns the value of a label, or empty string if not present.
// Labels like severity, instance, job are user-defined and may not exist.
func (a *Alert) GetLabel(name string) string {
	if a.Labels == nil {
		return ""
	}
	return a.Labels[name]
}

// GetAnnotation returns the value of an annotation, or empty string if not present.
// Annotations like summary, description are user-defined and may not exist.
func (a *Alert) GetAnnotation(name string) string {
	if a.Annotations == nil {
		return ""
	}
	return a.Annotations[name]
}
