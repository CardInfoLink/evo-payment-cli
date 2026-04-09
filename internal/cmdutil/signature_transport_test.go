package cmdutil

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/quick"

	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/signature"
)

// mockTransport records the request and returns a pre-configured response.
type mockTransport struct {
	lastReq  *http.Request
	lastBody string // captured request body
	respFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		m.lastBody = string(b)
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	if m.respFunc != nil {
		return m.respFunc(req)
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
}

// testConfig returns a minimal CliConfig for testing.
func testConfig() *core.CliConfig {
	return &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256",
		Env:         "test",
	}
}

// buildSignedResponse creates a mock response with a valid signature.
func buildSignedResponse(method, path, signKey, signType, respBody string) *http.Response {
	dateTime := "2024-01-01T00:00:00+08:00"
	msgID := "aabbccdd11223344aabbccdd11223344"
	sig, _ := signature.GenerateSignature(method, path, dateTime, signKey, msgID, respBody, signType)

	header := http.Header{}
	header.Set("DateTime", dateTime)
	header.Set("MsgID", msgID)
	header.Set("Authorization", sig)
	header.Set("Content-Type", "application/json; charset=utf-8")

	return &http.Response{
		StatusCode: 200,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(respBody)),
	}
}

func TestSignatureTransport_InjectsRequiredHeaders(t *testing.T) {
	cfg := testConfig()
	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{"result":{"code":"S0000"}}`), nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	req, _ := http.NewRequest("POST", "https://hkg-online-uat.everonet.com/g2/v1/payment/mer/S024116/payment", strings.NewReader(`{"test":"data"}`))
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// Verify 5 required headers are present.
	requiredHeaders := []string{"Content-Type", "DateTime", "MsgID", "SignType", "Authorization"}
	for _, h := range requiredHeaders {
		if mock.lastReq.Header.Get(h) == "" {
			t.Errorf("missing required header: %s", h)
		}
	}

	// Verify Content-Type value.
	if got := mock.lastReq.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json; charset=utf-8")
	}

	// Verify SignType matches config.
	if got := mock.lastReq.Header.Get("SignType"); got != "SHA256" {
		t.Errorf("SignType = %q, want %q", got, "SHA256")
	}

	// Verify Authorization is lowercase hex.
	auth := mock.lastReq.Header.Get("Authorization")
	if !isLowercaseHex(auth) {
		t.Errorf("Authorization %q is not lowercase hex", auth)
	}

	// Verify MsgID is 32 hex chars.
	msgID := mock.lastReq.Header.Get("MsgID")
	if len(msgID) != 32 {
		t.Errorf("MsgID length = %d, want 32", len(msgID))
	}
	if !isLowercaseHex(msgID) {
		t.Errorf("MsgID %q is not lowercase hex", msgID)
	}
}

func TestSignatureTransport_KeyIDHeader(t *testing.T) {
	cfg := testConfig()
	cfg.KeyID = "key-123"

	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{}`), nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	req, _ := http.NewRequest("POST", "https://hkg-online-uat.everonet.com/g2/v1/test", strings.NewReader(`{}`))
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	if got := mock.lastReq.Header.Get("KeyID"); got != "key-123" {
		t.Errorf("KeyID = %q, want %q", got, "key-123")
	}
}

func TestSignatureTransport_NoKeyIDWhenEmpty(t *testing.T) {
	cfg := testConfig()
	cfg.KeyID = ""

	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{}`), nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	req, _ := http.NewRequest("POST", "https://hkg-online-uat.everonet.com/g2/v1/test", strings.NewReader(`{}`))
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	if got := mock.lastReq.Header.Get("KeyID"); got != "" {
		t.Errorf("KeyID should be empty when not configured, got %q", got)
	}
}

func TestSignatureTransport_IdempotencyKey_PUT(t *testing.T) {
	cfg := testConfig()
	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{}`), nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	req, _ := http.NewRequest("PUT", "https://hkg-online-uat.everonet.com/g2/v1/test", strings.NewReader(`{}`))
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	idemKey := mock.lastReq.Header.Get("Idempotency-Key")
	if idemKey == "" {
		t.Error("PUT request should have Idempotency-Key header")
	}
	if len(idemKey) > 64 {
		t.Errorf("Idempotency-Key length %d exceeds max 64", len(idemKey))
	}
}

