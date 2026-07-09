package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRelayConfigRoundTripAndPerms(t *testing.T) {
	dir := t.TempDir()
	rc := RelayConfig{GatewayURL: "https://gw.example", DeviceToken: "tok",
		MachineID: "m1", PrivateKey: []byte{1, 2}, PublicKey: []byte{3, 4}}
	if err := SaveRelay(dir, rc); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRelay(dir)
	if err != nil || got.GatewayURL != rc.GatewayURL || got.DeviceToken != "tok" ||
		got.MachineID != "m1" || string(got.PrivateKey) != string(rc.PrivateKey) ||
		string(got.PublicKey) != string(rc.PublicKey) {
		t.Fatalf("got %+v err=%v", got, err)
	}
	fi, _ := os.Stat(filepath.Join(dir, "relay.yaml"))
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("relay.yaml perms = %v, want 0600", fi.Mode().Perm())
	}
	// absent file -> os.IsNotExist-compatible error, not a hard error nor zero value
	if _, err := LoadRelay(t.TempDir()); err == nil {
		t.Fatal("missing relay.yaml should error (os.IsNotExist-compatible)")
	} else if !os.IsNotExist(err) {
		t.Fatalf("missing relay.yaml error = %v, want os.IsNotExist", err)
	}
}
