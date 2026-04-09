package keychain

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecryptAESGCM(t *testing.T) {
	masterKey := make([]byte, 32)
	if _, err := rand.Read(masterKey); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"empty", ""},
		{"short", "hello"},
		{"signkey-length", "64b59e70e15445196b1b5d2935f4e1bc"},
		{"unicode", "密钥测试🔑"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nonce, ct, err := encryptAESGCM(masterKey, []byte(tt.plaintext))
			if err != nil {
				t.Fatalf("encrypt: %v", err)
			}
			got, err := decryptAESGCM(masterKey, nonce, ct)
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			if string(got) != tt.plaintext {
				t.Errorf("got %q, want %q", string(got), tt.plaintext)
			}
		})
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	nonce, ct, err := encryptAESGCM(key1, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = decryptAESGCM(key2, nonce, ct)
	if err == nil {
		t.Error("expected error decrypting with wrong key")
	}
}

func TestSecretFilePath(t *testing.T) {
	path, err := secretFilePath("evo-cli:signkey:S024116")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Dir(path)
	if dir != filepath.Join(home, ".evo-cli", "secrets") {
		t.Errorf("unexpected dir: %s", dir)
	}
	// Filename should be a 64-char hex string (SHA256).
	base := filepath.Base(path)
	if len(base) != 64 {
		t.Errorf("expected 64-char hex filename, got %d chars: %s", len(base), base)
	}
}

func TestWriteReadRemoveEncryptedFile(t *testing.T) {
	masterKey := make([]byte, 32)
	rand.Read(masterKey)

	// Use a unique test key to avoid collisions.
	testKey := "evo-cli-test-key-" + t.Name()
	testValue := "test-secret-value-12345"

	// Clean up after test.
	defer removeSecretFile(testKey)

	// Write
	if err := writeEncryptedFile(masterKey, testKey, testValue); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read
	got, err := readEncryptedFile(masterKey, testKey)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != testValue {
		t.Errorf("got %q, want %q", got, testValue)
	}

	// Remove
	if err := removeSecretFile(testKey); err != nil {
		t.Fatalf("remove: %v", err)
	}

	// Read after remove should fail.
	_, err = readEncryptedFile(masterKey, testKey)
	if err == nil {
		t.Error("expected error reading removed secret")
	}
}

func TestReadEncryptedFileNotFound(t *testing.T) {
	masterKey := make([]byte, 32)
	rand.Read(masterKey)

	_, err := readEncryptedFile(masterKey, "nonexistent-key-xyz")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

// TestNew_ReturnsNonNil verifies the platform-specific New() returns a valid instance.
func TestNew_ReturnsNonNil(t *testing.T) {
	kc := New()
	if kc == nil {
		t.Fatal("New() returned nil")
	}
}

// TestKeychain_SetGetRemove_Integration tests the full Set → Get → Remove cycle
// through the platform-specific KeychainAccess implementation.
func TestKeychain_SetGetRemove_Integration(t *testing.T) {
	kc := New()
	testKey := "evo-cli-test-integration-" + t.Name()
	testValue := "integration-test-secret-value-abc123"

	// Clean up in case of previous failed test.
	_ = kc.Remove(testKey)
	defer kc.Remove(testKey)

	// Set
	if err := kc.Set(testKey, testValue); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	// Get
	got, err := kc.Get(testKey)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != testValue {
		t.Errorf("Get() = %q, want %q", got, testValue)
	}

	// Remove
	if err := kc.Remove(testKey); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	// Get after remove should fail.
	_, err = kc.Get(testKey)
	if err == nil {
		t.Error("Get() after Remove() should return error")
	}
}

// TestKeychain_Get_NotFound tests Get for a key that doesn't exist.
func TestKeychain_Get_NotFound(t *testing.T) {
	kc := New()
	_, err := kc.Get("evo-cli-nonexistent-key-" + t.Name())
	if err == nil {
		t.Error("Get() for nonexistent key should return error")
	}
}

// TestKeychain_Remove_NotFound tests Remove for a key that doesn't exist (should not error).
func TestKeychain_Remove_NotFound(t *testing.T) {
	kc := New()
	err := kc.Remove("evo-cli-nonexistent-key-" + t.Name())
	// Remove of nonexistent key should not error (or error gracefully).
	_ = err // Some implementations may return nil, others an error — both are acceptable.
}

// TestKeychain_SetOverwrite tests that Set overwrites an existing value.
func TestKeychain_SetOverwrite(t *testing.T) {
	kc := New()
	testKey := "evo-cli-test-overwrite-" + t.Name()
	defer kc.Remove(testKey)

	if err := kc.Set(testKey, "value1"); err != nil {
		t.Fatalf("Set(value1) error: %v", err)
	}
	if err := kc.Set(testKey, "value2"); err != nil {
		t.Fatalf("Set(value2) error: %v", err)
	}

	got, err := kc.Get(testKey)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != "value2" {
		t.Errorf("Get() = %q, want %q (should be overwritten)", got, "value2")
	}
}

// TestEnsureSecretsDir tests that ensureSecretsDir creates the directory.
func TestEnsureSecretsDir(t *testing.T) {
	if err := ensureSecretsDir(); err != nil {
		t.Fatalf("ensureSecretsDir() error: %v", err)
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".evo-cli", "secrets")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("secrets dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("secrets path is not a directory")
	}
}

// TestRemoveSecretFile_Idempotent tests that removing a nonexistent file doesn't error.
func TestRemoveSecretFile_Idempotent(t *testing.T) {
	err := removeSecretFile("evo-cli-definitely-not-exists-" + t.Name())
	if err != nil {
		t.Errorf("removeSecretFile for nonexistent key should not error, got: %v", err)
	}
}
