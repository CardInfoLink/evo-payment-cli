package cmdutil

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/quick"

	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/output"
)

// newTestEvoClient creates an EvoClient pointing at the given test server URL.
// It uses a plain http.Client (no signature transport) for unit testing.
func newTestEvoClient(baseURL string) *EvoClient {
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256",
		Env:         "test",
	}
	// Override the resolved base URL to point at the test server.
	cfg.SetResolvedBaseURL(baseURL)

	return NewEvoClient(&http.Client{}, cfg, DefaultIOStreams())
}

// --- DoAPI Tests ---

func TestDoAPI_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"}}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	resp, err := client.DoAPI("POST", "/g2/v1/payment/mer/S024116/payment", nil, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("DoAPI() error = %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	result, ok := resp.Body["result"].(map[string]interface{})
	if !ok {
		t.Fatal("expected result object in body")
	}
	if result["code"] != "S0000" {
		t.Errorf("result.code = %v, want S0000", result["code"])
	}
}

func TestDoAPI_WithQueryParams(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	params := map[string]string{"merchantTransID": "TX001", "page": "1"}
	_, err := client.DoAPI("GET", "/g2/v1/payment", params, nil)
	if err != nil {
		t.Fatalf("DoAPI() error = %v", err)
	}

	if !strings.Contains(capturedQuery, "merchantTransID=TX001") {
		t.Errorf("query %q missing merchantTransID=TX001", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "page=1") {
		t.Errorf("query %q missing page=1", capturedQuery)
	}
}

func TestDoAPI_WithBody(t *testing.T) {
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	body := map[string]interface{}{
		"amount":   "100.00",
		"currency": "USD",
	}
	_, err := client.DoAPI("POST", "/g2/v1/payment", nil, body)
	if err != nil {
		t.Fatalf("DoAPI() error = %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(capturedBody), &parsed); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if parsed["amount"] != "100.00" {
		t.Errorf("body.amount = %v, want 100.00", parsed["amount"])
	}
	if parsed["currency"] != "USD" {
		t.Errorf("body.currency = %v, want USD", parsed["currency"])
	}
}

func TestDoAPI_NilBody(t *testing.T) {
	var capturedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":"ok"}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	resp, err := client.DoAPI("GET", "/g2/v1/test", nil, nil)
	if err != nil {
		t.Fatalf("DoAPI() error = %v", err)
	}
	if capturedMethod != "GET" {
		t.Errorf("method = %q, want GET", capturedMethod)
	}
	if resp.Body["data"] != "ok" {
		t.Errorf("body.data = %v, want ok", resp.Body["data"])
	}
}

func TestDoAPI_NonJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, "plain text response")
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	resp, err := client.DoAPI("GET", "/health", nil, nil)
	if err != nil {
		t.Fatalf("DoAPI() error = %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	// Non-JSON body should be stored under "raw" key.
	if _, ok := resp.Body["raw"]; !ok {
		t.Error("expected 'raw' key for non-JSON response body")
	}
}

func TestDoAPI_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	resp, err := client.DoAPI("DELETE", "/g2/v1/test", nil, nil)
	if err != nil {
		t.Fatalf("DoAPI() error = %v", err)
	}
	if resp.StatusCode != 204 {
		t.Errorf("StatusCode = %d, want 204", resp.StatusCode)
	}
}

// --- CallAPI Tests ---

func TestCallAPI_SuccessWithPaymentStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"result": {"code": "S0000", "message": "Success"},
			"payment": {"status": "Captured", "transAmount": {"currency": "USD", "value": "100.00"}}
		}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	env, err := client.CallAPI("POST", "/g2/v1/payment/mer/S024116/payment", nil, map[string]string{"test": "data"})
	if err != nil {
		t.Fatalf("CallAPI() error = %v", err)
	}
	if !env.OK {
		t.Error("expected ok=true")
	}
	if env.Meta == nil {
		t.Fatal("expected meta to be non-nil")
	}
	if env.Meta.BusinessStatus != "Captured" {
		t.Errorf("meta.businessStatus = %q, want %q", env.Meta.BusinessStatus, "Captured")
	}
}

