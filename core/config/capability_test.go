package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateCapabilityKeyPersistsPrivateStableKey(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreateCapabilityKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateCapabilityKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 32 || !bytes.Equal(first, second) {
		t.Fatalf("key lengths=%d/%d equal=%v", len(first), len(second), bytes.Equal(first, second))
	}
	info, err := os.Stat(filepath.Join(dir, "capability.key"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("capability.key mode=%o, want 600", got)
	}

	first[0] ^= 0xff
	third, err := LoadOrCreateCapabilityKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, third) {
		t.Fatal("returned key aliases persisted key storage")
	}
}

func TestLoadOrCreateCapabilityKeyRejectsUnsafeExistingFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func(string) error
	}{
		{name: "wrong-size", make: func(path string) error { return os.WriteFile(path, []byte("short"), 0o600) }},
		{name: "loose-mode", make: func(path string) error { return os.WriteFile(path, make([]byte, 32), 0o644) }},
		{name: "symlink", make: func(path string) error {
			target := filepath.Join(filepath.Dir(path), "target")
			if err := os.WriteFile(target, make([]byte, 32), 0o600); err != nil {
				return err
			}
			return os.Symlink(target, path)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := tc.make(filepath.Join(dir, "capability.key")); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadOrCreateCapabilityKey(dir); err == nil {
				t.Fatal("expected unsafe key file to fail closed")
			}
		})
	}
}
