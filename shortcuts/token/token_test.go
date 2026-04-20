package token

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
		In:     io.NopCloser(strings.NewReader("yes\n")),
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
		t.Errorf("expected 3 token shortcuts, got %d", len(all))
	}
	names := map[string]bool{}
	for _, sc := range all {
		names[sc.Command] = true
	}
	for _, want := range []string{"+create", "+query", "+delete"} {
		if !names[want] {
			t.Errorf("missing shortcut %q", want)
		}
	}
}

func TestCreate_DryRun(t *testing.T) {
	ios, out, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"token", "+create",
		"--payment-type", "card",
		"--vault-id", "V001",
		"--user-reference", "user@example.com",
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
	if !strings.Contains(url, "/paymentMethod") {
		t.Errorf("url should contain /paymentMethod, got: %s", url)
	}
	body, _ := result["body"].(map[string]interface{})
	pm, _ := body["paymentMethod"].(map[string]interface{})
	card, _ := pm["card"].(map[string]interface{})
	if card["vaultID"] != "V001" {
		t.Errorf("body.paymentMethod.card.vaultID = %v, want V001", card["vaultID"])
	}
	// transInitiator should be present
	ti, _ := body["transInitiator"].(map[string]interface{})
	if ti["platform"] != "WEB" {
		t.Errorf("body.transInitiator.platform = %v, want WEB", ti["platform"])
	}
}

func TestCreate_MissingRequired(t *testing.T) {
	ios, _, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"token", "+create", "--payment-type", "card"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestCreate_DryRun_WithCardInfo(t *testing.T) {
	ios, out, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"token", "+create",
		"--payment-type", "card",
		"--vault-id", "V001",
		"--user-reference", "user@example.com",
		"--card-number", "4111111111111111",
		"--card-expiry", "1226",
		"--card-cvc", "123",
		"--dry-run",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}
	body, _ := result["body"].(map[string]interface{})
	pm, _ := body["paymentMethod"].(map[string]interface{})
	card, _ := pm["card"].(map[string]interface{})
	if card["vaultID"] != "V001" {
		t.Errorf("body.paymentMethod.card.vaultID = %v, want V001", card["vaultID"])
	}
	cardInfo, _ := card["cardInfo"].(map[string]interface{})
	if cardInfo == nil {
		t.Fatal("expected cardInfo when card flags provided")
	}
	if cardInfo["cardNumber"] != "4111111111111111" {
		t.Errorf("cardNumber = %v, want 4111111111111111", cardInfo["cardNumber"])
	}
	if cardInfo["expiryDate"] != "1226" {
		t.Errorf("expiryDate = %v, want 1226", cardInfo["expiryDate"])
	}
	if cardInfo["cvc"] != "123" {
		t.Errorf("cvc = %v, want 123", cardInfo["cvc"])
	}
	// transInitiator should be present
	ti, _ := body["transInitiator"].(map[string]interface{})
	if ti["platform"] != "WEB" {
		t.Errorf("body.transInitiator.platform = %v, want WEB", ti["platform"])
	}
}

func TestCreate_DryRun_WithAllowAuthentication(t *testing.T) {
	ios, out, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"token", "+create",
		"--payment-type", "card",
		"--vault-id", "V001",
		"--user-reference", "user@example.com",
		"--allow-authentication", "true",
		"--dry-run",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}
	body, _ := result["body"].(map[string]interface{})
	if body["allowAuthentication"] != true {
		t.Errorf("body.allowAuthentication = %v, want true", body["allowAuthentication"])
	}
}

func TestQuery_DryRun(t *testing.T) {
	ios, out, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, QueryShortcut())
	root.SetArgs([]string{"token", "+query", "--merchant-tx-id", "TX999", "--dry-run"})
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
	if !strings.Contains(url, "merchantTransID=TX999") {
		t.Errorf("url should contain merchantTransID param, got: %s", url)
	}
}

func TestDelete_DryRun(t *testing.T) {
	ios, out, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, DeleteShortcut())
	root.SetArgs([]string{"token", "+delete",
		"--token-id", "TK001",
		"--yes",
		"--dry-run",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["method"] != "DELETE" {
		t.Errorf("method = %v, want DELETE", result["method"])
	}
	body, _ := result["body"].(map[string]interface{})
	if body == nil {
		t.Fatal("expected body with initiatingReason")
	}
	if body["initiatingReason"] != "deleted via CLI" {
		t.Errorf("body.initiatingReason = %v, want 'deleted via CLI'", body["initiatingReason"])
	}
	url, _ := result["url"].(string)
	if !strings.Contains(url, "token=TK001") {
		t.Errorf("url should contain token=TK001, got: %s", url)
	}
}

func TestDelete_IsHighRisk(t *testing.T) {
	sc := DeleteShortcut()
	if sc.Risk != shortcuts.RiskHighRiskWrite {
		t.Errorf("delete risk = %q, want %q", sc.Risk, shortcuts.RiskHighRiskWrite)
	}
}

// --- Execute path tests (httptest server) ---

func TestCreate_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/paymentMethod") {
			t.Errorf("path should contain /paymentMethod, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"},"tokenID":"TK100","status":"Active"}`)
	}))
	defer srv.Close()

	ios, out, _ := newTestIO()
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"token", "+create",
		"--payment-type", "card",
		"--vault-id", "V001",
		"--user-reference", "user@example.com",
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
	data, _ := envelope["data"].(map[string]interface{})
	if data["tokenID"] != "TK100" {
		t.Errorf("data.tokenID = %v, want TK100", data["tokenID"])
	}
}

func TestCreate_Execute_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"B0013","message":"Invalid payment method"}}`)
	}))
	defer srv.Close()

	ios, _, errOut := newTestIO()
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"token", "+create",
		"--payment-type", "card",
		"--vault-id", "V001",
		"--user-reference", "user@example.com",
	})
	// Execute should not return error (shortcut swallows it via OutFormat)
	_ = root.Execute()
	var envelope map[string]interface{}
	if err := json.Unmarshal(errOut.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON on stderr: %v\nraw: %s", err, errOut.String())
	}
	if envelope["ok"] != false {
		t.Errorf("ok = %v, want false", envelope["ok"])
	}
	errObj, _ := envelope["error"].(map[string]interface{})
	if errObj == nil {
		t.Fatal("expected error object in envelope")
	}
}