func TestCallAPI_SuccessWithCaptureStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"result": {"code": "S0000", "message": "Success"},
			"capture": {"status": "Success"}
		}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	env, err := client.CallAPI("POST", "/g2/v1/payment/mer/S024116/capture", nil, nil)
	if err != nil {
		t.Fatalf("CallAPI() error = %v", err)
	}
	if env.Meta.BusinessStatus != "Success" {
		t.Errorf("meta.businessStatus = %q, want %q", env.Meta.BusinessStatus, "Success")
	}
}

func TestCallAPI_SuccessWithCancelStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"result": {"code": "S0000", "message": "Success"},
			"cancel": {"status": "Success"}
		}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	env, err := client.CallAPI("POST", "/g2/v1/payment/mer/S024116/cancel", nil, nil)
	if err != nil {
		t.Fatalf("CallAPI() error = %v", err)
	}
	if env.Meta.BusinessStatus != "Success" {
		t.Errorf("meta.businessStatus = %q, want %q", env.Meta.BusinessStatus, "Success")
	}
}

func TestCallAPI_SuccessWithRefundStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"result": {"code": "S0000", "message": "Success"},
			"refund": {"status": "Received"}
		}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	env, err := client.CallAPI("POST", "/g2/v1/payment/mer/S024116/refund", nil, nil)
	if err != nil {
		t.Fatalf("CallAPI() error = %v", err)
	}
	if env.Meta.BusinessStatus != "Received" {
		t.Errorf("meta.businessStatus = %q, want %q", env.Meta.BusinessStatus, "Received")
	}
}

func TestCallAPI_ActionRequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"result": {"code": "S0000", "message": "Success"},
			"action": {"type": "redirectUser", "redirectData": {"url": "https://example.com"}},
			"payment": {"status": "Pending"}
		}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	env, err := client.CallAPI("POST", "/g2/v1/payment/mer/S024116/payment", nil, nil)
	if err != nil {
		t.Fatalf("CallAPI() error = %v", err)
	}
	if !env.OK {
		t.Error("expected ok=true")
	}
	if env.Meta == nil {
		t.Fatal("expected meta to be non-nil")
	}
	if !env.Meta.ActionRequired {
		t.Error("expected meta.actionRequired=true when action object present")
	}
	if env.Meta.BusinessStatus != "Pending" {
		t.Errorf("meta.businessStatus = %q, want %q", env.Meta.BusinessStatus, "Pending")
	}
}

func TestCallAPI_HTTPNon200_ReturnsHTTPError(t *testing.T) {
	statusCodes := []int{400, 401, 403, 404, 500, 502, 503}
	for _, code := range statusCodes {
		t.Run(fmt.Sprintf("HTTP_%d", code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				fmt.Fprint(w, `{"error":"something went wrong"}`)
			}))
			defer srv.Close()

			client := newTestEvoClient(srv.URL)
			_, err := client.CallAPI("POST", "/g2/v1/test", nil, nil)
			if err == nil {
				t.Fatalf("expected error for HTTP %d, got nil", code)
			}

			httpErr, ok := err.(*HTTPError)
			if !ok {
				t.Fatalf("expected *HTTPError, got %T: %v", err, err)
			}
			if httpErr.StatusCode != code {
				t.Errorf("HTTPError.StatusCode = %d, want %d", httpErr.StatusCode, code)
			}
			if httpErr.Type() != "http_error" {
				t.Errorf("HTTPError.Type() = %q, want %q", httpErr.Type(), "http_error")
			}
			if httpErr.Code() != fmt.Sprintf("%d", code) {
				t.Errorf("HTTPError.Code() = %q, want %q", httpErr.Code(), fmt.Sprintf("%d", code))
			}
		})
	}
}

