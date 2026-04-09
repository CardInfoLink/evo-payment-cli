package cmdutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFactory(t *testing.T) {
	ios := DefaultIOStreams()
	f := NewFactory(ios)
	if f == nil {
		t.Fatal("NewFactory returned nil")
	}
	if f.IOStreams() != ios {
		t.Error("IOStreams mismatch")
	}
}

func TestDefaultFactory_IOStreams(t *testing.T) {
	ios := DefaultIOStreams()
	f := NewFactory(ios)
	if f.IOStreams().Out != os.Stdout {
		t.Error("expected stdout")
	}
}

func TestDefaultFactory_Config_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	ios := DefaultIOStreams()
	f := NewFactory(ios)
	_, err := f.Config()
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestDefaultFactory_Config_ValidFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfgDir := filepath.Join(dir, ".evo-cli")
	os.MkdirAll(cfgDir, 0700)
	os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"merchantSid":"S001","signType":"SHA256","env":"test"}`), 0600)

	ios := DefaultIOStreams()
	f := NewFactory(ios)
	cfg, err := f.Config()
	if err != nil {
		t.Fatalf("Config() error: %v", err)
	}
	if cfg.MerchantSid != "S001" {
		t.Errorf("MerchantSid=%q want S001", cfg.MerchantSid)
	}
}

func TestDefaultFactory_Registry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	ios := DefaultIOStreams()
	f := NewFactory(ios)
	reg, err := f.Registry()
	if err != nil {
		t.Fatalf("Registry() error: %v", err)
	}
	if reg == nil {
		t.Fatal("Registry returned nil")
	}
	if len(reg.Services) == 0 {
		t.Error("expected at least one service from embedded data")
	}
}

func TestDefaultFactory_Keychain(t *testing.T) {
	ios := DefaultIOStreams()
	f := NewFactory(ios)
	kc := f.Keychain()
	if kc == nil {
		t.Fatal("Keychain returned nil")
	}
}

func TestDefaultFactory_EvoClient_NoConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	ios := DefaultIOStreams()
	f := NewFactory(ios)
	_, err := f.EvoClient()
	if err == nil {
		t.Fatal("expected error when config is missing")
	}
}

func TestDefaultFactory_HttpClient_NoConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	ios := DefaultIOStreams()
	f := NewFactory(ios)
	// HttpClient creates the transport chain which needs config for SignatureTransport.
	// But the transport chain is created eagerly, config is loaded lazily on first request.
	client, err := f.HttpClient()
	if err != nil {
		t.Fatalf("HttpClient() error: %v", err)
	}
	if client == nil {
		t.Fatal("HttpClient returned nil")
	}
}
