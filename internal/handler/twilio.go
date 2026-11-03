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
	twilioRequestTimeout = 10 * time.Second // per attempt
	maxErrorBodySize     = 4 << 10
)

// TwilioClient is an interface for sending SMS messages
type TwilioClient interface {
	SendMessage(ctx context.Context, to, from, body string) error
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
// It stops retrying once ctx is done.
func (t *TwilioHTTPClient) SendMessage(ctx context.Context, to, from, body string) error {
	apiURL := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", t.baseURL, t.accountSid)
	data := url.Values{}
	data.Set("To", to)
	data.Set("From", from)
	data.Set("Body", body)
	encoded := data.Encode()

	var lastErr error
	backoff := []time.Duration{0, time.Second, 2 * time.Second}

	for attempt := range twilioMaxRetries {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("%w (last error: %w)", ctx.Err(), lastErr)
			case <-time.After(backoff[attempt]):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(encoded))
		if err != nil {
			return fmt.Errorf("twilio: failed to create HTTP request: %w", err)
		}
		req.SetBasicAuth(t.authUser, t.authPassword)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := t.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("twilio: failed to send HTTP request: %w", err)
			if ctx.Err() == nil && isRetryableNetError(err) {
				continue
			}
			return lastErr
		}

		// A 2xx means Twilio accepted the message: never retry it, even if the
		// body can't be read, or the SMS would be sent twice.
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			_ = resp.Body.Close()
			return nil
		}

		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		_ = resp.Body.Close()
		lastErr = fmt.Errorf("twilio: API error (status %d): %s", resp.StatusCode, respBody)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
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
