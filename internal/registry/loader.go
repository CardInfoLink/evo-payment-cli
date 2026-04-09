package registry

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

//go:embed meta_data.json
var embeddedMetaData []byte

// EmbeddedMetaData returns the raw embedded meta_data.json bytes.
// Useful for tests that need to load the registry from the embedded data.
func EmbeddedMetaData() []byte {
	return embeddedMetaData
}

const (
	// CacheTTL is the default time-to-live for the local cache (24 hours).
	CacheTTL = 24 * time.Hour

	// cacheDir is the subdirectory under the user's home for CLI cache.
	cacheDir = ".evo-cli/cache"

	// cacheFileName is the cached meta_data.json file name.
	cacheFileName = "meta_data.json"
)

// LoadFromJSON parses raw JSON bytes into a Registry.
func LoadFromJSON(data []byte) (*Registry, error) {
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("failed to parse meta_data.json: %w", err)
	}
	return &reg, nil
}

// LoadRegistry loads the API metadata registry with the following priority:
//  1. Local cache (~/.evo-cli/cache/meta_data.json) if it exists and is fresh (< CacheTTL)
//  2. Embedded meta_data.json (compiled into the binary)
//
// After loading, it starts a background goroutine to refresh the cache if stale.
func LoadRegistry() (*Registry, error) {
	// Try local cache first.
	if data, err := loadFreshCache(); err == nil {
		reg, parseErr := LoadFromJSON(data)
		if parseErr == nil {
			return reg, nil
		}
		// Cache is corrupt — fall through to embedded data.
	}

	// Fall back to embedded data.
	reg, err := LoadFromJSON(embeddedMetaData)
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded registry: %w", err)
	}

	// Start background refresh if cache is stale or missing.
	go refreshCacheInBackground()

	return reg, nil
}

// cachePath returns the full path to the cached meta_data.json.
func cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, cacheDir, cacheFileName), nil
}

// loadFreshCache reads the local cache file if it exists and is within the TTL.
func loadFreshCache() ([]byte, error) {
	path, err := cachePath()
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Check TTL — file must be modified within the last CacheTTL.
	if time.Since(info.ModTime()) > CacheTTL {
		return nil, fmt.Errorf("cache expired")
	}

	return os.ReadFile(path)
}

// writeCache writes data to the local cache file, creating directories as needed.
func writeCache(data []byte) error {
	path, err := cachePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// refreshCacheInBackground attempts to fetch fresh meta_data.json from a remote
// endpoint and overlay it onto the local cache. Currently a stub — remote
// fetching will be implemented when a remote endpoint is available.
func refreshCacheInBackground() {
	// TODO: Implement remote fetch when endpoint is available.
	// For now, if we have embedded data and no cache, write embedded data as cache
	// so subsequent loads within TTL use the cache path.
	path, err := cachePath()
	if err != nil {
		return
	}

	// Only write if cache doesn't exist yet.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_ = writeCache(embeddedMetaData)
	}
}
