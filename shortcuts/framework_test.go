package shortcuts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"testing/quick"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/output"
	"github.com/evopayment/evo-cli/internal/registry"
	"github.com/spf13/cobra"
)

// --- test helpers ---

type stubFactory struct {
	config    *core.CliConfig
	configErr error
	io        *cmdutil.IOStreams
}

func (f *stubFactory) Config() (*core.CliConfig, error)       { return f.config, f.configErr }
func (f *stubFactory) HttpClient() (*http.Client, error)      { return nil, nil }
func (f *stubFactory) EvoClient() (*cmdutil.EvoClient, error) { return nil, nil }
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

func newTestIOStreamsWithInput(input string) (*cmdutil.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &cmdutil.IOStreams{
		In:     io.NopCloser(strings.NewReader(input)),
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

// buildTestCommand creates a root command with global flags and mounts a shortcut for testing.
func buildTestCommand(f cmdutil.Factory, sc Shortcut) *cobra.Command {
	root := &cobra.Command{Use: "evo-cli"}
	root.PersistentFlags().Bool("dry-run", false, "Preview the request without sending")
	root.PersistentFlags().String("format", "json", "Output format")
	root.PersistentFlags().Bool("yes", false, "Skip confirmation")

	parent := &cobra.Command{Use: sc.Service, Short: sc.Service + " commands"}
	root.AddCommand(parent)
	Mount(parent, f, []Shortcut{sc})
	return root
}

// --- RuntimeContext tests ---

func TestRuntimeContext_Str(t *testing.T) {
	rt := &RuntimeContext{
		Flags: map[string]string{"amount": "100", "currency": "USD"},
	}
	if got := rt.Str("amount"); got != "100" {
		t.Errorf("Str(amount) = %q, want %q", got, "100")
	}
	if got := rt.Str("missing"); got != "" {
		t.Errorf("Str(missing) = %q, want empty", got)
	}
}

func TestRuntimeContext_Bool(t *testing.T) {
	rt := &RuntimeContext{
		Flags: map[string]string{"verbose": "true", "quiet": "false", "empty": ""},
	}
	if !rt.Bool("verbose") {
		t.Error("Bool(verbose) = false, want true")
	}
	if rt.Bool("quiet") {
		t.Error("Bool(quiet) = true, want false")
	}
	if rt.Bool("empty") {
		t.Error("Bool(empty) = true, want false")
	}
	if rt.Bool("missing") {
		t.Error("Bool(missing) = true, want false")
	}
}

// --- validateRequired tests ---

func TestValidateRequired_AllPresent(t *testing.T) {
	flags := []Flag{
		{Name: "amount", Required: true},
		{Name: "currency", Required: true},
		{Name: "note", Required: false},
	}
	values := map[string]string{"amount": "100", "currency": "USD", "note": ""}
	if err := validateRequired(flags, values); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateRequired_MissingSingle(t *testing.T) {
	flags := []Flag{
		{Name: "amount", Required: true},
		{Name: "currency", Required: true},
	}
	values := map[string]string{"amount": "100", "currency": ""}
	err := validateRequired(flags, values)
	if err == nil {
		t.Fatal("expected error for missing required flag")
	}
	if !strings.Contains(err.Error(), "--currency") {
		t.Errorf("error should mention --currency, got: %v", err)
	}
}

func TestValidateRequired_MissingMultiple(t *testing.T) {
	flags := []Flag{
		{Name: "amount", Required: true},
		{Name: "currency", Required: true},
	}
	values := map[string]string{"amount": "", "currency": ""}
	err := validateRequired(flags, values)
	if err == nil {
		t.Fatal("expected error for missing required flags")
	}
	if !strings.Contains(err.Error(), "--amount") || !strings.Contains(err.Error(), "--currency") {
		t.Errorf("error should mention both flags, got: %v", err)
	}
}

// --- validateEnum tests ---

func TestValidateEnum_ValidValue(t *testing.T) {
	flags := []Flag{
		{Name: "currency", Enum: []string{"USD", "EUR", "CNY"}},
	}
	values := map[string]string{"currency": "USD"}
	if err := validateEnum(flags, values); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateEnum_InvalidValue(t *testing.T) {
	flags := []Flag{
		{Name: "currency", Enum: []string{"USD", "EUR", "CNY"}},
	}
	values := map[string]string{"currency": "GBP"}
	err := validateEnum(flags, values)
	if err == nil {
		t.Fatal("expected error for invalid enum value")
	}
	if !strings.Contains(err.Error(), "GBP") || !strings.Contains(err.Error(), "--currency") {
		t.Errorf("error should mention value and flag, got: %v", err)
	}
}

func TestValidateEnum_EmptyValueSkipped(t *testing.T) {
	flags := []Flag{
		{Name: "currency", Enum: []string{"USD", "EUR"}},
	}
	values := map[string]string{"currency": ""}
	if err := validateEnum(flags, values); err != nil {
		t.Errorf("empty value should be skipped, got: %v", err)
	}
}

func TestValidateEnum_NoEnumDefined(t *testing.T) {
	flags := []Flag{
		{Name: "amount"},
	}
	values := map[string]string{"amount": "anything"}
	if err := validateEnum(flags, values); err != nil {
		t.Errorf("no enum should pass any value, got: %v", err)
	}
}

// --- Mount and execution pipeline tests ---

func TestMount_RegistersSubcommand(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	parent := &cobra.Command{Use: "payment"}
	sc := Shortcut{
		Service:     "payment",
		Command:     "+test",
		Description: "test shortcut",
		Risk:        RiskRead,
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			return nil
		},
	}
	Mount(parent, f, []Shortcut{sc})

	found := false
	for _, cmd := range parent.Commands() {
		if cmd.Use == "+test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected +test subcommand to be registered")
	}
}

func TestShortcut_ExecuteHappyPath(t *testing.T) {
	ios, out, _ := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	executed := false
	sc := Shortcut{
		Service:     "payment",
		Command:     "+test",
		Description: "test shortcut",
		Flags: []Flag{
			{Name: "amount", Required: true},
		},
		Risk: RiskWrite,
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			executed = true
			if rt.Str("amount") != "100" {
				t.Errorf("amount = %q, want %q", rt.Str("amount"), "100")
			}
			fmt.Fprintln(rt.IO.Out, `{"ok":true}`)
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+test", "--amount", "100"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Error("Execute handler was not called")
	}
	if !strings.Contains(out.String(), `"ok":true`) {
		t.Errorf("unexpected output: %s", out.String())
	}
}

func TestShortcut_MissingRequiredFlag(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	sc := Shortcut{
		Service:     "payment",
		Command:     "+test",
		Description: "test",
		Flags: []Flag{
			{Name: "amount", Required: true, Desc: "amount"},
		},
		Risk: RiskWrite,
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			t.Error("Execute should not be called when required flag is missing")
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+test"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing required flag")
	}

	// Check structured error output.
	var env output.ErrorEnvelope
	if jsonErr := json.Unmarshal(errOut.Bytes(), &env); jsonErr != nil {
		t.Fatalf("failed to parse error output: %v\nraw: %s", jsonErr, errOut.String())
	}
	if env.Error.Type != "validation" {
		t.Errorf("error type = %q, want %q", env.Error.Type, "validation")
	}
	if !strings.Contains(env.Error.Message, "--amount") {
		t.Errorf("error message should mention --amount, got: %s", env.Error.Message)
	}
}

func TestShortcut_InvalidEnumValue(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	sc := Shortcut{
		Service:     "payment",
		Command:     "+test",
		Description: "test",
		Flags: []Flag{
			{Name: "currency", Enum: []string{"USD", "EUR"}, Desc: "currency"},
		},
		Risk: RiskRead,
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			t.Error("Execute should not be called for invalid enum")
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+test", "--currency", "GBP"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid enum value")
	}

	var env output.ErrorEnvelope
	if jsonErr := json.Unmarshal(errOut.Bytes(), &env); jsonErr != nil {
		t.Fatalf("failed to parse error output: %v\nraw: %s", jsonErr, errOut.String())
	}
	if env.Error.Type != "validation" {
		t.Errorf("error type = %q, want %q", env.Error.Type, "validation")
	}
}

