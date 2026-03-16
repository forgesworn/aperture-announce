package key

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Generate creates a new 32-byte random secret key as 64-char hex.
func Generate() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand failed: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Validate checks that a key is a 64-character lowercase hex string.
func Validate(k string) error {
	if len(k) != 64 {
		return fmt.Errorf("key must be 64 hex characters, got %d", len(k))
	}
	for _, c := range k {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return fmt.Errorf("key contains invalid character: %c", c)
		}
	}
	return nil
}

// Persist writes a key to a file with 0600 permissions.
// Creates parent directories with 0700 if needed.
func Persist(key, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create key directory: %w", err)
	}
	return os.WriteFile(path, []byte(key+"\n"), 0o600)
}

// Load reads a key from a file.
func Load(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// Resolve returns the key to use: explicit if provided, otherwise
// auto-generates and persists to keyDir/announce.key.
func Resolve(explicit, keyDir string) (string, error) {
	if explicit != "" {
		if err := Validate(explicit); err != nil {
			return "", fmt.Errorf("invalid explicit key: %w", err)
		}
		return explicit, nil
	}

	path := filepath.Join(keyDir, "announce.key")

	// Try loading existing
	if k, err := Load(path); err == nil {
		if err := Validate(k); err != nil {
			return "", fmt.Errorf("corrupted key file %s: %w", path, err)
		}
		return k, nil
	}

	// Generate and persist
	k, err := Generate()
	if err != nil {
		return "", err
	}
	if err := Persist(k, path); err != nil {
		return "", fmt.Errorf("persist auto-generated key: %w", err)
	}
	return k, nil
}