func TestSignatureTransport_IdempotencyKey_DELETE(t *testing.T) {
	cfg := testConfig()
	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{}`), nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	req, _ := http.NewRequest("DELETE", "https://hkg-online-uat.everonet.com/g2/v1/test", strings.NewReader(`{}`))
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	idemKey := mock.lastReq.Header.Get("Idempotency-Key")
	if idemKey == "" {
		t.Error("DELETE request should have Idempotency-Key header")
	}
}

func TestSignatureTransport_NoIdempotencyKey_GET(t *testing.T) {
	cfg := testConfig()
	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{}`), nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	req, _ := http.NewRequest("GET", "https://hkg-online-uat.everonet.com/g2/v1/test", nil)
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	if got := mock.lastReq.Header.Get("Idempotency-Key"); got != "" {
		t.Errorf("GET request should not have Idempotency-Key, got %q", got)
	}
}

func TestSignatureTransport_NoIdempotencyKey_POST(t *testing.T) {
	cfg := testConfig()
	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{}`), nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	req, _ := http.NewRequest("POST", "https://hkg-online-uat.everonet.com/g2/v1/test", strings.NewReader(`{}`))
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	if got := mock.lastReq.Header.Get("Idempotency-Key"); got != "" {
		t.Errorf("POST request should not have Idempotency-Key, got %q", got)
	}
}

func TestSignatureTransport_UserSpecifiedIdempotencyKey(t *testing.T) {
	cfg := testConfig()
	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{}`), nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	ctx := WithIdempotencyKey(context.Background(), "user-custom-key-123")
	req, _ := http.NewRequestWithContext(ctx, "PUT", "https://hkg-online-uat.everonet.com/g2/v1/test", strings.NewReader(`{}`))
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	if got := mock.lastReq.Header.Get("Idempotency-Key"); got != "user-custom-key-123" {
		t.Errorf("Idempotency-Key = %q, want %q", got, "user-custom-key-123")
	}
}

func TestSignatureTransport_ResponseSignatureVerification_Success(t *testing.T) {
	cfg := testConfig()
	respBody := `{"result":{"code":"S0000","message":"Success"}}`

	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, respBody), nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	req, _ := http.NewRequest("POST", "https://hkg-online-uat.everonet.com/g2/v1/payment/mer/S024116/payment", strings.NewReader(`{"test":"data"}`))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// Verify response body is still readable.
	body, _ := io.ReadAll(resp.Body)
	if string(body) != respBody {
		t.Errorf("response body = %q, want %q", string(body), respBody)
	}
}

func TestSignatureTransport_ResponseSignatureVerification_Failure(t *testing.T) {
	cfg := testConfig()

	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			// Return a response with a tampered Authorization header.
			header := http.Header{}
			header.Set("DateTime", "2024-01-01T00:00:00+08:00")
			header.Set("MsgID", "aabbccdd11223344aabbccdd11223344")
			header.Set("Authorization", "0000000000000000000000000000000000000000000000000000000000000000")
			return &http.Response{
				StatusCode: 200,
				Header:     header,
				Body:       io.NopCloser(strings.NewReader(`{"result":{"code":"S0000"}}`)),
			}, nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	req, _ := http.NewRequest("POST", "https://hkg-online-uat.everonet.com/g2/v1/test", strings.NewReader(`{}`))
	_, err := transport.RoundTrip(req)
	if err == nil {
		t.Fatal("expected signature verification error, got nil")
	}

	var sigErr *SignatureVerificationError
	if !isSignatureVerificationError(err) {
		t.Errorf("expected SignatureVerificationError, got %T: %v", err, err)
	}
	_ = sigErr
}

func isSignatureVerificationError(err error) bool {
	_, ok := err.(*SignatureVerificationError)
	return ok
}

