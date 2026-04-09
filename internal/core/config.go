// Package core provides configuration management for evo-cli.
package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Default endpoint URLs for each environment.
const (
	DefaultTestEndpoint        = "https://hkg-online-uat.everonet.com"
	DefaultProdEndpoint        = "https://hkg-online.everonet.com"
	DefaultLinkPayTestEndpoint = "https://hkg-counter-uat.everonet.com"
	DefaultLinkPayProdEndpoint = "https://hkg-counter.everonet.com"
	DefaultConfigDir           = ".evo-cli"
	DefaultConfigFile          = "config.json"
)

// Environment constants.
const (
	EnvTest       = "test"
	EnvProduction = "production"
)

// Environment variable names that override config file values.
const (
	EnvVarMerchantSid    = "EVO_MERCHANT_SID"
	EnvVarSignKey        = "EVO_SIGN_KEY"
	EnvVarSignType       = "EVO_SIGN_TYPE"
	EnvVarBaseURL        = "EVO_API_BASE_URL"
	EnvVarLinkPayBaseURL = "EVO_LINKPAY_BASE_URL"
)

// CliConfig holds the CLI configuration loaded from ~/.evo-cli/config.json.
type CliConfig struct {
	MerchantSid      string      `json:"merchantSid"`
	SignKey          SecretInput `json:"signKey"`
	SignType         string      `json:"signType"`
	KeyID            string      `json:"keyID,omitempty"`
	Env              string      `json:"env"`
	Endpoints        *Endpoints  `json:"endpoints,omitempty"`
	LinkPayEndpoints *Endpoints  `json:"linkPayEndpoints,omitempty"`

	// resolvedBaseURL is set when EVO_API_BASE_URL env var overrides the endpoint.
	resolvedBaseURL string
	// resolvedLinkPayBaseURL is set when EVO_LINKPAY_BASE_URL env var overrides the endpoint.
	resolvedLinkPayBaseURL string
}

// Endpoints holds per-environment API base URLs.
type Endpoints struct {
	Test string `json:"test"`
	Prod string `json:"prod"`
}

// SecretInput is a union type: either a plain string value or a keychain/file reference.
// JSON representation:
//   - Plain string: "my-secret-key"
//   - Reference: {"source":"keychain","id":"evo-cli:signkey:S024116"}
type SecretInput struct {
	Value string     `json:"-"`
	Ref   *SecretRef `json:"-"`
}

// SecretRef points to a secret stored in keychain or an encrypted file.
type SecretRef struct {
	Source string `json:"source"` // "keychain" or "file"
	ID     string `json:"id"`
}

// MarshalJSON implements json.Marshaler for the union type.
// Plain string values serialize as a JSON string.
// References serialize as {"source":"...","id":"..."}.
func (s SecretInput) MarshalJSON() ([]byte, error) {
	if s.Ref != nil {
		return json.Marshal(s.Ref)
	}
	return json.Marshal(s.Value)
}

// UnmarshalJSON implements json.Unmarshaler for the union type.
// Accepts either a JSON string (plain value) or an object with source/id fields.
func (s *SecretInput) UnmarshalJSON(data []byte) error {
	// Try plain string first.
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		s.Value = str
		s.Ref = nil
		return nil
	}

	// Try object (SecretRef).
	var ref SecretRef
	if err := json.Unmarshal(data, &ref); err == nil && ref.Source != "" {
		s.Ref = &ref
		s.Value = ""
		return nil
	}

	return fmt.Errorf("signKey must be a string or {\"source\":\"...\",\"id\":\"...\"}")
}

// IsRef returns true if the SecretInput is a keychain/file reference.
func (s SecretInput) IsRef() bool {
	return s.Ref != nil
}

// ConfigMissingError is returned when the config file does not exist.
type ConfigMissingError struct {
	Path string
}

func (e *ConfigMissingError) Error() string {
	return fmt.Sprintf("config file not found: %s", e.Path)
}

// Type returns the structured error type for JSON output.
func (e *ConfigMissingError) Type() string {
	return "config_missing"
}

// Hint returns an actionable fix suggestion.
func (e *ConfigMissingError) Hint() string {
	return "run: evo-cli config init"
}

// DefaultConfigPath returns the default config file path: ~/.evo-cli/config.json.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, DefaultConfigDir, DefaultConfigFile), nil
}

