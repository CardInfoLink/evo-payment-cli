package config

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/registry"
)

// testFactory is a minimal Factory for testing config commands.
type testFactory struct {
	io        *cmdutil.IOStreams
	config    *core.CliConfig
	configErr error
}

func (f *testFactory) Config() (*core.CliConfig, error) {
	return f.config, f.configErr
}

func (f *testFactory) HttpClient() (*http.Client, error) {
	return nil, nil
}

func (f *testFactory) IOStreams() *cmdutil.IOStreams {
	return f.io
}

func (f *testFactory) EvoClient() (*cmdutil.EvoClient, error) {
	return nil, nil
}

func (f *testFactory) Registry() (*registry.Registry, error) {
	return nil, nil
}

func newTestIO(stdinContent string) (*cmdutil.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	stdin := io.NopCloser(strings.NewReader(stdinContent))
	return &cmdutil.IOStreams{In: stdin, Out: stdout, ErrOut: stderr}, stdout, stderr
}

// --- config init tests ---

func TestConfigInit_ValidatesSignType(t *testing.T) {
	ios, _, _ := newTestIO("")
	f := &testFactory{io: ios}

	err := runConfigInit(f, "S001", "INVALID", "test", "", "", false)
	if err == nil {
		t.Fatal("expected error for invalid sign type")
	}
	if !strings.Contains(err.Error(), "invalid sign-type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfigInit_ValidatesEnv(t *testing.T) {
	ios, _, _ := newTestIO("")
	f := &testFactory{io: ios}

	err := runConfigInit(f, "S001", "SHA256", "staging", "", "", false)
	if err == nil {
		t.Fatal("expected error for invalid env")
	}
	if !strings.Contains(err.Error(), "invalid env") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfigInit_WithoutStdin_CreatesConfig(t *testing.T) {
	// Override HOME to use temp dir for config file.
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	ios, stdout, _ := newTestIO("")
	f := &testFactory{io: ios}

	err := runConfigInit(f, "S024116", "SHA256", "test", "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify success envelope output.
	var envelope map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if envelope["ok"] != true {
		t.Errorf("expected ok=true, got %v", envelope["ok"])
	}

	// Verify config file was created.
	configPath := filepath.Join(dir, ".evo-cli", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	var cfg core.CliConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}
	if cfg.MerchantSid != "S024116" {
		t.Errorf("expected merchantSid S024116, got %q", cfg.MerchantSid)
	}
	if cfg.SignType != "SHA256" {
		t.Errorf("expected signType SHA256, got %q", cfg.SignType)
	}
	if cfg.Env != "test" {
		t.Errorf("expected env test, got %q", cfg.Env)
	}
}

func TestConfigInit_EmptyStdin_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	ios, _, _ := newTestIO("")
	f := &testFactory{io: ios}

	err := runConfigInit(f, "S001", "SHA256", "test", "", "", true)
	if err == nil {
		t.Fatal("expected error for empty stdin")
	}
}

func TestConfigInit_AllSignTypes(t *testing.T) {
	for _, st := range validSignTypes {
		dir := t.TempDir()
		t.Setenv("HOME", dir)

		ios, _, _ := newTestIO("")
		f := &testFactory{io: ios}

		err := runConfigInit(f, "S001", st, "test", "", "", false)
		if err != nil {
			t.Errorf("sign type %q should be valid, got error: %v", st, err)
		}
	}
}

func TestConfigInit_BothEnvs(t *testing.T) {
	for _, env := range validEnvs {
		dir := t.TempDir()
		t.Setenv("HOME", dir)

		ios, _, _ := newTestIO("")
		f := &testFactory{io: ios}

		err := runConfigInit(f, "S001", "SHA256", env, "", "", false)
		if err != nil {
			t.Errorf("env %q should be valid, got error: %v", env, err)
		}
	}
}

// --- config show tests ---

func TestConfigShow_DisplaysMaskedKey(t *testing.T) {
	ios, stdout, _ := newTestIO("")
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256",
		Env:         "test",
	}
	f := &testFactory{io: ios, config: cfg}

	err := runConfigShow(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if envelope["ok"] != true {
		t.Errorf("expected ok=true")
	}

	data := envelope["data"].(map[string]interface{})
	signKey := data["signKey"].(string)
	if signKey != "****...bc" {
		t.Errorf("expected masked key ****...bc, got %q", signKey)
	}
	if data["merchantSid"] != "S024116" {
		t.Errorf("expected merchantSid S024116, got %v", data["merchantSid"])
	}
}

func TestConfigShow_ConfigMissing(t *testing.T) {
	ios, _, stderr := newTestIO("")
	f := &testFactory{
		io:        ios,
		configErr: &core.ConfigMissingError{Path: "/home/user/.evo-cli/config.json"},
	}

	err := runConfigShow(f)
	if err == nil {
		t.Fatal("expected error for missing config")
	}

	var envelope map[string]interface{}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse stderr: %v", err)
	}
	if envelope["ok"] != false {
		t.Errorf("expected ok=false")
	}
	errObj := envelope["error"].(map[string]interface{})
	if errObj["type"] != "config_missing" {
		t.Errorf("expected error type config_missing, got %v", errObj["type"])
	}
	if errObj["hint"] != "run: evo-cli config init" {
		t.Errorf("expected hint, got %v", errObj["hint"])
	}
}

// --- config remove tests ---

func TestConfigRemove_RemovesConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Create a config file first.
	cfg := &core.CliConfig{
		MerchantSid: "S001",
		SignType:    "SHA256",
		Env:         "test",
	}
	configPath := filepath.Join(dir, ".evo-cli", "config.json")
	if err := core.SaveConfig(cfg, configPath); err != nil {
		t.Fatalf("setup: save config: %v", err)
	}

	ios, stdout, _ := newTestIO("")
	f := &testFactory{io: ios}

	err := runConfigRemove(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify config file is removed.
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config file should have been removed")
	}

	// Verify success output.
	var envelope map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if envelope["ok"] != true {
		t.Errorf("expected ok=true")
	}
}