func TestCallAPI_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"B0013","message":"Total refund amount cannot be greater than payment amount"}}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	_, err := client.CallAPI("POST", "/g2/v1/payment/mer/S024116/refund", nil, nil)
	if err == nil {
		t.Fatal("expected error for business error result code")
	}

	resultErr, ok := err.(*ResultError)
	if !ok {
		t.Fatalf("expected *ResultError, got %T: %v", err, err)
	}
	if resultErr.ErrType != "business" {
		t.Errorf("ResultError.ErrType = %q, want %q", resultErr.ErrType, "business")
	}
	if resultErr.Code != "B0013" {
		t.Errorf("ResultError.Code = %q, want %q", resultErr.Code, "B0013")
	}
	if resultErr.ExitCode != output.ExitBusinessError {
		t.Errorf("ResultError.ExitCode = %d, want %d", resultErr.ExitCode, output.ExitBusinessError)
	}
}

func TestCallAPI_ValidationError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"V0000","message":"Invalid format"}}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	_, err := client.CallAPI("POST", "/g2/v1/test", nil, nil)
	if err == nil {
		t.Fatal("expected error for validation result code")
	}

	resultErr, ok := err.(*ResultError)
	if !ok {
		t.Fatalf("expected *ResultError, got %T: %v", err, err)
	}
	if resultErr.ErrType != "validation" {
		t.Errorf("ResultError.ErrType = %q, want %q", resultErr.ErrType, "validation")
	}
	if resultErr.ExitCode != output.ExitValidation {
		t.Errorf("ResultError.ExitCode = %d, want %d", resultErr.ExitCode, output.ExitValidation)
	}
}

func TestCallAPI_SignatureError_V0010(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"V0010","message":"Signature error"}}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	_, err := client.CallAPI("POST", "/g2/v1/test", nil, nil)
	if err == nil {
		t.Fatal("expected error for V0010 signature error")
	}

	resultErr, ok := err.(*ResultError)
	if !ok {
		t.Fatalf("expected *ResultError, got %T: %v", err, err)
	}
	if resultErr.ErrType != "signature_error" {
		t.Errorf("ResultError.ErrType = %q, want %q", resultErr.ErrType, "signature_error")
	}
	if resultErr.ExitCode != output.ExitAuthError {
		t.Errorf("ResultError.ExitCode = %d, want %d", resultErr.ExitCode, output.ExitAuthError)
	}
}

func TestCallAPI_PSPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"P0098","message":"PSP timeout"}}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	_, err := client.CallAPI("POST", "/g2/v1/test", nil, nil)
	if err == nil {
		t.Fatal("expected error for PSP error")
	}

	resultErr, ok := err.(*ResultError)
	if !ok {
		t.Fatalf("expected *ResultError, got %T: %v", err, err)
	}
	if resultErr.ErrType != "psp_error" {
		t.Errorf("ResultError.ErrType = %q, want %q", resultErr.ErrType, "psp_error")
	}
	if resultErr.ExitCode != output.ExitPSPError {
		t.Errorf("ResultError.ExitCode = %d, want %d", resultErr.ExitCode, output.ExitPSPError)
	}
}

func TestCallAPI_InternalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"E0000","message":"Internal error"}}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	_, err := client.CallAPI("POST", "/g2/v1/test", nil, nil)
	if err == nil {
		t.Fatal("expected error for internal error")
	}

	resultErr, ok := err.(*ResultError)
	if !ok {
		t.Fatalf("expected *ResultError, got %T: %v", err, err)
	}
	if resultErr.ErrType != "internal_error" {
		t.Errorf("ResultError.ErrType = %q, want %q", resultErr.ErrType, "internal_error")
	}
	if resultErr.ExitCode != output.ExitNetworkError {
		t.Errorf("ResultError.ExitCode = %d, want %d", resultErr.ExitCode, output.ExitNetworkError)
	}
}

