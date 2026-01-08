package handler

// SendResponse represents the JSON response for the /send endpoint
type SendResponse struct {
	Errors  []string `json:"errors"`
	Sent    int      `json:"sent"`
	Failed  int      `json:"failed"`
	Success bool     `json:"success"`
}

// HealthResponse represents the JSON response for the /health endpoint
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  string `json:"uptime"`
}