func TestQuery_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("merchantTransID") != "TX999" {
			t.Errorf("expected merchantTransID=TX999, got %s", r.URL.Query().Get("merchantTransID"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"},"tokenID":"TK200","status":"Active"}`)
	}))
	defer srv.Close()

	ios, out, _ := newTestIO()
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}
	root := buildCmd(f, QueryShortcut())
	root.SetArgs([]string{"token", "+query", "--merchant-tx-id", "TX999"})
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

func TestQuery_Execute_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"B0013","message":"Token not found"}}`)
	}))
	defer srv.Close()

	ios, _, errOut := newTestIO()
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}
	root := buildCmd(f, QueryShortcut())
	root.SetArgs([]string{"token", "+query", "--merchant-tx-id", "TX999"})
	_ = root.Execute()
	var envelope map[string]interface{}
	if err := json.Unmarshal(errOut.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON on stderr: %v\nraw: %s", err, errOut.String())
	}
	if envelope["ok"] != false {
		t.Errorf("ok = %v, want false", envelope["ok"])
	}
}

func TestDelete_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Query().Get("token") != "TK001" {
			t.Errorf("expected token=TK001 query param, got %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"}}`)
	}))
	defer srv.Close()

	ios, out, _ := newTestIO()
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}
	root := buildCmd(f, DeleteShortcut())
	root.SetArgs([]string{"token", "+delete", "--token-id", "TK001", "--yes"})
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

func TestDelete_Execute_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"B0013","message":"Token not found"}}`)
	}))
	defer srv.Close()

	ios, _, errOut := newTestIO()
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}
	root := buildCmd(f, DeleteShortcut())
	root.SetArgs([]string{"token", "+delete", "--token-id", "TK001", "--yes"})
	_ = root.Execute()
	var envelope map[string]interface{}
	if err := json.Unmarshal(errOut.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON on stderr: %v\nraw: %s", err, errOut.String())
	}
	if envelope["ok"] != false {
		t.Errorf("ok = %v, want false", envelope["ok"])
	}
}

func TestQuery_MissingRequired(t *testing.T) {
	ios, _, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, QueryShortcut())
	root.SetArgs([]string{"token", "+query"}) // no --merchant-tx-id
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --merchant-tx-id")
	}
	if !strings.Contains(err.Error(), "--merchant-tx-id") {
		t.Errorf("error should mention --merchant-tx-id, got: %v", err)
	}
}

func TestDelete_MissingRequired(t *testing.T) {
	ios, _, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, DeleteShortcut())
	root.SetArgs([]string{"token", "+delete", "--yes"}) // no --token-id
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --token-id")
	}
	if !strings.Contains(err.Error(), "--token-id") {
		t.Errorf("error should mention --token-id, got: %v", err)
	}
}

func TestDelete_PromptConfirmation(t *testing.T) {
	// Without --yes, the framework prompts for confirmation.
	// Stdin provides "yes\n" so it should proceed.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"}}`)
	}))
	defer srv.Close()

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	ios := &cmdutil.IOStreams{
		In:     io.NopCloser(strings.NewReader("yes\n")),
		Out:    out,
		ErrOut: errOut,
	}
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: newTestConfig(), client: client, io: ios}
	root := buildCmd(f, DeleteShortcut())
	root.SetArgs([]string{"token", "+delete", "--token-id", "TK001"}) // no --yes
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have prompted on stderr
	if !strings.Contains(errOut.String(), "high-risk") {
		t.Errorf("expected high-risk prompt on stderr, got: %s", errOut.String())
	}
	// Should have executed successfully
	var envelope map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out.String())
	}
	if envelope["ok"] != true {
		t.Errorf("ok = %v, want true", envelope["ok"])
	}
}

func TestDelete_PromptDeclined(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	ios := &cmdutil.IOStreams{
		In:     io.NopCloser(strings.NewReader("no\n")),
		Out:    out,
		ErrOut: errOut,
	}
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, DeleteShortcut())
	root.SetArgs([]string{"token", "+delete", "--token-id", "TK001"}) // no --yes
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when user declines confirmation")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("error should mention cancelled, got: %v", err)
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

	// Verify "token" subcommand was added
	tokenCmd, _, err := root.Find([]string{"token"})
	if err != nil {
		t.Fatalf("token command not found: %v", err)
	}
	if tokenCmd.Use != "token" {
		t.Errorf("expected 'token' command, got %q", tokenCmd.Use)
	}

	// Verify all three subcommands exist
	for _, name := range []string{"+create", "+query", "+delete"} {
		found := false
		for _, sub := range tokenCmd.Commands() {
			if sub.Use == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing subcommand %q under token", name)
		}
	}
}
