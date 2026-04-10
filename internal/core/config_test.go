package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/quick"
)

// --- SecretInput JSON marshal/unmarshal tests ---

func TestSecretInput_MarshalPlainString(t *testing.T) {
	s := SecretInput{Value: "my-secret"}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if string(data) != `"my-secret"` {
		t.Errorf("expected %q, got %s", "my-secret", data)
	}
}

func TestSecretInput_MarshalRef(t *testing.T) {
	s := SecretInput{Ref: &SecretRef{Source: "keychain", ID: "evo-cli:signkey:S024116"}}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var ref SecretRef
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatalf("unmarshal ref error: %v", err)
	}
	if ref.Source != "keychain" || ref.ID != "evo-cli:signkey:S024116" {
		t.Errorf("unexpected ref: %+v", ref)
	}
}

func TestSecretInput_MarshalEmptyValue(t *testing.T) {
	s := SecretInput{}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if string(data) != `""` {
		t.Errorf("expected empty string JSON, got %s", data)
	}
}

func TestSecretInput_UnmarshalPlainString(t *testing.T) {
	var s SecretInput
	if err := json.Unmarshal([]byte(`"plain-key"`), &s); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if s.Value != "plain-key" {
		t.Errorf("expected plain-key, got %q", s.Value)
	}
	if s.Ref != nil {
		t.Errorf("expected nil ref, got %+v", s.Ref)
	}
}

func TestSecretInput_UnmarshalRef(t *testing.T) {
	var s SecretInput
	if err := json.Unmarshal([]byte(`{"source":"keychain","id":"evo-cli:signkey:S024116"}`), &s); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if s.Value != "" {
		t.Errorf("expected empty value, got %q", s.Value)
	}
	if s.Ref == nil {
		t.Fatal("expected non-nil ref")
	}
	if s.Ref.Source != "keychain" || s.Ref.ID != "evo-cli:signkey:S024116" {
		t.Errorf("unexpected ref: %+v", s.Ref)
	}
}

func TestSecretInput_RoundTrip_PlainString(t *testing.T) {
	original := SecretInput{Value: "test-key-12345"}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded SecretInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Value != original.Value {
		t.Errorf("round-trip mismatch: %q != %q", decoded.Value, original.Value)
	}
}

func TestSecretInput_RoundTrip_Ref(t *testing.T) {
	original := SecretInput{Ref: &SecretRef{Source: "file", ID: "/path/to/secret"}}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded SecretInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Ref == nil || decoded.Ref.Source != "file" || decoded.Ref.ID != "/path/to/secret" {
		t.Errorf("round-trip mismatch: %+v", decoded)
	}
}

// --- CliConfig JSON round-trip with SecretInput ---

