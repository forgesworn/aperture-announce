package key

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	k, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if len(k) != 64 {
		t.Errorf("key length = %d, want 64", len(k))
	}
	for _, c := range k {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("key contains non-hex char: %c", c)
			break
		}
	}
}

func TestPersistAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "announce.key")

	k, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if err := Persist(k, path); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != k {
		t.Errorf("loaded key = %q, want %q", loaded, k)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file permissions = %o, want 600", perm)
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/key")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestResolve_ExplicitKey(t *testing.T) {
	k, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := Resolve(k, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if resolved != k {
		t.Error("explicit key should be returned as-is")
	}
}

func TestResolve_AutoGenerate(t *testing.T) {
	dir := t.TempDir()
	k1, err := Resolve("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(k1) != 64 {
		t.Error("auto-generated key should be 64 chars")
	}

	k2, err := Resolve("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if k1 != k2 {
		t.Error("second call should return persisted key")
	}
}

func TestValidate(t *testing.T) {
	k, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(k); err != nil {
		t.Errorf("valid key rejected: %v", err)
	}

	// Uppercase hex should be accepted
	upper := "AABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDD"
	if err := Validate(upper); err != nil {
		t.Errorf("uppercase hex rejected: %v", err)
	}

	invalid := []struct {
		name string
		key  string
	}{
		{"too short", "abcd"},
		{"too long", "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddee"},
		{"non-hex chars", "gghhiijjgghhiijjgghhiijjgghhiijjgghhiijjgghhiijjgghhiijjgghhiijj"},
		{"empty", ""},
	}
	for _, tc := range invalid {
		if err := Validate(tc.key); err == nil {
			t.Errorf("Validate(%q) [%s]: expected error", tc.key, tc.name)
		}
	}
}

func TestResolve_UppercaseKeyNormalised(t *testing.T) {
	upper := "AABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDD"
	resolved, err := Resolve(upper, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	expected := "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd"
	if resolved != expected {
		t.Errorf("expected normalised lowercase, got %q", resolved)
	}
}

func TestResolve_InvalidExplicitKey(t *testing.T) {
	_, err := Resolve("not-a-valid-key", t.TempDir())
	if err == nil {
		t.Error("expected error for invalid explicit key")
	}
}

func TestResolve_CorruptedKeyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "announce.key")
	if err := os.WriteFile(path, []byte("corrupted\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve("", dir)
	if err == nil {
		t.Error("expected error for corrupted key file")
	}
}