func TestShortcut_CustomValidation(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	sc := Shortcut{
		Service:     "payment",
		Command:     "+test",
		Description: "test",
		Risk:        RiskRead,
		Validate: func(ctx context.Context, rt *RuntimeContext) error {
			return fmt.Errorf("custom validation failed")
		},
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			t.Error("Execute should not be called when validation fails")
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+test"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error from custom validation")
	}

	var env output.ErrorEnvelope
	if jsonErr := json.Unmarshal(errOut.Bytes(), &env); jsonErr != nil {
		t.Fatalf("failed to parse error output: %v\nraw: %s", jsonErr, errOut.String())
	}
	if !strings.Contains(env.Error.Message, "custom validation failed") {
		t.Errorf("error message = %q, want custom validation message", env.Error.Message)
	}
}

func TestShortcut_DryRunRouting(t *testing.T) {
	ios, out, _ := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	dryRunCalled := false
	sc := Shortcut{
		Service:     "payment",
		Command:     "+test",
		Description: "test",
		Risk:        RiskWrite,
		DryRun: func(ctx context.Context, rt *RuntimeContext) error {
			dryRunCalled = true
			return DryRunOutput(rt.IO, "POST", "https://hkg-online-uat.everonet.com/test", nil, nil)
		},
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			t.Error("Execute should not be called during dry-run")
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+test", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dryRunCalled {
		t.Error("DryRun handler was not called")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("dry-run output is not valid JSON: %v", err)
	}
	if result["method"] != "POST" {
		t.Errorf("method = %v, want POST", result["method"])
	}
}