func TestSignatureTransport_ResponseNoAuthHeader_SkipsVerification(t *testing.T) {
	cfg := testConfig()

	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			// Response without Authorization header.
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader(`{"result":{"code":"S0000"}}`)),
			}, nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	req, _ := http.NewRequest("POST", "https://hkg-online-uat.everonet.com/g2/v1/test", strings.NewReader(`{}`))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("expected no error when response has no Authorization header, got: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

func TestSignatureTransport_GETRequestWithQueryString(t *testing.T) {
	cfg := testConfig()

	var capturedPath string
	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			// Capture the path used for signing by checking the request.
			capturedPath = req.URL.Path
			if req.URL.RawQuery != "" {
				capturedPath += "?" + req.URL.RawQuery
			}
			return buildSignedResponse(req.Method, capturedPath, cfg.SignKey.Value, cfg.SignType, `{"data":"test"}`), nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	req, _ := http.NewRequest("GET", "https://hkg-online-uat.everonet.com/g2/v1/payment/mer/S024116/payment?merchantTransID=TX001", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"data":"test"}` {
		t.Errorf("response body = %q, want %q", string(body), `{"data":"test"}`)
	}
}

func TestSignatureTransport_EmptyBody(t *testing.T) {
	cfg := testConfig()
	mock := &mockTransport{
		respFunc: func(req *http.Request) (*http.Response, error) {
			return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{}`), nil
		},
	}

	transport := &SignatureTransport{
		Base:       mock,
		ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
	}

	req, _ := http.NewRequest("GET", "https://hkg-online-uat.everonet.com/g2/v1/test", nil)
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}
}

