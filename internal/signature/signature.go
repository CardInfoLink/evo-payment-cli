// Package signature provides Evo Payment message signature generation and verification.
package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"
)

// BuildSignString constructs the 6-line signature string per Evo Payment spec.
// Each non-empty line ends with \n (0x0A) except the last non-empty line.
// Empty lines are omitted entirely.
func BuildSignString(method, path, dateTime, signKey, msgID, body string) string {
	lines := []string{method, path, dateTime, signKey, msgID, body}

	// Filter out empty lines
	var nonEmpty []string
	for _, line := range lines {
		if line != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}

	if len(nonEmpty) == 0 {
		return ""
	}

	// Join with \n — last line has no trailing \n
	return strings.Join(nonEmpty, "\n")
}

// GenerateSignature builds the signature string and computes the hash.
// Supported signType values: SHA256, SHA512, HMAC-SHA256, HMAC-SHA512.
// Returns lowercase hex-encoded signature.
func GenerateSignature(method, path, dateTime, signKey, msgID, body, signType string) (string, error) {
	signStr := BuildSignString(method, path, dateTime, signKey, msgID, body)

	switch strings.ToUpper(signType) {
	case "SHA256":
		h := sha256.Sum256([]byte(signStr))
		return hex.EncodeToString(h[:]), nil

	case "SHA512":
		h := sha512.Sum512([]byte(signStr))
		return hex.EncodeToString(h[:]), nil

	case "HMAC-SHA256":
		mac := hmac.New(func() hash.Hash { return sha256.New() }, []byte(signKey))
		mac.Write([]byte(signStr))
		return hex.EncodeToString(mac.Sum(nil)), nil

	case "HMAC-SHA512":
		mac := hmac.New(func() hash.Hash { return sha512.New() }, []byte(signKey))
		mac.Write([]byte(signStr))
		return hex.EncodeToString(mac.Sum(nil)), nil

	default:
		return "", fmt.Errorf("unsupported sign type: %s", signType)
	}
}

// VerifySignature generates a signature and compares it to the expected value.
// Comparison is case-insensitive.
func VerifySignature(method, path, dateTime, signKey, msgID, body, signType, expected string) bool {
	sig, err := GenerateSignature(method, path, dateTime, signKey, msgID, body, signType)
	if err != nil {
		return false
	}
	return strings.EqualFold(sig, expected)
}