func TestCliConfig_JSON_WithKeychainRef(t *testing.T) {
	cfg := CliConfig{
		MerchantSid: "S024116",
		SignKey:     SecretInput{Ref: &SecretRef{Source: "keychain", ID: "evo-cli:signkey:S024116"}},
		SignType:    "SHA256",
		Env:         "test",
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded CliConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.MerchantSid != "S024116" {
		t.Errorf("merchantSid mismatch: %q", decoded.MerchantSid)
	}
	if decoded.SignKey.Ref == nil || decoded.SignKey.Ref.Source != "keychain" {
		t.Errorf("signKey ref mismatch: %+v", decoded.SignKey)
	}
}

// --- LoadConfig / SaveConfig tests ---

func TestLoadConfig_FileNotExist(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	cmErr, ok := err.(*ConfigMissingError)
	if !ok {
		t.Fatalf("expected ConfigMissingError, got %T: %v", err, err)
	}
	if cmErr.Type() != "config_missing" {
		t.Errorf("expected type config_missing, got %q", cmErr.Type())
	}
	if cmErr.Hint() != "run: evo-cli config init" {
		t.Errorf("unexpected hint: %q", cmErr.Hint())
	}
}

func TestSaveConfig_AndLoadConfig(t *testing.T) {
	// Clear env vars that would override config file values.
	t.Setenv(EnvVarMerchantSid, "")
	t.Setenv(EnvVarSignKey, "")
	t.Setenv(EnvVarSignType, "")
	t.Setenv(EnvVarBaseURL, "")
	t.Setenv(EnvVarLinkPayBaseURL, "")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &CliConfig{
		MerchantSid: "S999",
		SignKey:     SecretInput{Value: "test-sign-key-32chars-abcdefgh"},
		SignType:    "SHA256",
		Env:         "test",
		Endpoints:   &Endpoints{Test: "https://custom-test.example.com", Prod: "https://custom-prod.example.com"},
	}

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.MerchantSid != "S999" {
		t.Errorf("merchantSid mismatch: %q", loaded.MerchantSid)
	}
	if loaded.SignKey.Value != "test-sign-key-32chars-abcdefgh" {
		t.Errorf("signKey mismatch: %q", loaded.SignKey.Value)
	}
	if loaded.SignType != "SHA256" {
		t.Errorf("signType mismatch: %q", loaded.SignType)
	}
	if loaded.Env != "test" {
		t.Errorf("env mismatch: %q", loaded.Env)
	}
}

func TestSaveConfig_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "config.json")

	cfg := &CliConfig{MerchantSid: "S001", Env: "test"}
	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("save error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

// --- Environment variable override tests ---

func TestLoadConfig_EnvOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &CliConfig{
		MerchantSid: "S_FILE",
		SignKey:     SecretInput{Value: "file-key"},
		SignType:    "SHA256",
		Env:         "test",
	}
	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Set env vars.
	t.Setenv(EnvVarMerchantSid, "S_ENV")
	t.Setenv(EnvVarSignKey, "env-key")
	t.Setenv(EnvVarSignType, "HMAC-SHA256")
	t.Setenv(EnvVarBaseURL, "https://custom.example.com")

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.MerchantSid != "S_ENV" {
		t.Errorf("expected S_ENV, got %q", loaded.MerchantSid)
	}
	if loaded.SignKey.Value != "env-key" {
		t.Errorf("expected env-key, got %q", loaded.SignKey.Value)
	}
	if loaded.SignType != "HMAC-SHA256" {
		t.Errorf("expected HMAC-SHA256, got %q", loaded.SignType)
	}
	if loaded.ResolveBaseURL("") != "https://custom.example.com" {
		t.Errorf("expected custom URL, got %q", loaded.ResolveBaseURL(""))
	}
}

func TestLoadConfig_EnvPartialOverride(t *testing.T) {
	// Clear all env vars first, then selectively set only the one we want to test.
	t.Setenv(EnvVarMerchantSid, "")
	t.Setenv(EnvVarSignKey, "")
	t.Setenv(EnvVarSignType, "")
	t.Setenv(EnvVarBaseURL, "")
	t.Setenv(EnvVarLinkPayBaseURL, "")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &CliConfig{
		MerchantSid: "S_FILE",
		SignKey:     SecretInput{Value: "file-key"},
		SignType:    "SHA256",
		Env:         "test",
	}
	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Only override merchantSid.
	t.Setenv(EnvVarMerchantSid, "S_ENV_ONLY")

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.MerchantSid != "S_ENV_ONLY" {
		t.Errorf("expected S_ENV_ONLY, got %q", loaded.MerchantSid)
	}
	// Other fields should remain from file.
	if loaded.SignKey.Value != "file-key" {
		t.Errorf("expected file-key, got %q", loaded.SignKey.Value)
	}
	if loaded.SignType != "SHA256" {
		t.Errorf("expected SHA256, got %q", loaded.SignType)
	}
}

// --- ResolveBaseURL tests ---

func TestResolveBaseURL_DefaultTest(t *testing.T) {
	cfg := &CliConfig{Env: "test"}
	if got := cfg.ResolveBaseURL(""); got != DefaultTestEndpoint {
		t.Errorf("expected %q, got %q", DefaultTestEndpoint, got)
	}
}

func TestResolveBaseURL_DefaultProd(t *testing.T) {
	cfg := &CliConfig{Env: "production"}
	if got := cfg.ResolveBaseURL(""); got != DefaultProdEndpoint {
		t.Errorf("expected %q, got %q", DefaultProdEndpoint, got)
	}
}

func TestResolveBaseURL_ProdAlias(t *testing.T) {
	cfg := &CliConfig{Env: "prod"}
	if got := cfg.ResolveBaseURL(""); got != DefaultProdEndpoint {
		t.Errorf("expected %q, got %q", DefaultProdEndpoint, got)
	}
}