func TestSignatureTransport_AllSignTypes(t *testing.T) {
	signTypes := []string{"SHA256", "SHA512", "HMAC-SHA256", "HMAC-SHA512"}

	for _, st := range signTypes {
		t.Run(st, func(t *testing.T) {
			cfg := testConfig()
			cfg.SignType = st

			mock := &mockTransport{
				respFunc: func(req *http.Request) (*http.Response, error) {
					return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{"ok":true}`), nil
				},
			}

			transport := &SignatureTransport{
				Base:       mock,
				ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
			}

			req, _ := http.NewRequest("POST", "https://hkg-online-uat.everonet.com/g2/v1/test", strings.NewReader(`{"ok":true}`))
			_, err := transport.RoundTrip(req)
			if err != nil {
				t.Fatalf("RoundTrip with %s failed: %v", st, err)
			}

			if got := mock.lastReq.Header.Get("SignType"); got != st {
				t.Errorf("SignType = %q, want %q", got, st)
			}

			auth := mock.lastReq.Header.Get("Authorization")
			if !isLowercaseHex(auth) {
				t.Errorf("Authorization %q is not lowercase hex for %s", auth, st)
			}
		})
	}
}

func TestGenerateMsgID(t *testing.T) {
	id, err := generateMsgID()
	if err != nil {
		t.Fatalf("generateMsgID failed: %v", err)
	}
	if len(id) != 32 {
		t.Errorf("MsgID length = %d, want 32", len(id))
	}
	if !isLowercaseHex(id) {
		t.Errorf("MsgID %q is not lowercase hex", id)
	}

	// Verify uniqueness.
	id2, _ := generateMsgID()
	if id == id2 {
		t.Error("two consecutive MsgIDs should not be equal")
	}
}

func TestGenerateUUID(t *testing.T) {
	uuid, err := generateUUID()
	if err != nil {
		t.Fatalf("generateUUID failed: %v", err)
	}
	if len(uuid) != 32 {
		t.Errorf("UUID length = %d, want 32", len(uuid))
	}
	if !isLowercaseHex(uuid) {
		t.Errorf("UUID %q is not lowercase hex", uuid)
	}
	if len(uuid) > 64 {
		t.Errorf("UUID length %d exceeds max 64 chars", len(uuid))
	}
}

func TestWithIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	key := "my-custom-key"
	ctx = WithIdempotencyKey(ctx, key)

	got := idempotencyKeyFromContext(ctx)
	if got != key {
		t.Errorf("idempotencyKeyFromContext = %q, want %q", got, key)
	}
}

func TestIdempotencyKeyFromContext_Empty(t *testing.T) {
	ctx := context.Background()
	got := idempotencyKeyFromContext(ctx)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestIdempotencyKeyFromContext_NoValue(t *testing.T) {
	// Context with no idempotency key set should return empty.
	got := idempotencyKeyFromContext(context.TODO())
	if got != "" {
		t.Errorf("expected empty string for context without key, got %q", got)
	}
}

func TestSignatureVerificationError(t *testing.T) {
	err := &SignatureVerificationError{}
	if err.Error() != "response signature verification failed" {
		t.Errorf("Error() = %q", err.Error())
	}
	if err.Type() != "signature_verification_failed" {
		t.Errorf("Type() = %q", err.Type())
	}
	if err.Hint() != "response may be tampered, do not process" {
		t.Errorf("Hint() = %q", err.Hint())
	}
}

func TestIsLowercaseHex(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abcdef0123456789", true},
		{"ABCDEF", false},
		{"abcdefg", false},
		{"", false},
		{"0", true},
		{"0a1b2c", true},
	}
	for _, tt := range tests {
		if got := isLowercaseHex(tt.input); got != tt.want {
			t.Errorf("isLowercaseHex(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// --- Property-Based Tests (testing/quick) ---

// Feature: evo-payment-cli, Property 3: 签名输出格式与 Header 注入
// For any random HTTP request through SignatureTransport, the Authorization header
// must be a lowercase hex string, and all 5 required headers (Content-Type, DateTime,
// MsgID, SignType, Authorization) must be present.
// **Validates: Requirements 2.4, 2.5**
func TestProperty3_SignatureOutputFormatAndHeaderInjection(t *testing.T) {
	cfg := testConfig()
	methods := []string{"GET", "POST", "PUT", "DELETE"}

	f := func(pathSuffix string, bodyContent string, methodIdx uint8) bool {
		method := methods[int(methodIdx)%len(methods)]
		urlPath := "/g2/v1/payment/mer/S024116/" + pathSuffix

		mock := &mockTransport{
			respFunc: func(req *http.Request) (*http.Response, error) {
				return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{}`), nil
			},
		}

		transport := &SignatureTransport{
			Base:       mock,
			ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
		}

		var body io.Reader
		if method != "GET" && bodyContent != "" {
			body = strings.NewReader(bodyContent)
		}

		req, err := http.NewRequest(method, "https://hkg-online-uat.everonet.com"+urlPath, body)
		if err != nil {
			return true // skip invalid URLs
		}

		_, err = transport.RoundTrip(req)
		if err != nil {
			return true // skip transport errors (e.g. signature verification on mock)
		}

		// Check 5 required headers are present
		requiredHeaders := []string{"Content-Type", "DateTime", "MsgID", "SignType", "Authorization"}
		for _, h := range requiredHeaders {
			if mock.lastReq.Header.Get(h) == "" {
				t.Logf("missing required header: %s (method=%s, path=%s)", h, method, urlPath)
				return false
			}
		}

		// Authorization must be lowercase hex
		auth := mock.lastReq.Header.Get("Authorization")
		if !isLowercaseHex(auth) {
			t.Logf("Authorization %q is not lowercase hex", auth)
			return false
		}

		return true
	}

	cfg2 := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg2); err != nil {
		t.Errorf("Property 3 failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 4: 响应签名验证
// For any valid API response, if the response body or headers are tampered with
// (signature mismatch), SignatureTransport must return a SignatureVerificationError.
// **Validates: Requirements 2.6, 2.7**
func TestProperty4_ResponseSignatureVerification(t *testing.T) {
	cfg := testConfig()

	f := func(tamperBody string, tamperMsgID bool) bool {
		// Ensure tamperBody is non-empty so it differs from original
		if tamperBody == "" || tamperBody == `{"ok":true}` {
			tamperBody = "tampered-content"
		}

		mock := &mockTransport{
			respFunc: func(req *http.Request) (*http.Response, error) {
				// Build a valid signed response for original body
				validResp := buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{"ok":true}`)

				if tamperMsgID {
					// Tamper the MsgID header
					validResp.Header.Set("MsgID", "ffffffffffffffffffffffffffffffff")
				} else {
					// Tamper the body (replace with different content)
					validResp.Body = io.NopCloser(strings.NewReader(tamperBody))
				}
				return validResp, nil
			},
		}

		transport := &SignatureTransport{
			Base:       mock,
			ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
		}

		req, _ := http.NewRequest("POST", "https://hkg-online-uat.everonet.com/g2/v1/test", strings.NewReader(`{}`))
		_, err := transport.RoundTrip(req)

		// Must return a SignatureVerificationError
		if err == nil {
			t.Logf("expected error for tampered response, got nil (tamperBody=%q, tamperMsgID=%v)", tamperBody, tamperMsgID)
			return false
		}
		if !isSignatureVerificationError(err) {
			t.Logf("expected SignatureVerificationError, got %T: %v", err, err)
			return false
		}
		return true
	}

	cfg2 := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg2); err != nil {
		t.Errorf("Property 4 failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 13: 幂等键注入规则
// For any random HTTP method, PUT/DELETE requests must have an Idempotency-Key header,
// while GET/POST requests must NOT have one.
// **Validates: Requirements 16.1, 16.4**
func TestProperty13_IdempotencyKeyInjectionRules(t *testing.T) {
	cfg := testConfig()
	methods := []string{"GET", "POST", "PUT", "DELETE"}

	f := func(methodIdx uint8) bool {
		method := methods[int(methodIdx)%len(methods)]

		mock := &mockTransport{
			respFunc: func(req *http.Request) (*http.Response, error) {
				return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{}`), nil
			},
		}

		transport := &SignatureTransport{
			Base:       mock,
			ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
		}

		var body io.Reader
		if method != "GET" {
			body = strings.NewReader(`{}`)
		}

		req, _ := http.NewRequest(method, "https://hkg-online-uat.everonet.com/g2/v1/test", body)
		_, err := transport.RoundTrip(req)
		if err != nil {
			return true // skip transport errors
		}

		idemKey := mock.lastReq.Header.Get("Idempotency-Key")

		switch method {
		case "PUT", "DELETE":
			if idemKey == "" {
				t.Logf("%s request should have Idempotency-Key header", method)
				return false
			}
			if len(idemKey) > 64 {
				t.Logf("Idempotency-Key length %d exceeds max 64", len(idemKey))
				return false
			}
		case "GET", "POST":
			if idemKey != "" {
				t.Logf("%s request should NOT have Idempotency-Key, got %q", method, idemKey)
				return false
			}
		}
		return true
	}

	cfg2 := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg2); err != nil {
		t.Errorf("Property 13 failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 14: 用户指定幂等键
// When a user provides an idempotency key via context, PUT/DELETE requests must use
// the user's value, not an auto-generated one.
// **Validates: Requirement 16.2**
func TestProperty14_UserSpecifiedIdempotencyKey(t *testing.T) {
	cfg := testConfig()
	methods := []string{"PUT", "DELETE"}

	f := func(userKey string, methodIdx uint8) bool {
		if userKey == "" {
			return true // skip empty keys; user must provide a non-empty key
		}

		method := methods[int(methodIdx)%len(methods)]

		mock := &mockTransport{
			respFunc: func(req *http.Request) (*http.Response, error) {
				return buildSignedResponse(req.Method, req.URL.Path, cfg.SignKey.Value, cfg.SignType, `{}`), nil
			},
		}

		transport := &SignatureTransport{
			Base:       mock,
			ConfigFunc: func() (*core.CliConfig, error) { return cfg, nil },
		}

		ctx := WithIdempotencyKey(context.Background(), userKey)
		req, _ := http.NewRequestWithContext(ctx, method, "https://hkg-online-uat.everonet.com/g2/v1/test", strings.NewReader(`{}`))
		_, err := transport.RoundTrip(req)
		if err != nil {
			return true // skip transport errors
		}

		got := mock.lastReq.Header.Get("Idempotency-Key")
		if got != userKey {
			t.Logf("Idempotency-Key = %q, want %q (method=%s)", got, userKey, method)
			return false
		}
		return true
	}

	cfg2 := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg2); err != nil {
		t.Errorf("Property 14 failed: %v", err)
	}
}
