package signature

import (
	"regexp"
	"strings"
	"testing"
	"testing/quick"
)

// testBody is the exact body from API Rules.md example (4-space indentation).
const testBody = "{\n" +
	"    \"merchantTransInfo\": {\n" +
	"        \"merchantTransID\": \"e05b93cc849046a6b570ba144c328c7f\",\n" +
	"        \"merchantTransTime\": \"2021-12-31T08:30:59+08:00\"\n" +
	"    },\n" +
	"    \"storeInfo\": {\n" +
	"        \"mcc\": \"5411\"\n" +
	"    },\n" +
	"    \"transAmount\": {\n" +
	"        \"currency\": \"USD\",\n" +
	"        \"value\": \"10.00\"\n" +
	"    },\n" +
	"    \"validTime\": \"120\",\n" +
	"    \"returnURL\": \"https://YOUR_COMPANY.com/RETURNURL\",\n" +
	"    \"paymentMethod\": {\n" +
	"        \"type\": \"e-wallet\",\n" +
	"        \"e-wallet\": {\n" +
	"            \"paymentBrand\": \"Alipay\"\n" +
	"        }\n" +
	"    },\n" +
	"    \"transInitiator\": {\n" +
	"        \"platform\": \"WEB\"\n" +
	"    },\n" +
	"    \"tradeInfo\": {\n" +
	"        \"tradeType\": \"Sale of goods\",\n" +
	"        \"goodsName\": \"iPhone 13\",\n" +
	"        \"goodsDescription\": \"This is an iPhone 13\",\n" +
	"        \"totalQuantity\": \"2\"\n" +
	"    },\n" +
	"    \"webhook\": \"https://YOUR_COMPANY.com/WEBHOOK\",\n" +
	"    \"metadata\": \"This is a metadata\"\n" +
	"}"

const (
	testMethod   = "POST"
	testPath     = "/g2/v1/payment/mer/S024116/payment"
	testDateTime = "2021-12-31T08:30:59+08:00"
	testSignKey  = "64b59e70e15445196b1b5d2935f4e1bc"
	testMsgID    = "2d21a5715c034efb7e0aa383b885fc7a"
)

// hexPattern matches a lowercase hex string of any length.
var hexPattern = regexp.MustCompile(`^[0-9a-f]+$`)

// --- Test 1: Known test vectors from API Rules.md ---

func TestGenerateSignature_SHA256_KnownVector(t *testing.T) {
	sig, err := GenerateSignature(testMethod, testPath, testDateTime, testSignKey, testMsgID, testBody, "SHA256")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "41e4d284fce485523b62a20922ade75f92469c7eed742dfaa0d8e0b4f213f0ae"
	if sig != want {
		t.Errorf("SHA256 signature mismatch:\ngot:  %s\nwant: %s", sig, want)
	}
}

func TestGenerateSignature_HMACSHA256_KnownVector(t *testing.T) {
	sig, err := GenerateSignature(testMethod, testPath, testDateTime, testSignKey, testMsgID, testBody, "HMAC-SHA256")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "ef949039abf8ba97f82cb80afb2e595a0edccfea9c330ff39cc40d9cf1ec3e05"
	if sig != want {
		t.Errorf("HMAC-SHA256 signature mismatch:\ngot:  %s\nwant: %s", sig, want)
	}
}

// --- Test 2: Empty line omission behavior ---

