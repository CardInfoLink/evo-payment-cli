package output

import "strings"

// ClassifyResultCode maps an Evo Payment result.code to an exit code and error type.
// The mapping is based on the result.code prefix:
//
//	S prefix → ExitSuccess (0), ""
//	V0010    → ExitAuthError (3), "signature_error"
//	V prefix → ExitValidation (2), "validation"
//	B prefix → ExitBusinessError (1), "business"
//	C prefix → ExitBusinessError (1), "resource"
//	P prefix → ExitPSPError (6), "psp_error"
//	I prefix → ExitPSPError (6), "issuer"
//	E prefix → ExitNetworkError (4), "internal_error"
//
// Unknown codes default to ExitCLIError (5), "cli_error".
func ClassifyResultCode(code string) (exitCode int, errType string) {
	if code == "" {
		return ExitCLIError, "cli_error"
	}

	// Special case: V0010 is a signature/auth error, not a validation error.
	if code == "V0010" {
		return ExitAuthError, "signature_error"
	}

	prefix := strings.ToUpper(code[:1])
	switch prefix {
	case "S":
		return ExitSuccess, ""
	case "V":
		return ExitValidation, "validation"
	case "B":
		return ExitBusinessError, "business"
	case "C":
		return ExitBusinessError, "resource"
	case "P":
		return ExitPSPError, "psp_error"
	case "I":
		return ExitPSPError, "issuer"
	case "E":
		return ExitNetworkError, "internal_error"
	default:
		return ExitCLIError, "cli_error"
	}
}

// EnhanceError returns an actionable hint based on the error type, result code,
// and message. Returns an empty string if no specific hint applies.
//
// Enhancement rules:
//   - V0010 (signature error) → "check sign key"
//   - config_missing → "run: evo-cli config init"
//   - HTTP 503 → "retry with same idempotency key"
func EnhanceError(errType, code, message string) string {
	// Signature error (V0010).
	if code == "V0010" || errType == "signature_error" {
		return "check sign key"
	}

	// Config missing.
	if errType == "config_missing" {
		return "run: evo-cli config init"
	}

	// HTTP 503 Service Unavailable.
	if errType == "http_error" && code == "503" {
		return "retry with same idempotency key"
	}

	return ""
}