func TestShortcut_DryRunDefaultHandler(t *testing.T) {
	ios, out, _ := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	sc := Shortcut{
		Service:     "payment",
		Command:     "+test",
		Description: "test",
		Risk:        RiskRead,
		// No DryRun handler set.
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			t.Error("Execute should not be called during dry-run")
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+test", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "dry_run") {
		t.Errorf("expected default dry-run message, got: %s", out.String())
	}
}

// --- High-risk confirmation tests ---

func TestShortcut_HighRiskWrite_YesFlag(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	executed := false
	sc := Shortcut{
		Service:     "payment",
		Command:     "+refund",
		Description: "refund",
		Risk:        RiskHighRiskWrite,
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			executed = true
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+refund", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Error("Execute should be called when --yes is provided")
	}
}

func TestShortcut_HighRiskWrite_ConfirmYes(t *testing.T) {
	ios, _, _ := newTestIOStreamsWithInput("yes\n")
	f := &stubFactory{config: newTestConfig(), io: ios}

	executed := false
	sc := Shortcut{
		Service:     "payment",
		Command:     "+refund",
		Description: "refund",
		Risk:        RiskHighRiskWrite,
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			executed = true
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+refund"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Error("Execute should be called when user confirms with 'yes'")
	}
}

func TestShortcut_HighRiskWrite_ConfirmNo(t *testing.T) {
	ios, _, _ := newTestIOStreamsWithInput("no\n")
	f := &stubFactory{config: newTestConfig(), io: ios}

	sc := Shortcut{
		Service:     "payment",
		Command:     "+refund",
		Description: "refund",
		Risk:        RiskHighRiskWrite,
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			t.Error("Execute should not be called when user declines")
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+refund"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when user declines confirmation")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("error should mention cancelled, got: %v", err)
	}
}

func TestShortcut_HighRiskWrite_EmptyInput(t *testing.T) {
	ios, _, _ := newTestIOStreamsWithInput("\n")
	f := &stubFactory{config: newTestConfig(), io: ios}

	sc := Shortcut{
		Service:     "payment",
		Command:     "+refund",
		Description: "refund",
		Risk:        RiskHighRiskWrite,
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			t.Error("Execute should not be called on empty input")
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+refund"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when user provides empty input")
	}
}

// --- Config error tests ---

