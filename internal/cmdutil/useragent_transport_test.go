package cmdutil

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/evopayment/evo-cli/internal/build"
)

func TestUserAgentTransport_InjectsHeader(t *testing.T) {
	var capturedUA string
	mock := &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			capturedUA = req.Header.Get("User-Agent")
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
		},
	}

	ua := &UserAgentTransport{Base: mock}
	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	_, err := ua.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "evo-cli/" + build.Version
	if capturedUA != expected {
		t.Errorf("User-Agent = %q, want %q", capturedUA, expected)
	}
}

func TestUserAgentTransport_OverridesExistingHeader(t *testing.T) {
	var capturedUA string
	mock := &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			capturedUA = req.Header.Get("User-Agent")
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
		},
	}

	ua := &UserAgentTransport{Base: mock}
	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	req.Header.Set("User-Agent", "old-agent/1.0")
	_, err := ua.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "evo-cli/" + build.Version
	if capturedUA != expected {
		t.Errorf("User-Agent = %q, want %q (should override existing)", capturedUA, expected)
	}
}

func TestUserAgentTransport_PassesThroughResponse(t *testing.T) {
	mock := &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 201,
				Body:       io.NopCloser(strings.NewReader(`{"created":true}`)),
			}, nil
		},
	}

	ua := &UserAgentTransport{Base: mock}
	req, _ := http.NewRequest("POST", "https://example.com/test", strings.NewReader(`{}`))
	r, err := ua.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.StatusCode != 201 {
		t.Errorf("StatusCode = %d, want 201", r.StatusCode)
	}
	body, _ := io.ReadAll(r.Body)
	if string(body) != `{"created":true}` {
		t.Errorf("body = %q, want %q", string(body), `{"created":true}`)
	}
}

func TestUserAgentTransport_UsesDevVersionByDefault(t *testing.T) {
	// build.Version defaults to "dev" when not injected via ldflags.
	var capturedUA string
	mock := &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			capturedUA = req.Header.Get("User-Agent")
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
		},
	}

	ua := &UserAgentTransport{Base: mock}
	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	ua.RoundTrip(req)

	if !strings.HasPrefix(capturedUA, "evo-cli/") {
		t.Errorf("User-Agent = %q, should start with 'evo-cli/'", capturedUA)
	}
}
