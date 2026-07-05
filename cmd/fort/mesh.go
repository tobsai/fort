package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/exec/native"
)

// meshHTTP is the client for all mesh CLI calls. A non-zero timeout is
// essential: without it a daemon/hub that accepts the TCP connection but never
// responds hangs the CLI forever (worst on the remote `join` path to an
// operator-supplied hub).
var meshHTTP = &http.Client{Timeout: 30 * time.Second}

// managedRegistryPath resolves where the daemon writes the mesh registry: the
// Fort-managed machines.yaml in the data dir, unless an operator set
// FORT_MACHINES (in which case that path is used but enrollment refuses to
// touch it — the Managed flag, not this path, gates writes).
func managedRegistryPath(cfg config.Config) string {
	if cfg.MachinesManaged || cfg.MachinesPath == "" {
		return filepath.Join(cfg.DataDir(), "machines.yaml")
	}
	return cfg.MachinesPath
}

// probeAgents returns the names of the provider CLIs found on $PATH, in the
// canonical provider order. This is both the hub's self-entry agent list (at
// invite time) and the worker's default offered-agents list (at join time).
func probeAgents() []string {
	var out []string
	for _, p := range native.DefaultProviders() {
		if _, err := exec.LookPath(p.Name); err == nil {
			out = append(out, p.Name)
		}
	}
	return out
}

// loopbackURL builds the hub-local admin URL from cfg.Addr's port: mesh
// invite/remove talk to the running daemon over loopback (spec 024 D7/D8).
func loopbackURL(cfg config.Config, path string) (string, error) {
	_, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return "", fmt.Errorf("mesh: cannot parse FORT_ADDR %q: %w", cfg.Addr, err)
	}
	return "http://127.0.0.1:" + port + path, nil
}

// cmdMesh dispatches the mesh enrollment subcommands (spec 024). invite/remove
// are thin loopback clients of the running daemon; join talks to a remote hub.
func cmdMesh(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: fort mesh <invite|join|remove> ...")
	}
	switch args[0] {
	case "invite":
		return cmdMeshInvite(args[1:])
	case "join":
		return cmdMeshJoin(args[1:])
	case "remove":
		return cmdMeshRemove(args[1:])
	default:
		return fmt.Errorf("usage: fort mesh <invite|join|remove> ...; unknown subcommand %q", args[0])
	}
}

// notRunning turns a connection-refused error into the operator-facing
// "start fort serve first" guidance for the loopback admin verbs (spec 024 D8).
func notRunning(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return errors.New("fort serve is not running on this machine — start it first (mesh invite runs inside the daemon)")
	}
	return err
}

// hubUnreachable turns a connection-refused error on the remote join path into
// guidance to check the hub, rather than surfacing a raw dial error.
func hubUnreachable(hubURL string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("cannot reach the hub at %s — is `fort serve` running and reachable on that address?", hubURL)
	}
	return err
}

// --- invite ---

func cmdMeshInvite(args []string) error {
	fs := flag.NewFlagSet("mesh invite", flag.ExitOnError)
	ttl := fs.String("ttl", "", "invite time-to-live (default 15m, capped at 1h)")
	advertise := fs.String("advertise", "", "override the advertised hub URL")
	_ = fs.Parse(args)

	cfg := config.Load(os.Getenv)
	url, err := loopbackURL(cfg, "/api/mesh/invite")
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{"ttl": *ttl, "advertise": *advertise})
	resp, err := meshHTTP.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return notRunning(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mesh invite: %s", strings.TrimSpace(string(raw)))
	}
	var out struct {
		Code    string `json:"code"`
		HubURL  string `json:"hub_url"`
		JoinCmd string `json:"join_cmd"`
		Minted  bool   `json:"minted"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("mesh invite: bad response: %w", err)
	}
	if out.Minted {
		fmt.Println("mesh token created — this hub now also accepts mesh exec requests (see docs/notes/threat-model.md)")
	}
	fmt.Println(out.JoinCmd)
	return nil
}

// --- join ---

func cmdMeshJoin(args []string) error {
	// The paste-ready form is `fort mesh join <hub-url> --code C`, i.e. the
	// positional URL comes FIRST. Go's stdlib flag package stops at the first
	// non-flag arg, so pull the leading positional off before parsing flags.
	usage := "usage: fort mesh join <hub-url> --code C [--name N] [--port 4087] [--agents a,b] [--advertise URL]"
	if len(args) < 1 || strings.HasPrefix(args[0], "-") {
		return errors.New(usage)
	}
	hubURL := strings.TrimRight(args[0], "/")

	fs := flag.NewFlagSet("mesh join", flag.ExitOnError)
	code := fs.String("code", "", "invite code from `fort mesh invite` (required)")
	name := fs.String("name", "", "this machine's name (default: hostname)")
	port := fs.Int("port", 4087, "this machine's listen port")
	agents := fs.String("agents", "", "comma-separated agent CLIs to offer (default: probe $PATH)")
	advertise := fs.String("advertise", "", "override this machine's advertised URL")
	_ = fs.Parse(args[1:])

	if *code == "" {
		return errors.New("mesh join: --code is required")
	}

	nodeName := *name
	if nodeName == "" {
		nodeName, _ = os.Hostname()
	}

	var offered []string
	if *agents != "" {
		for _, a := range strings.Split(*agents, ",") {
			if a = strings.TrimSpace(a); a != "" {
				offered = append(offered, a)
			}
		}
	} else {
		offered = probeAgents()
	}
	if len(offered) == 0 {
		return errors.New("no agent CLIs found on PATH — pass --agents")
	}

	cfg := config.Load(os.Getenv)
	body, _ := json.Marshal(map[string]any{
		"code":          *code,
		"port":          *port,
		"name":          nodeName,
		"agents":        offered,
		"advertise_url": *advertise,
	})
	resp, err := meshHTTP.Post(hubURL+"/api/mesh/join", "application/json", bytes.NewReader(body))
	if err != nil {
		return hubUnreachable(hubURL, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("mesh join: %s", strings.TrimSpace(string(raw)))
	}
	var out struct {
		Token string `json:"token"`
		Name  string `json:"name"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("mesh join: bad response: %w", err)
	}
	if err := config.SaveNodeFile(cfg.DataDir(), config.NodeFile{
		Name:  out.Name,
		Token: out.Token,
		Addr:  fmt.Sprintf("0.0.0.0:%d", *port),
	}); err != nil {
		return fmt.Errorf("mesh join: saving node identity: %w", err)
	}
	fmt.Printf("joined the mesh as %q — registered at %s\n", out.Name, out.URL)
	fmt.Println("start `fort serve` (or restart the service) to begin accepting work")
	return nil
}

// --- remove ---

func cmdMeshRemove(args []string) error {
	fs := flag.NewFlagSet("mesh remove", flag.ExitOnError)
	_ = fs.Parse(args)

	rest := fs.Args()
	if len(rest) < 1 {
		return errors.New("usage: fort mesh remove <name>")
	}
	name := rest[0]

	cfg := config.Load(os.Getenv)
	url, err := loopbackURL(cfg, "/api/mesh/machines/"+name)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := meshHTTP.Do(req)
	if err != nil {
		return notRunning(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mesh remove: %s", strings.TrimSpace(string(raw)))
	}
	var out struct {
		Warning string `json:"warning"`
	}
	if err := json.Unmarshal(raw, &out); err == nil && out.Warning != "" {
		fmt.Println(out.Warning)
	} else {
		fmt.Printf("removed %q from the mesh\n", name)
	}
	return nil
}