func TestShortcut_ConfigError(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	f := &stubFactory{
		configErr: &core.ConfigMissingError{Path: "/test/config.json"},
		io:        ios,
	}

	sc := Shortcut{
		Service:     "payment",
		Command:     "+test",
		Description: "test",
		Risk:        RiskRead,
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			t.Error("Execute should not be called when config is missing")
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+test"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when config is missing")
	}

	var env output.ErrorEnvelope
	if jsonErr := json.Unmarshal(errOut.Bytes(), &env); jsonErr != nil {
		t.Fatalf("failed to parse error output: %v\nraw: %s", jsonErr, errOut.String())
	}
	if env.Error.Type != "config_missing" {
		t.Errorf("error type = %q, want %q", env.Error.Type, "config_missing")
	}
}

// --- Flag default value tests ---

func TestShortcut_FlagDefaultValue(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	var gotFormat string
	sc := Shortcut{
		Service:     "payment",
		Command:     "+test",
		Description: "test",
		Flags: []Flag{
			{Name: "currency", Default: "USD", Desc: "currency"},
		},
		Risk: RiskRead,
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			gotFormat = rt.Str("currency")
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+test"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotFormat != "USD" {
		t.Errorf("default currency = %q, want %q", gotFormat, "USD")
	}
}

// --- DryRunOutput helper test ---

