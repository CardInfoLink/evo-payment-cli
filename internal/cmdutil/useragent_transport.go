package cmdutil

import (
	"net/http"

	"github.com/evopayment/evo-cli/internal/build"
)

// UserAgentTransport implements http.RoundTripper.
// It injects the User-Agent header with the CLI version into every request.
type UserAgentTransport struct {
	Base http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *UserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "evo-cli/"+build.Version)

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
