package linkpay

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
	cfg.SetResolvedLinkPayBaseURL("https://hkg-counter-uat.everonet.com")
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
		t.Errorf("expected 3 linkpay shortcuts, got %d", len(all))
	}
	names := map[string]bool{}
	for _, sc := range all {
		names[sc.Command] = true
	}
	for _, want := range []string{"+create", "+query", "+refund"} {
		if !names[want] {
			t.Errorf("missing shortcut %q", want)
		}
	}
}

func TestCreate_DryRun(t *testing.T) {
	ios, out, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"linkpay", "+create",
		"--amount", "200.00",
		"--currency", "EUR",
		"--order-id", "ORD001",
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
	if !strings.Contains(url, "/evo.e-commerce.linkpay") {
		t.Errorf("url should contain linkpay path, got: %s", url)
	}
	if !strings.Contains(url, "hkg-counter-uat") {
		t.Errorf("url should use LinkPay base URL (hkg-counter-uat), got: %s", url)
	}
	body, _ := result["body"].(map[string]interface{})
	orderInfo, _ := body["merchantOrderInfo"].(map[string]interface{})
	if orderInfo["merchantOrderID"] != "ORD001" {
		t.Errorf("body.merchantOrderInfo.merchantOrderID = %v, want ORD001", orderInfo["merchantOrderID"])
	}
}

func TestCreate_MissingRequired(t *testing.T) {
	ios, _, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"linkpay", "+create", "--amount", "100"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestQuery_DryRun(t *testing.T) {
	ios, out, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, QueryShortcut())
	root.SetArgs([]string{"linkpay", "+query", "--merchant-order-id", "ORD001", "--dry-run"})
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
	if !strings.Contains(url, "ORD001") {
		t.Errorf("url should contain order ID, got: %s", url)
	}
	if !strings.Contains(url, "hkg-counter-uat") {
		t.Errorf("url should use LinkPay base URL (hkg-counter-uat), got: %s", url)
	}
}

