package validate

import (
	"math/rand"
	"strings"
	"testing"
	"testing/quick"
	"unicode"
)

// ============================================================
// Unit Tests — ValidatePath
// ============================================================

func TestValidatePath_RejectsTraversal(t *testing.T) {
	bad := []string{
		"../etc/passwd",
		"foo/../bar",
		"foo/bar/..",
		`foo\..\bar`,
		"..",
	}
	for _, p := range bad {
		if err := ValidatePath(p); err == nil {
			t.Errorf("ValidatePath(%q) should reject path traversal", p)
		}
	}
}

func TestValidatePath_AcceptsLegitimate(t *testing.T) {
	good := []string{
		"foo/bar",
		"output.json",
		"./relative/path",
		"some.file.with.dots",
		"a/b/c/d",
	}
	for _, p := range good {
		if err := ValidatePath(p); err != nil {
			t.Errorf("ValidatePath(%q) should accept: %v", p, err)
		}
	}
}

// ============================================================
// Unit Tests — SanitizeInput
// ============================================================

func TestSanitizeInput_StripsANSI(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[1;32mbold green\x1b[0m", "bold green"},
		{"no escape", "no escape"},
		{"hello\x1b]0;title\x07world", "helloworld"},
	}
	for _, tc := range cases {
		got := SanitizeInput(tc.in)
		if got != tc.want {
			t.Errorf("SanitizeInput(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSanitizeInput_StripsControlChars(t *testing.T) {
	// Control chars except \n and \t should be stripped
	in := "hello\x00world\x01\nkeep\ttabs"
	want := "helloworld\nkeep\ttabs"
	got := SanitizeInput(in)
	if got != want {
		t.Errorf("SanitizeInput(%q) = %q, want %q", in, got, want)
	}
}

func TestSanitizeInput_PreservesNewlineAndTab(t *testing.T) {
	in := "line1\nline2\tcol"
	got := SanitizeInput(in)
	if got != in {
		t.Errorf("SanitizeInput should preserve \\n and \\t, got %q", got)
	}
}

// ============================================================
// Unit Tests — ValidateResourceName
// ============================================================

func TestValidateResourceName_AcceptsValid(t *testing.T) {
	good := []string{
		"S024116",
		"my-resource",
		"my_resource",
		"my.resource",
		"abc123",
		"a",
	}
	for _, n := range good {
		if err := ValidateResourceName(n); err != nil {
			t.Errorf("ValidateResourceName(%q) should accept: %v", n, err)
		}
	}
}

func TestValidateResourceName_RejectsInvalid(t *testing.T) {
	bad := []string{
		"",
		"foo bar",
		"foo/bar",
		"foo@bar",
		"hello!",
		"a b",
		"name\x00null",
	}
	for _, n := range bad {
		if err := ValidateResourceName(n); err == nil {
			t.Errorf("ValidateResourceName(%q) should reject", n)
		}
	}
}

// ============================================================
// Unit Tests — ValidateHTTPS
// ============================================================

func TestValidateHTTPS_AcceptsHTTPS(t *testing.T) {
	good := []string{
		"https://hkg-online.everonet.com",
		"https://hkg-online-uat.everonet.com/g2/v1/payment",
		"HTTPS://API.EXAMPLE.COM",
	}
	for _, u := range good {
		if err := ValidateHTTPS(u); err != nil {
			t.Errorf("ValidateHTTPS(%q) should accept: %v", u, err)
		}
	}
}

func TestValidateHTTPS_AcceptsLocalhost(t *testing.T) {
	good := []string{
		"http://localhost:8080",
		"http://127.0.0.1:9090/test",
		"http://localhost/path",
	}
	for _, u := range good {
		if err := ValidateHTTPS(u); err != nil {
			t.Errorf("ValidateHTTPS(%q) should accept localhost: %v", u, err)
		}
	}
}

func TestValidateHTTPS_RejectsHTTP(t *testing.T) {
	bad := []string{
		"http://hkg-online.everonet.com",
		"http://example.com",
		"ftp://files.example.com",
	}
	for _, u := range bad {
		if err := ValidateHTTPS(u); err == nil {
			t.Errorf("ValidateHTTPS(%q) should reject non-HTTPS", u)
		}
	}
}

// ============================================================
// Property-Based Tests (testing/quick)
// ============================================================

// --- Property 21: 输入安全校验 ---

// Feature: evo-payment-cli, Property 21: 输入安全校验
// For random paths containing "..", ValidatePath must reject.
// **Validates: Requirements 12.3**
func TestProperty21_PathTraversalRejected(t *testing.T) {
	f := func(prefix, suffix string) bool {
		// Inject ".." as a path segment
		path := prefix + "/../" + suffix
		return ValidatePath(path) != nil
	}
	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 21 (path traversal) failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 21: 输入安全校验
// For random inputs with ANSI escape sequences, SanitizeInput must strip them.
// **Validates: Requirements 12.4**
func TestProperty21_ANSISanitization(t *testing.T) {
	ansiCodes := []string{
		"\x1b[31m", "\x1b[0m", "\x1b[1;32m", "\x1b[4m",
		"\x1b[38;5;196m", "\x1b[48;2;0;255;0m",
	}

	f := func(text string, codeIdx uint8) bool {
		code := ansiCodes[int(codeIdx)%len(ansiCodes)]
		input := code + text + "\x1b[0m"
		result := SanitizeInput(input)
		// Result must not contain ESC character
		return !strings.Contains(result, "\x1b")
	}
	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 21 (ANSI sanitization) failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 21: 输入安全校验
// For random resource names with special chars, ValidateResourceName must reject invalid ones.
// **Validates: Requirements 12.5**
func TestProperty21_ResourceNameValidation(t *testing.T) {
	// Characters that are NOT allowed in resource names
	badChars := []rune{'/', ' ', '@', '!', '#', '$', '%', '^', '&', '*', '(', ')', '+', '=', '~', '`'}

	f := func(base string, charIdx uint8) bool {
		if base == "" {
			base = "a"
		}
		// Inject a bad character
		bad := badChars[int(charIdx)%len(badChars)]
		name := base + string(bad) + "x"
		return ValidateResourceName(name) != nil
	}
	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 21 (resource name) failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 21: 输入安全校验
// SanitizeInput output must never contain control characters (except \n, \t).
// **Validates: Requirements 12.4**
func TestProperty21_NoControlCharsInOutput(t *testing.T) {
	f := func(s string) bool {
		result := SanitizeInput(s)
		for _, r := range result {
			if unicode.IsControl(r) && r != '\n' && r != '\t' {
				return false
			}
		}
		return true
	}
	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 21 (no control chars) failed: %v", err)
	}
}

// --- Property 22: HTTPS 强制 ---

// Feature: evo-payment-cli, Property 22: HTTPS 强制
// For random non-HTTPS URLs (http:// with non-localhost hosts), ValidateHTTPS must reject.
// **Validates: Requirements 12.6**
func TestProperty22_NonHTTPSRejected(t *testing.T) {
	// Generate random hostnames that are NOT localhost/127.0.0.1
	f := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		// Generate a random domain-like host
		letters := "abcdefghijklmnopqrstuvwxyz"
		length := r.Intn(10) + 3
		var host strings.Builder
		for i := 0; i < length; i++ {
			host.WriteByte(letters[r.Intn(len(letters))])
		}
		host.WriteString(".com")

		rawURL := "http://" + host.String()
		return ValidateHTTPS(rawURL) != nil
	}
	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 22 (non-HTTPS rejected) failed: %v", err)
	}
}

// Feature: evo-payment-cli, Property 22: HTTPS 强制
// For random HTTPS URLs, ValidateHTTPS must accept.
// **Validates: Requirements 12.6**
func TestProperty22_HTTPSAccepted(t *testing.T) {
	f := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		letters := "abcdefghijklmnopqrstuvwxyz"
		length := r.Intn(10) + 3
		var host strings.Builder
		for i := 0; i < length; i++ {
			host.WriteByte(letters[r.Intn(len(letters))])
		}
		host.WriteString(".com")

		rawURL := "https://" + host.String()
		return ValidateHTTPS(rawURL) == nil
	}
	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 22 (HTTPS accepted) failed: %v", err)
	}
}
