// Package doctor implements the `evo-cli doctor` health check command.
// It runs 4 sequential checks: config file existence, SignKey readability,
// signature computation verification, and API endpoint connectivity.
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/signature"
)

// CheckStatus represents the result of a single health check.
type CheckStatus string

const (
	StatusPass CheckStatus = "pass"
	StatusFail CheckStatus = "fail"
	StatusSkip CheckStatus = "skip"
)

// CheckResult holds the outcome of a single doctor check.
type CheckResult struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	Message string      `json:"message,omitempty"`
	Hint    string      `json:"hint,omitempty"`
}

// DoctorReport is the JSON envelope output by the doctor command.
type DoctorReport struct {
	OK     bool          `json:"ok"`
	Checks []CheckResult `json:"checks"`
}

// NewCmdDoctor creates the `evo-cli doctor` cobra command.
func NewCmdDoctor(f cmdutil.Factory) *cobra.Command {
	var offline bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run health checks on CLI configuration and connectivity",
		Long:  "Sequentially checks config file, SignKey, signature computation, and API connectivity.",
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := RunDoctor(f, offline)

			allPass := true
			for _, c := range checks {
				if c.Status == StatusFail {
					allPass = false
					break
				}
			}

			report := DoctorReport{
				OK:     allPass,
				Checks: checks,
			}

			enc := json.NewEncoder(f.IOStreams().Out)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			return enc.Encode(report)
		},
	}

	cmd.Flags().BoolVar(&offline, "offline", false, "Skip API endpoint connectivity check")
	return cmd
}

// RunDoctor executes the 4 sequential health checks and returns results.
func RunDoctor(f cmdutil.Factory, offline bool) []CheckResult {
	var checks []CheckResult

	// Check 1: Config file existence.
	cfg, cfgErr := checkConfig(f)
	checks = append(checks, cfg)

	// Check 2: SignKey readability (only if config loaded).
	signKey, keyResult := checkSignKey(f, cfgErr)
	checks = append(checks, keyResult)

	// Check 3: Signature computation verification.
	checks = append(checks, checkSignature(cfgErr, signKey))

	// Check 4: API endpoint connectivity.
	checks = append(checks, checkConnectivity(f, cfgErr, offline))

	return checks
}

// checkConfig verifies that the config file exists and is parseable.
// Returns the CheckResult and the config load error (nil on success).
func checkConfig(f cmdutil.Factory) (CheckResult, error) {
	_, err := f.Config()
	if err != nil {
		hint := "run: evo-cli config init"
		if _, ok := err.(*core.ConfigMissingError); !ok {
			hint = fmt.Sprintf("config file exists but failed to load: %v", err)
		}
		return CheckResult{
			Name:    "config_file",
			Status:  StatusFail,
			Message: err.Error(),
			Hint:    hint,
		}, err
	}
	return CheckResult{
		Name:    "config_file",
		Status:  StatusPass,
		Message: "config file loaded successfully",
	}, nil
}

// checkSignKey verifies that the SignKey can be resolved from keychain/file/env.
func checkSignKey(f cmdutil.Factory, cfgErr error) (string, CheckResult) {
	if cfgErr != nil {
		return "", CheckResult{
			Name:    "sign_key",
			Status:  StatusFail,
			Message: "skipped: config not available",
			Hint:    "fix config file first",
		}
	}

	cfg, _ := f.Config()

	// Use the factory's keychain as the resolver.
	df, ok := f.(*cmdutil.DefaultFactory)
	var resolver core.KeychainResolver
	if ok {
		resolver = df.Keychain()
	}

	key, err := cfg.ResolveSignKey(resolver)
	if err != nil {
		return "", CheckResult{
			Name:    "sign_key",
			Status:  StatusFail,
			Message: fmt.Sprintf("cannot resolve SignKey: %v", err),
			Hint:    "run: evo-cli config init --api-key-stdin, or set EVO_SIGN_KEY env var",
		}
	}
	if key == "" {
		return "", CheckResult{
			Name:    "sign_key",
			Status:  StatusFail,
			Message: "SignKey is empty",
			Hint:    "run: evo-cli config init --api-key-stdin, or set EVO_SIGN_KEY env var",
		}
	}

	return key, CheckResult{
		Name:    "sign_key",
		Status:  StatusPass,
		Message: fmt.Sprintf("SignKey resolved (%s)", core.MaskSecret(key)),
	}
}

// checkSignature verifies signature computation using known test inputs.
func checkSignature(cfgErr error, signKey string) CheckResult {
	if cfgErr != nil || signKey == "" {
		return CheckResult{
			Name:    "signature",
			Status:  StatusFail,
			Message: "skipped: SignKey not available",
			Hint:    "fix SignKey first",
		}
	}

	// Use known test inputs to generate and verify a signature.
	testMethod := "POST"
	testPath := "/g2/v1/payment/mer/TEST/payment"
	testDateTime := "2024-01-01T00:00:00+00:00"
	testMsgID := "00000000000000000000000000000001"
	testBody := `{"test":"doctor"}`
	testSignType := "SHA256"

	sig, err := signature.GenerateSignature(testMethod, testPath, testDateTime, signKey, testMsgID, testBody, testSignType)
	if err != nil {
		return CheckResult{
			Name:    "signature",
			Status:  StatusFail,
			Message: fmt.Sprintf("signature generation failed: %v", err),
			Hint:    "check signType in config (supported: SHA256, SHA512, HMAC-SHA256, HMAC-SHA512)",
		}
	}

	ok := signature.VerifySignature(testMethod, testPath, testDateTime, signKey, testMsgID, testBody, testSignType, sig)
	if !ok {
		return CheckResult{
			Name:    "signature",
			Status:  StatusFail,
			Message: "signature round-trip verification failed",
			Hint:    "possible internal error — report this issue",
		}
	}

	return CheckResult{
		Name:    "signature",
		Status:  StatusPass,
		Message: "signature generation and verification OK",
	}
}

// checkConnectivity verifies API endpoint reachability.
func checkConnectivity(f cmdutil.Factory, cfgErr error, offline bool) CheckResult {
	if offline {
		return CheckResult{
			Name:    "connectivity",
			Status:  StatusSkip,
			Message: "skipped (--offline)",
		}
	}

	if cfgErr != nil {
		return CheckResult{
			Name:    "connectivity",
			Status:  StatusFail,
			Message: "skipped: config not available",
			Hint:    "fix config file first",
		}
	}

	cfg, _ := f.Config()
	baseURL := cfg.ResolveBaseURL("")

	// Simple GET to the base URL with a short timeout.
	client := &http.Client{Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return CheckResult{
			Name:    "connectivity",
			Status:  StatusFail,
			Message: fmt.Sprintf("failed to create request: %v", err),
			Hint:    "check baseUrl in config",
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return CheckResult{
			Name:    "connectivity",
			Status:  StatusFail,
			Message: fmt.Sprintf("cannot reach %s: %v", baseURL, err),
			Hint:    "check network connection and baseUrl in config",
		}
	}
	defer resp.Body.Close()

	return CheckResult{
		Name:    "connectivity",
		Status:  StatusPass,
		Message: fmt.Sprintf("endpoint %s reachable (HTTP %d)", baseURL, resp.StatusCode),
	}
}
