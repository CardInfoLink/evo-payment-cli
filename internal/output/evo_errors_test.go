package output

import (
	"fmt"
	"math/rand"
	"testing"
	"testing/quick"
)

func TestClassifyResultCode(t *testing.T) {
	tests := []struct {
		code     string
		wantExit int
		wantType string
	}{
		// S prefix → success
		{"S0000", ExitSuccess, ""},
		{"S0003", ExitSuccess, ""},

		// V0010 special case → auth error
		{"V0010", ExitAuthError, "signature_error"},

		// V prefix (non-V0010) → validation
		{"V0000", ExitValidation, "validation"},
		{"V0001", ExitValidation, "validation"},

		// B prefix → business error
		{"B0012", ExitBusinessError, "business"},
		{"B0013", ExitBusinessError, "business"},

		// C prefix → resource error
		{"C0004", ExitBusinessError, "resource"},
		{"C0009", ExitBusinessError, "resource"},

		// P prefix → PSP error
		{"P0000", ExitPSPError, "psp_error"},
		{"P0098", ExitPSPError, "psp_error"},

		// I prefix → issuer error
		{"I0051", ExitPSPError, "issuer"},
		{"I0054", ExitPSPError, "issuer"},

		// E prefix → internal/network error
		{"E0000", ExitNetworkError, "internal_error"},
		{"E0004", ExitNetworkError, "internal_error"},

		// Empty code → CLI error
		{"", ExitCLIError, "cli_error"},

		// Unknown prefix → CLI error
		{"X9999", ExitCLIError, "cli_error"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			gotExit, gotType := ClassifyResultCode(tt.code)
			if gotExit != tt.wantExit {
				t.Errorf("ClassifyResultCode(%q) exitCode = %d, want %d", tt.code, gotExit, tt.wantExit)
			}
			if gotType != tt.wantType {
				t.Errorf("ClassifyResultCode(%q) errType = %q, want %q", tt.code, gotType, tt.wantType)
			}
		})
	}
}

func TestEnhanceError(t *testing.T) {
	tests := []struct {
		name    string
		errType string
		code    string
		message string
		want    string
	}{
		{
			name:    "V0010 signature error by code",
			errType: "signature_error",
			code:    "V0010",
			message: "Signature error",
			want:    "check sign key",
		},
		{
			name:    "signature_error type without V0010 code",
			errType: "signature_error",
			code:    "",
			message: "bad signature",
			want:    "check sign key",
		},
		{
			name:    "config_missing",
			errType: "config_missing",
			code:    "",
			message: "config file not found",
			want:    "run: evo-cli config init",
		},
		{
			name:    "HTTP 503",
			errType: "http_error",
			code:    "503",
			message: "Service Unavailable",
			want:    "retry with same idempotency key",
		},
		{
			name:    "HTTP 500 no special hint",
			errType: "http_error",
			code:    "500",
			message: "Internal Server Error",
			want:    "",
		},
		{
			name:    "business error no special hint",
			errType: "business",
			code:    "B0013",
			message: "Amount exceeded",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnhanceError(tt.errType, tt.code, tt.message)
			if got != tt.want {
				t.Errorf("EnhanceError(%q, %q, %q) = %q, want %q",
					tt.errType, tt.code, tt.message, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Property-Based Tests (testing/quick)
// =============================================================================

// Feature: evo-payment-cli, Property 9: result.code 分类映射
// **Validates: Requirements 7.3, 7.4**
//
// For any random result.code with a known prefix (S/V/B/C/P/I/E),
// ClassifyResultCode must return the correct exit code and error type:
//
//	S → ExitSuccess(0), ""
//	V0010 → ExitAuthError(3), "signature_error"
//	V (non-V0010) → ExitValidation(2), "validation"
//	B → ExitBusinessError(1), "business"
//	C → ExitBusinessError(1), "resource"
//	P → ExitPSPError(6), "psp_error"
//	I → ExitPSPError(6), "issuer"
//	E → ExitNetworkError(4), "internal_error"
func TestProperty9_ResultCodeClassification(t *testing.T) {
	// Expected mapping for each prefix.
	type expected struct {
		exitCode int
		errType  string
	}
	prefixMap := map[string]expected{
		"S": {ExitSuccess, ""},
		"V": {ExitValidation, "validation"},
		"B": {ExitBusinessError, "business"},
		"C": {ExitBusinessError, "resource"},
		"P": {ExitPSPError, "psp_error"},
		"I": {ExitPSPError, "issuer"},
		"E": {ExitNetworkError, "internal_error"},
	}
	prefixes := []string{"S", "V", "B", "C", "P", "I", "E"}

	f := func(seed uint32) bool {
		rng := rand.New(rand.NewSource(int64(seed)))

		// Pick a random prefix.
		prefix := prefixes[rng.Intn(len(prefixes))]

		// Generate a random 4-digit suffix.
		suffix := fmt.Sprintf("%04d", rng.Intn(10000))
		code := prefix + suffix

		gotExit, gotType := ClassifyResultCode(code)

		// Special case: V0010 → signature_error.
		if code == "V0010" {
			if gotExit != ExitAuthError {
				t.Logf("V0010: exitCode=%d, want %d", gotExit, ExitAuthError)
				return false
			}
			if gotType != "signature_error" {
				t.Logf("V0010: errType=%q, want %q", gotType, "signature_error")
				return false
			}
			return true
		}

		want := prefixMap[prefix]
		if gotExit != want.exitCode {
			t.Logf("code=%q: exitCode=%d, want %d", code, gotExit, want.exitCode)
			return false
		}
		if gotType != want.errType {
			t.Logf("code=%q: errType=%q, want %q", code, gotType, want.errType)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 9 failed: %v", err)
	}
}
