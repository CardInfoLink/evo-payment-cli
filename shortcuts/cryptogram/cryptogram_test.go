package cryptogram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/registry"
	"github.com/evopayment/evo-cli/shortcuts"
	"github.com/spf13/cobra"
)

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

func newTestIO() (*cmdutil.IOStreams, *bytes.Buffer, *bytes.Buffer) {
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

func newTestEvoClient(baseURL string, ios *cmdutil.IOStreams) *cmdutil.EvoClient {
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256",
		Env:         "test",
	}
	cfg.SetResolvedBaseURL(baseURL)
	return cmdutil.NewEvoClient(&http.Client{}, cfg, ios)
}

func buildCmd(f cmdutil.Factory, sc shortcuts.Shortcut) *cobra.Command {
	root := &cobra.Command{Use: "evo-cli"}
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().String("format", "json", "")
	root.PersistentFlags().Bool("yes", false, "")
	parent := &cobra.Command{Use: sc.Service}
	root.AddCommand(parent)
	shortcuts.Mount(parent, f, []shortcuts.Shortcut{sc})
	return root
}

func TestAllShortcuts_Count(t *testing.T) {
	all := AllShortcuts()
	if len(all) != 3 {
		t.Errorf("expected 3 cryptogram shortcuts, got %d", len(all))
	}
	names := map[string]bool{}
	for _, sc := range all {
		names[sc.Command] = true
	}
	for _, want := range []string{"+create", "+query", "+pay"} {
		if !names[want] {
			t.Errorf("missing shortcut %q", want)
		}
	}
}

func TestCreate_DryRun(t *testing.T) {
	ios, out, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"cryptogram", "+create",
		"--network-token-id", "NTK001",
		"--original-merchant-tx-id", "TX001",
		"--dry-run",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}
	if result["method"] != "POST" {
		t.Errorf("method = %v, want POST", result["method"])
	}
	url, _ := result["url"].(string)
	if !strings.Contains(url, "/cryptogram") {
		t.Errorf("url should contain /cryptogram, got: %s", url)
	}
	if !strings.Contains(url, "merchantTransID=TX001") {
		t.Errorf("url should contain merchantTransID=TX001, got: %s", url)
	}
	body, _ := result["body"].(map[string]interface{})
	pm, _ := body["paymentMethod"].(map[string]interface{})
	nt, _ := pm["networkToken"].(map[string]interface{})
	if nt["tokenID"] != "NTK001" {
		t.Errorf("body.paymentMethod.networkToken.tokenID = %v, want NTK001", nt["tokenID"])
	}
}

func TestCreate_MissingRequired(t *testing.T) {
	ios, _, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"cryptogram", "+create", "--network-token-id", "NTK001"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --original-merchant-tx-id")
	}
}

func TestQuery_DryRun(t *testing.T) {
	ios, out, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, QueryShortcut())
	root.SetArgs([]string{"cryptogram", "+query", "--merchant-tx-id", "CRYPTO001", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["method"] != "GET" {
		t.Errorf("method = %v, want GET", result["method"])
	}
	url, _ := result["url"].(string)
	if !strings.Contains(url, "merchantTransID=CRYPTO001") {
		t.Errorf("url should contain merchantTransID=CRYPTO001, got: %s", url)
	}
}

func TestQuery_MissingRequired(t *testing.T) {
	ios, _, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, QueryShortcut())
	root.SetArgs([]string{"cryptogram", "+query"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --merchant-tx-id")
	}
}

func TestCreate_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/cryptogram") {
			t.Errorf("path should contain /cryptogram, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"},"cryptogram":{"status":"Success"}}`)
	}))
	defer srv.Close()

	ios, out, _ := newTestIO()
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"cryptogram", "+create",
		"--network-token-id", "NTK001",
		"--original-merchant-tx-id", "TX001",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}
	if envelope["ok"] != true {
		t.Errorf("ok = %v, want true", envelope["ok"])
	}
}

func TestQuery_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("merchantTransID") != "CRYPTO001" {
			t.Errorf("expected merchantTransID=CRYPTO001, got %s", r.URL.Query().Get("merchantTransID"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"},"cryptogram":{"status":"Success"}}`)
	}))
	defer srv.Close()

	ios, out, _ := newTestIO()
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}
	root := buildCmd(f, QueryShortcut())
	root.SetArgs([]string{"cryptogram", "+query", "--merchant-tx-id", "CRYPTO001"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}
	if envelope["ok"] != true {
		t.Errorf("ok = %v, want true", envelope["ok"])
	}
}