func TestBuildSignString_EmptyLineOmission(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		dateTime string
		signKey  string
		msgID    string
		body     string
		want     string
	}{
		{
			name:     "all fields present",
			method:   "POST",
			path:     "/api/v1/test",
			dateTime: "2024-01-01T00:00:00+00:00",
			signKey:  "abcdef1234567890abcdef1234567890",
			msgID:    "msg123",
			body:     `{"key":"value"}`,
			want:     "POST\n/api/v1/test\n2024-01-01T00:00:00+00:00\nabcdef1234567890abcdef1234567890\nmsg123\n{\"key\":\"value\"}",
		},
		{
			name:     "empty body",
			method:   "GET",
			path:     "/api/v1/test",
			dateTime: "2024-01-01T00:00:00+00:00",
			signKey:  "abcdef1234567890abcdef1234567890",
			msgID:    "msg123",
			body:     "",
			want:     "GET\n/api/v1/test\n2024-01-01T00:00:00+00:00\nabcdef1234567890abcdef1234567890\nmsg123",
		},
		{
			name:     "empty path (notification scenario)",
			method:   "POST",
			path:     "",
			dateTime: "2024-01-01T00:00:00+00:00",
			signKey:  "abcdef1234567890abcdef1234567890",
			msgID:    "msg123",
			body:     `{"event":"test"}`,
			want:     "POST\n2024-01-01T00:00:00+00:00\nabcdef1234567890abcdef1234567890\nmsg123\n{\"event\":\"test\"}",
		},
		{
			name:     "multiple empty fields",
			method:   "POST",
			path:     "",
			dateTime: "2024-01-01T00:00:00+00:00",
			signKey:  "abcdef1234567890abcdef1234567890",
			msgID:    "",
			body:     "",
			want:     "POST\n2024-01-01T00:00:00+00:00\nabcdef1234567890abcdef1234567890",
		},
		{
			name:     "all empty",
			method:   "",
			path:     "",
			dateTime: "",
			signKey:  "",
			msgID:    "",
			body:     "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSignString(tt.method, tt.path, tt.dateTime, tt.signKey, tt.msgID, tt.body)
			if got != tt.want {
				t.Errorf("BuildSignString mismatch:\ngot:  %q\nwant: %q", got, tt.want)
			}
			// Verify no empty lines exist (no consecutive \n\n)
			if strings.Contains(got, "\n\n") {
				t.Errorf("sign string contains empty line (consecutive \\n)")
			}
		})
	}
}

// --- Test 3: Round-trip: GenerateSignature → VerifySignature returns true ---

func TestRoundTrip(t *testing.T) {
	signTypes := []string{"SHA256", "SHA512", "HMAC-SHA256", "HMAC-SHA512"}
	for _, st := range signTypes {
		t.Run(st, func(t *testing.T) {
			sig, err := GenerateSignature(testMethod, testPath, testDateTime, testSignKey, testMsgID, testBody, st)
			if err != nil {
				t.Fatalf("GenerateSignature error: %v", err)
			}
			if !VerifySignature(testMethod, testPath, testDateTime, testSignKey, testMsgID, testBody, st, sig) {
				t.Errorf("VerifySignature returned false for sign type %s with signature %s", st, sig)
			}
		})
	}
}

// --- Test 4: Case-insensitive verification ---

func TestVerifySignature_CaseInsensitive(t *testing.T) {
	sig, err := GenerateSignature(testMethod, testPath, testDateTime, testSignKey, testMsgID, testBody, "SHA256")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify with uppercase
	upper := strings.ToUpper(sig)
	if !VerifySignature(testMethod, testPath, testDateTime, testSignKey, testMsgID, testBody, "SHA256", upper) {
		t.Error("VerifySignature should accept uppercase signature")
	}

	// Verify with mixed case
	mixed := strings.ToUpper(sig[:16]) + sig[16:]
	if !VerifySignature(testMethod, testPath, testDateTime, testSignKey, testMsgID, testBody, "SHA256", mixed) {
		t.Error("VerifySignature should accept mixed-case signature")
	}

	// Verify wrong signature fails
	if VerifySignature(testMethod, testPath, testDateTime, testSignKey, testMsgID, testBody, "SHA256", "0000000000000000000000000000000000000000000000000000000000000000") {
		t.Error("VerifySignature should reject wrong signature")
	}
}

// --- Test 5: All 4 sign types produce valid hex output ---

