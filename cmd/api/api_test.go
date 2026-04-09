package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/quick"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/registry"
)

// --- test helpers ---

type stubFactory struct {
	config    *core.CliConfig
	configErr error
	client    *cmdutil.EvoClient
	clientErr error
	io        *cmdutil.IOStreams
}

func (f *stubFactory) Config() (*core.CliConfig, error)       { return f.config, f.configErr }
func (f *stubFactory) HttpClient() (*http.Client, error)      { return nil, nil }
func (f *stubFactory) EvoClient() (*cmdutil.EvoClient, error) { return f.client, f.clientErr }
func (f *stubFactory) IOStreams() *cmdutil.IOStreams          { return f.io }
func (f *stubFactory) Registry() (*registry.Registry, error)  { return nil, nil }

func newTestIOStreams() (*cmdutil.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &cmdutil.IOStreams{
		In:     io.NopCloser(strings.NewReader("")),
		Out:    out,
		ErrOut: errOut,
	}, out, errOut
}

func newTestConfig() *core.CliConfig {
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignType:    "SHA256",
		Env:         "test",
	}
	cfg.SetResolvedBaseURL("https://hkg-online-uat.everonet.com")
	return cfg
}

// newTestEvoClient creates an EvoClient backed by a test server (no signature transport).
func newTestEvoClient(baseURL string) *cmdutil.EvoClient {
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256",
		Env:         "test",
	}
	cfg.SetResolvedBaseURL(baseURL)
	return cmdutil.NewEvoClient(&http.Client{}, cfg, cmdutil.DefaultIOStreams())
}

// --- Unit Tests ---

func TestNewCmdAPI_InvalidMethod(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"PATCH", "/test"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid method")
	}
	if !strings.Contains(errOut.String(), "unsupported HTTP method") {
		t.Errorf("stderr = %q, want 'unsupported HTTP method'", errOut.String())
	}
}

func TestNewCmdAPI_MethodCaseInsensitive(t *testing.T) {
	methods := []string{"get", "post", "put", "delete", "Get", "Post"}
	for _, m := range methods {
		t.Run(m, func(t *testing.T) {
			var capturedMethod string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedMethod = r.Method
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"}}`)
			}))
			defer srv.Close()

			ios, _, _ := newTestIOStreams()
			client := newTestEvoClient(srv.URL)
			f := &stubFactory{config: newTestConfig(), client: client, io: ios}

			cmd := NewCmdAPI(f)
			cmd.SetArgs([]string{m, "/test"})
			err := cmd.Execute()
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if capturedMethod != strings.ToUpper(m) {
				t.Errorf("method = %q, want %q", capturedMethod, strings.ToUpper(m))
			}
		})
	}
}

func TestNewCmdAPI_RequiresExactly2Args(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	tests := []struct {
		name string
		args []string
	}{
		{"no args", []string{}},
		{"one arg", []string{"GET"}},
		{"three args", []string{"GET", "/test", "extra"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCmdAPI(f)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error for wrong number of args")
			}
		})
	}
}

func TestNewCmdAPI_DataJSONString(t *testing.T) {
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"}}`)
	}))
	defer srv.Close()

	ios, _, _ := newTestIOStreams()
	client := newTestEvoClient(srv.URL)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}

	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"POST", "/test", "--data", `{"amount":"100","currency":"USD"}`})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(capturedBody), &parsed); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if parsed["amount"] != "100" {
		t.Errorf("body.amount = %v, want 100", parsed["amount"])
	}
	if parsed["currency"] != "USD" {
		t.Errorf("body.currency = %v, want USD", parsed["currency"])
	}
}

func TestNewCmdAPI_DataFromFile(t *testing.T) {
	// Create a temp file with JSON content.
	tmpDir := t.TempDir()
	dataFile := filepath.Join(tmpDir, "body.json")
	if err := os.WriteFile(dataFile, []byte(`{"key":"from_file"}`), 0644); err != nil {
		t.Fatal(err)
	}

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"}}`)
	}))
	defer srv.Close()

	ios, _, _ := newTestIOStreams()
	client := newTestEvoClient(srv.URL)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}

	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"POST", "/test", "--data", "@" + dataFile})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(capturedBody), &parsed); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if parsed["key"] != "from_file" {
		t.Errorf("body.key = %v, want from_file", parsed["key"])
	}
}

