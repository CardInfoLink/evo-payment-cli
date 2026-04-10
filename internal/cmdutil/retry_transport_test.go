package cmdutil

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/quick"
	"time"
)

// timeoutError implements net.Error with Timeout() = true.
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "connection timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

// countingTransport counts how many times RoundTrip is called and returns configurable responses.
type countingTransport struct {
	calls     int
	responses []*http.Response // one per call; cycles last if exhausted
	errs      []error          // one per call; cycles last if exhausted
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	idx := c.calls
	c.calls++

	var respErr error
	if len(c.errs) > 0 {
		if idx < len(c.errs) {
			respErr = c.errs[idx]
		} else {
			respErr = c.errs[len(c.errs)-1]
		}
	}
	if respErr != nil {
		return nil, respErr
	}

	if len(c.responses) > 0 {
		if idx < len(c.responses) {
			return c.responses[idx], nil
		}
		return c.responses[len(c.responses)-1], nil
	}

	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))}, nil
}

func resp(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}
}

func TestRetryTransport_NoRetryOn200(t *testing.T) {
	ct := &countingTransport{
		responses: []*http.Response{resp(200)},
	}
	rt := &RetryTransport{Base: ct, MaxRetries: 3, sleepFunc: func(d time.Duration) {}}

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	r, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", r.StatusCode)
	}
	if ct.calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry)", ct.calls)
	}
}

func TestRetryTransport_RetriesOn500(t *testing.T) {
	ct := &countingTransport{
		responses: []*http.Response{resp(500), resp(500), resp(200)},
	}
	rt := &RetryTransport{Base: ct, MaxRetries: 3, sleepFunc: func(d time.Duration) {}}

	req, _ := http.NewRequest("POST", "https://example.com/test", strings.NewReader(`{"a":1}`))
	r, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", r.StatusCode)
	}
	if ct.calls != 3 {
		t.Errorf("calls = %d, want 3 (1 original + 2 retries)", ct.calls)
	}
}

func TestRetryTransport_RetriesOn503(t *testing.T) {
	ct := &countingTransport{
		responses: []*http.Response{resp(503), resp(200)},
	}
	rt := &RetryTransport{Base: ct, MaxRetries: 3, sleepFunc: func(d time.Duration) {}}

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	r, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", r.StatusCode)
	}
	if ct.calls != 2 {
		t.Errorf("calls = %d, want 2", ct.calls)
	}
}

func TestRetryTransport_MaxRetriesExhausted(t *testing.T) {
	ct := &countingTransport{
		responses: []*http.Response{resp(500), resp(500), resp(500), resp(500)},
	}
	rt := &RetryTransport{Base: ct, MaxRetries: 3, sleepFunc: func(d time.Duration) {}}

	req, _ := http.NewRequest("POST", "https://example.com/test", strings.NewReader(`{}`))
	r, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After 1 original + 3 retries = 4 calls, returns last 500 response.
	if r.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", r.StatusCode)
	}
	if ct.calls != 4 {
		t.Errorf("calls = %d, want 4 (1 original + 3 retries)", ct.calls)
	}
}

func TestRetryTransport_RetriesOnTimeout(t *testing.T) {
	calls := 0
	ct := &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return nil, &timeoutError{}
			}
			return resp(200), nil
		},
	}
	rt := &RetryTransport{Base: ct, MaxRetries: 3, sleepFunc: func(d time.Duration) {}}

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	r, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", r.StatusCode)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestRetryTransport_NoRetryOnNonTimeoutError(t *testing.T) {
	ct := &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("dns resolution failed")
		},
	}
	rt := &RetryTransport{Base: ct, MaxRetries: 3, sleepFunc: func(d time.Duration) {}}

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	_, err := rt.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "dns resolution failed" {
		t.Errorf("error = %q, want %q", err.Error(), "dns resolution failed")
	}
}

func TestRetryTransport_NoRetryOn4xx(t *testing.T) {
	ct := &countingTransport{
		responses: []*http.Response{resp(400)},
	}
	rt := &RetryTransport{Base: ct, MaxRetries: 3, sleepFunc: func(d time.Duration) {}}

	req, _ := http.NewRequest("POST", "https://example.com/test", strings.NewReader(`{}`))
	r, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", r.StatusCode)
	}
	if ct.calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 4xx)", ct.calls)
	}
}

func TestRetryTransport_ExponentialBackoff(t *testing.T) {
	ct := &countingTransport{
		responses: []*http.Response{resp(500), resp(500), resp(500), resp(200)},
	}

	var sleepDurations []time.Duration
	rt := &RetryTransport{
		Base:       ct,
		MaxRetries: 3,
		sleepFunc: func(d time.Duration) {
			sleepDurations = append(sleepDurations, d)
		},
	}

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect 3 sleeps: 1s, 2s, 4s
	expected := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	if len(sleepDurations) != len(expected) {
		t.Fatalf("sleep count = %d, want %d", len(sleepDurations), len(expected))
	}
	for i, d := range expected {
		if sleepDurations[i] != d {
			t.Errorf("sleep[%d] = %v, want %v", i, sleepDurations[i], d)
		}
	}
}

