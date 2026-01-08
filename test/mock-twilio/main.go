package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

// MessageResponse represents a Twilio message response
type MessageResponse struct {
	Sid         string    `json:"sid"`
	DateCreated time.Time `json:"date_created"`
	DateUpdated time.Time `json:"date_updated"`
	DateSent    time.Time `json:"date_sent"`
	AccountSid  string    `json:"account_sid"`
	To          string    `json:"to"`
	From        string    `json:"from"`
	Body        string    `json:"body"`
	Status      string    `json:"status"`
	NumSegments string    `json:"num_segments"`
	Direction   string    `json:"direction"`
	APIVersion  string    `json:"api_version"`
	Price       *string   `json:"price"`
	PriceUnit   string    `json:"price_unit"`
	URI         string    `json:"uri"`
}

// MessageStore stores sent messages for verification
type MessageStore struct {
	mu       sync.RWMutex
	messages []MessageResponse
}

func (s *MessageStore) Add(msg MessageResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

func (s *MessageStore) GetAll() []MessageResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]MessageResponse, len(s.messages))
	copy(result, s.messages)
	return result
}

func (s *MessageStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
}

func (s *MessageStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.messages)
}

var store = &MessageStore{}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Match Twilio's Messages.json endpoint pattern
	messagesPattern := regexp.MustCompile(`^/2010-04-01/Accounts/([^/]+)/Messages\.json$`)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Health check
		if r.URL.Path == "/" || r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "mock-twilio healthy")
			return
		}

		// GET /messages - retrieve all sent messages for verification
		if r.URL.Path == "/messages" {
			if r.Method == http.MethodGet {
				messages := store.GetAll()
				w.Header().Set("Content-Type", "application/json")
				response := map[string]interface{}{
					"count":    len(messages),
					"messages": messages,
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					log.Printf("Failed to encode messages response: %v", err)
				}
				return
			}
			if r.Method == http.MethodDelete {
				store.Clear()
				w.WriteHeader(http.StatusNoContent)
				log.Printf("Message store cleared")
				return
			}
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Handle Messages.json endpoint (Twilio API)
		matches := messagesPattern.FindStringSubmatch(r.URL.Path)
		if matches == nil {
			log.Printf("404 Not Found: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}

		if r.Method != http.MethodPost {
			log.Printf("405 Method Not Allowed: %s %s", r.Method, r.URL.Path)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		accountSid := matches[1]

		// Parse form data
		if err := r.ParseForm(); err != nil {
			log.Printf("400 Bad Request: failed to parse form: %v", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		to := r.FormValue("To")
		from := r.FormValue("From")
		body := r.FormValue("Body")

		log.Printf("Received message request: To=%s, From=%s, Body=%s", to, from, body)

		// Return a mock Twilio response
		now := time.Now().UTC()
		sid := generateMockSid()
		response := MessageResponse{
			Sid:         fmt.Sprintf("SM%s", sid),
			DateCreated: now,
			DateUpdated: now,
			DateSent:    now,
			AccountSid:  accountSid,
			To:          to,
			From:        from,
			Body:        body,
			Status:      "queued",
			NumSegments: "1",
			Direction:   "outbound-api",
			APIVersion:  "2010-04-01",
			Price:       nil,
			PriceUnit:   "USD",
			URI:         fmt.Sprintf("/2010-04-01/Accounts/%s/Messages/SM%s.json", accountSid, sid),
		}

		// Store the message for verification
		store.Add(response)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	})

	log.Printf("Mock Twilio server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func generateMockSid() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