func TestNewCmdAPI_DataFilePathTraversal(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"POST", "/test", "--data", "@../../etc/passwd"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for path traversal in --data @file")
	}
	if !strings.Contains(errOut.String(), "path traversal") {
		t.Errorf("stderr = %q, want path traversal error", errOut.String())
	}
}

func TestNewCmdAPI_InvalidDataJSON(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"POST", "/test", "--data", "not-json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON in --data")
	}
	if !strings.Contains(errOut.String(), "parse --data as JSON") {
		t.Errorf("stderr = %q, want JSON parse error", errOut.String())
	}
}

func TestNewCmdAPI_Params(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"}}`)
	}))
	defer srv.Close()

	ios, _, _ := newTestIOStreams()
	client := newTestEvoClient(srv.URL)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}

	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"GET", "/test", "--params", `{"merchantTransID":"TX001","page":"2"}`})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(capturedQuery, "merchantTransID=TX001") {
		t.Errorf("query %q missing merchantTransID=TX001", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "page=2") {
		t.Errorf("query %q missing page=2", capturedQuery)
	}
}

func TestNewCmdAPI_InvalidParams(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"GET", "/test", "--params", "not-json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON in --params")
	}
	if !strings.Contains(errOut.String(), "parse --params") {
		t.Errorf("stderr = %q, want params parse error", errOut.String())
	}
}

func TestNewCmdAPI_DryRun(t *testing.T) {
	ios, out, _ := newTestIOStreams()
	cfg := newTestConfig()
	f := &stubFactory{config: cfg, io: ios}

	cmd := NewCmdAPI(f)
	// Set the persistent dry-run flag on the root (simulating global flag).
	cmd.Flags().Bool("dry-run", false, "")
	cmd.SetArgs([]string{"POST", "/g2/v1/payment", "--data", `{"amount":"100"}`, "--dry-run"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result["method"] != "POST" {
		t.Errorf("method = %v, want POST", result["method"])
	}
	urlStr, _ := result["url"].(string)
	if !strings.Contains(urlStr, "/g2/v1/payment") {
		t.Errorf("url = %q, want to contain /g2/v1/payment", urlStr)
	}
	if result["body"] == nil {
		t.Error("expected body in dry-run output")
	}
}

func TestNewCmdAPI_DryRunNoHTTPSent(t *testing.T) {
	// Ensure no HTTP request is sent during dry-run.
	requestReceived := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ios, _, _ := newTestIOStreams()
	cfg := newTestConfig()
	cfg.SetResolvedBaseURL(srv.URL)
	client := newTestEvoClient(srv.URL)
	f := &stubFactory{config: cfg, client: client, io: ios}

	cmd := NewCmdAPI(f)
	cmd.Flags().Bool("dry-run", false, "")
	cmd.SetArgs([]string{"GET", "/test", "--dry-run"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if requestReceived {
		t.Error("HTTP request was sent during dry-run")
	}
}

func TestNewCmdAPI_DryRunWithParams(t *testing.T) {
	ios, out, _ := newTestIOStreams()
	cfg := newTestConfig()
	f := &stubFactory{config: cfg, io: ios}

	cmd := NewCmdAPI(f)
	cmd.Flags().Bool("dry-run", false, "")
	cmd.SetArgs([]string{"GET", "/test", "--params", `{"key":"val"}`, "--dry-run"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	urlStr, _ := result["url"].(string)
	if !strings.Contains(urlStr, "key=val") {
		t.Errorf("url = %q, want to contain key=val", urlStr)
	}
}

func TestNewCmdAPI_DryRunWithIdempotencyKey(t *testing.T) {
	ios, out, _ := newTestIOStreams()
	cfg := newTestConfig()
	f := &stubFactory{config: cfg, io: ios}

	cmd := NewCmdAPI(f)
	cmd.Flags().Bool("dry-run", false, "")
	cmd.SetArgs([]string{"PUT", "/test", "--idempotency-key", "my-key-123", "--dry-run"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	headers, _ := result["headers"].(map[string]interface{})
	if headers["Idempotency-Key"] != "my-key-123" {
		t.Errorf("headers.Idempotency-Key = %v, want my-key-123", headers["Idempotency-Key"])
	}
}

func TestNewCmdAPI_OutputToFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"},"data":"test"}`)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "response.json")

	ios, _, _ := newTestIOStreams()
	client := newTestEvoClient(srv.URL)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}

	cmd := NewCmdAPI(f)
	// Simulate the global -o flag.
	cmd.Flags().StringP("output", "o", "", "")
	cmd.SetArgs([]string{"GET", "/test", "-o", outFile})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if len(content) == 0 {
		t.Error("output file is empty")
	}
	// Verify it's valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Errorf("output file is not valid JSON: %v", err)
	}
}

func TestNewCmdAPI_OutputPathTraversal(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	cmd := NewCmdAPI(f)
	cmd.Flags().StringP("output", "o", "", "")
	cmd.SetArgs([]string{"GET", "/test", "-o", "../../../tmp/evil.json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for path traversal in --output")
	}
	if !strings.Contains(errOut.String(), "path traversal") {
		t.Errorf("stderr = %q, want path traversal error", errOut.String())
	}
}

func TestNewCmdAPI_SuccessEnvelopeOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"},"payment":{"status":"Captured"}}`)
	}))
	defer srv.Close()

	ios, out, _ := newTestIOStreams()
	client := newTestEvoClient(srv.URL)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}

	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"POST", "/test"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var envelope map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if envelope["ok"] != true {
		t.Errorf("ok = %v, want true", envelope["ok"])
	}
	if envelope["data"] == nil {
		t.Error("expected data in envelope")
	}
}