func TestRefund_DryRun(t *testing.T) {
	ios, out, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, RefundShortcut())
	root.SetArgs([]string{"linkpay", "+refund",
		"--merchant-order-id", "ORD001",
		"--amount", "50.00",
		"--currency", "USD",
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
	if result["method"] != "POST" {
		t.Errorf("method = %v, want POST", result["method"])
	}
	url, _ := result["url"].(string)
	if !strings.Contains(url, "linkpayRefund") {
		t.Errorf("url should contain linkpayRefund, got: %s", url)
	}
	if !strings.Contains(url, "hkg-counter-uat") {
		t.Errorf("url should use LinkPay base URL (hkg-counter-uat), got: %s", url)
	}
}

func TestRefund_IsHighRisk(t *testing.T) {
	sc := RefundShortcut()
	if sc.Risk != shortcuts.RiskHighRiskWrite {
		t.Errorf("refund risk = %q, want %q", sc.Risk, shortcuts.RiskHighRiskWrite)
	}
}

// --- Execute path tests (httptest server) ---

func TestCreate_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/evo.e-commerce.linkpay") {
			t.Errorf("path should contain /evo.e-commerce.linkpay, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"},"linkPayURL":"https://pay.example.com/LP001","status":"Created"}`)
	}))
	defer srv.Close()

	ios, out, _ := newTestIO()
	cfg := newTestConfig()
	cfg.SetResolvedLinkPayBaseURL(srv.URL)
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: cfg, client: client, io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"linkpay", "+create",
		"--amount", "200.00",
		"--currency", "EUR",
		"--order-id", "ORD001",
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
	if data["linkPayURL"] != "https://pay.example.com/LP001" {
		t.Errorf("data.linkPayURL = %v, want https://pay.example.com/LP001", data["linkPayURL"])
	}
}

func TestCreate_Execute_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"B0013","message":"Invalid order"}}`)
	}))
	defer srv.Close()

	ios, _, errOut := newTestIO()
	cfg := newTestConfig()
	cfg.SetResolvedLinkPayBaseURL(srv.URL)
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: cfg, client: client, io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"linkpay", "+create",
		"--amount", "200.00",
		"--currency", "EUR",
		"--order-id", "ORD001",
	})
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
		if !strings.Contains(r.URL.Path, "ORD001") {
			t.Errorf("path should contain ORD001, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"},"status":"Paid","amount":"200.00"}`)
	}))
	defer srv.Close()

	ios, out, _ := newTestIO()
	cfg := newTestConfig()
	cfg.SetResolvedLinkPayBaseURL(srv.URL)
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: cfg, client: client, io: ios}
	root := buildCmd(f, QueryShortcut())
	root.SetArgs([]string{"linkpay", "+query", "--merchant-order-id", "ORD001"})
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
		fmt.Fprint(w, `{"result":{"code":"B0013","message":"Order not found"}}`)
	}))
	defer srv.Close()

	ios, _, errOut := newTestIO()
	cfg := newTestConfig()
	cfg.SetResolvedLinkPayBaseURL(srv.URL)
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: cfg, client: client, io: ios}
	root := buildCmd(f, QueryShortcut())
	root.SetArgs([]string{"linkpay", "+query", "--merchant-order-id", "ORD001"})
	_ = root.Execute()
	var envelope map[string]interface{}
	if err := json.Unmarshal(errOut.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON on stderr: %v\nraw: %s", err, errOut.String())
	}
	if envelope["ok"] != false {
		t.Errorf("ok = %v, want false", envelope["ok"])
	}
}

func TestRefund_Execute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "linkpayRefund") {
			t.Errorf("path should contain linkpayRefund, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"},"refund":{"status":"Refunded"}}`)
	}))
	defer srv.Close()

	ios, out, _ := newTestIO()
	cfg := newTestConfig()
	cfg.SetResolvedLinkPayBaseURL(srv.URL)
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: cfg, client: client, io: ios}
	root := buildCmd(f, RefundShortcut())
	root.SetArgs([]string{"linkpay", "+refund",
		"--merchant-order-id", "ORD001",
		"--amount", "50.00",
		"--currency", "USD",
		"--yes",
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

func TestRefund_Execute_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"B0013","message":"Refund not allowed"}}`)
	}))
	defer srv.Close()

	ios, _, errOut := newTestIO()
	cfg := newTestConfig()
	cfg.SetResolvedLinkPayBaseURL(srv.URL)
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: cfg, client: client, io: ios}
	root := buildCmd(f, RefundShortcut())
	root.SetArgs([]string{"linkpay", "+refund",
		"--merchant-order-id", "ORD001",
		"--amount", "50.00",
		"--currency", "USD",
		"--yes",
	})
	_ = root.Execute()
	var envelope map[string]interface{}
	if err := json.Unmarshal(errOut.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON on stderr: %v\nraw: %s", err, errOut.String())
	}
	if envelope["ok"] != false {
		t.Errorf("ok = %v, want false", envelope["ok"])
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
	cmd, _, _ := root.Find([]string{"linkpay"})
	if cmd == nil || cmd.Use != "linkpay" {
		t.Fatal("linkpay not registered")
	}
}

func TestQuery_MissingRequired(t *testing.T) {
	ios, _, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, QueryShortcut())
	root.SetArgs([]string{"linkpay", "+query"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --merchant-order-id")
	}
}

func TestRefund_MissingRequired(t *testing.T) {
	ios, _, _ := newTestIO()
	f := &stubFactory{config: newTestConfig(), io: ios}
	root := buildCmd(f, RefundShortcut())
	root.SetArgs([]string{"linkpay", "+refund", "--amount", "50", "--yes"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestCreate_Execute_OptionalFlags(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"}}`)
	}))
	defer srv.Close()
	ios, _, _ := newTestIO()
	cfg := newTestConfig()
	cfg.SetResolvedLinkPayBaseURL(srv.URL)
	client := newTestEvoClient(srv.URL, ios)
	f := &stubFactory{config: cfg, client: client, io: ios}
	root := buildCmd(f, CreateShortcut())
	root.SetArgs([]string{"linkpay", "+create", "--amount", "100", "--currency", "USD", "--order-id", "O1",
		"--return-url", "https://r.co", "--webhook", "https://w.co", "--valid-time", "30"})
	root.Execute()
	for _, s := range []string{"r.co", "w.co", "30"} {
		if !strings.Contains(body, s) {
			t.Errorf("body missing %s", s)
		}
	}
}