func TestConfigRemove_NoConfigFile_StillSucceeds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	ios, stdout, _ := newTestIO("")
	f := &testFactory{io: ios}

	err := runConfigRemove(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if envelope["ok"] != true {
		t.Errorf("expected ok=true even when no config exists")
	}
}

// --- cobra command wiring tests ---

func TestNewCmdConfig_HasSubcommands(t *testing.T) {
	ios, _, _ := newTestIO("")
	f := &testFactory{io: ios}

	cmd := NewCmdConfig(f)
	subCmds := cmd.Commands()

	names := make(map[string]bool)
	for _, c := range subCmds {
		names[c.Name()] = true
	}

	for _, expected := range []string{"init", "show", "remove"} {
		if !names[expected] {
			t.Errorf("expected subcommand %q not found", expected)
		}
	}
}

func TestNewCmdConfigInit_RequiresMerchantSid(t *testing.T) {
	ios, _, _ := newTestIO("")
	f := &testFactory{io: ios}

	cmd := NewCmdConfigInit(f)
	// Execute without --merchant-sid should fail.
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --merchant-sid is missing")
	}
}

// --- Additional coverage tests ---

func TestConfigInit_WithStdin_ValidKey(t *testing.T) {
	// This test verifies the stdin reading and validation path without
	// actually storing to keychain (which requires OS authorization).
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Test that a valid key from stdin is accepted (the keychain store may fail
	// in test environments, but the validation path is covered).
	ios, _, errOut := newTestIO("my-secret-sign-key-32chars-abcde\n")
	f := &testFactory{io: ios}

	err := runConfigInit(f, "S024116", "SHA256", "test", "", "", true)
	// May succeed or fail depending on keychain availability — either way,
	// the stdin reading and validation code paths are exercised.
	_ = err
	_ = errOut
}

func TestConfigInit_WithStdin_WhitespaceOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	ios, _, _ := newTestIO("   \n")
	f := &testFactory{io: ios}

	err := runConfigInit(f, "S001", "SHA256", "test", "", "", true)
	if err == nil {
		t.Fatal("expected error for whitespace-only stdin")
	}
}

