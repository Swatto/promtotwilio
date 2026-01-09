package handler

import (
	"sync"
)

// MockTwilioClient is a mock implementation of TwilioClient for testing
type MockTwilioClient struct {
	SendMessageFunc func(to, from, body string) error
	Calls           []MockCall
	mu              sync.Mutex
}

// MockCall represents a single call to SendMessage
type MockCall struct {
	To   string
	From string
	Body string
}

// SendMessage implements the TwilioClient interface
func (m *MockTwilioClient) SendMessage(to, from, body string) error {
	m.mu.Lock()
	m.Calls = append(m.Calls, MockCall{To: to, From: from, Body: body})
	m.mu.Unlock()
	if m.SendMessageFunc != nil {
		return m.SendMessageFunc(to, from, body)
	}
	return nil
}

// CallCount returns the number of times SendMessage was called
func (m *MockTwilioClient) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Calls)
}

// GetCall returns the call at the specified index
func (m *MockTwilioClient) GetCall(index int) MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Calls[index]
}
