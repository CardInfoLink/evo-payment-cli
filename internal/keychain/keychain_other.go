//go:build !darwin && !windows

package keychain

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/user"
	"strings"
)

// OtherKeychain implements KeychainAccess on Linux and other non-macOS/non-Windows platforms.
// The master key is derived from machine characteristics (machine-id + username + home dir).
// Secrets are AES-256-GCM encrypted in files under ~/.evo-cli/secrets/.
type OtherKeychain struct{}

// New returns a new KeychainAccess for the current platform (Linux/other).
func New() KeychainAccess {
	return &OtherKeychain{}
}

// Get retrieves a secret by key.
func (k *OtherKeychain) Get(key string) (string, error) {
	masterKey, err := deriveMasterKey()
	if err != nil {
		return "", fmt.Errorf("derive master key: %w", err)
	}
	return readEncryptedFile(masterKey, key)
}

// Set stores a secret.
func (k *OtherKeychain) Set(key string, value string) error {
	masterKey, err := deriveMasterKey()
	if err != nil {
		return fmt.Errorf("derive master key: %w", err)
	}
	return writeEncryptedFile(masterKey, key, value)
}

// Remove deletes a secret file.
func (k *OtherKeychain) Remove(key string) error {
	return removeSecretFile(key)
}

// deriveMasterKey produces a deterministic 32-byte key from machine characteristics.
// Sources (in order, concatenated):
//  1. /etc/machine-id (Linux standard)
//  2. Current username
//  3. Home directory path
//
// The concatenated string is SHA-256 hashed to produce the 32-byte key.
func deriveMasterKey() ([]byte, error) {
	var parts []string

	// 1. Machine ID
	machineID, _ := readMachineID()
	parts = append(parts, machineID)

	// 2. Username
	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}
	parts = append(parts, u.Username)

	// 3. Home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	parts = append(parts, home)

	combined := strings.Join(parts, ":")
	hash := sha256.Sum256([]byte(combined))
	return hash[:], nil
}

// readMachineID reads /etc/machine-id, falling back to /var/lib/dbus/machine-id.
func readMachineID() (string, error) {
	paths := []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			return strings.TrimSpace(string(data)), nil
		}
	}
	// Fallback: use hostname if machine-id is unavailable.
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown", nil
	}
	return hostname, nil
}