func TestAllSignTypes_ValidHexOutput(t *testing.T) {
	signTypes := []string{"SHA256", "SHA512", "HMAC-SHA256", "HMAC-SHA512"}
	expectedLens := map[string]int{
		"SHA256":      64,  // 256 bits = 32 bytes = 64 hex chars
		"SHA512":      128, // 512 bits = 64 bytes = 128 hex chars
		"HMAC-SHA256": 64,
		"HMAC-SHA512": 128,
	}

	for _, st := range signTypes {
		t.Run(st, func(t *testing.T) {
			sig, err := GenerateSignature(testMethod, testPath, testDateTime, testSignKey, testMsgID, testBody, st)
			if err != nil {
				t.Fatalf("GenerateSignature error for %s: %v", st, err)
			}
			if !hexPattern.MatchString(sig) {
				t.Errorf("%s: signature is not lowercase hex: %s", st, sig)
			}
			if len(sig) != expectedLens[st] {
				t.Errorf("%s: expected length %d, got %d", st, expectedLens[st], len(sig))
			}
		})
	}
}

// --- Test: Unsupported sign type returns error ---

func TestGenerateSignature_UnsupportedSignType(t *testing.T) {
	_, err := GenerateSignature("POST", "/test", "2024-01-01T00:00:00Z", "key123", "msg1", "{}", "MD5")
	if err == nil {
		t.Error("expected error for unsupported sign type MD5")
	}
}

// --- Test: VerifySignature returns false for unsupported sign type ---

func TestVerifySignature_UnsupportedSignType(t *testing.T) {
	if VerifySignature("POST", "/test", "2024-01-01T00:00:00Z", "key123", "msg1", "{}", "MD5", "abc") {
		t.Error("VerifySignature should return false for unsupported sign type")
	}
}

// --- Property-Based Tests (testing/quick) ---

// Feature: evo-payment-cli, Property 1: 签名 Round-Trip
// For any random valid signature inputs and any of the 4 supported algorithms,
// GenerateSignature → VerifySignature must return true.
// **Validates: Requirements 2.1, 2.3, 2.8**
func TestProperty1_SignatureRoundTrip(t *testing.T) {
	signTypes := []string{"SHA256", "SHA512", "HMAC-SHA256", "HMAC-SHA512"}

	f := func(method, path, dateTime, signKey, msgID, body string, signTypeIdx uint8) bool {
		st := signTypes[int(signTypeIdx)%len(signTypes)]
		sig, err := GenerateSignature(method, path, dateTime, signKey, msgID, body, st)
		if err != nil {
			return false
		}
		return VerifySignature(method, path, dateTime, signKey, msgID, body, st, sig)
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 1 failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 2: 签名字符串空行省略
// For any random signature inputs where some fields may be empty,
// BuildSignString must not contain consecutive \n\n (no empty lines),
// and non-empty lines must maintain their relative order.
// **Validates: Requirement 2.2**
func TestProperty2_EmptyLineOmission(t *testing.T) {
	f := func(method, path, dateTime, signKey, msgID, body string) bool {
		result := BuildSignString(method, path, dateTime, signKey, msgID, body)

		// 1. No consecutive newlines (no empty lines)
		if strings.Contains(result, "\n\n") {
			return false
		}

		// 2. Non-empty input lines must appear in the result in their original relative order
		inputs := []string{method, path, dateTime, signKey, msgID, body}
		var nonEmpty []string
		for _, s := range inputs {
			if s != "" {
				nonEmpty = append(nonEmpty, s)
			}
		}

		if len(nonEmpty) == 0 {
			return result == ""
		}

		resultLines := strings.Split(result, "\n")
		if len(resultLines) != len(nonEmpty) {
			return false
		}
		for i, line := range resultLines {
			if line != nonEmpty[i] {
				return false
			}
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 2 failed: %v", err)
	}
}
