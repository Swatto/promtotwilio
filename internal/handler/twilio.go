package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTwilioBaseURL = "https://api.twilio.com"

// TwilioClient is an interface for sending SMS messages
type TwilioClient interface {
	SendMessage(to, from, body string) error
}

// TwilioHTTPClient sends SMS via direct HTTP calls to Twilio API
type TwilioHTTPClient struct {
	httpClient   *http.Client
	accountSid   string // For URL construction
	authUser     string // API Key SID or Account SID
	authPassword string // API Key Secret or Auth Token
	baseURL      string
}

// NewTwilioClient creates a new TwilioHTTPClient
// accountSid is used for URL construction, authUser/authPassword are for HTTP Basic Auth.
// If baseURL is empty, defaults to the official Twilio API URL.
func NewTwilioClient(accountSid, authUser, authPassword, baseURL string) *TwilioHTTPClient {
	if baseURL == "" {
		baseURL = defaultTwilioBaseURL
	}
	return &TwilioHTTPClient{
		accountSid:   accountSid,
		authUser:     authUser,
		authPassword: authPassword,
		baseURL:      baseURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// SendMessage sends an SMS using the Twilio REST API
func (t *TwilioHTTPClient) SendMessage(to, from, body string) error {
	apiURL := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", t.baseURL, t.accountSid)

	data := url.Values{}
	data.Set("To", to)
	data.Set("From", from)
	data.Set("Body", body)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("twilio: failed to create HTTP request: %w", err)
	}

	req.SetBasicAuth(t.authUser, t.authPassword)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("twilio: failed to send HTTP request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("twilio: failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("twilio: API error (status %d), failed to read error response", resp.StatusCode)
		}
		return fmt.Errorf("twilio: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
