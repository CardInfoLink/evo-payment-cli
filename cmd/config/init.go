package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/keychain"
)

// Valid sign types accepted by --sign-type flag.
var validSignTypes = []string{"SHA256", "SHA512", "HMAC-SHA256", "HMAC-SHA512"}

// Valid environments accepted by --env flag.
var validEnvs = []string{"test", "production"}

// NewCmdConfigInit creates the "config init" command.
func NewCmdConfigInit(f cmdutil.Factory) *cobra.Command {
	var (
		merchantSid    string
		signType       string
		env            string
		baseURL        string
		linkPayBaseURL string
		apiKeyStdin    bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize evo-cli configuration",
		Long: `Create ~/.evo-cli/config.json with merchant credentials.

SignKey is read from stdin when --api-key-stdin is set (recommended).
The key is stored securely in the OS keychain, never as plaintext in the config file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigInit(f, merchantSid, signType, env, baseURL, linkPayBaseURL, apiKeyStdin)
		},
	}

	cmd.Flags().StringVar(&merchantSid, "merchant-sid", "", "Merchant store identifier (required)")
	cmd.Flags().StringVar(&signType, "sign-type", "SHA256", "Signature algorithm: SHA256|SHA512|HMAC-SHA256|HMAC-SHA512")
	cmd.Flags().StringVar(&env, "env", "test", "Environment: test|production")
	cmd.Flags().StringVar(&baseURL, "api-base-url", "", "Custom API base URL (overrides default for the selected env)")
	cmd.Flags().StringVar(&linkPayBaseURL, "linkpay-base-url", "", "Custom LinkPay API base URL")
	cmd.Flags().BoolVar(&apiKeyStdin, "api-key-stdin", false, "Read SignKey from stdin")

	_ = cmd.MarkFlagRequired("merchant-sid")

	return cmd
}

func runConfigInit(f cmdutil.Factory, merchantSid, signType, env, baseURL, linkPayBaseURL string, apiKeyStdin bool) error {
	io := f.IOStreams()

	// Validate sign type.
	if !isValidSignType(signType) {
		return writeInitError(io, fmt.Sprintf("invalid sign-type %q, must be one of: %s", signType, strings.Join(validSignTypes, ", ")))
	}

	// Validate environment.
	if !isValidEnv(env) {
		return writeInitError(io, fmt.Sprintf("invalid env %q, must be one of: %s", env, strings.Join(validEnvs, ", ")))
	}

	cfg := &core.CliConfig{
		MerchantSid: merchantSid,
		SignType:    signType,
		Env:         env,
	}

	// Store custom base URL in the endpoints field for the selected env.
	if baseURL != "" {
		ep := &core.Endpoints{}
		if env == "production" {
			ep.Prod = baseURL
		} else {
			ep.Test = baseURL
		}
		cfg.Endpoints = ep
	}

	// Store custom LinkPay base URL in the linkPayEndpoints field for the selected env.
	if linkPayBaseURL != "" {
		ep := &core.Endpoints{}
		if env == "production" {
			ep.Prod = linkPayBaseURL
		} else {
			ep.Test = linkPayBaseURL
		}
		cfg.LinkPayEndpoints = ep
	}

	// Read SignKey from stdin if requested.
	if apiKeyStdin {
		scanner := bufio.NewScanner(io.In)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return writeInitError(io, fmt.Sprintf("failed to read SignKey from stdin: %v", err))
			}
			return writeInitError(io, "no SignKey provided on stdin")
		}
		signKey := strings.TrimSpace(scanner.Text())
		if signKey == "" {
			return writeInitError(io, "SignKey cannot be empty")
		}

		// Store in keychain.
		keychainID := fmt.Sprintf("evo-cli:signkey:%s", merchantSid)
		kc := keychain.New()
		if err := kc.Set(keychainID, signKey); err != nil {
			return writeInitError(io, fmt.Sprintf("failed to store SignKey in keychain: %v", err))
		}

		cfg.SignKey = core.SecretInput{
			Ref: &core.SecretRef{
				Source: "keychain",
				ID:     keychainID,
			},
		}
	}

	// Save config file.
	if err := core.SaveConfig(cfg, ""); err != nil {
		return writeInitError(io, fmt.Sprintf("failed to save config: %v", err))
	}

	// Output success envelope.
	resolvedURL := cfg.ResolveBaseURL("")
	resolvedLinkPayURL := cfg.ResolveLinkPayBaseURL("")
	envelope := map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"message":        "Configuration initialized successfully",
			"merchantSid":    merchantSid,
			"signType":       signType,
			"env":            env,
			"baseUrl":        resolvedURL,
			"linkPayBaseUrl": resolvedLinkPayURL,
		},
	}
	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

func writeInitError(io *cmdutil.IOStreams, msg string) error {
	envelope := map[string]interface{}{
		"ok": false,
		"error": map[string]interface{}{
			"type":    "validation",
			"message": msg,
		},
	}
	enc := json.NewEncoder(io.ErrOut)
	enc.SetIndent("", "  ")
	_ = enc.Encode(envelope)
	return fmt.Errorf("%s", msg)
}

func isValidSignType(s string) bool {
	for _, v := range validSignTypes {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

func isValidEnv(e string) bool {
	for _, v := range validEnvs {
		if strings.EqualFold(e, v) {
			return true
		}
	}
	return false
}
