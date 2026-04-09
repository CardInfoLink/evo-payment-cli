package output

import (
	"encoding/json"
	"io"
)

// Envelope is the success response wrapper written to stdout.
type Envelope struct {
	OK   bool        `json:"ok"`
	Data interface{} `json:"data,omitempty"`
	Meta *Meta       `json:"meta,omitempty"`
}

// ErrorEnvelope is the error response wrapper written to stderr.
type ErrorEnvelope struct {
	OK    bool       `json:"ok"`
	Error *ErrDetail `json:"error"`
}

// ErrDetail holds structured error information.
type ErrDetail struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// Meta holds optional metadata about the response.
type Meta struct {
	ActionRequired bool   `json:"actionRequired,omitempty"`
	BusinessStatus string `json:"businessStatus,omitempty"`
}

// WriteSuccess writes a success Envelope as JSON to the given writer (typically stdout).
// Returns the number of bytes written and any error.
func WriteSuccess(w io.Writer, data interface{}, meta *Meta) error {
	env := Envelope{
		OK:   true,
		Data: data,
		Meta: meta,
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(env)
}

// WriteError writes an ErrorEnvelope as JSON to the given writer (typically stderr).
// It automatically applies error enhancement to populate the hint field if empty.
// Returns the exit code that should be used for os.Exit.
func WriteError(w io.Writer, errType, code, message, hint string) int {
	// Auto-enhance hint if not already provided.
	if hint == "" {
		hint = EnhanceError(errType, code, message)
	}

	env := ErrorEnvelope{
		OK: false,
		Error: &ErrDetail{
			Type:    errType,
			Code:    code,
			Message: message,
			Hint:    hint,
		},
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(env)

	// Determine exit code from error type.
	return exitCodeFromErrorType(errType)
}

// exitCodeFromErrorType maps error.type to the corresponding exit code.
func exitCodeFromErrorType(errType string) int {
	switch errType {
	case "config_missing", "cli_error":
		return ExitCLIError
	case "validation":
		return ExitValidation
	case "signature_error", "signature_verification_failed":
		return ExitAuthError
	case "business", "resource":
		return ExitBusinessError
	case "psp_error", "issuer":
		return ExitPSPError
	case "http_error", "network", "internal_error":
		return ExitNetworkError
	default:
		return ExitCLIError
	}
}
