package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/keychain"
	"github.com/evopayment/evo-cli/internal/registry"
)

// --- Fake Factory for testing ---

type fakeFactory struct {
	cfg    *core.CliConfig
	cfgErr error
	io     *cmdutil.IOStreams
	kc     keychain.KeychainAccess
}

func (f *fakeFactory) Config() (*core.CliConfig, error) {
	return f.cfg, f.cfgErr
}
func (f *fakeFactory) HttpClient() (*http.Client, error) {
	return &http.Client{}, nil
}
func (f *fakeFactory) EvoClient() (*cmdutil.EvoClient, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeFactory) IOStreams() *cmdutil.IOStreams {
	return f.io
}
func (f *fakeFactory) Registry() (*registry.Registry, error) {
	return nil, fmt.Errorf("not implemented")
}

// --- Tests ---

func TestDoctor_AllPass(t *testing.T) {
	// Start a test server to simulate API endpoint.
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256",
		Env:         "test",
	}
	// Point base URL to the test server.
	cfg.SetResolvedBaseURL(ts.URL)

	out := &bytes.Buffer{}
	f := &fakeFactory{
		cfg: cfg,
		io:  &cmdutil.IOStreams{Out: out, ErrOut: &bytes.Buffer{}},
	}

	checks := RunDoctor(f, false)

	// We expect 4 checks: config_file, sign_key, signature, connectivity.
	if len(checks) != 4 {
		t.Fatalf("expected 4 checks, got %d", len(checks))
	}

	// Config, SignKey, Signature should pass.
	for _, name := range []string{"config_file", "sign_key", "signature"} {
		found := false
		for _, c := range checks {
			if c.Name == name {
				found = true
				if c.Status != StatusPass {
					t.Errorf("check %q: expected pass, got %s (%s)", name, c.Status, c.Message)
				}
				break
			}
		}
		if !found {
			t.Errorf("check %q not found in results", name)
		}
	}

	// Connectivity: the test server uses HTTPS (TLS), so it should pass.
	// Note: httptest.NewTLSServer uses a self-signed cert, but our doctor
	// uses a plain http.Client which won't trust it. We test offline mode separately.
}

func TestDoctor_ConfigMissing(t *testing.T) {
	out := &bytes.Buffer{}
	f := &fakeFactory{
		cfgErr: &core.ConfigMissingError{Path: "/home/test/.evo-cli/config.json"},
		io:     &cmdutil.IOStreams{Out: out, ErrOut: &bytes.Buffer{}},
	}

	checks := RunDoctor(f, true)

	// Check 1: config_file should fail.
	if checks[0].Status != StatusFail {
		t.Errorf("config_file: expected fail, got %s", checks[0].Status)
	}
	if checks[0].Hint != "run: evo-cli config init" {
		t.Errorf("config_file hint: expected 'run: evo-cli config init', got %q", checks[0].Hint)
	}

	// Check 2: sign_key should fail (config not available).
	if checks[1].Status != StatusFail {
		t.Errorf("sign_key: expected fail, got %s", checks[1].Status)
	}

	// Check 3: signature should fail (no sign key).
	if checks[2].Status != StatusFail {
		t.Errorf("signature: expected fail, got %s", checks[2].Status)
	}

	// Check 4: connectivity should be skipped (offline).
	if checks[3].Status != StatusSkip {
		t.Errorf("connectivity: expected skip, got %s", checks[3].Status)
	}
}

func TestDoctor_OfflineSkipsConnectivity(t *testing.T) {
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256",
		Env:         "test",
	}

	out := &bytes.Buffer{}
	f := &fakeFactory{
		cfg: cfg,
		io:  &cmdutil.IOStreams{Out: out, ErrOut: &bytes.Buffer{}},
	}

	checks := RunDoctor(f, true)

	// Connectivity should be skipped.
	connCheck := checks[3]
	if connCheck.Name != "connectivity" {
		t.Fatalf("expected check 4 to be connectivity, got %s", connCheck.Name)
	}
	if connCheck.Status != StatusSkip {
		t.Errorf("connectivity: expected skip, got %s", connCheck.Status)
	}
}

