package cmdutil

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/signature"
)

// contextKey is the type for context keys used by SignatureTransport.
type contextKey string

// IdempotencyKeyCtx is the context key for user-specified idempotency key.
const IdempotencyKeyCtx contextKey = "idempotency-key"

// SignatureTransport implements http.RoundTripper.
// It automatically generates DateTime/MsgID, computes the message signature,
// injects required HTTP headers, and verifies response signatures.
type SignatureTransport struct {
	Base             http.RoundTripper
	ConfigFunc       func() (*core.CliConfig, error)
	KeychainResolver core.KeychainResolver
}

// RoundTrip implements http.RoundTripper.
func (t *SignatureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cfg, err := t.ConfigFunc()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// 1. Read request body (needed for signature).
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
		req.Body.Close()
		// Restore body for downstream transports.
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	bodyStr := string(bodyBytes)

	// 2. Generate DateTime (ISO 8601) and MsgID (UUID, 32 hex chars, no hyphens).
	dateTime := time.Now().Format("2006-01-02T15:04:05-07:00")
	msgID, err := generateMsgID()
	if err != nil {
		return nil, fmt.Errorf("generate MsgID: %w", err)
	}

	// 3. Resolve SignKey from config (using keychain resolver if available).
	signKey, err := cfg.ResolveSignKey(t.KeychainResolver)
	if err != nil {
		return nil, fmt.Errorf("resolve sign key: %w", err)
	}

	// 4. Build request path (path + query string, no domain).
	// Sort query params alphabetically — Evo Payment API requires sorted params for signature.
	reqPath := req.URL.Path
	if req.URL.RawQuery != "" {
		parts := strings.Split(req.URL.RawQuery, "&")
		sort.Strings(parts)
		reqPath = reqPath + "?" + strings.Join(parts, "&")
	}
	// Cryptogram API: signature must NOT include query string.
	if strings.Contains(req.URL.Path, "/cryptogram") && req.Method == http.MethodPost {
		reqPath = req.URL.Path
	}

	// 5. Compute signature.
	sig, err := signature.GenerateSignature(
		req.Method, reqPath, dateTime, signKey, msgID, bodyStr, cfg.SignType,
	)
	if err != nil {
		return nil, fmt.Errorf("generate signature: %w", err)
	}

	// 6. Inject required headers.
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("DateTime", dateTime)
	req.Header.Set("MsgID", msgID)
	req.Header.Set("SignType", cfg.SignType)
	req.Header.Set("Authorization", sig)

	// 7. If config has KeyID, inject KeyID header.
	if cfg.KeyID != "" {
		req.Header.Set("KeyID", cfg.KeyID)
	}

	// 8. For PUT/DELETE: inject Idempotency-Key header.
	if req.Method == http.MethodPut || req.Method == http.MethodDelete {
		idempotencyKey := idempotencyKeyFromContext(req.Context())
		if idempotencyKey == "" {
			idempotencyKey, err = generateUUID()
			if err != nil {
				return nil, fmt.Errorf("generate idempotency key: %w", err)
			}
		}
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	// 9. Send request via Base transport.
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	// EVO_DEBUG=1: dump HTTP request
	if os.Getenv("EVO_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "\n[DEBUG] >>> %s %s\n", req.Method, req.URL.String())
		for k, v := range req.Header {
			fmt.Fprintf(os.Stderr, "[DEBUG] >>> %s: %s\n", k, strings.Join(v, ", "))
		}
		if bodyStr != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG] >>> Body: %s\n", bodyStr)
		}
	}

	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// EVO_DEBUG=1: dump HTTP response status and headers
	if os.Getenv("EVO_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[DEBUG] <<< HTTP %d\n", resp.StatusCode)
		for k, v := range resp.Header {
			fmt.Fprintf(os.Stderr, "[DEBUG] <<< %s: %s\n", k, strings.Join(v, ", "))
		}
	}

	// 10. Verify response signature.
	if err := t.verifyResponse(resp, req.Method, reqPath, signKey, cfg.SignType); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

// verifyResponse reads the response body, verifies the signature, and restores the body.
func (t *SignatureTransport) verifyResponse(resp *http.Response, method, reqPath, signKey, signType string) error {
	// Read response body.
	var respBodyBytes []byte
	var err error
	if resp.Body != nil {
		respBodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			// Body read failed — restore an empty body and return the error.
			resp.Body = io.NopCloser(bytes.NewReader(nil))
			return fmt.Errorf("read response body: %w", err)
		}
		resp.Body.Close()
	}
	respBodyStr := string(respBodyBytes)

	// EVO_DEBUG=1: dump HTTP response body
	if os.Getenv("EVO_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[DEBUG] <<< Body: %s\n\n", respBodyStr)
	}

	// Use response DateTime and MsgID headers.
	respDateTime := resp.Header.Get("DateTime")
	respMsgID := resp.Header.Get("MsgID")
	respAuth := resp.Header.Get("Authorization")

	// If no Authorization header in response, skip verification.
	if respAuth == "" {
		resp.Body = io.NopCloser(bytes.NewReader(respBodyBytes))
		return nil
	}

	// Verify using request method and path (same as used for request signature).
	ok := signature.VerifySignature(method, reqPath, respDateTime, signKey, respMsgID, respBodyStr, signType, respAuth)
	if !ok {
		// Fallback: GET /FXRateInquiry computes response signature with POST + path without query string.
		pathOnly := strings.SplitN(reqPath, "?", 2)[0]
		ok = signature.VerifySignature("POST", pathOnly, respDateTime, signKey, respMsgID, respBodyStr, signType, respAuth)
	}
	if !ok {
		// Restore body before returning error so callers can still read it.
		resp.Body = io.NopCloser(bytes.NewReader(respBodyBytes))
		return &SignatureVerificationError{}
	}

	// Restore response body.
	resp.Body = io.NopCloser(bytes.NewReader(respBodyBytes))
	return nil
}

// SignatureVerificationError is returned when response signature verification fails.
type SignatureVerificationError struct{}

func (e *SignatureVerificationError) Error() string {
	return "response signature verification failed"
}

// Type returns the structured error type for JSON output.
func (e *SignatureVerificationError) Type() string {
	return "signature_verification_failed"
}

// Hint returns an actionable fix suggestion.
func (e *SignatureVerificationError) Hint() string {
	return "response may be tampered, do not process"
}

// generateMsgID generates a 32-character hex string (UUID without hyphens).
func generateMsgID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateUUID generates a UUID v4 string without hyphens (32 hex chars).
func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Set version 4 bits.
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant bits.
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b), nil
}

// idempotencyKeyFromContext extracts the user-specified idempotency key from context.
func idempotencyKeyFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v := ctx.Value(IdempotencyKeyCtx)
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// WithIdempotencyKey returns a new context with the given idempotency key.
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, IdempotencyKeyCtx, key)
}

// isLowercaseHex checks if a string contains only lowercase hex characters.
func isLowercaseHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return len(s) > 0
}