func TestRegisterShortcuts(t *testing.T) {
	ios, _, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := &cobra.Command{Use: "evo-cli"}
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().String("format", "json", "")
	root.PersistentFlags().Bool("yes", false, "")
	RegisterShortcuts(root, f)

	cryptoCmd, _, err := root.Find([]string{"cryptogram"})
	if err != nil {
		t.Fatalf("cryptogram command not found: %v", err)
	}
	if cryptoCmd.Use != "cryptogram" {
		t.Errorf("expected 'cryptogram' command, got %q", cryptoCmd.Use)
	}
	for _, name := range []string{"+create", "+query", "+pay"} {
		found := false
		for _, sub := range cryptoCmd.Commands() {
			if sub.Use == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing subcommand %q under cryptogram", name)
		}
	}
}

func TestPay_DryRun(t *testing.T) {
	ios, out, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, PayShortcut())
	root.SetArgs([]string{"cryptogram", "+pay",
		"--network-token-value", "2222030194871591",
		"--token-expiry-date", "1226",
		"--token-cryptogram", "AAKQJPQZFWufAAJ5j4JeAAADFA==",
		"--eci", "06",
		"--payment-brand", "Mastercard",
		"--amount", "10.00",
		"--currency", "USD",
		"--dry-run",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}
	if result["method"] != "POST" {
		t.Errorf("method = %v, want POST", result["method"])
	}
	url, _ := result["url"].(string)
	if !strings.Contains(url, "/payment") {
		t.Errorf("url should contain /payment, got: %s", url)
	}
	body, _ := result["body"].(map[string]interface{})
	pm, _ := body["paymentMethod"].(map[string]interface{})
	if pm["type"] != "token" {
		t.Errorf("paymentMethod.type = %v, want token", pm["type"])
	}
	nt, _ := pm["token"].(map[string]interface{})
	if nt["type"] != "networkToken" {
		t.Errorf("token.type = %v, want networkToken", nt["type"])
	}
	if nt["value"] != "2222030194871591" {
		t.Errorf("token.value = %v, want 2222030194871591", nt["value"])
	}
	if nt["expiryDate"] != "1226" {
		t.Errorf("token.expiryDate = %v, want 1226", nt["expiryDate"])
	}
	if nt["tokenCryptogram"] != "AAKQJPQZFWufAAJ5j4JeAAADFA==" {
		t.Errorf("token.tokenCryptogram = %v", nt["tokenCryptogram"])
	}
	if nt["eci"] != "06" {
		t.Errorf("token.eci = %v, want 06", nt["eci"])
	}
}

func TestPay_MissingRequired(t *testing.T) {
	ios, _, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, PayShortcut())
	root.SetArgs([]string{"cryptogram", "+pay", "--network-token-value", "2222030194871591", "--token-expiry-date", "1226"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestPay_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/payment") {
			t.Errorf("path should contain /payment, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"},"payment":{"status":"Authorised"}}`)
	}))
	defer srv.Close()

	ios, out, _ := newTestIO()
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}
	root := buildCmd(f, PayShortcut())
	root.SetArgs([]string{"cryptogram", "+pay",
		"--network-token-value", "2222030194871591",
		"--token-expiry-date", "1226",
		"--token-cryptogram", "AAKQJPQZFWufAAJ5j4JeAAADFA==",
		"--eci", "06",
		"--payment-brand", "Mastercard",
		"--amount", "10.00",
		"--currency", "USD",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}
	if envelope["ok"] != true {
		t.Errorf("ok = %v, want true", envelope["ok"])
	}
}