func TestNewCmdAPI_ErrorEnvelopeOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":"internal"}`)
	}))
	defer srv.Close()

	ios, _, errOut := newTestIOStreams()
	client := newTestEvoClient(srv.URL)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}

	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"POST", "/test"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}

	var envelope map[string]interface{}
	if err := json.Unmarshal(errOut.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\noutput: %s", err, errOut.String())
	}
	if envelope["ok"] != false {
		t.Errorf("ok = %v, want false", envelope["ok"])
	}
	errObj, _ := envelope["error"].(map[string]interface{})
	if errObj["type"] != "http_error" {
		t.Errorf("error.type = %v, want http_error", errObj["type"])
	}
}

func TestNewCmdAPI_ConfigError(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	f := &stubFactory{
		configErr: &core.ConfigMissingError{Path: "/fake/path"},
		clientErr: &core.ConfigMissingError{Path: "/fake/path"},
		io:        ios,
	}

	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"GET", "/test"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when config is missing")
	}
	if !strings.Contains(errOut.String(), "config_missing") {
		t.Errorf("stderr = %q, want config_missing error", errOut.String())
	}
}

// --- parseData Tests ---

func TestParseData_JSONString(t *testing.T) {
	result, err := parseData(`{"key":"value"}`)
	if err != nil {
		t.Fatalf("parseData() error = %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["key"] != "value" {
		t.Errorf("key = %v, want value", m["key"])
	}
}

func TestParseData_FileReference(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "data.json")
	os.WriteFile(f, []byte(`{"from":"file"}`), 0644)

	result, err := parseData("@" + f)
	if err != nil {
		t.Fatalf("parseData() error = %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["from"] != "file" {
		t.Errorf("from = %v, want file", m["from"])
	}
}

func TestParseData_FilePathTraversal(t *testing.T) {
	_, err := parseData("@../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error = %v, want path traversal", err)
	}
}

