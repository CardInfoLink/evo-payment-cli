//go:build darwin

package keychain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

const (
	darwinServiceName = "evo-cli"
	darwinMasterAcct  = "evo-cli-master-key"
	masterKeyByteLen  = 32 // AES-256
)

// DarwinKeychain implements KeychainAccess on macOS.
// Master key is stored in the system keychain via the `security` CLI tool.
// Actual secrets are AES-256-GCM encrypted in files under ~/.evo-cli/secrets/.
type DarwinKeychain struct{}

// New returns a new KeychainAccess for the current platform (macOS).
func New() KeychainAccess {
	return &DarwinKeychain{}
}

// Get retrieves a secret by key. It reads the master key from the system keychain,
// then decrypts the secret file.
func (k *DarwinKeychain) Get(key string) (string, error) {
	masterKey, err := k.getMasterKey()
	if err != nil {
		return "", fmt.Errorf("get master key: %w", err)
	}
	return readEncryptedFile(masterKey, key)
}

// Set stores a secret. It ensures a master key exists in the system keychain,
// then encrypts the value and writes it to a file.
func (k *DarwinKeychain) Set(key string, value string) error {
	masterKey, err := k.getOrCreateMasterKey()
	if err != nil {
		return fmt.Errorf("get or create master key: %w", err)
	}
	return writeEncryptedFile(masterKey, key, value)
}

// Remove deletes a secret file. The master key in the system keychain is kept.
func (k *DarwinKeychain) Remove(key string) error {
	return removeSecretFile(key)
}

// getMasterKey reads the master key from the macOS system keychain.
func (k *DarwinKeychain) getMasterKey() ([]byte, error) {
	// security find-generic-password -s <service> -a <account> -w
	out, err := exec.Command("security", "find-generic-password",
		"-s", darwinServiceName,
		"-a", darwinMasterAcct,
		"-w",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("master key not found in keychain (run 'evo-cli config init' first): %w", err)
	}
	hexKey := strings.TrimSpace(string(out))
	return hex.DecodeString(hexKey)
}

// getOrCreateMasterKey reads the master key from the keychain, or generates
// a new one and stores it if none exists.
func (k *DarwinKeychain) getOrCreateMasterKey() ([]byte, error) {
	masterKey, err := k.getMasterKey()
	if err == nil {
		return masterKey, nil
	}

	// Generate a new 32-byte master key.
	masterKey = make([]byte, masterKeyByteLen)
	if _, err := rand.Read(masterKey); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}

	hexKey := hex.EncodeToString(masterKey)

	// security add-generic-password -s <service> -a <account> -w <password> -U
	cmd := exec.Command("security", "add-generic-password",
		"-s", darwinServiceName,
		"-a", darwinMasterAcct,
		"-w", hexKey,
		"-U", // update if exists
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("store master key in keychain: %s: %w", string(out), err)
	}

	return masterKey, nil
}