func TestResolveBaseURL_CustomEndpoints(t *testing.T) {
	cfg := &CliConfig{
		Env:       "test",
		Endpoints: &Endpoints{Test: "https://custom-test.example.com", Prod: "https://custom-prod.example.com"},
	}
	if got := cfg.ResolveBaseURL(""); got != "https://custom-test.example.com" {
		t.Errorf("expected custom test URL, got %q", got)
	}
	if got := cfg.ResolveBaseURL("production"); got != "https://custom-prod.example.com" {
		t.Errorf("expected custom prod URL, got %q", got)
	}
}

func TestResolveBaseURL_EnvOverrideTakesPriority(t *testing.T) {
	cfg := &CliConfig{
		Env:             "test",
		Endpoints:       &Endpoints{Test: "https://custom-test.example.com"},
		resolvedBaseURL: "https://env-override.example.com",
	}
	if got := cfg.ResolveBaseURL("production"); got != "https://env-override.example.com" {
		t.Errorf("expected env override URL, got %q", got)
	}
}

func TestResolveBaseURL_FlagOverridesConfigEnv(t *testing.T) {
	cfg := &CliConfig{Env: "test"}
	if got := cfg.ResolveBaseURL("production"); got != DefaultProdEndpoint {
		t.Errorf("expected prod URL from flag override, got %q", got)
	}
}

func TestResolveBaseURL_EmptyEnvDefaultsToTest(t *testing.T) {
	cfg := &CliConfig{}
	if got := cfg.ResolveBaseURL(""); got != DefaultTestEndpoint {
		t.Errorf("expected test URL for empty env, got %q", got)
	}
}

// --- ResolveSignKey tests ---

type mockResolver struct {
	secrets map[string]string
}

func (m *mockResolver) Get(id string) (string, error) {
	v, ok := m.secrets[id]
	if !ok {
		return "", os.ErrNotExist
	}
	return v, nil
}

