//go:build windows

package keychain

import (
	"encoding/hex"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	dllCrypt32  = syscall.NewLazyDLL("crypt32.dll")
	dllKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procEncrypt = dllCrypt32.NewProc("CryptProtectData")
	procDecrypt = dllCrypt32.NewProc("CryptUnprotectData")
	procFree    = dllKernel32.NewProc("LocalFree")
)

// dataBlob is the Windows DATA_BLOB structure used by DPAPI.
type dataBlob struct {
	cbData uint32
	pbData *byte
}

// WindowsKeychain implements KeychainAccess on Windows using DPAPI.
// Secrets are DPAPI-encrypted and stored in files under ~/.evo-cli/secrets/.
type WindowsKeychain struct{}

// New returns a new KeychainAccess for the current platform (Windows).
func New() KeychainAccess {
	return &WindowsKeychain{}
}

// Get retrieves a secret by key. Reads the DPAPI-encrypted file and decrypts it.
func (k *WindowsKeychain) Get(key string) (string, error) {
	path, err := secretFilePath(key)
	if err != nil {
		return "", err
	}
	hexData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("secret not found: %s", key)
		}
		return "", fmt.Errorf("read secret file: %w", err)
	}
	encrypted, err := hex.DecodeString(string(hexData))
	if err != nil {
		return "", fmt.Errorf("decode secret file: %w", err)
	}
	plaintext, err := dpapiDecrypt(encrypted)
	if err != nil {
		return "", fmt.Errorf("dpapi decrypt: %w", err)
	}
	return string(plaintext), nil
}

// Set stores a secret. DPAPI-encrypts the value and writes to a file.
func (k *WindowsKeychain) Set(key string, value string) error {
	if err := ensureSecretsDir(); err != nil {
		return err
	}
	path, err := secretFilePath(key)
	if err != nil {
		return err
	}
	encrypted, err := dpapiEncrypt([]byte(value))
	if err != nil {
		return fmt.Errorf("dpapi encrypt: %w", err)
	}
	hexData := hex.EncodeToString(encrypted)
	return os.WriteFile(path, []byte(hexData), 0600)
}

// Remove deletes a secret file.
func (k *WindowsKeychain) Remove(key string) error {
	return removeSecretFile(key)
}

// dpapiEncrypt encrypts data using Windows DPAPI (CryptProtectData).
func dpapiEncrypt(data []byte) ([]byte, error) {
	input := newBlob(data)
	var output dataBlob

	r, _, err := procEncrypt.Call(
		uintptr(unsafe.Pointer(&input)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&output)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptProtectData: %w", err)
	}
	defer procFree.Call(uintptr(unsafe.Pointer(output.pbData)))

	return blobToBytes(output), nil
}

// dpapiDecrypt decrypts data using Windows DPAPI (CryptUnprotectData).
func dpapiDecrypt(data []byte) ([]byte, error) {
	input := newBlob(data)
	var output dataBlob

	r, _, err := procDecrypt.Call(
		uintptr(unsafe.Pointer(&input)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&output)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptUnprotectData: %w", err)
	}
	defer procFree.Call(uintptr(unsafe.Pointer(output.pbData)))

	return blobToBytes(output), nil
}

// newBlob creates a DATA_BLOB from a byte slice.
func newBlob(data []byte) dataBlob {
	if len(data) == 0 {
		return dataBlob{}
	}
	return dataBlob{
		cbData: uint32(len(data)),
		pbData: &data[0],
	}
}

// blobToBytes copies data from a DATA_BLOB into a Go byte slice.
func blobToBytes(blob dataBlob) []byte {
	if blob.cbData == 0 {
		return nil
	}
	out := make([]byte, blob.cbData)
	copy(out, unsafe.Slice(blob.pbData, blob.cbData))
	return out
}
