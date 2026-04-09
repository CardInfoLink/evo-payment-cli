package output

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"testing"
	"testing/quick"
)

func TestWriteSuccess(t *testing.T) {
	t.Run("with data and meta", func(t *testing.T) {
		var buf bytes.Buffer
		data := map[string]interface{}{"amount": "100.00", "currency": "USD"}
		meta := &Meta{ActionRequired: false, BusinessStatus: "Captured"}

		err := WriteSuccess(&buf, data, meta)
		if err != nil {
			t.Fatalf("WriteSuccess() error = %v", err)
		}

		var env Envelope
		if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if !env.OK {
			t.Error("expected ok=true")
		}
		if env.Data == nil {
			t.Error("expected data to be non-nil")
		}
		if env.Meta == nil {
			t.Fatal("expected meta to be non-nil")
		}
		if env.Meta.BusinessStatus != "Captured" {
			t.Errorf("meta.businessStatus = %q, want %q", env.Meta.BusinessStatus, "Captured")
		}
	})

	t.Run("with nil meta", func(t *testing.T) {
		var buf bytes.Buffer
		data := map[string]string{"status": "ok"}

		err := WriteSuccess(&buf, data, nil)
		if err != nil {
			t.Fatalf("WriteSuccess() error = %v", err)
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if _, exists := raw["meta"]; exists {
			t.Error("expected meta to be omitted when nil")
		}
	})

	t.Run("with nil data", func(t *testing.T) {
		var buf bytes.Buffer
		err := WriteSuccess(&buf, nil, nil)
		if err != nil {
			t.Fatalf("WriteSuccess() error = %v", err)
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if raw["ok"] != true {
			t.Error("expected ok=true")
		}
	})
}

func TestWriteError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		var buf bytes.Buffer
		exitCode := WriteError(&buf, "business", "B0013", "Amount exceeded", "")

		if exitCode != ExitBusinessError {
			t.Errorf("exit code = %d, want %d", exitCode, ExitBusinessError)
		}

		var env ErrorEnvelope
		if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if env.OK {
			t.Error("expected ok=false")
		}
		if env.Error == nil {
			t.Fatal("expected error to be non-nil")
		}
		if env.Error.Type != "business" {
			t.Errorf("error.type = %q, want %q", env.Error.Type, "business")
		}
		if env.Error.Code != "B0013" {
			t.Errorf("error.code = %q, want %q", env.Error.Code, "B0013")
		}
		if env.Error.Message != "Amount exceeded" {
			t.Errorf("error.message = %q, want %q", env.Error.Message, "Amount exceeded")
		}
	})

	t.Run("auto-enhanced hint for V0010", func(t *testing.T) {
		var buf bytes.Buffer
		exitCode := WriteError(&buf, "signature_error", "V0010", "Signature error", "")

		if exitCode != ExitAuthError {
			t.Errorf("exit code = %d, want %d", exitCode, ExitAuthError)
		}

		var env ErrorEnvelope
		if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if env.Error.Hint != "check sign key" {
			t.Errorf("error.hint = %q, want %q", env.Error.Hint, "check sign key")
		}
	})

	t.Run("auto-enhanced hint for config_missing", func(t *testing.T) {
		var buf bytes.Buffer
		exitCode := WriteError(&buf, "config_missing", "", "config file not found", "")

		if exitCode != ExitCLIError {
			t.Errorf("exit code = %d, want %d", exitCode, ExitCLIError)
		}

		var env ErrorEnvelope
		if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if env.Error.Hint != "run: evo-cli config init" {
			t.Errorf("error.hint = %q, want %q", env.Error.Hint, "run: evo-cli config init")
		}
	})

	t.Run("auto-enhanced hint for HTTP 503", func(t *testing.T) {
		var buf bytes.Buffer
		exitCode := WriteError(&buf, "http_error", "503", "Service Unavailable", "")

		if exitCode != ExitNetworkError {
			t.Errorf("exit code = %d, want %d", exitCode, ExitNetworkError)
		}

		var env ErrorEnvelope
		if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if env.Error.Hint != "retry with same idempotency key" {
			t.Errorf("error.hint = %q, want %q", env.Error.Hint, "retry with same idempotency key")
		}
	})

	t.Run("explicit hint not overridden", func(t *testing.T) {
		var buf bytes.Buffer
		WriteError(&buf, "signature_error", "V0010", "Signature error", "custom hint")

		var env ErrorEnvelope
		if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if env.Error.Hint != "custom hint" {
			t.Errorf("error.hint = %q, want %q", env.Error.Hint, "custom hint")
		}
	})

	t.Run("exit code mapping for all error types", func(t *testing.T) {
		cases := []struct {
			errType  string
			wantExit int
		}{
			{"config_missing", ExitCLIError},
			{"cli_error", ExitCLIError},
			{"validation", ExitValidation},
			{"signature_error", ExitAuthError},
			{"signature_verification_failed", ExitAuthError},
			{"business", ExitBusinessError},
			{"resource", ExitBusinessError},
			{"psp_error", ExitPSPError},
			{"issuer", ExitPSPError},
			{"http_error", ExitNetworkError},
			{"network", ExitNetworkError},
			{"internal_error", ExitNetworkError},
			{"unknown_type", ExitCLIError},
		}
		for _, c := range cases {
			var buf bytes.Buffer
			got := WriteError(&buf, c.errType, "", "test", "")
			if got != c.wantExit {
				t.Errorf("WriteError(%q) exit code = %d, want %d", c.errType, got, c.wantExit)
			}
		}
	})
}