func TestDoctor_EmptySignKey(t *testing.T) {
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: ""},
		SignType:    "SHA256",
		Env:         "test",
	}

	out := &bytes.Buffer{}
	f := &fakeFactory{
		cfg: cfg,
		io:  &cmdutil.IOStreams{Out: out, ErrOut: &bytes.Buffer{}},
	}

	checks := RunDoctor(f, true)

	// sign_key should fail.
	if checks[1].Status != StatusFail {
		t.Errorf("sign_key: expected fail, got %s", checks[1].Status)
	}

	// signature should also fail (no key).
	if checks[2].Status != StatusFail {
		t.Errorf("signature: expected fail, got %s", checks[2].Status)
	}
}

func TestDoctor_JSONOutput(t *testing.T) {
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256",
		Env:         "test",
	}

	out := &bytes.Buffer{}
	f := &fakeFactory{
		cfg: cfg,
		io:  &cmdutil.IOStreams{Out: out, ErrOut: &bytes.Buffer{}},
	}

	cmd := NewCmdDoctor(f)
	cmd.SetArgs([]string{"--offline"})
	cmd.SetOut(out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	// Parse JSON output.
	var report DoctorReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, out.String())
	}

	if !report.OK {
		t.Errorf("expected report.OK=true, got false")
	}
	if len(report.Checks) != 4 {
		t.Errorf("expected 4 checks, got %d", len(report.Checks))
	}

	// Verify check names in order.
	expectedNames := []string{"config_file", "sign_key", "signature", "connectivity"}
	for i, name := range expectedNames {
		if report.Checks[i].Name != name {
			t.Errorf("check[%d]: expected name %q, got %q", i, name, report.Checks[i].Name)
		}
	}
}

func TestDoctor_ConnectivityPass(t *testing.T) {
	// Start a plain HTTP test server (not TLS) for connectivity check.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256",
		Env:         "test",
	}
	cfg.SetResolvedBaseURL(ts.URL)

	out := &bytes.Buffer{}
	f := &fakeFactory{
		cfg: cfg,
		io:  &cmdutil.IOStreams{Out: out, ErrOut: &bytes.Buffer{}},
	}

	checks := RunDoctor(f, false)

	connCheck := checks[3]
	if connCheck.Name != "connectivity" {
		t.Fatalf("expected check 4 to be connectivity, got %s", connCheck.Name)
	}
	if connCheck.Status != StatusPass {
		t.Errorf("connectivity: expected pass, got %s (%s)", connCheck.Status, connCheck.Message)
	}
}

func TestDoctor_ConnectivityFail(t *testing.T) {
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256",
		Env:         "test",
	}
	// Point to an unreachable address.
	cfg.SetResolvedBaseURL("http://127.0.0.1:1")

	out := &bytes.Buffer{}
	f := &fakeFactory{
		cfg: cfg,
		io:  &cmdutil.IOStreams{Out: out, ErrOut: &bytes.Buffer{}},
	}

	checks := RunDoctor(f, false)

	connCheck := checks[3]
	if connCheck.Status != StatusFail {
		t.Errorf("connectivity: expected fail, got %s", connCheck.Status)
	}
	if connCheck.Hint == "" {
		t.Error("connectivity: expected non-empty hint on failure")
	}
}

func TestDoctor_ReportOKFalseOnFailure(t *testing.T) {
	out := &bytes.Buffer{}
	f := &fakeFactory{
		cfgErr: &core.ConfigMissingError{Path: "/tmp/missing"},
		io:     &cmdutil.IOStreams{Out: out, ErrOut: &bytes.Buffer{}},
	}

	cmd := NewCmdDoctor(f)
	cmd.SetArgs([]string{"--offline"})
	cmd.SetOut(out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	var report DoctorReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if report.OK {
		t.Error("expected report.OK=false when config is missing")
	}
}