// LoadConfig loads the CLI configuration from the given path.
// If path is empty, it uses the default path (~/.evo-cli/config.json).
// After loading from file, environment variables are applied as overrides.
func LoadConfig(path string) (*CliConfig, error) {
	if path == "" {
		p, err := DefaultConfigPath()
		if err != nil {
			return nil, err
		}
		path = p
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ConfigMissingError{Path: path}
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg CliConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides.
	applyEnvOverrides(&cfg)

	return &cfg, nil
}

// SaveConfig writes the CLI configuration to the given path.
// If path is empty, it uses the default path (~/.evo-cli/config.json).
// The parent directory is created if it does not exist.
func SaveConfig(cfg *CliConfig, path string) error {
	if path == "" {
		p, err := DefaultConfigPath()
		if err != nil {
			return err
		}
		path = p
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides to the config.
// Environment variables take precedence over config file values.
func applyEnvOverrides(cfg *CliConfig) {
	if v := os.Getenv(EnvVarMerchantSid); v != "" {
		cfg.MerchantSid = v
	}
	if v := os.Getenv(EnvVarSignKey); v != "" {
		// Env var sign key is always a plain value, overrides any ref.
		cfg.SignKey = SecretInput{Value: v}
	}
	if v := os.Getenv(EnvVarSignType); v != "" {
		cfg.SignType = v
	}
	if v := os.Getenv(EnvVarBaseURL); v != "" {
		cfg.resolvedBaseURL = v
	}
	if v := os.Getenv(EnvVarLinkPayBaseURL); v != "" {
		cfg.resolvedLinkPayBaseURL = v
	}
}

// ResolveSignKey resolves the actual sign key string from the SecretInput.
// Resolution order:
//  1. If the value is a plain string, return it directly.
//  2. If the value is a keychain/file reference, attempt to resolve it.
//
// For keychain references, a KeychainResolver must be provided.
// If resolver is nil and the input is a ref, an error is returned.
func (c *CliConfig) ResolveSignKey(resolver KeychainResolver) (string, error) {
	// Plain value (including env var override).
	if c.SignKey.Value != "" {
		return c.SignKey.Value, nil
	}

	// Keychain/file reference.
	if c.SignKey.Ref != nil {
		if resolver == nil {
			return "", fmt.Errorf("sign key is a %s reference but no resolver is available", c.SignKey.Ref.Source)
		}
		return resolver.Get(c.SignKey.Ref.ID)
	}

	return "", fmt.Errorf("sign key is not configured")
}

// KeychainResolver is the interface for resolving secrets from keychain/file storage.
type KeychainResolver interface {
	Get(id string) (string, error)
}

// ResolveBaseURL returns the correct endpoint URL based on the environment.
// Resolution order:
//  1. EVO_API_BASE_URL env var (stored in resolvedBaseURL during load).
//  2. Custom endpoints from config file (endpoints.test or endpoints.prod).
//  3. Default endpoints.
//
// The envOverride parameter allows --env flag to override the config env field.
func (c *CliConfig) ResolveBaseURL(envOverride string) string {
	// EVO_API_BASE_URL env var takes highest priority.
	if c.resolvedBaseURL != "" {
		return c.resolvedBaseURL
	}

	env := c.Env
	if envOverride != "" {
		env = envOverride
	}

	// Normalize: accept "prod" as alias for "production".
	if env == "prod" {
		env = EnvProduction
	}

	// Default to test if not specified.
	if env == "" {
		env = EnvTest
	}

	switch env {
	case EnvProduction:
		if c.Endpoints != nil && c.Endpoints.Prod != "" {
			return c.Endpoints.Prod
		}
		return DefaultProdEndpoint
	default: // "test" or any unrecognized value defaults to test
		if c.Endpoints != nil && c.Endpoints.Test != "" {
			return c.Endpoints.Test
		}
		return DefaultTestEndpoint
	}
}

// SetResolvedBaseURL overrides the resolved base URL (useful for testing).
func (c *CliConfig) SetResolvedBaseURL(url string) {
	c.resolvedBaseURL = url
}

// ResolveLinkPayBaseURL returns the correct LinkPay endpoint URL based on the environment.
// Resolution order:
//  1. EVO_LINKPAY_BASE_URL env var (stored in resolvedLinkPayBaseURL during load).
//  2. Custom linkPayEndpoints from config file (linkPayEndpoints.test or linkPayEndpoints.prod).
//  3. Default LinkPay endpoints.
//
// The envOverride parameter allows --env flag to override the config env field.
func (c *CliConfig) ResolveLinkPayBaseURL(envOverride string) string {
	// EVO_LINKPAY_BASE_URL env var takes highest priority.
	if c.resolvedLinkPayBaseURL != "" {
		return c.resolvedLinkPayBaseURL
	}

	env := c.Env
	if envOverride != "" {
		env = envOverride
	}

	// Normalize: accept "prod" as alias for "production".
	if env == "prod" {
		env = EnvProduction
	}

	// Default to test if not specified.
	if env == "" {
		env = EnvTest
	}

	switch env {
	case EnvProduction:
		if c.LinkPayEndpoints != nil && c.LinkPayEndpoints.Prod != "" {
			return c.LinkPayEndpoints.Prod
		}
		return DefaultLinkPayProdEndpoint
	default: // "test" or any unrecognized value defaults to test
		if c.LinkPayEndpoints != nil && c.LinkPayEndpoints.Test != "" {
			return c.LinkPayEndpoints.Test
		}
		return DefaultLinkPayTestEndpoint
	}
}

// SetResolvedLinkPayBaseURL overrides the resolved LinkPay base URL (useful for testing).
func (c *CliConfig) SetResolvedLinkPayBaseURL(url string) {
	c.resolvedLinkPayBaseURL = url
}

// IsLinkPayPath returns true if the API path is a LinkPay endpoint
// that uses the separate LinkPay base URL.
// All paths containing "evo.e-commerce.linkpay" (create, query, cancelOrRefund, refund, refundQuery)
// use the LinkPay base URL.
func IsLinkPayPath(path string) bool {
	return strings.Contains(path, "evo.e-commerce.linkpay")
}

// ResolveBaseURLForPath returns the appropriate base URL for the given API path.
// LinkPay endpoints use a separate base URL from the main payment API.
func (c *CliConfig) ResolveBaseURLForPath(path string) string {
	if IsLinkPayPath(path) {
		return c.ResolveLinkPayBaseURL("")
	}
	return c.ResolveBaseURL("")
}

// For keys longer than 4 characters, shows "****..." + last 2 chars.
// For shorter keys, shows all asterisks.
func (c *CliConfig) MaskSignKey(resolver KeychainResolver) string {
	key, err := c.ResolveSignKey(resolver)
	if err != nil || key == "" {
		return "****"
	}
	return MaskSecret(key)
}

// MaskSecret masks a secret string for display.
func MaskSecret(s string) string {
	if len(s) <= 4 {
		masked := ""
		for range s {
			masked += "*"
		}
		return masked
	}
	return "****..." + s[len(s)-2:]
}