func TestRetryTransport_BodyPreservedOnRetry(t *testing.T) {
	var bodies []string
	ct := &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			if req.Body != nil {
				b, _ := io.ReadAll(req.Body)
				bodies = append(bodies, string(b))
			}
			if len(bodies) < 2 {
				return resp(500), nil
			}
			return resp(200), nil
		},
	}
	rt := &RetryTransport{Base: ct, MaxRetries: 3, sleepFunc: func(d time.Duration) {}}

	req, _ := http.NewRequest("POST", "https://example.com/test", strings.NewReader(`{"key":"value"}`))
	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Body should be the same on every attempt.
	for i, b := range bodies {
		if b != `{"key":"value"}` {
			t.Errorf("body[%d] = %q, want %q", i, b, `{"key":"value"}`)
		}
	}
}

func TestRetryTransport_DefaultMaxRetries(t *testing.T) {
	ct := &countingTransport{
		responses: []*http.Response{resp(500), resp(500), resp(500), resp(500)},
	}
	// MaxRetries = 0 should default to 3.
	rt := &RetryTransport{Base: ct, MaxRetries: 0, sleepFunc: func(d time.Duration) {}}

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	rt.RoundTrip(req)

	if ct.calls != 4 {
		t.Errorf("calls = %d, want 4 (default MaxRetries=3 → 1+3=4)", ct.calls)
	}
}

// mockRoundTripper is a simple function-based mock.
type mockRoundTripper struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

func TestIsTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"timeout error", &timeoutError{}, true},
		{"regular error", errors.New("something failed"), false},
		{"error with timeout in message", errors.New("connection timeout occurred"), true},
		{"error with deadline exceeded", errors.New("context deadline exceeded"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTimeout(tt.err); got != tt.want {
				t.Errorf("isTimeout(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// --- Property-Based Tests (testing/quick) ---

// Feature: evo-payment-cli, Property 24: Retry Transport 重试行为
// For any 5xx HTTP response or network timeout, RetryTransport must use exponential
// backoff retry, and total requests must never exceed 4 (1 original + 3 retries).
// **Validates: Requirement 3.2**
func TestProperty24_RetryTransportBehavior(t *testing.T) {
	f := func(statusCode uint16, useTimeout bool) bool {
		// Normalize status code to a retryable range (500-599) or non-retryable
		code := int(statusCode)%600 + 1 // 1..600

		var totalCalls int
		var sleepDurations []time.Duration

		ct := &mockRoundTripper{
			fn: func(req *http.Request) (*http.Response, error) {
				totalCalls++
				if useTimeout && totalCalls == 1 {
					// First call times out
					return nil, &timeoutError{}
				}
				return resp(code), nil
			},
		}

		rt := &RetryTransport{
			Base:       ct,
			MaxRetries: 3,
			sleepFunc: func(d time.Duration) {
				sleepDurations = append(sleepDurations, d)
			},
		}

		req, _ := http.NewRequest("POST", "https://example.com/test", strings.NewReader(`{}`))
		rt.RoundTrip(req)

		// Total calls must never exceed 4 (1 original + 3 retries)
		if totalCalls > 4 {
			t.Logf("total calls = %d, exceeds max 4 (code=%d, useTimeout=%v)", totalCalls, code, useTimeout)
			return false
		}

		// If retries happened, verify exponential backoff durations
		expectedBackoffs := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
		for i, d := range sleepDurations {
			if i >= len(expectedBackoffs) {
				break
			}
			if d != expectedBackoffs[i] {
				t.Logf("sleep[%d] = %v, want %v", i, d, expectedBackoffs[i])
				return false
			}
		}

		// If status is 5xx, should have retried (totalCalls > 1) unless all retries exhausted
		if code >= 500 && code < 600 && !useTimeout {
			if totalCalls != 4 {
				t.Logf("5xx code %d: expected 4 total calls, got %d", code, totalCalls)
				return false
			}
		}

		// If status is not 5xx and no timeout, should NOT retry
		if code < 500 && !useTimeout {
			if totalCalls != 1 {
				t.Logf("non-5xx code %d: expected 1 call, got %d", code, totalCalls)
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 24 failed: %v", err)
	}
}

// --- Last attempt body preservation on 5xx ---

func TestRetryTransport_LastAttemptBodyReadable(t *testing.T) {
	ct := &countingTransport{
		responses: []*http.Response{
			{StatusCode: 500, Body: io.NopCloser(strings.NewReader(`{"error":"attempt1"}`))},
			{StatusCode: 500, Body: io.NopCloser(strings.NewReader(`{"error":"attempt2"}`))},
			{StatusCode: 500, Body: io.NopCloser(strings.NewReader(`{"error":"attempt3"}`))},
			{StatusCode: 502, Body: io.NopCloser(strings.NewReader(`{"error":"last attempt"}`))},
		},
	}
	rt := &RetryTransport{Base: ct, MaxRetries: 3, sleepFunc: func(d time.Duration) {}}

	req, _ := http.NewRequest("POST", "https://example.com/test", strings.NewReader(`{}`))
	r, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.StatusCode != 502 {
		t.Errorf("StatusCode = %d, want 502", r.StatusCode)
	}

	// The last response body must still be readable (not closed).
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read last response body: %v", err)
	}
	if string(body) != `{"error":"last attempt"}` {
		t.Errorf("body = %q, want %q", string(body), `{"error":"last attempt"}`)
	}
}
