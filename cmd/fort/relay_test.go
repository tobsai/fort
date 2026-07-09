package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/config"
)

// TestRelayJoinPostsAndPersists drives relayJoin against a fake gateway,
// asserting the POST body shape and that relay.yaml lands 0600 with the token.
func TestRelayJoinPostsAndPersists(t *testing.T) {
	dir := t.TempDir()
	var gotPath string
	var gotBody map[string]any
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"device_token": "dev-tok-123",
			"machine_id":   "mac-9",
		})
	}))
	defer gw.Close()

	if err := relayJoin(gw.URL, "AAAA-BBBB", "laptop", dir); err != nil {
		t.Fatalf("relayJoin: %v", err)
	}

	if gotPath != "/api/relay/join" {
		t.Errorf("join POST path = %q, want /api/relay/join", gotPath)
	}
	for _, k := range []string{"code", "name", "public_key"} {
		if _, ok := gotBody[k]; !ok {
			t.Errorf("join body missing %q: %+v", k, gotBody)
		}
	}
	if gotBody["code"] != "AAAA-BBBB" || gotBody["name"] != "laptop" {
		t.Errorf("join body = %+v", gotBody)
	}
	if s, _ := gotBody["public_key"].(string); s == "" {
		t.Errorf("public_key empty in join body: %+v", gotBody)
	}

	fi, err := os.Stat(filepath.Join(dir, "relay.yaml"))
	if err != nil {
		t.Fatalf("relay.yaml: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("relay.yaml perms = %v, want 0600", fi.Mode().Perm())
	}
	rc, err := config.LoadRelay(dir)
	if err != nil || rc.DeviceToken != "dev-tok-123" || rc.MachineID != "mac-9" ||
		rc.GatewayURL != gw.URL {
		t.Fatalf("loaded %+v err=%v", rc, err)
	}
	if len(rc.PublicKey) == 0 || len(rc.PrivateKey) == 0 {
		t.Errorf("keypair not persisted: pub=%d priv=%d", len(rc.PublicKey), len(rc.PrivateKey))
	}
}

// TestRelayJoinReusedCodeSurfacesServerError asserts a 409 body reaches the user
// and no relay.yaml is written on a failed join.
func TestRelayJoinReusedCodeSurfacesServerError(t *testing.T) {
	dir := t.TempDir()
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, "join code already used")
	}))
	defer gw.Close()

	err := relayJoin(gw.URL, "AAAA-BBBB", "laptop", dir)
	if err == nil || !strings.Contains(err.Error(), "join code already used") {
		t.Fatalf("want server message surfaced, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "relay.yaml")); statErr == nil {
		t.Error("relay.yaml must not be written on a failed join")
	}
}