func TestExitCodeFromErrorType(t *testing.T) {
	if got := exitCodeFromErrorType("validation"); got != ExitValidation {
		t.Errorf("exitCodeFromErrorType(validation) = %d, want %d", got, ExitValidation)
	}
	if got := exitCodeFromErrorType(""); got != ExitCLIError {
		t.Errorf("exitCodeFromErrorType('') = %d, want %d", got, ExitCLIError)
	}
}

// =============================================================================
// Property-Based Tests (testing/quick)
// =============================================================================

// Feature: evo-payment-cli, Property 10: Envelope 结构一致性
// **Validates: Requirements 7.1, 7.2**
//
// For any random success result, WriteSuccess stdout must be valid JSON with
// ok=true and data present.
// For any random failure result, WriteError stderr must be valid JSON with
// ok=false and error containing type and message.
func TestProperty10_EnvelopeStructureConsistency(t *testing.T) {
	errTypes := []string{"business", "validation", "http_error", "signature_error",
		"config_missing", "cli_error", "psp_error", "issuer", "network", "internal_error", "resource"}

	f := func(seed uint32) bool {
		rng := rand.New(rand.NewSource(int64(seed)))

		// --- Success path ---
		{
			var buf bytes.Buffer
			data := map[string]interface{}{
				"amount":   rng.Intn(100000),
				"currency": "USD",
			}
			var meta *Meta
			if rng.Intn(2) == 0 {
				meta = &Meta{
					ActionRequired: rng.Intn(2) == 1,
					BusinessStatus: "Captured",
				}
			}

			if err := WriteSuccess(&buf, data, meta); err != nil {
				t.Logf("WriteSuccess error: %v", err)
				return false
			}

			var raw map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
				t.Logf("success output is not valid JSON: %v", err)
				return false
			}

			// ok must be true.
			ok, exists := raw["ok"]
			if !exists {
				t.Log("success output missing 'ok' field")
				return false
			}
			if ok != true {
				t.Logf("success output ok=%v, want true", ok)
				return false
			}

			// data must be present.
			if _, exists := raw["data"]; !exists {
				t.Log("success output missing 'data' field")
				return false
			}
		}

		// --- Error path ---
		{
			var buf bytes.Buffer
			errType := errTypes[rng.Intn(len(errTypes))]
			code := "TEST001"
			message := "random error message"

			WriteError(&buf, errType, code, message, "")

			var raw map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
				t.Logf("error output is not valid JSON: %v", err)
				return false
			}

			// ok must be false.
			ok, exists := raw["ok"]
			if !exists {
				t.Log("error output missing 'ok' field")
				return false
			}
			if ok != false {
				t.Logf("error output ok=%v, want false", ok)
				return false
			}

			// error must be present with type and message.
			errObj, exists := raw["error"]
			if !exists {
				t.Log("error output missing 'error' field")
				return false
			}
			errMap, ok2 := errObj.(map[string]interface{})
			if !ok2 {
				t.Log("error field is not an object")
				return false
			}
			if _, hasType := errMap["type"]; !hasType {
				t.Log("error object missing 'type' field")
				return false
			}
			if _, hasMsg := errMap["message"]; !hasMsg {
				t.Log("error object missing 'message' field")
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 10 failed: %v", err)
	}
}