func TestParseData_FileNotFound(t *testing.T) {
	_, err := parseData("@/nonexistent/file.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseData_InvalidJSON(t *testing.T) {
	_, err := parseData("not-json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- parseParams Tests ---

func TestParseParams_Valid(t *testing.T) {
	result, err := parseParams(`{"key":"value","num":"42"}`)
	if err != nil {
		t.Fatalf("parseParams() error = %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("key = %q, want value", result["key"])
	}
	if result["num"] != "42" {
		t.Errorf("num = %q, want 42", result["num"])
	}
}

func TestParseParams_InvalidJSON(t *testing.T) {
	_, err := parseParams("not-json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- validatePathSafety Tests ---

func TestValidatePathSafety(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"output.json", false},
		{"/tmp/output.json", false},
		{"./output.json", false},
		{"../evil.json", true},
		{"../../etc/passwd", true},
		{"foo/../bar", true},
		{"foo/..bar", true}, // contains ".." substring
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			err := validatePathSafety(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePathSafety(%q) error = %v, wantErr = %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

// --- Property-Based Tests ---

// Feature: evo-payment-cli, Property 12: Dry-Run 不发送请求
// For any random api command with --dry-run, no HTTP request should be sent,
// and the output should contain the request details (method, URL, headers, body).
// **Validates: Requirements 4.7, 8.7**
func TestProperty12_DryRunNoRequest(t *testing.T) {
	// methodIdx picks one of the 4 valid methods.
	type dryRunInput struct {
		MethodIdx uint8
		Path      string
		HasBody   bool
		BodyKey   string
		BodyVal   string
	}

	methods := []string{"GET", "POST", "PUT", "DELETE"}

	f := func(input dryRunInput) bool {
		method := methods[int(input.MethodIdx)%len(methods)]

		// Sanitize path: must start with / and contain only safe chars.
		apiPath := "/" + sanitizePath(input.Path)

		// Track whether the test server receives any request.
		requestReceived := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestReceived = true
			w.WriteHeader(200)
		}))
		defer srv.Close()

		ios, out, _ := newTestIOStreams()
		cfg := newTestConfig()
		cfg.SetResolvedBaseURL(srv.URL)
		client := newTestEvoClient(srv.URL)
		sf := &stubFactory{config: cfg, client: client, io: ios}

		cmd := NewCmdAPI(sf)
		cmd.Flags().Bool("dry-run", false, "")

		args := []string{method, apiPath, "--dry-run"}
		if input.HasBody && input.BodyKey != "" && input.BodyVal != "" {
			safeKey := sanitizeJSONKey(input.BodyKey)
			safeVal := sanitizeJSONVal(input.BodyVal)
			if safeKey != "" {
				bodyJSON := fmt.Sprintf(`{"%s":"%s"}`, safeKey, safeVal)
				args = append(args, "--data", bodyJSON)
			}
		}

		cmd.SetArgs(args)
		err := cmd.Execute()
		if err != nil {
			// Some inputs may cause validation errors (e.g., bad JSON); skip those.
			return true
		}

		// Property: no HTTP request should be sent.
		if requestReceived {
			t.Logf("FAIL: HTTP request was sent during dry-run for method=%s path=%s", method, apiPath)
			return false
		}

		// Property: output should be valid JSON with "method", "url", "headers" keys.
		var result map[string]interface{}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Logf("FAIL: dry-run output is not valid JSON: %v\noutput: %s", err, out.String())
			return false
		}
		if _, ok := result["method"]; !ok {
			t.Logf("FAIL: dry-run output missing 'method' key")
			return false
		}
		if _, ok := result["url"]; !ok {
			t.Logf("FAIL: dry-run output missing 'url' key")
			return false
		}
		if _, ok := result["headers"]; !ok {
			t.Logf("FAIL: dry-run output missing 'headers' key")
			return false
		}

		// Verify method matches.
		if result["method"] != method {
			t.Logf("FAIL: dry-run method=%v, want %s", result["method"], method)
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 12 failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 15: 请求构造保真
// For any random JSON body provided via --data and any random query params via --params,
// the HTTP request sent by the api command must contain exactly those values:
// body matches --data JSON, URL query params match --params keys/values.
// **Validates: Requirements 4.3, 4.4, 4.5**
func TestProperty15_RequestFidelity(t *testing.T) {
	type fidelityInput struct {
		BodyKey1 string
		BodyVal1 string
		BodyKey2 string
		BodyVal2 string
		ParamK1  string
		ParamV1  string
		ParamK2  string
		ParamV2  string
	}

	f := func(input fidelityInput) bool {
		// Sanitize keys/values to produce valid JSON and query params.
		bk1 := sanitizeJSONKey(input.BodyKey1)
		bv1 := sanitizeJSONVal(input.BodyVal1)
		bk2 := sanitizeJSONKey(input.BodyKey2)
		bv2 := sanitizeJSONVal(input.BodyVal2)
		pk1 := sanitizeParamKey(input.ParamK1)
		pv1 := sanitizeJSONVal(input.ParamV1)
		pk2 := sanitizeParamKey(input.ParamK2)
		pv2 := sanitizeJSONVal(input.ParamV2)

		// Need at least one valid body key and one valid param key.
		if bk1 == "" || pk1 == "" {
			return true // skip degenerate inputs
		}
		// Ensure body keys are distinct.
		if bk1 == bk2 {
			bk2 = ""
		}
		// Ensure param keys are distinct.
		if pk1 == pk2 {
			pk2 = ""
		}

		// Build expected body JSON.
		bodyMap := map[string]string{bk1: bv1}
		if bk2 != "" {
			bodyMap[bk2] = bv2
		}
		bodyJSON, err := json.Marshal(bodyMap)
		if err != nil {
			return true
		}

		// Build expected params JSON.
		paramMap := map[string]string{pk1: pv1}
		if pk2 != "" {
			paramMap[pk2] = pv2
		}
		paramsJSON, err := json.Marshal(paramMap)
		if err != nil {
			return true
		}

		// Capture the actual request.
		var capturedBody []byte
		var capturedQuery string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			capturedBody = b
			capturedQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"}}`)
		}))
		defer srv.Close()

		ios, _, _ := newTestIOStreams()
		client := newTestEvoClient(srv.URL)
		sf := &stubFactory{config: newTestConfig(), client: client, io: ios}

		cmd := NewCmdAPI(sf)
		cmd.SetArgs([]string{
			"POST", "/test",
			"--data", string(bodyJSON),
			"--params", string(paramsJSON),
		})
		err = cmd.Execute()
		if err != nil {
			t.Logf("Execute() error = %v for body=%s params=%s", err, bodyJSON, paramsJSON)
			return false
		}

		// Verify body fidelity: the captured body should unmarshal to the same map.
		var actualBody map[string]interface{}
		if err := json.Unmarshal(capturedBody, &actualBody); err != nil {
			t.Logf("FAIL: captured body is not valid JSON: %v\nbody: %s", err, capturedBody)
			return false
		}
		for k, v := range bodyMap {
			actual, ok := actualBody[k]
			if !ok {
				t.Logf("FAIL: body missing key %q", k)
				return false
			}
			if fmt.Sprintf("%v", actual) != v {
				t.Logf("FAIL: body[%q] = %v, want %q", k, actual, v)
				return false
			}
		}

		// Verify query params fidelity: each param key/value should appear in the URL.
		for k, v := range paramMap {
			// url.QueryEscape the key and value for comparison.
			expected := url.QueryEscape(k) + "=" + url.QueryEscape(v)
			if !strings.Contains(capturedQuery, expected) {
				// Also try with + encoding for spaces.
				if !queryContainsParam(capturedQuery, k, v) {
					t.Logf("FAIL: query %q missing param %s=%s (expected %s)", capturedQuery, k, v, expected)
					return false
				}
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 15 failed: %v", err)
	}
}

// --- PBT helpers ---

// sanitizePath produces a safe URL path segment from arbitrary string input.
func sanitizePath(s string) string {
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '/' {
			b.WriteRune(c)
		}
	}
	result := b.String()
	if result == "" {
		return "test"
	}
	return result
}

// sanitizeJSONKey produces a non-empty alphanumeric key suitable for JSON.
func sanitizeJSONKey(s string) string {
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// sanitizeJSONVal produces a safe JSON string value (alphanumeric + limited chars).
func sanitizeJSONVal(s string) string {
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == ' ' || c == '-' || c == '_' {
			b.WriteRune(c)
		}
	}
	result := b.String()
	if result == "" {
		return "val"
	}
	return result
}

// sanitizeParamKey produces a non-empty alphanumeric key suitable for query params.
func sanitizeParamKey(s string) string {
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// queryContainsParam checks if a raw query string contains the given key=value pair,
// accounting for URL encoding differences.
func queryContainsParam(rawQuery, key, value string) bool {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return false
	}
	actual := values.Get(key)
	return actual == value
}

// Feature: e2e-test-suite, Property 3: Idempotency-Key in PUT/DELETE requests
// For any PUT or DELETE request with --dry-run --idempotency-key <key>,
// the dry-run output headers must contain Idempotency-Key with the specified value.
// GET and POST requests must NOT include the header.
// **Validates: Requirements 2.5, 2.6**
func TestProperty3_IdempotencyKeyInPutDelete(t *testing.T) {
	type idempInput struct {
		MethodIdx uint8
		KeyValue  string
	}

	methods := []string{"GET", "POST", "PUT", "DELETE"}

	f := func(input idempInput) bool {
		method := methods[int(input.MethodIdx)%len(methods)]
		key := sanitizeJSONVal(input.KeyValue)
		if key == "" {
			key = "test-key"
		}

		ios, out, _ := newTestIOStreams()
		cfg := newTestConfig()
		sf := &stubFactory{config: cfg, io: ios}

		cmd := NewCmdAPI(sf)
		cmd.Flags().Bool("dry-run", false, "")

		args := []string{method, "/test", "--dry-run", "--idempotency-key", key}
		cmd.SetArgs(args)
		err := cmd.Execute()
		if err != nil {
			return true // skip validation errors
		}

		var result map[string]interface{}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Logf("FAIL: output is not valid JSON: %v", err)
			return false
		}

		headers, _ := result["headers"].(map[string]interface{})
		idempKey, hasKey := headers["Idempotency-Key"]

		if method == "PUT" || method == "DELETE" {
			// PUT/DELETE MUST include Idempotency-Key header.
			if !hasKey {
				t.Logf("FAIL: %s request missing Idempotency-Key header", method)
				return false
			}
			if idempKey != key {
				t.Logf("FAIL: Idempotency-Key = %v, want %q", idempKey, key)
				return false
			}
		} else {
			// GET/POST must NOT include Idempotency-Key header.
			if hasKey {
				t.Logf("FAIL: %s request should NOT have Idempotency-Key header", method)
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 3 failed: %v", err)
	}
}

// Feature: e2e-test-suite, Property 4: Invalid HTTP methods are rejected
// For any HTTP method string NOT in {GET, POST, PUT, DELETE},
// the api command must return an error.
// **Validates: Requirements 3.2**
func TestProperty4_InvalidHTTPMethodsRejected(t *testing.T) {
	f := func(method string) bool {
		upper := strings.ToUpper(method)
		if validMethods[upper] {
			return true // skip valid methods
		}

		ios, _, _ := newTestIOStreams()
		cfg := newTestConfig()
		sf := &stubFactory{config: cfg, io: ios}

		cmd := NewCmdAPI(sf)
		cmd.SetArgs([]string{method, "/test"})
		err := cmd.Execute()

		if err == nil {
			t.Logf("FAIL: method %q should be rejected but was accepted", method)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 4 failed: %v", err)
	}
}

// Feature: e2e-test-suite, Property 5: Invalid JSON data is rejected
// For any non-JSON string passed to --data, the api command must return an error.
// **Validates: Requirements 3.3**
func TestProperty5_InvalidJSONDataRejected(t *testing.T) {
	f := func(data string) bool {
		// Skip empty strings (no --data value to test).
		if data == "" {
			return true
		}
		// Skip strings that happen to be valid JSON.
		var js json.RawMessage
		if json.Unmarshal([]byte(data), &js) == nil {
			return true
		}
		// Skip strings starting with "@" (file reference path).
		if strings.HasPrefix(data, "@") {
			return true
		}

		_, err := parseData(data)
		if err == nil {
			t.Logf("FAIL: invalid JSON data %q should be rejected by parseData but was accepted", data)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 5 failed: %v", err)
	}
}
