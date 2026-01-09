package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
)

// State holds the current metric state
type State struct {
	mu           sync.RWMutex
	alertTrigger int // 0 = healthy, 1 = unhealthy (triggers alert)
}

func (s *State) GetAlertTrigger() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.alertTrigger
}

func (s *State) SetAlertTrigger(value int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alertTrigger = value
}

var state = &State{}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9100"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "mock-exporter healthy")
			return
		}
		http.NotFound(w, r)
	})

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	// Prometheus metrics endpoint
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		alertTrigger := state.GetAlertTrigger()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		// Output metrics in Prometheus exposition format
		fmt.Fprintf(w, "# HELP test_alert_trigger A metric to trigger test alerts (0=healthy, 1=unhealthy)\n")
		fmt.Fprintf(w, "# TYPE test_alert_trigger gauge\n")
		fmt.Fprintf(w, "test_alert_trigger %d\n", alertTrigger)

		// Add some additional metadata metrics
		fmt.Fprintf(w, "# HELP mock_exporter_info Information about the mock exporter\n")
		fmt.Fprintf(w, "# TYPE mock_exporter_info gauge\n")
		fmt.Fprintf(w, "mock_exporter_info{version=\"1.0.0\"} 1\n")
	})

	// Control endpoint to set metric values
	http.HandleFunc("/control", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// GET /control - return current state
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"alert_trigger": state.GetAlertTrigger(),
			})

		case http.MethodPost:
			// POST /control - set state
			var req struct {
				AlertTrigger *int `json:"alert_trigger"`
			}

			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
				return
			}

			if req.AlertTrigger != nil {
				value := *req.AlertTrigger
				if value < 0 || value > 1 {
					http.Error(w, "alert_trigger must be 0 or 1", http.StatusBadRequest)
					return
				}
				state.SetAlertTrigger(value)
				log.Printf("Set alert_trigger to %d", value)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":       true,
				"alert_trigger": state.GetAlertTrigger(),
			})

		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// Status endpoint (alias for GET /control)
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"alert_trigger": state.GetAlertTrigger(),
		})
	})

	log.Printf("Mock exporter starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
