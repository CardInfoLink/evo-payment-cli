// Package keychain provides cross-platform secret storage.
//
// Platform implementations:
//   - macOS: system keychain stores master key, AES-256-GCM encrypted file stores actual secret
//   - Windows: DPAPI encryption
//   - Linux/other: AES-256-GCM encrypted file with machine-derived master key
//
// Storage location: ~/.evo-cli/secrets/
// File naming: SHA256 hash of the key name as filename.
package keychain

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// KeychainAccess is the interface for cross-platform secret storage.
// It satisfies the KeychainResolver interface from internal/core/config.go.
type KeychainAccess interface {
	Get(key string) (string, error)
	Set(key string, value string) error
	Remove(key string) error
}

// secretsDir is the subdirectory under ~/.evo-cli/ for encrypted secret files.
const secretsDir = "secrets"

// encryptedPayload is the JSON structure stored in each secret file.
type encryptedPayload struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// secretFilePath returns the path to the encrypted file for a given key.
// The filename is the SHA256 hex digest of the key name.
func secretFilePath(key string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	hash := sha256.Sum256([]byte(key))
	filename := hex.EncodeToString(hash[:])
	return filepath.Join(home, ".evo-cli", secretsDir, filename), nil
}

// ensureSecretsDir creates the ~/.evo-cli/secrets/ directory if it does not exist.
func ensureSecretsDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".evo-cli", secretsDir)
	return os.MkdirAll(dir, 0700)
}

// encryptAESGCM encrypts plaintext using AES-256-GCM with the given 32-byte key.
func encryptAESGCM(masterKey []byte, plaintext []byte) (nonce, ciphertext []byte, err error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

// decryptAESGCM decrypts ciphertext using AES-256-GCM with the given 32-byte key.
func decryptAESGCM(masterKey, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm.Open: %w", err)
	}
	return plaintext, nil
}

// writeEncryptedFile encrypts value with masterKey and writes to the secret file for key.
func writeEncryptedFile(masterKey []byte, key, value string) error {
	if err := ensureSecretsDir(); err != nil {
		return err
	}
	path, err := secretFilePath(key)
	if err != nil {
		return err
	}
	nonce, ct, err := encryptAESGCM(masterKey, []byte(value))
	if err != nil {
		return fmt.Errorf("encrypt secret: %w", err)
	}
	payload := encryptedPayload{
		Nonce:      hex.EncodeToString(nonce),
		Ciphertext: hex.EncodeToString(ct),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// readEncryptedFile reads and decrypts the secret file for key using masterKey.
func readEncryptedFile(masterKey []byte, key string) (string, error) {
	path, err := secretFilePath(key)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("secret not found: %s", key)
		}
		return "", fmt.Errorf("read secret file: %w", err)
	}
	var payload encryptedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("parse secret file: %w", err)
	}
	nonce, err := hex.DecodeString(payload.Nonce)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}
	ct, err := hex.DecodeString(payload.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	plaintext, err := decryptAESGCM(masterKey, nonce, ct)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}

// removeSecretFile removes the encrypted secret file for key.
func removeSecretFile(key string) error {
	path, err := secretFilePath(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove secret file: %w", err)
	}
	return nil
}