func TestCallAPI_NoResultCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"payment":{"status":"Authorised"}}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	env, err := client.CallAPI("GET", "/g2/v1/test", nil, nil)
	if err != nil {
		t.Fatalf("CallAPI() error = %v", err)
	}
	if !env.OK {
		t.Error("expected ok=true when no result code")
	}
	if env.Meta.BusinessStatus != "Authorised" {
		t.Errorf("meta.businessStatus = %q, want %q", env.Meta.BusinessStatus, "Authorised")
	}
}

func TestCallAPI_NoBusinessStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"},"data":"something"}`)
	}))
	defer srv.Close()

	client := newTestEvoClient(srv.URL)
	env, err := client.CallAPI("GET", "/g2/v1/test", nil, nil)
	if err != nil {
		t.Fatalf("CallAPI() error = %v", err)
	}
	if env.Meta.BusinessStatus != "" {
		t.Errorf("meta.businessStatus = %q, want empty", env.Meta.BusinessStatus)
	}
}

// --- extractBusinessStatus Tests ---

func TestExtractBusinessStatus(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
		want string
	}{
		{
			name: "payment status",
			body: map[string]interface{}{
				"payment": map[string]interface{}{"status": "Captured"},
			},
			want: "Captured",
		},
		{
			name: "capture status",
			body: map[string]interface{}{
				"capture": map[string]interface{}{"status": "Success"},
			},
			want: "Success",
		},
		{
			name: "cancel status",
			body: map[string]interface{}{
				"cancel": map[string]interface{}{"status": "Success"},
			},
			want: "Success",
		},
		{
			name: "refund status",
			body: map[string]interface{}{
				"refund": map[string]interface{}{"status": "Received"},
			},
			want: "Received",
		},
		{
			name: "payment takes priority over capture",
			body: map[string]interface{}{
				"payment": map[string]interface{}{"status": "Authorised"},
				"capture": map[string]interface{}{"status": "Success"},
			},
			want: "Authorised",
		},
		{
			name: "no status fields",
			body: map[string]interface{}{
				"data": "something",
			},
			want: "",
		},
		{
			name: "empty status string",
			body: map[string]interface{}{
				"payment": map[string]interface{}{"status": ""},
			},
			want: "",
		},
		{
			name: "nil body",
			body: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBusinessStatus(tt.body)
			if got != tt.want {
				t.Errorf("extractBusinessStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Error Type Tests ---

func TestHTTPError(t *testing.T) {
	err := &HTTPError{StatusCode: 503, Body: map[string]interface{}{"error": "unavailable"}}
	if err.Error() != "HTTP 503" {
		t.Errorf("Error() = %q, want %q", err.Error(), "HTTP 503")
	}
	if err.Type() != "http_error" {
		t.Errorf("Type() = %q, want %q", err.Type(), "http_error")
	}
	if err.Code() != "503" {
		t.Errorf("Code() = %q, want %q", err.Code(), "503")
	}
}

func TestResultError(t *testing.T) {
	err := &ResultError{
		ExitCode: output.ExitBusinessError,
		ErrType:  "business",
		Code:     "B0013",
		Message:  "Amount exceeded",
	}
	if err.Type() != "business" {
		t.Errorf("Type() = %q, want %q", err.Type(), "business")
	}
	expectedMsg := "business: B0013 — Amount exceeded"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestNewEvoClient(t *testing.T) {
	cfg := &core.CliConfig{MerchantSid: "S024116"}
	io := DefaultIOStreams()
	httpClient := &http.Client{}

	client := NewEvoClient(httpClient, cfg, io)
	if client == nil {
		t.Fatal("NewEvoClient returned nil")
	}
	if client.httpClient != httpClient {
		t.Error("httpClient not set correctly")
	}
	if client.config != cfg {
		t.Error("config not set correctly")
	}
	if client.ioStreams != io {
		t.Error("ioStreams not set correctly")
	}
}

// =============================================================================
// Property-Based Tests (testing/quick)
// =============================================================================

// Feature: evo-payment-cli, Property 19: 响应元数据提取
// **Validates: Requirements 17.2, 17.3**
//
// For any random response with/without action object, actionRequired must be
// true when action is present.
// For any random response with payment/capture/cancel/refund status,
// businessStatus must match.
func TestProperty19_ResponseMetadataExtraction(t *testing.T) {
	statusKeys := []string{"payment", "capture", "cancel", "refund"}
	statusValues := []string{"Authorised", "Captured", "Pending", "Success", "Failed", "Received", "Cancelled", "Refunded"}

	f := func(seed uint32) bool {
		rng := rand.New(rand.NewSource(int64(seed)))

		// Build a response body with result.code=S0000 (success).
		body := map[string]interface{}{
			"result": map[string]interface{}{
				"code":    "S0000",
				"message": "Success",
			},
		}

		// Randomly include an action object.
		hasAction := rng.Intn(2) == 1
		if hasAction {
			body["action"] = map[string]interface{}{
				"type": "redirectUser",
				"redirectData": map[string]interface{}{
					"url": "https://example.com/redirect",
				},
			}
		}

		// Randomly include a business status.
		var expectedStatus string
		hasStatus := rng.Intn(2) == 1
		if hasStatus {
			key := statusKeys[rng.Intn(len(statusKeys))]
			expectedStatus = statusValues[rng.Intn(len(statusValues))]
			body[key] = map[string]interface{}{
				"status": expectedStatus,
			}
		}

		// Serialize body to JSON for the test server.
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return true // skip
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write(bodyBytes)
		}))
		defer srv.Close()

		client := newTestEvoClient(srv.URL)
		env, err := client.CallAPI("POST", "/g2/v1/test", nil, nil)
		if err != nil {
			t.Logf("CallAPI error: %v", err)
			return false
		}

		// Verify actionRequired.
		if hasAction && !env.Meta.ActionRequired {
			t.Log("expected actionRequired=true when action present")
			return false
		}
		if !hasAction && env.Meta.ActionRequired {
			t.Log("expected actionRequired=false when action absent")
			return false
		}

		// Verify businessStatus.
		if hasStatus {
			if env.Meta.BusinessStatus != expectedStatus {
				t.Logf("businessStatus=%q, want %q", env.Meta.BusinessStatus, expectedStatus)
				return false
			}
		} else {
			if env.Meta.BusinessStatus != "" {
				t.Logf("expected empty businessStatus, got %q", env.Meta.BusinessStatus)
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 19 failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 20: HTTP 非 200 错误处理
// **Validates: Requirement 17.4**
//
// For any random non-200 HTTP status code, CallAPI must return an HTTPError
// with Type()="http_error".
func TestProperty20_HTTPNon200ErrorHandling(t *testing.T) {
	// Valid non-200 HTTP status codes to test.
	nonOKCodes := []int{
		400, 401, 403, 404, 405, 408, 409, 429,
		500, 501, 502, 503, 504,
	}

	f := func(seed uint32) bool {
		rng := rand.New(rand.NewSource(int64(seed)))

		statusCode := nonOKCodes[rng.Intn(len(nonOKCodes))]

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
			fmt.Fprintf(w, `{"error":"test error %d"}`, statusCode)
		}))
		defer srv.Close()

		client := newTestEvoClient(srv.URL)
		_, err := client.CallAPI("POST", "/g2/v1/test", nil, nil)
		if err == nil {
			t.Logf("expected error for HTTP %d, got nil", statusCode)
			return false
		}

		httpErr, ok := err.(*HTTPError)
		if !ok {
			t.Logf("expected *HTTPError for HTTP %d, got %T: %v", statusCode, err, err)
			return false
		}

		if httpErr.Type() != "http_error" {
			t.Logf("HTTPError.Type()=%q, want %q", httpErr.Type(), "http_error")
			return false
		}

		if httpErr.StatusCode != statusCode {
			t.Logf("HTTPError.StatusCode=%d, want %d", httpErr.StatusCode, statusCode)
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 20 failed: %v", err)
	}
}
