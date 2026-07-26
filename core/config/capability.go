package config

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const capabilityKeyBytes = 32

// LoadOrCreateCapabilityKey returns the node-local key used to derive opaque
// capability binding revisions. The raw key never leaves the execution plane.
func LoadOrCreateCapabilityKey(dir string) ([]byte, error) {
	path := filepath.Join(dir, "capability.key")
	key, err := readCapabilityKey(path)
	if err == nil {
		return append([]byte(nil), key...), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("config: capability key directory unavailable")
	}

	temp, err := os.CreateTemp(dir, ".capability-key-*")
	if err != nil {
		return nil, fmt.Errorf("config: capability key unavailable")
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return nil, fmt.Errorf("config: capability key unavailable")
	}
	created := make([]byte, capabilityKeyBytes)
	if _, err := rand.Read(created); err != nil {
		temp.Close()
		return nil, fmt.Errorf("config: capability key unavailable")
	}
	if _, err := temp.Write(created); err != nil {
		temp.Close()
		return nil, fmt.Errorf("config: capability key unavailable")
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return nil, fmt.Errorf("config: capability key unavailable")
	}
	if err := temp.Close(); err != nil {
		return nil, fmt.Errorf("config: capability key unavailable")
	}
	if err := os.Link(tempName, path); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("config: capability key unavailable")
	}
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	key, err = readCapabilityKey(path)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), key...), nil
}

func readCapabilityKey(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() != capabilityKeyBytes {
		return nil, fmt.Errorf("config: capability key is not a private 32-byte regular file")
	}
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: capability key unavailable")
	}
	if len(key) != capabilityKeyBytes {
		return nil, fmt.Errorf("config: capability key has invalid length")
	}
	return key, nil
}
