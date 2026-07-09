package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RelayConfig is the persisted remote-gateway identity of one machine (spec
// 028): written by `fort relay join`. It holds the device token and this
// machine's static Noise keypair, so relay.yaml is always written 0600.
type RelayConfig struct {
	GatewayURL  string
	DeviceToken string
	MachineID   string
	PrivateKey  []byte
	PublicKey   []byte
}

// relayFile is the on-disk shape of relay.yaml: the two Noise keys are stored
// as explicit base64 strings so the file stays human-readable.
type relayFile struct {
	GatewayURL  string `yaml:"gateway_url"`
	DeviceToken string `yaml:"device_token"`
	MachineID   string `yaml:"machine_id"`
	PrivateKey  string `yaml:"private_key"` // base64(std)
	PublicKey   string `yaml:"public_key"`  // base64(std)
}

// LoadRelay reads dir/relay.yaml. Unlike ReadNodeFile, a missing file is NOT
// swallowed: the os.IsNotExist-compatible error is returned so callers can use
// `if _, err := LoadRelay(dir); err == nil` to decide whether a relay is
// configured.
func LoadRelay(dir string) (RelayConfig, error) {
	data, err := os.ReadFile(filepath.Join(dir, "relay.yaml"))
	if err != nil {
		return RelayConfig{}, err // missing file surfaces os.IsNotExist to the caller
	}
	var rf relayFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return RelayConfig{}, fmt.Errorf("config: relay.yaml: %w", err)
	}
	priv, err := base64.StdEncoding.DecodeString(rf.PrivateKey)
	if err != nil {
		return RelayConfig{}, fmt.Errorf("config: relay.yaml private_key: %w", err)
	}
	pub, err := base64.StdEncoding.DecodeString(rf.PublicKey)
	if err != nil {
		return RelayConfig{}, fmt.Errorf("config: relay.yaml public_key: %w", err)
	}
	return RelayConfig{
		GatewayURL:  rf.GatewayURL,
		DeviceToken: rf.DeviceToken,
		MachineID:   rf.MachineID,
		PrivateKey:  priv,
		PublicKey:   pub,
	}, nil
}

// SaveRelay writes dir/relay.yaml atomically with mode 0600 throughout — the
// temp file is created 0600 before any bytes are written, so the device token
// and private key are never world-readable, even transiently. Mirrors
// SaveNodeFile.
func SaveRelay(dir string, rc RelayConfig) error {
	data, err := yaml.Marshal(relayFile{
		GatewayURL:  rc.GatewayURL,
		DeviceToken: rc.DeviceToken,
		MachineID:   rc.MachineID,
		PrivateKey:  base64.StdEncoding.EncodeToString(rc.PrivateKey),
		PublicKey:   base64.StdEncoding.EncodeToString(rc.PublicKey),
	})
	if err != nil {
		return fmt.Errorf("config: relay.yaml: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	tmp := filepath.Join(dir, ".relay.yaml.tmp")
	// O_CREATE applies the 0600 mode only when the file is created fresh; a
	// stale temp file would keep its old (possibly looser) permissions. Remove
	// it first so the mode always holds. The error is ignored — the file
	// usually doesn't exist.
	os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("config: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("config: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, "relay.yaml")); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("config: %w", err)
	}
	return nil
}