func TestResolveSignKey_PlainValue(t *testing.T) {
	cfg := &CliConfig{SignKey: SecretInput{Value: "plain-key-value"}}
	key, err := cfg.ResolveSignKey(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "plain-key-value" {
		t.Errorf("expected plain-key-value, got %q", key)
	}
}

func TestResolveSignKey_KeychainRef(t *testing.T) {
	cfg := &CliConfig{
		SignKey: SecretInput{Ref: &SecretRef{Source: "keychain", ID: "evo-cli:signkey:S001"}},
	}
	resolver := &mockResolver{secrets: map[string]string{"evo-cli:signkey:S001": "resolved-key"}}
	key, err := cfg.ResolveSignKey(resolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "resolved-key" {
		t.Errorf("expected resolved-key, got %q", key)
	}
}

func TestResolveSignKey_RefWithoutResolver(t *testing.T) {
	cfg := &CliConfig{
		SignKey: SecretInput{Ref: &SecretRef{Source: "keychain", ID: "some-id"}},
	}
	_, err := cfg.ResolveSignKey(nil)
	if err == nil {
		t.Fatal("expected error when resolver is nil")
	}
}

func TestResolveSignKey_Empty(t *testing.T) {
	cfg := &CliConfig{}
	_, err := cfg.ResolveSignKey(nil)
	if err == nil {
		t.Fatal("expected error for empty sign key")
	}
}

// --- MaskSecret tests ---

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"a", "*"},
		{"ab", "**"},
		{"abcd", "****"},
		{"abcde", "****...de"},
		{"64b59e70e15445196b1b5d2935f4e1bc", "****...bc"},
	}
	for _, tt := range tests {
		got := MaskSecret(tt.input)
		if got != tt.expected {
			t.Errorf("MaskSecret(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- ConfigMissingError tests ---

func TestConfigMissingError(t *testing.T) {
	err := &ConfigMissingError{Path: "/home/user/.evo-cli/config.json"}
	if err.Type() != "config_missing" {
		t.Errorf("expected type config_missing, got %q", err.Type())
	}
	if err.Hint() != "run: evo-cli config init" {
		t.Errorf("unexpected hint: %q", err.Hint())
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// =============================================================================
// Property-Based Tests (testing/quick)
// =============================================================================

// Feature: evo-payment-cli, Property 5: 环境变量优先级
// **Validates: Requirements 1.6, 1.7**
//
// For any random config values + random env var values, after LoadConfig with
// env vars set, the final value must equal the env var value for all 4 env vars:
// EVO_MERCHANT_SID, EVO_SIGN_KEY, EVO_SIGN_TYPE, EVO_API_BASE_URL.
func TestProperty5_EnvVarPriority(t *testing.T) {
	f := func(fileSid, fileKey, fileSignType, fileEnv string,
		envSid, envKey, envSignType, envBaseURL string) bool {
		// Skip empty env values — the property only applies when env vars are set.
		if envSid == "" || envKey == "" || envSignType == "" || envBaseURL == "" {
			return true
		}

		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")

		cfg := &CliConfig{
			MerchantSid: fileSid,
			SignKey:     SecretInput{Value: fileKey},
			SignType:    fileSignType,
			Env:         fileEnv,
		}
		if err := SaveConfig(cfg, path); err != nil {
			return true // skip if save fails (e.g. invalid JSON chars)
		}

		t.Setenv(EnvVarMerchantSid, envSid)
		t.Setenv(EnvVarSignKey, envKey)
		t.Setenv(EnvVarSignType, envSignType)
		t.Setenv(EnvVarBaseURL, envBaseURL)

		loaded, err := LoadConfig(path)
		if err != nil {
			return false
		}

		if loaded.MerchantSid != envSid {
			t.Logf("MerchantSid: got %q, want %q", loaded.MerchantSid, envSid)
			return false
		}
		if loaded.SignKey.Value != envKey {
			t.Logf("SignKey: got %q, want %q", loaded.SignKey.Value, envKey)
			return false
		}
		if loaded.SignType != envSignType {
			t.Logf("SignType: got %q, want %q", loaded.SignType, envSignType)
			return false
		}
		if loaded.ResolveBaseURL("") != envBaseURL {
			t.Logf("BaseURL: got %q, want %q", loaded.ResolveBaseURL(""), envBaseURL)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 5 failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 6: 配置缺失错误
// **Validates: Requirement 1.5**
//
// For any random non-existent path, LoadConfig returns ConfigMissingError
// with Type() == "config_missing".
func TestProperty6_ConfigMissingError(t *testing.T) {
	f := func(randomSuffix string) bool {
		// Build a path that definitely does not exist.
		path := filepath.Join(os.TempDir(), "nonexistent-evo-cli-test", randomSuffix, "config.json")

		_, err := LoadConfig(path)
		if err == nil {
			return false
		}

		cmErr, ok := err.(*ConfigMissingError)
		if !ok {
			t.Logf("expected ConfigMissingError, got %T: %v", err, err)
			return false
		}
		if cmErr.Type() != "config_missing" {
			t.Logf("expected type config_missing, got %q", cmErr.Type())
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 6 failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 7: SignKey 掩码显示
// **Validates: Requirement 1.3**
//
// For any random SignKey string, MaskSecret output must NOT contain the full
// plaintext. For keys > 4 chars, output must be "****..." + last 2 chars.
func TestProperty7_SignKeyMaskDisplay(t *testing.T) {
	f := func(key string) bool {
		if key == "" {
			// MaskSecret("") == "", which trivially does not contain plaintext.
			return MaskSecret(key) == ""
		}

		masked := MaskSecret(key)

		// The masked output must never equal the full plaintext.
		if masked == key {
			t.Logf("masked output equals plaintext for key %q", key)
			return false
		}

		if len(key) > 4 {
			expected := "****..." + key[len(key)-2:]
			if masked != expected {
				t.Logf("MaskSecret(%q) = %q, want %q", key, masked, expected)
				return false
			}
			// Must not contain the full plaintext.
			if strings.Contains(masked, key) {
				t.Logf("masked output contains full plaintext for key %q", key)
				return false
			}
		} else {
			// For keys <= 4 bytes, output should be all asterisks matching rune count.
			runeCount := 0
			for range key {
				runeCount++
			}
			if len(masked) != runeCount {
				return false
			}
			for _, c := range masked {
				if c != '*' {
					return false
				}
			}
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 7 failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 8: SignKey Keychain 存储
// **Validates: Requirements 1.2, 12.1**
//
// After storing a SignKey via a keychain ref, the config file's signKey field
// must be a keychain ref object {"source":"keychain","id":"..."}, never plaintext.
func TestProperty8_SignKeyKeychainStorage(t *testing.T) {
	f := func(merchantSid, keychainID string) bool {
		if keychainID == "" {
			return true // skip empty IDs
		}

		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")

		cfg := &CliConfig{
			MerchantSid: merchantSid,
			SignKey:     SecretInput{Ref: &SecretRef{Source: "keychain", ID: keychainID}},
			SignType:    "SHA256",
			Env:         "test",
		}
		if err := SaveConfig(cfg, path); err != nil {
			return true // skip if save fails
		}

		// Read raw JSON and verify signKey is a ref object, not a plain string.
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			return false
		}

		signKeyRaw := raw["signKey"]

		// Must NOT be a plain JSON string (starts with '"').
		// It must be a JSON object (starts with '{').
		trimmed := strings.TrimSpace(string(signKeyRaw))
		if len(trimmed) == 0 {
			return false
		}
		if trimmed[0] != '{' {
			t.Logf("signKey is not a JSON object: %s", trimmed)
			return false
		}

		// Parse and verify the ref fields.
		var ref SecretRef
		if err := json.Unmarshal(signKeyRaw, &ref); err != nil {
			return false
		}
		if ref.Source != "keychain" {
			t.Logf("expected source=keychain, got %q", ref.Source)
			return false
		}
		if ref.ID != keychainID {
			t.Logf("expected id=%q, got %q", keychainID, ref.ID)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 8 failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 23: 环境切换
// **Validates: Requirement 1.8**
//
// For env="test", ResolveBaseURL returns test endpoint.
// For env="production" or "prod", ResolveBaseURL returns prod endpoint.
// Custom endpoints override defaults.
func TestProperty23_EnvironmentSwitch(t *testing.T) {
	f := func(customTest, customProd string) bool {
		// --- Test environment with defaults ---
		cfg := &CliConfig{Env: "test"}
		if cfg.ResolveBaseURL("") != DefaultTestEndpoint {
			return false
		}

		// --- Production environment with defaults ---
		cfg = &CliConfig{Env: "production"}
		if cfg.ResolveBaseURL("") != DefaultProdEndpoint {
			return false
		}

		// --- "prod" alias ---
		cfg = &CliConfig{Env: "prod"}
		if cfg.ResolveBaseURL("") != DefaultProdEndpoint {
			return false
		}

		// --- Custom endpoints override defaults ---
		if customTest != "" && customProd != "" {
			cfg = &CliConfig{
				Env:       "test",
				Endpoints: &Endpoints{Test: customTest, Prod: customProd},
			}
			if cfg.ResolveBaseURL("") != customTest {
				t.Logf("custom test: got %q, want %q", cfg.ResolveBaseURL(""), customTest)
				return false
			}
			if cfg.ResolveBaseURL("production") != customProd {
				t.Logf("custom prod: got %q, want %q", cfg.ResolveBaseURL("production"), customProd)
				return false
			}
			if cfg.ResolveBaseURL("prod") != customProd {
				t.Logf("custom prod alias: got %q, want %q", cfg.ResolveBaseURL("prod"), customProd)
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 23 failed: %v", err)
	}
}

// --- Additional coverage tests ---

func TestIsRef_True(t *testing.T) {
	s := SecretInput{Ref: &SecretRef{Source: "keychain", ID: "test"}}
	if !s.IsRef() {
		t.Error("IsRef() should return true for ref")
	}
}

func TestIsRef_False(t *testing.T) {
	s := SecretInput{Value: "plain"}
	if s.IsRef() {
		t.Error("IsRef() should return false for plain value")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	p, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath() error: %v", err)
	}
	if !strings.Contains(p, ".evo-cli") || !strings.Contains(p, "config.json") {
		t.Errorf("unexpected path: %s", p)
	}
}

func TestSetResolvedBaseURL(t *testing.T) {
	cfg := &CliConfig{}
	cfg.SetResolvedBaseURL("https://custom.example.com")
	if got := cfg.ResolveBaseURL(""); got != "https://custom.example.com" {
		t.Errorf("ResolveBaseURL() = %q, want https://custom.example.com", got)
	}
}

func TestMaskSignKey_PlainValue(t *testing.T) {
	cfg := &CliConfig{SignKey: SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"}}
	got := cfg.MaskSignKey(nil)
	if got != "****...bc" {
		t.Errorf("MaskSignKey() = %q, want ****...bc", got)
	}
}

func TestMaskSignKey_EmptyKey(t *testing.T) {
	cfg := &CliConfig{}
	got := cfg.MaskSignKey(nil)
	if got != "****" {
		t.Errorf("MaskSignKey() = %q, want ****", got)
	}
}

func TestMaskSignKey_WithResolver(t *testing.T) {
	cfg := &CliConfig{
		SignKey: SecretInput{Ref: &SecretRef{Source: "keychain", ID: "test-id"}},
	}
	resolver := &mockResolver{secrets: map[string]string{"test-id": "abcdefghij"}}
	got := cfg.MaskSignKey(resolver)
	if got != "****...ij" {
		t.Errorf("MaskSignKey() = %q, want ****...ij", got)
	}
}

func TestLoadConfig_DefaultPath(t *testing.T) {
	// LoadConfig("") uses DefaultConfigPath — test that it returns ConfigMissingError
	// when the default path doesn't exist (which is the case in test env with temp HOME).
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	_, err := LoadConfig("")
	if err == nil {
		t.Fatal("expected error for missing default config")
	}
	if _, ok := err.(*ConfigMissingError); !ok {
		t.Errorf("expected ConfigMissingError, got %T", err)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte("{invalid json"), 0600)
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if strings.Contains(err.Error(), "config_missing") {
		t.Error("should be a parse error, not config_missing")
	}
}

func TestSaveConfig_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfg := &CliConfig{MerchantSid: "S001", Env: "test"}
	if err := SaveConfig(cfg, ""); err != nil {
		t.Fatalf("SaveConfig('') error: %v", err)
	}
	// Verify file was created at default path.
	p := filepath.Join(dir, ".evo-cli", "config.json")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Error("config file not created at default path")
	}
}

// --- ResolveLinkPayBaseURL tests ---

func TestResolveLinkPayBaseURL_DefaultTest(t *testing.T) {
	cfg := &CliConfig{Env: "test"}
	if got := cfg.ResolveLinkPayBaseURL(""); got != DefaultLinkPayTestEndpoint {
		t.Errorf("expected %q, got %q", DefaultLinkPayTestEndpoint, got)
	}
}

func TestResolveLinkPayBaseURL_DefaultProd(t *testing.T) {
	cfg := &CliConfig{Env: "production"}
	if got := cfg.ResolveLinkPayBaseURL(""); got != DefaultLinkPayProdEndpoint {
		t.Errorf("expected %q, got %q", DefaultLinkPayProdEndpoint, got)
	}
}

func TestResolveLinkPayBaseURL_ProdAlias(t *testing.T) {
	cfg := &CliConfig{Env: "prod"}
	if got := cfg.ResolveLinkPayBaseURL(""); got != DefaultLinkPayProdEndpoint {
		t.Errorf("expected %q, got %q", DefaultLinkPayProdEndpoint, got)
	}
}

func TestResolveLinkPayBaseURL_CustomEndpoints(t *testing.T) {
	cfg := &CliConfig{
		Env:              "test",
		LinkPayEndpoints: &Endpoints{Test: "https://custom-lp-test.example.com", Prod: "https://custom-lp-prod.example.com"},
	}
	if got := cfg.ResolveLinkPayBaseURL(""); got != "https://custom-lp-test.example.com" {
		t.Errorf("expected custom test URL, got %q", got)
	}
	if got := cfg.ResolveLinkPayBaseURL("production"); got != "https://custom-lp-prod.example.com" {
		t.Errorf("expected custom prod URL, got %q", got)
	}
}

func TestResolveLinkPayBaseURL_EnvOverrideTakesPriority(t *testing.T) {
	cfg := &CliConfig{
		Env:                    "test",
		LinkPayEndpoints:       &Endpoints{Test: "https://custom-lp-test.example.com"},
		resolvedLinkPayBaseURL: "https://env-lp-override.example.com",
	}
	if got := cfg.ResolveLinkPayBaseURL("production"); got != "https://env-lp-override.example.com" {
		t.Errorf("expected env override URL, got %q", got)
	}
}

func TestResolveLinkPayBaseURL_FlagOverridesConfigEnv(t *testing.T) {
	cfg := &CliConfig{Env: "test"}
	if got := cfg.ResolveLinkPayBaseURL("production"); got != DefaultLinkPayProdEndpoint {
		t.Errorf("expected prod URL from flag override, got %q", got)
	}
}

func TestResolveLinkPayBaseURL_EmptyEnvDefaultsToTest(t *testing.T) {
	cfg := &CliConfig{}
	if got := cfg.ResolveLinkPayBaseURL(""); got != DefaultLinkPayTestEndpoint {
		t.Errorf("expected test URL for empty env, got %q", got)
	}
}

func TestSetResolvedLinkPayBaseURL(t *testing.T) {
	cfg := &CliConfig{}
	cfg.SetResolvedLinkPayBaseURL("https://custom-lp.example.com")
	if got := cfg.ResolveLinkPayBaseURL(""); got != "https://custom-lp.example.com" {
		t.Errorf("ResolveLinkPayBaseURL() = %q, want https://custom-lp.example.com", got)
	}
}

func TestLoadConfig_LinkPayEnvOverride(t *testing.T) {
	// Clear env vars that would override config file values.
	t.Setenv(EnvVarSignKey, "")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &CliConfig{
		MerchantSid: "S_FILE",
		SignKey:     SecretInput{Value: "file-key"},
		SignType:    "SHA256",
		Env:         "test",
	}
	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("save error: %v", err)
	}

	t.Setenv(EnvVarLinkPayBaseURL, "https://lp-env.example.com")

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.ResolveLinkPayBaseURL("") != "https://lp-env.example.com" {
		t.Errorf("expected LinkPay env override URL, got %q", loaded.ResolveLinkPayBaseURL(""))
	}
}

func TestSaveConfig_WithLinkPayEndpoints(t *testing.T) {
	// Clear env vars that would override config file values.
	t.Setenv(EnvVarSignKey, "")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &CliConfig{
		MerchantSid:      "S999",
		SignKey:          SecretInput{Value: "test-key"},
		SignType:         "SHA256",
		Env:              "test",
		LinkPayEndpoints: &Endpoints{Test: "https://lp-test.example.com", Prod: "https://lp-prod.example.com"},
	}

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.LinkPayEndpoints == nil {
		t.Fatal("expected LinkPayEndpoints to be loaded")
	}
	if loaded.LinkPayEndpoints.Test != "https://lp-test.example.com" {
		t.Errorf("expected lp test endpoint, got %q", loaded.LinkPayEndpoints.Test)
	}
	if loaded.LinkPayEndpoints.Prod != "https://lp-prod.example.com" {
		t.Errorf("expected lp prod endpoint, got %q", loaded.LinkPayEndpoints.Prod)
	}
}

// --- IsLinkPayPath tests ---

func TestIsLinkPayPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/g2/v0/payment/mer/{sid}/evo.e-commerce.linkpay", true},
		{"/g2/v0/payment/mer/{sid}/evo.e-commerce.linkpay/ORDER001", true},
		{"/g2/v0/payment/mer/{sid}/evo.e-commerce.linkpayCancelorRefund/ORDER001", true},
		{"/g2/v0/payment/mer/{sid}/evo.e-commerce.linkpayRefund/ORDER001", true},
		{"/g2/v1/payment/mer/{sid}/payment", false},
		{"/g2/v1/payment/mer/{sid}/cryptogram", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsLinkPayPath(tt.path); got != tt.want {
				t.Errorf("IsLinkPayPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// --- ResolveBaseURLForPath tests ---

func TestResolveBaseURLForPath_LinkPayPath(t *testing.T) {
	cfg := &CliConfig{Env: "test"}
	path := "/g2/v0/payment/mer/S024116/evo.e-commerce.linkpay"
	got := cfg.ResolveBaseURLForPath(path)
	if got != DefaultLinkPayTestEndpoint {
		t.Errorf("ResolveBaseURLForPath(%q) = %q, want %q", path, got, DefaultLinkPayTestEndpoint)
	}
}

func TestResolveBaseURLForPath_NonLinkPayPath(t *testing.T) {
	cfg := &CliConfig{Env: "test"}
	path := "/g2/v1/payment/mer/S024116/payment"
	got := cfg.ResolveBaseURLForPath(path)
	if got != DefaultTestEndpoint {
		t.Errorf("ResolveBaseURLForPath(%q) = %q, want %q", path, got, DefaultTestEndpoint)
	}
}
