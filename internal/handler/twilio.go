package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTwilioBaseURL = "https://api.twilio.com"
	twilioMaxRetries     = 3
	twilioRequestTimeout = 30 * time.Second
)

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
		httpClient:   &http.Client{Timeout: twilioRequestTimeout},
	}
}

// SendMessage sends an SMS using the Twilio REST API with retries on 5xx, 429, and transient errors.
func (t *TwilioHTTPClient) SendMessage(to, from, body string) error {
	apiURL := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", t.baseURL, t.accountSid)
	data := url.Values{}
	data.Set("To", to)
	data.Set("From", from)
	data.Set("Body", body)
	encoded := data.Encode()

	var lastErr error
	backoff := []time.Duration{0, time.Second, 2 * time.Second}

	for attempt := 0; attempt < twilioMaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff[attempt])
		}

		ctx, cancel := context.WithTimeout(context.Background(), twilioRequestTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(encoded))
		if err != nil {
			cancel()
			return fmt.Errorf("twilio: failed to create HTTP request: %w", err)
		}
		req.SetBasicAuth(t.authUser, t.authPassword)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := t.httpClient.Do(req)
		cancel()
		if err != nil {
			lastErr = fmt.Errorf("twilio: failed to send HTTP request: %w", err)
			if isRetryableNetError(err) {
				continue
			}
			return lastErr
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("twilio: failed to read response: %w", readErr)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		lastErr = fmt.Errorf("twilio: API error (status %d): %s", resp.StatusCode, string(respBody))
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			continue
		}
		return lastErr
	}
	return lastErr
}

func isRetryableNetError(err error) bool {
	var netErr interface{ Timeout() bool }
	return errors.As(err, &netErr) && netErr.Timeout() || errors.Is(err, context.DeadlineExceeded)
}
