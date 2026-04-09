// Package validate provides input security validation (path traversal, ANSI injection, HTTPS enforcement).
package validate

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// ValidatePath rejects paths containing ".." to prevent path traversal attacks.
func ValidatePath(path string) error {
	// Split on both forward and back slashes to catch all traversal patterns
	for _, seg := range strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if seg == ".." {
			return fmt.Errorf("path traversal detected: %q contains '..'", path)
		}
	}
	// Also catch bare ".." without separators
	if path == ".." {
		return fmt.Errorf("path traversal detected: %q contains '..'", path)
	}
	return nil
}

// ansiEscapeRe matches ANSI escape sequences (CSI, OSC, and ESC-based sequences).
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b[^[\]()]`)

// SanitizeInput strips ANSI escape sequences and control characters from s,
// keeping printable characters plus newline (\n) and tab (\t).
func SanitizeInput(s string) string {
	// First strip ANSI escape sequences
	s = ansiEscapeRe.ReplaceAllString(s, "")

	// Then strip remaining control characters (keep printable + \n + \t)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\t' || (unicode.IsPrint(r) && r != '\x1b') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// resourceNameRe allows alphanumeric characters plus hyphen, underscore, and dot.
var resourceNameRe = regexp.MustCompile(`^[a-zA-Z0-9\-_.]+$`)

// ValidateResourceName validates that name contains only alphanumeric characters
// and limited special characters (-, _, .).
func ValidateResourceName(name string) error {
	if name == "" {
		return fmt.Errorf("resource name must not be empty")
	}
	if !resourceNameRe.MatchString(name) {
		return fmt.Errorf("invalid resource name %q: only alphanumeric, '-', '_', '.' allowed", name)
	}
	return nil
}

// ValidateHTTPS rejects non-HTTPS URLs, allowing http://localhost and
// http://127.0.0.1 for local testing.
func ValidateHTTPS(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	if strings.EqualFold(u.Scheme, "https") {
		return nil
	}

	// Allow http for localhost / 127.0.0.1 (testing convenience)
	if strings.EqualFold(u.Scheme, "http") {
		host := u.Hostname()
		if host == "localhost" || host == "127.0.0.1" {
			return nil
		}
	}

	return fmt.Errorf("non-HTTPS URL rejected: %q (only HTTPS is allowed, except localhost/127.0.0.1)", rawURL)
}