func TestDryRunOutput(t *testing.T) {
	ios, out, _ := newTestIOStreams()
	headers := map[string]string{"Content-Type": "application/json"}
	body := map[string]string{"amount": "100"}

	err := DryRunOutput(ios, "POST", "https://example.com/api", headers, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if result["method"] != "POST" {
		t.Errorf("method = %v, want POST", result["method"])
	}
	if result["url"] != "https://example.com/api" {
		t.Errorf("url = %v, want https://example.com/api", result["url"])
	}
	if result["body"] == nil {
		t.Error("expected body in output")
	}
}

func TestDryRunOutput_NoBody(t *testing.T) {
	ios, out, _ := newTestIOStreams()

	err := DryRunOutput(ios, "GET", "https://example.com/api", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, hasBody := result["body"]; hasBody {
		t.Error("GET request should not have body in dry-run output")
	}
}

// --- Risk level constants test ---

func TestRiskLevelConstants(t *testing.T) {
	if RiskRead != "read" {
		t.Errorf("RiskRead = %q, want %q", RiskRead, "read")
	}
	if RiskWrite != "write" {
		t.Errorf("RiskWrite = %q, want %q", RiskWrite, "write")
	}
	if RiskHighRiskWrite != "high-risk-write" {
		t.Errorf("RiskHighRiskWrite = %q, want %q", RiskHighRiskWrite, "high-risk-write")
	}
}

// --- Multiple shortcuts mount test ---

func TestMount_MultipleShortcuts(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	parent := &cobra.Command{Use: "payment"}
	shortcuts := []Shortcut{
		{Service: "payment", Command: "+pay", Description: "pay", Risk: RiskWrite,
			Execute: func(ctx context.Context, rt *RuntimeContext) error { return nil }},
		{Service: "payment", Command: "+query", Description: "query", Risk: RiskRead,
			Execute: func(ctx context.Context, rt *RuntimeContext) error { return nil }},
		{Service: "payment", Command: "+refund", Description: "refund", Risk: RiskHighRiskWrite,
			Execute: func(ctx context.Context, rt *RuntimeContext) error { return nil }},
	}
	Mount(parent, f, shortcuts)

	if len(parent.Commands()) != 3 {
		t.Errorf("expected 3 subcommands, got %d", len(parent.Commands()))
	}

	names := make(map[string]bool)
	for _, cmd := range parent.Commands() {
		names[cmd.Use] = true
	}
	for _, name := range []string{"+pay", "+query", "+refund"} {
		if !names[name] {
			t.Errorf("expected subcommand %q to be registered", name)
		}
	}
}

// --- Read/Write risk levels don't prompt ---

func TestShortcut_ReadRisk_NoConfirmation(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	executed := false
	sc := Shortcut{
		Service:     "payment",
		Command:     "+query",
		Description: "query",
		Risk:        RiskRead,
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			executed = true
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+query"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Error("Execute should be called for read risk without confirmation")
	}
}

func TestShortcut_WriteRisk_NoConfirmation(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	f := &stubFactory{config: newTestConfig(), io: ios}

	executed := false
	sc := Shortcut{
		Service:     "payment",
		Command:     "+pay",
		Description: "pay",
		Risk:        RiskWrite,
		Execute: func(ctx context.Context, rt *RuntimeContext) error {
			executed = true
			return nil
		},
	}

	root := buildTestCommand(f, sc)
	root.SetArgs([]string{"payment", "+pay"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Error("Execute should be called for write risk without confirmation")
	}
}

// --- OutFormat helper test ---

func TestRuntimeContext_OutFormat_Success(t *testing.T) {
	ios, out, _ := newTestIOStreams()
	rt := &RuntimeContext{IO: ios}

	data := map[string]string{"status": "ok"}
	rt.OutFormat(data, nil, nil)

	var env output.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, out.String())
	}
	if !env.OK {
		t.Error("expected ok=true")
	}
}

func TestRuntimeContext_OutFormat_Error(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	rt := &RuntimeContext{IO: ios}

	rt.OutFormat(nil, nil, fmt.Errorf("something went wrong"))

	var env output.ErrorEnvelope
	if err := json.Unmarshal(errOut.Bytes(), &env); err != nil {
		t.Fatalf("error output is not valid JSON: %v\nraw: %s", err, errOut.String())
	}
	if env.OK {
		t.Error("expected ok=false")
	}
}

// Feature: evo-payment-cli, Property 25: High-Risk 操作确认
// Validates: Requirement 8.8
//
// For any high-risk-write shortcut, when --yes is NOT provided and stdin does NOT
// contain "yes", Execute must NOT be called. When --yes IS provided, Execute must
// be called.
func TestProperty25_HighRiskConfirmation(t *testing.T) {
	// Non-confirming inputs: anything that does not trim to "yes".
	nonConfirmInputs := []string{"", "no", "n", "Y", "YES", "yep", "nope", "cancel", "ye", "y e s", "yess"}

	// Fixed set of valid command names to avoid random Unicode issues with cobra.
	cmdNames := []string{"+refund", "+cancel", "+delete", "+drop", "+purge"}

	cfg := &quick.Config{
		MaxCount: 100,
		Rand:     rand.New(rand.NewSource(42)),
	}

	err := quick.Check(func(cmdIdx uint8, inputIdx uint8, useYesFlag bool) bool {
		cmdName := cmdNames[int(cmdIdx)%len(cmdNames)]

		executed := false
		sc := Shortcut{
			Service:     "payment",
			Command:     cmdName,
			Description: "test high-risk shortcut",
			Risk:        RiskHighRiskWrite,
			Execute: func(ctx context.Context, rt *RuntimeContext) error {
				executed = true
				return nil
			},
		}

		if useYesFlag {
			// Case 1: --yes is provided → Execute MUST be called.
			ios, _, _ := newTestIOStreams()
			f := &stubFactory{config: newTestConfig(), io: ios}
			root := buildTestCommand(f, sc)
			root.SetArgs([]string{sc.Service, cmdName, "--yes"})
			_ = root.Execute()
			if !executed {
				t.Logf("--yes provided but Execute was not called (cmd=%s)", cmdName)
				return false
			}
		} else {
			// Case 2: --yes NOT provided, stdin is non-confirming → Execute must NOT be called.
			input := nonConfirmInputs[int(inputIdx)%len(nonConfirmInputs)]
			ios, _, _ := newTestIOStreamsWithInput(input + "\n")
			f := &stubFactory{config: newTestConfig(), io: ios}
			root := buildTestCommand(f, sc)
			root.SetArgs([]string{sc.Service, cmdName})
			_ = root.Execute()
			if executed {
				t.Logf("no --yes, stdin=%q, but Execute was called (cmd=%s)", input, cmdName)
				return false
			}
		}

		return true
	}, cfg)

	if err != nil {
		t.Errorf("Property 25 failed: %v", err)
	}
}
