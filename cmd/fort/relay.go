package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/exec/relay/secure"
)

// cmdRelay dispatches the remote-gateway subcommands (spec 028). Unlike mesh's
// invite/remove (loopback clients of the local daemon), relay talks to the
// REMOTE gateway URL the operator supplies; `fort serve` then maintains the
// outbound tunnel when relay.yaml exists.
func cmdRelay(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: fort relay <join|status|remove> ...")
	}
	switch args[0] {
	case "join":
		return cmdRelayJoin(args[1:])
	case "status":
		return cmdRelayStatus(args[1:])
	case "remove":
		return cmdRelayRemove(args[1:])
	default:
		return fmt.Errorf("usage: fort relay <join|status|remove> ...; unknown subcommand %q", args[0])
	}
}

// --- join ---

func cmdRelayJoin(args []string) error {
	// Paste-ready form: `fort relay join <gateway-url> --code C`, positional URL
	// first (flag stops at the first non-flag arg), mirroring `mesh join`.
	usage := "usage: fort relay join <gateway-url> --code XXXX-XXXX [--name N]"
	if len(args) < 1 || strings.HasPrefix(args[0], "-") {
		return errors.New(usage)
	}
	gatewayURL := strings.TrimRight(args[0], "/")

	fs := flag.NewFlagSet("relay join", flag.ExitOnError)
	code := fs.String("code", "", "join code from the gateway (required)")
	name := fs.String("name", "", "this machine's name (default: hostname)")
	_ = fs.Parse(args[1:])

	if *code == "" {
		return errors.New("relay join: --code is required")
	}
	nodeName := *name
	if nodeName == "" {
		nodeName, _ = os.Hostname()
	}

	cfg := config.Load(os.Getenv)
	return relayJoin(gatewayURL, *code, nodeName, cfg.DataDir())
}

// relayJoin mints a static Noise keypair, registers its public key with the
// gateway, and persists relay.yaml. Factored out of cmdRelayJoin (no os.Args /
// no config.Load) so tests can drive it against an httptest gateway with an
// explicit dataDir.
func relayJoin(gatewayURL, code, name, dataDir string) error {
	kp, err := secure.GenerateKeypair()
	if err != nil {
		return fmt.Errorf("relay join: keypair: %w", err)
	}
	body, _ := json.Marshal(map[string]string{
		"code":       code,
		"name":       name,
		"public_key": base64.StdEncoding.EncodeToString(kp.Public),
	})
	resp, err := meshHTTP.Post(gatewayURL+"/api/relay/join", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("relay join: cannot reach the gateway at %s: %w", gatewayURL, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("relay join: %s", strings.TrimSpace(string(raw)))
	}
	var out struct {
		DeviceToken string `json:"device_token"`
		MachineID   string `json:"machine_id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("relay join: bad response: %w", err)
	}
	if err := config.SaveRelay(dataDir, config.RelayConfig{
		GatewayURL:  gatewayURL,
		DeviceToken: out.DeviceToken,
		MachineID:   out.MachineID,
		PrivateKey:  kp.Private,
		PublicKey:   kp.Public,
	}); err != nil {
		return fmt.Errorf("relay join: saving relay identity: %w", err)
	}
	fmt.Printf("joined gateway %s as machine %s\n", gatewayURL, out.MachineID)
	fmt.Printf("key fingerprint: %s\n", secure.FingerprintOf(kp.Public))
	fmt.Println("verify this fingerprint matches this machine's entry on the gateway's machine list before trusting the tunnel")
	fmt.Println("start `fort serve` (or restart the service) to bring the tunnel up")
	return nil
}

// --- status ---

func cmdRelayStatus(args []string) error {
	fs := flag.NewFlagSet("relay status", flag.ExitOnError)
	_ = fs.Parse(args)

	cfg := config.Load(os.Getenv)
	rc, err := config.LoadRelay(cfg.DataDir())
	if os.IsNotExist(err) {
		fmt.Println("not joined to a gateway (run `fort relay join`)")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("gateway    : %s\n", rc.GatewayURL)
	fmt.Printf("machine id : %s\n", rc.MachineID)
	fmt.Printf("fingerprint: %s\n", secure.FingerprintOf(rc.PublicKey))
	return nil
}

// --- remove ---

func cmdRelayRemove(args []string) error {
	fs := flag.NewFlagSet("relay remove", flag.ExitOnError)
	_ = fs.Parse(args)

	cfg := config.Load(os.Getenv)
	dataDir := cfg.DataDir()
	rc, err := config.LoadRelay(dataDir)
	if os.IsNotExist(err) {
		fmt.Println("not joined to a gateway — nothing to remove")
		return nil
	}
	if err != nil {
		return err
	}

	// Best-effort gateway-side revocation. Failures are ignored: deleting
	// relay.yaml locally stops this machine from dialing, but only the gateway
	// dropping the machine (authoritative) actually revokes its access.
	if req, rerr := http.NewRequest(http.MethodDelete, rc.GatewayURL+"/api/relay/machines/"+rc.MachineID, nil); rerr == nil {
		req.Header.Set("Authorization", "Bearer "+rc.DeviceToken)
		if resp, derr := meshHTTP.Do(req); derr == nil {
			resp.Body.Close()
		}
	}

	if err := os.Remove(filepath.Join(dataDir, "relay.yaml")); err != nil {
		return fmt.Errorf("relay remove: %w", err)
	}
	fmt.Printf("removed local relay config for machine %s\n", rc.MachineID)
	fmt.Println("note: gateway-side revocation is authoritative — remove this machine on the gateway to fully revoke access")
	return nil
}
