package payment

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
	ios       *cmdutil.IOStreams
}

func (f *stubFactory) Config() (*core.CliConfig, error)       { return f.config, f.configErr }
func (f *stubFactory) HttpClient() (*http.Client, error)      { return nil, nil }
func (f *stubFactory) EvoClient() (*cmdutil.EvoClient, error) { return f.client, f.clientErr }
func (f *stubFactory) IOStreams() *cmdutil.IOStreams          { return f.ios }
func (f *stubFactory) Registry() (*registry.Registry, error)  { return nil, nil }

func mkIO() (*cmdutil.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &cmdutil.IOStreams{
		In:     io.NopCloser(strings.NewReader("yes\n")),
		Out:    out,
		ErrOut: errOut,
	}, out, errOut
}

func mkCfg() *core.CliConfig {
	cfg := &core.CliConfig{MerchantSid: "S024116", SignType: "SHA256", Env: "test"}
	cfg.SetResolvedBaseURL("https://hkg-online-uat.everonet.com")
	return cfg
}

func mkClient(url string, ios *cmdutil.IOStreams) *cmdutil.EvoClient {
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256", Env: "test",
	}
	cfg.SetResolvedBaseURL(url)
	return cmdutil.NewEvoClient(&http.Client{}, cfg, ios)
}

func mkCmd(f cmdutil.Factory, sc shortcuts.Shortcut) *cobra.Command {
	root := &cobra.Command{Use: "evo-cli"}
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().String("format", "json", "")
	root.PersistentFlags().Bool("yes", false, "")
	p := &cobra.Command{Use: sc.Service}
	root.AddCommand(p)
	shortcuts.Mount(p, f, []shortcuts.Shortcut{sc})
	return root
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"},"payment":{"status":"Captured"}}`)
}

func errHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"result":{"code":"B0013","message":"Amount exceeded"}}`)
}

func TestAllShortcuts_Count(t *testing.T) {
	all := AllShortcuts()
	if len(all) != 5 {
		t.Errorf("expected 5, got %d", len(all))
	}
}

func TestRegisterShortcuts(t *testing.T) {
	ios, _, _ := mkIO()
	f := &stubFactory{config: mkCfg(), ios: ios}
	root := &cobra.Command{Use: "evo-cli"}
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().String("format", "json", "")
	root.PersistentFlags().Bool("yes", false, "")
	RegisterShortcuts(root, f)
	cmd, _, _ := root.Find([]string{"payment"})
	if cmd == nil || cmd.Use != "payment" {
		t.Fatal("payment not registered")
	}
}

func TestPay_DryRun(t *testing.T) {
	ios, out, _ := mkIO()
	f := &stubFactory{config: mkCfg(), ios: ios}
	root := mkCmd(f, PayShortcut())
	root.SetArgs([]string{"payment", "+pay", "--amount", "100", "--currency", "USD", "--payment-brand", "VISA", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var r map[string]interface{}
	json.Unmarshal(out.Bytes(), &r)
	if r["method"] != "POST" {
		t.Errorf("method=%v", r["method"])
	}
}

func TestPay_Execute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(okHandler))
	defer srv.Close()
	ios, out, _ := mkIO()
	f := &stubFactory{config: mkCfg(), client: mkClient(srv.URL, ios), ios: ios}
	root := mkCmd(f, PayShortcut())
	root.SetArgs([]string{"payment", "+pay", "--amount", "100", "--currency", "USD", "--payment-brand", "VISA"})
	root.Execute()
	var env map[string]interface{}
	json.Unmarshal(out.Bytes(), &env)
	if env["ok"] != true {
		t.Errorf("ok=%v", env["ok"])
	}
}

func TestPay_Execute_OptionalFlags(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		okHandler(w, r)
	}))
	defer srv.Close()
	ios, _, _ := mkIO()
	f := &stubFactory{config: mkCfg(), client: mkClient(srv.URL, ios), ios: ios}
	root := mkCmd(f, PayShortcut())
	root.SetArgs([]string{"payment", "+pay", "--amount", "1", "--currency", "USD", "--payment-brand", "V",
		"--merchant-tx-id", "TX9", "--webhook", "https://h.co", "--return-url", "https://r.co"})
	root.Execute()
	for _, s := range []string{"TX9", "h.co", "r.co"} {
		if !strings.Contains(body, s) {
			t.Errorf("body missing %s: %s", s, body)
		}
	}
}

func TestPay_Execute_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(errHandler))
	defer srv.Close()
	ios, _, errOut := mkIO()
	f := &stubFactory{config: mkCfg(), client: mkClient(srv.URL, ios), ios: ios}
	root := mkCmd(f, PayShortcut())
	root.SetArgs([]string{"payment", "+pay", "--amount", "1", "--currency", "USD", "--payment-brand", "V"})
	root.Execute()
	if !strings.Contains(errOut.String(), "false") {
		t.Error("expected error envelope")
	}
}

func TestPay_MissingRequired(t *testing.T) {
	ios, _, _ := mkIO()
	f := &stubFactory{config: mkCfg(), ios: ios}
	root := mkCmd(f, PayShortcut())
	root.SetArgs([]string{"payment", "+pay", "--amount", "1"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error")
	}
}

func TestPay_InvalidEnum(t *testing.T) {
	ios, _, _ := mkIO()
	f := &stubFactory{config: mkCfg(), ios: ios}
	root := mkCmd(f, PayShortcut())
	root.SetArgs([]string{"payment", "+pay", "--amount", "1", "--currency", "U", "--payment-brand", "V", "--payment-type", "bad"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error")
	}
}

func TestQuery_DryRun(t *testing.T) {
	ios, out, _ := mkIO()
	f := &stubFactory{config: mkCfg(), ios: ios}
	root := mkCmd(f, QueryShortcut())
	root.SetArgs([]string{"payment", "+query", "--merchant-tx-id", "TX1", "--dry-run"})
	root.Execute()
	var r map[string]interface{}
	json.Unmarshal(out.Bytes(), &r)
	if r["method"] != "GET" {
		t.Errorf("method=%v", r["method"])
	}
}

func TestQuery_Execute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(okHandler))
	defer srv.Close()
	ios, out, _ := mkIO()
	f := &stubFactory{config: mkCfg(), client: mkClient(srv.URL, ios), ios: ios}
	root := mkCmd(f, QueryShortcut())
	root.SetArgs([]string{"payment", "+query", "--merchant-tx-id", "TX1"})
	root.Execute()
	var env map[string]interface{}
	json.Unmarshal(out.Bytes(), &env)
	if env["ok"] != true {
		t.Errorf("ok=%v", env["ok"])
	}
}

func TestQuery_MissingRequired(t *testing.T) {
	ios, _, _ := mkIO()
	f := &stubFactory{config: mkCfg(), ios: ios}
	root := mkCmd(f, QueryShortcut())
	root.SetArgs([]string{"payment", "+query"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error")
	}
}

func TestCapture_DryRun(t *testing.T) {
	ios, out, _ := mkIO()
	f := &stubFactory{config: mkCfg(), ios: ios}
	root := mkCmd(f, CaptureShortcut())
	root.SetArgs([]string{"payment", "+capture", "--original-merchant-tx-id", "TX1", "--amount", "50", "--currency", "EUR", "--dry-run"})
	root.Execute()
	var r map[string]interface{}
	json.Unmarshal(out.Bytes(), &r)
	if r["method"] != "POST" {
		t.Errorf("method=%v", r["method"])
	}
}

func TestCapture_Execute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(okHandler))
	defer srv.Close()
	ios, out, _ := mkIO()
	f := &stubFactory{config: mkCfg(), client: mkClient(srv.URL, ios), ios: ios}
	root := mkCmd(f, CaptureShortcut())
	root.SetArgs([]string{"payment", "+capture", "--original-merchant-tx-id", "TX1", "--amount", "50", "--currency", "EUR"})
	root.Execute()
	var env map[string]interface{}
	json.Unmarshal(out.Bytes(), &env)
	if env["ok"] != true {
		t.Errorf("ok=%v", env["ok"])
	}
}

func TestCapture_MissingRequired(t *testing.T) {
	ios, _, _ := mkIO()
	f := &stubFactory{config: mkCfg(), ios: ios}
	root := mkCmd(f, CaptureShortcut())
	root.SetArgs([]string{"payment", "+capture", "--amount", "50"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error")
	}
}

func TestCancel_DryRun(t *testing.T) {
	ios, out, _ := mkIO()
	f := &stubFactory{config: mkCfg(), ios: ios}
	root := mkCmd(f, CancelShortcut())
	root.SetArgs([]string{"payment", "+cancel", "--original-merchant-tx-id", "TX1", "--yes", "--dry-run"})
	root.Execute()
	var r map[string]interface{}
	json.Unmarshal(out.Bytes(), &r)
	if r["method"] != "POST" {
		t.Errorf("method=%v", r["method"])
	}
}

func TestCancel_Execute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(okHandler))
	defer srv.Close()
	ios, out, _ := mkIO()
	f := &stubFactory{config: mkCfg(), client: mkClient(srv.URL, ios), ios: ios}
	root := mkCmd(f, CancelShortcut())
	root.SetArgs([]string{"payment", "+cancel", "--original-merchant-tx-id", "TX1", "--yes"})
	root.Execute()
	var env map[string]interface{}
	json.Unmarshal(out.Bytes(), &env)
	if env["ok"] != true {
		t.Errorf("ok=%v", env["ok"])
	}
}

func TestCancel_IsHighRisk(t *testing.T) {
	if CancelShortcut().Risk != shortcuts.RiskHighRiskWrite {
		t.Error("cancel should be high-risk-write")
	}
}

func TestCancel_MissingRequired(t *testing.T) {
	ios, _, _ := mkIO()
	f := &stubFactory{config: mkCfg(), ios: ios}
	root := mkCmd(f, CancelShortcut())
	root.SetArgs([]string{"payment", "+cancel", "--yes"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error")
	}
}

func TestRefund_DryRun(t *testing.T) {
	ios, out, _ := mkIO()
	f := &stubFactory{config: mkCfg(), ios: ios}
	root := mkCmd(f, RefundShortcut())
	root.SetArgs([]string{"payment", "+refund", "--original-merchant-tx-id", "TX1", "--amount", "25", "--currency", "USD", "--yes", "--dry-run"})
	root.Execute()
	var r map[string]interface{}
	json.Unmarshal(out.Bytes(), &r)
	if r["method"] != "POST" {
		t.Errorf("method=%v", r["method"])
	}
}

func TestRefund_Execute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(okHandler))
	defer srv.Close()
	ios, out, _ := mkIO()
	f := &stubFactory{config: mkCfg(), client: mkClient(srv.URL, ios), ios: ios}
	root := mkCmd(f, RefundShortcut())
	root.SetArgs([]string{"payment", "+refund", "--original-merchant-tx-id", "TX1", "--amount", "25", "--currency", "USD", "--yes"})
	root.Execute()
	var env map[string]interface{}
	json.Unmarshal(out.Bytes(), &env)
	if env["ok"] != true {
		t.Errorf("ok=%v", env["ok"])
	}
}

func TestRefund_IsHighRisk(t *testing.T) {
	if RefundShortcut().Risk != shortcuts.RiskHighRiskWrite {
		t.Error("refund should be high-risk-write")
	}
}

func TestRefund_MissingRequired(t *testing.T) {
	ios, _, _ := mkIO()
	f := &stubFactory{config: mkCfg(), ios: ios}
	root := mkCmd(f, RefundShortcut())
	root.SetArgs([]string{"payment", "+refund", "--amount", "25", "--yes"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error")
	}
}
