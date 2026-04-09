package cmdutil

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"
)

// RetryTransport implements http.RoundTripper with exponential backoff retry
// for 5xx status codes and network timeouts.
type RetryTransport struct {
	Base       http.RoundTripper
	MaxRetries int // default 3

	// sleepFunc is used for testing to override time.Sleep.
	sleepFunc func(time.Duration)
}

// RoundTrip implements http.RoundTripper.
// It retries on 5xx status codes and network timeouts with exponential backoff: 1s, 2s, 4s.
func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	maxRetries := t.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	sleep := t.sleepFunc
	if sleep == nil {
		sleep = time.Sleep
	}

	// Buffer the request body so we can re-read it on retries.
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s * 2^(attempt-1) → 1s, 2s, 4s
			backoff := time.Second * time.Duration(1<<uint(attempt-1))
			sleep(backoff)

			// Restore request body for retry.
			if bodyBytes != nil {
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}

		resp, lastErr = base.RoundTrip(req)

		if lastErr != nil {
			// Retry on network timeouts.
			if isTimeout(lastErr) {
				continue
			}
			// Non-timeout network error: don't retry.
			return nil, lastErr
		}

		// Retry on 5xx status codes.
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			if attempt < maxRetries {
				// Drain and close the response body before retrying.
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				continue
			}
			// Last attempt — return the 5xx response with body intact.
			return resp, nil
		}

		// Success or non-retryable status code.
		return resp, nil
	}

	// All retries exhausted — return the last response or error.
	if lastErr != nil {
		return nil, lastErr
	}
	return resp, nil
}

// isTimeout checks if an error is a timeout error.
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	// Check for net.Error timeout interface.
	type timeoutErr interface {
		Timeout() bool
	}
	if te, ok := err.(timeoutErr); ok {
		return te.Timeout()
	}
	// Fallback: check error message for common timeout strings.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}