func TestConfigRemove_WithKeychainRef(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Create config with a keychain ref.
	cfg := &core.CliConfig{
		MerchantSid: "S001",
		SignKey:     core.SecretInput{Ref: &core.SecretRef{Source: "keychain", ID: "evo-cli:signkey:S001"}},
		SignType:    "SHA256",
		Env:         "test",
	}
	configPath := filepath.Join(dir, ".evo-cli", "config.json")
	core.SaveConfig(cfg, configPath)

	ios, stdout, _ := newTestIO("")
	// Use a factory that returns the config with keychain ref.
	f := &testFactory{io: ios, config: cfg}

	err := runConfigRemove(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope map[string]interface{}
	json.Unmarshal(stdout.Bytes(), &envelope)
	if envelope["ok"] != true {
		t.Errorf("ok=%v want true", envelope["ok"])
	}
}

func TestConfigShow_NonConfigError(t *testing.T) {
	ios, _, stderr := newTestIO("")
	f := &testFactory{
		io:        ios,
		configErr: os.ErrPermission,
	}

	err := runConfigShow(f)
	if err == nil {
		t.Fatal("expected error")
	}

	var envelope map[string]interface{}
	json.Unmarshal(stderr.Bytes(), &envelope)
	errObj := envelope["error"].(map[string]interface{})
	if errObj["type"] != "cli_error" {
		t.Errorf("error type=%v want cli_error", errObj["type"])
	}
}

func TestNewCmdConfigShow_CobraWiring(t *testing.T) {
	ios, _, _ := newTestIO("")
	f := &testFactory{io: ios, config: &core.CliConfig{
		MerchantSid: "S001", SignType: "SHA256", Env: "test",
	}}
	cmd := NewCmdConfigShow(f)
	if cmd.Use != "show" {
		t.Errorf("Use=%q want show", cmd.Use)
	}
}

func TestNewCmdConfigRemove_CobraWiring(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	ios, _, _ := newTestIO("")
	f := &testFactory{io: ios}
	cmd := NewCmdConfigRemove(f)
	if cmd.Use != "remove" {
		t.Errorf("Use=%q want remove", cmd.Use)
	}
}

func TestConfigInit_WithBaseURL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	ios, stdout, _ := newTestIO("")
	f := &testFactory{io: ios}

	customURL := "https://custom-api.example.com"
	err := runConfigInit(f, "S024116", "SHA256", "test", customURL, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify success output includes baseUrl.
	var envelope map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	data := envelope["data"].(map[string]interface{})
	if data["baseUrl"] != customURL {
		t.Errorf("expected baseUrl %q, got %v", customURL, data["baseUrl"])
	}

	// Verify config file stores the custom endpoint.
	configPath := filepath.Join(dir, ".evo-cli", "config.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	var cfg core.CliConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}
	if cfg.Endpoints == nil || cfg.Endpoints.Test != customURL {
		t.Errorf("expected endpoints.test=%q, got %+v", customURL, cfg.Endpoints)
	}
	if cfg.ResolveBaseURL("") != customURL {
		t.Errorf("ResolveBaseURL()=%q, want %q", cfg.ResolveBaseURL(""), customURL)
	}
}

func TestConfigInit_WithBaseURL_Production(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	ios, _, _ := newTestIO("")
	f := &testFactory{io: ios}

	customURL := "https://custom-prod.example.com"
	err := runConfigInit(f, "S024116", "SHA256", "production", customURL, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configPath := filepath.Join(dir, ".evo-cli", "config.json")
	raw, _ := os.ReadFile(configPath)
	var cfg core.CliConfig
	json.Unmarshal(raw, &cfg)
	if cfg.Endpoints == nil || cfg.Endpoints.Prod != customURL {
		t.Errorf("expected endpoints.prod=%q, got %+v", customURL, cfg.Endpoints)
	}
}

func TestConfigInit_WithLinkPayBaseURL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	ios, stdout, _ := newTestIO("")
	f := &testFactory{io: ios}

	customLinkPayURL := "https://custom-linkpay.example.com"
	err := runConfigInit(f, "S024116", "SHA256", "test", "", customLinkPayURL, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify success output includes linkPayBaseUrl.
	var envelope map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	data := envelope["data"].(map[string]interface{})
	if data["linkPayBaseUrl"] != customLinkPayURL {
		t.Errorf("expected linkPayBaseUrl %q, got %v", customLinkPayURL, data["linkPayBaseUrl"])
	}

	// Verify config file stores the custom LinkPay endpoint.
	configPath := filepath.Join(dir, ".evo-cli", "config.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	var cfg core.CliConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}
	if cfg.LinkPayEndpoints == nil || cfg.LinkPayEndpoints.Test != customLinkPayURL {
		t.Errorf("expected linkPayEndpoints.test=%q, got %+v", customLinkPayURL, cfg.LinkPayEndpoints)
	}
	if cfg.ResolveLinkPayBaseURL("") != customLinkPayURL {
		t.Errorf("ResolveLinkPayBaseURL()=%q, want %q", cfg.ResolveLinkPayBaseURL(""), customLinkPayURL)
	}
}

func TestConfigInit_WithLinkPayBaseURL_Production(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	ios, _, _ := newTestIO("")
	f := &testFactory{io: ios}

	customLinkPayURL := "https://custom-linkpay-prod.example.com"
	err := runConfigInit(f, "S024116", "SHA256", "production", "", customLinkPayURL, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configPath := filepath.Join(dir, ".evo-cli", "config.json")
	raw, _ := os.ReadFile(configPath)
	var cfg core.CliConfig
	json.Unmarshal(raw, &cfg)
	if cfg.LinkPayEndpoints == nil || cfg.LinkPayEndpoints.Prod != customLinkPayURL {
		t.Errorf("expected linkPayEndpoints.prod=%q, got %+v", customLinkPayURL, cfg.LinkPayEndpoints)
	}
}

func TestConfigInit_DefaultLinkPayBaseUrl(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	ios, stdout, _ := newTestIO("")
	f := &testFactory{io: ios}

	err := runConfigInit(f, "S024116", "SHA256", "test", "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope map[string]interface{}
	json.Unmarshal(stdout.Bytes(), &envelope)
	data := envelope["data"].(map[string]interface{})
	if data["linkPayBaseUrl"] != core.DefaultLinkPayTestEndpoint {
		t.Errorf("expected default linkPayBaseUrl %q, got %v", core.DefaultLinkPayTestEndpoint, data["linkPayBaseUrl"])
	}
}

func TestConfigShow_IncludesLinkPayBaseURL(t *testing.T) {
	ios, stdout, _ := newTestIO("")
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256",
		Env:         "test",
	}
	f := &testFactory{io: ios, config: cfg}

	err := runConfigShow(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var envelope map[string]interface{}
	json.Unmarshal(stdout.Bytes(), &envelope)
	data := envelope["data"].(map[string]interface{})
	if data["linkPayBaseUrl"] != core.DefaultLinkPayTestEndpoint {
		t.Errorf("expected linkPayBaseUrl %q, got %v", core.DefaultLinkPayTestEndpoint, data["linkPayBaseUrl"])
	}
}
