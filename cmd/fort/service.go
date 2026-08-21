// fort service manages the fort daemon as a launchd user agent on macOS
// (spec 032): install/start/stop/restart/status/uninstall. The Mac Phase 1 app
// uses only status plus Install/Start/Restart recovery; teardown remains an
// explicit CLI operation. Both paths reuse this Go-tested launchd plumbing.
package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tobsai/fort/core/config"
)

// defaultServiceLabel is the launchd label for the Fort daemon user agent.
const defaultServiceLabel = "io.tobsai.fort"

const (
	serviceRestartBootstrapAttempts = 5
	serviceRestartRetryDelay        = 250 * time.Millisecond
)

// serviceConfig is the launchd user-agent definition for the Fort daemon.
type serviceConfig struct {
	Label   string
	BinPath string
	Args    []string
	Addr    string
	DBPath  string
	LogDir  string
	// WorkDir becomes the agent's WorkingDirectory. launchd starts a user agent
	// with cwd "/" (read-only), so without this any relative config path makes
	// the daemon die with "mkdir .fort-native: read-only file system".
	WorkDir string
	// WorkRoot is FORT_WORKROOT: the per-run scratch tree. Absolute, for the
	// same reason as DBPath.
	WorkRoot string
	// Path is baked into the agent's environment. launchd hands a process a bare
	// PATH (/usr/bin:/bin:/usr/sbin:/sbin), which contains no agent CLI — neither
	// Homebrew's /opt/homebrew/bin nor a user's ~/.local/bin. Without this the
	// daemon probes zero agents and every dispatch fails placement. We inherit the
	// PATH of the shell running the first `fort service install`, so the daemon
	// can run exactly what the operator can. A later binary-only/app-driven
	// rollout preserves that installed PATH instead of replacing it with the
	// GUI process's restricted environment.
	Path string
	// CapabilityPlanning preserves an explicit rollout override in launchd.
	// Empty means use the binary default; "0" is the one-step rollback.
	CapabilityPlanning string
	// DisplayTimezone preserves an explicit shared Today timezone in launchd.
	// Empty resolves from the daemon host at startup.
	DisplayTimezone string
	// Explicit configuration paths and mesh identity are carried only through
	// their closed FORT_* keys. App-driven binary rollouts preserve an existing
	// value when the launching app did not explicitly supply a replacement.
	RulesPath    string
	FlowsPath    string
	MachinesPath string
	NodeName     string
	// PrimaryChannels is the closed Phase 1 startup mode. Empty is deliberately
	// omitted so the binary's off-by-default behavior remains authoritative.
	PrimaryChannels string
	// AgentChannels is the independent agent-first startup mode. Empty remains
	// off; launchd restarts must preserve an explicit primary cutover.
	AgentChannels string
	// AcceptedScheduleInventory is the exact nonsecret digest reviewed by the
	// operator. Fort never derives or substitutes this value during rollout.
	AcceptedScheduleInventory string
	// explicitEnvironment is populated by buildServiceConfig from LookupEnv.
	// A nil map means manually supplied nonempty fields are explicit (tests and
	// direct callers); a nonnil map distinguishes process defaults from actual
	// operator overrides during an app-driven restart.
	explicitEnvironment map[string]bool
}

var serviceEnvironmentKeys = []string{
	"PATH",
	"FORT_ADDR",
	"FORT_DB",
	"FORT_WORKROOT",
	"FORT_RULES",
	"FORT_FLOWS",
	"FORT_CAPABILITY_PLANNING",
	"FORT_DISPLAY_TIMEZONE",
	"FORT_MACHINES",
	"FORT_NODE_NAME",
	"FORT_PRIMARY_CHANNELS",
	"FORT_AGENT_CHANNELS",
	"FORT_ACCEPTED_SCHEDULE_INVENTORY",
}

func plistPath(home, label string) string {
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

func renderPlist(sc serviceConfig) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n<dict>\n")
	b.WriteString("  <key>Label</key>\n  <string>" + xmlEscape(sc.Label) + "</string>\n")
	b.WriteString("  <key>ProgramArguments</key>\n  <array>\n")
	b.WriteString("    <string>" + xmlEscape(sc.BinPath) + "</string>\n")
	for _, a := range sc.Args {
		b.WriteString("    <string>" + xmlEscape(a) + "</string>\n")
	}
	b.WriteString("  </array>\n")
	// Environment
	b.WriteString("  <key>EnvironmentVariables</key>\n  <dict>\n")
	for _, key := range serviceEnvironmentKeys {
		if value := sc.environmentValue(key); value != "" {
			b.WriteString("    <key>" + key + "</key>\n    <string>" + xmlEscape(value) + "</string>\n")
		}
	}
	b.WriteString("  </dict>\n")
	if sc.WorkDir != "" {
		b.WriteString("  <key>WorkingDirectory</key>\n  <string>" + xmlEscape(sc.WorkDir) + "</string>\n")
	}
	b.WriteString("  <key>RunAtLoad</key>\n  <true/>\n")
	b.WriteString("  <key>KeepAlive</key>\n  <true/>\n")
	if sc.LogDir != "" {
		b.WriteString("  <key>StandardOutPath</key>\n  <string>" + xmlEscape(filepath.Join(sc.LogDir, "fort.out.log")) + "</string>\n")
		b.WriteString("  <key>StandardErrorPath</key>\n  <string>" + xmlEscape(filepath.Join(sc.LogDir, "fort.err.log")) + "</string>\n")
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.String()
}

func (sc serviceConfig) environmentValue(key string) string {
	switch key {
	case "PATH":
		return sc.Path
	case "FORT_ADDR":
		return sc.Addr
	case "FORT_DB":
		return sc.DBPath
	case "FORT_WORKROOT":
		return sc.WorkRoot
	case "FORT_RULES":
		return sc.RulesPath
	case "FORT_FLOWS":
		return sc.FlowsPath
	case "FORT_CAPABILITY_PLANNING":
		return sc.CapabilityPlanning
	case "FORT_DISPLAY_TIMEZONE":
		return sc.DisplayTimezone
	case "FORT_MACHINES":
		return sc.MachinesPath
	case "FORT_NODE_NAME":
		return sc.NodeName
	case "FORT_PRIMARY_CHANNELS":
		return sc.PrimaryChannels
	case "FORT_AGENT_CHANNELS":
		return sc.AgentChannels
	case "FORT_ACCEPTED_SCHEDULE_INVENTORY":
		return sc.AcceptedScheduleInventory
	default:
		return ""
	}
}

func (sc *serviceConfig) setEnvironmentValue(key, value string) {
	switch key {
	case "PATH":
		sc.Path = value
	case "FORT_ADDR":
		sc.Addr = value
	case "FORT_DB":
		sc.DBPath = value
	case "FORT_WORKROOT":
		sc.WorkRoot = value
	case "FORT_RULES":
		sc.RulesPath = value
	case "FORT_FLOWS":
		sc.FlowsPath = value
	case "FORT_CAPABILITY_PLANNING":
		sc.CapabilityPlanning = value
	case "FORT_DISPLAY_TIMEZONE":
		sc.DisplayTimezone = value
	case "FORT_MACHINES":
		sc.MachinesPath = value
	case "FORT_NODE_NAME":
		sc.NodeName = value
	case "FORT_PRIMARY_CHANNELS":
		sc.PrimaryChannels = value
	case "FORT_AGENT_CHANNELS":
		sc.AgentChannels = value
	case "FORT_ACCEPTED_SCHEDULE_INVENTORY":
		sc.AcceptedScheduleInventory = value
	}
}

func (sc serviceConfig) environmentExplicit(key string) bool {
	if sc.explicitEnvironment != nil {
		return sc.explicitEnvironment[key]
	}
	return sc.environmentValue(key) != ""
}

// absUnderHome anchors a relative config path to home. An already-absolute path
// (or an empty one) passes through untouched.
func absUnderHome(home, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(home, p)
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func writePlist(home string, sc serviceConfig) error {
	dir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if sc.LogDir != "" {
		_ = os.MkdirAll(sc.LogDir, 0o755)
	}
	return os.WriteFile(plistPath(home, sc.Label), []byte(renderPlist(sc)), 0o644)
}

func removePlist(home, label string) error {
	err := os.Remove(plistPath(home, label))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// buildServiceConfig assembles the serviceConfig for this machine from the
// running fort-core config (Addr/DBPath), the current executable's path, and
// the standard macOS log location.
func buildServiceConfig() (serviceConfig, error) {
	cfg := config.Load(os.Getenv)
	bin, err := os.Executable()
	if err != nil {
		return serviceConfig{}, fmt.Errorf("service: resolving binary path: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return serviceConfig{}, fmt.Errorf("service: resolving home directory: %w", err)
	}
	explicitEnvironment := make(map[string]bool, len(serviceEnvironmentKeys))
	for _, key := range serviceEnvironmentKeys {
		if key == "PATH" {
			continue
		}
		_, explicitEnvironment[key] = os.LookupEnv(key)
	}
	return serviceConfig{
		Label:   defaultServiceLabel,
		BinPath: bin,
		Args:    []string{"serve"},
		Addr:    cfg.Addr,
		// Relative config paths resolve against launchd's read-only "/" cwd, so
		// anchor them to $HOME and give the agent a writable WorkingDirectory.
		DBPath:                    absUnderHome(home, cfg.DBPath),
		WorkRoot:                  absUnderHome(home, cfg.WorkRoot),
		WorkDir:                   home,
		Path:                      os.Getenv("PATH"), // inherit the installing shell's PATH (agent CLI discovery)
		RulesPath:                 os.Getenv("FORT_RULES"),
		FlowsPath:                 os.Getenv("FORT_FLOWS"),
		CapabilityPlanning:        os.Getenv("FORT_CAPABILITY_PLANNING"),
		DisplayTimezone:           cfg.DisplayTimezone,
		MachinesPath:              os.Getenv("FORT_MACHINES"),
		NodeName:                  os.Getenv("FORT_NODE_NAME"),
		PrimaryChannels:           os.Getenv("FORT_PRIMARY_CHANNELS"),
		AgentChannels:             os.Getenv("FORT_AGENT_CHANNELS"),
		AcceptedScheduleInventory: os.Getenv("FORT_ACCEPTED_SCHEDULE_INVENTORY"),
		explicitEnvironment:       explicitEnvironment,
		LogDir:                    filepath.Join(home, "Library", "Logs", "Fort"),
	}, nil
}

func validateServiceDisplayTimezone(sc serviceConfig) error {
	if _, err := (config.Config{DisplayTimezone: sc.DisplayTimezone}).DisplayLocation(); err != nil {
		return fmt.Errorf("service: %w", err)
	}
	return nil
}

// cmdService dispatches `fort service <verb>` (spec 032): the launchd
// daemon-lifecycle manager. The pure plist/path logic above is cross-platform
// testable; the actual launchctl invocations only run on darwin (a Fort
// daemon that self-manages via launchd only makes sense on macOS).
func cmdService(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: fort service <install|start|stop|restart|status|uninstall>")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("service: resolving home directory: %w", err)
	}
	sc, err := buildServiceConfig()
	if err != nil {
		return err
	}
	switch args[0] {
	case "install":
		if err := validateServiceDisplayTimezone(sc); err != nil {
			return err
		}
		return svcInstall(home, sc)
	case "start":
		return svcStart(sc)
	case "stop":
		return svcStop(sc)
	case "restart":
		if err := validateServiceDisplayTimezone(sc); err != nil {
			return err
		}
		if runtime.GOOS == "darwin" {
			if err := prepareServiceRestart(home, sc); err != nil {
				return err
			}
		}
		return svcRestart(home, sc)
	case "status":
		return svcStatus(sc)
	case "uninstall":
		return svcUninstall(home, sc)
	default:
		return fmt.Errorf("usage: fort service <install|start|stop|restart|status|uninstall>; unknown subcommand %q", args[0])
	}
}

// parseServiceEnvironment reads only Fort's closed launchd environment
// contract. Unknown keys are intentionally discarded instead of being copied
// into a refreshed service definition.
func parseServiceEnvironment(raw []byte) (map[string]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	result := make(map[string]string, len(serviceEnvironmentKeys))
	var currentKey string
	pendingEnvironmentDictionary := false
	inEnvironmentDictionary := false
	environmentDepth := 0

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return nil, fmt.Errorf("parse service plist: %w", err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "key":
				var key string
				if err := decoder.DecodeElement(&key, &element); err != nil {
					return nil, fmt.Errorf("parse service plist key: %w", err)
				}
				if !inEnvironmentDictionary && key == "EnvironmentVariables" {
					pendingEnvironmentDictionary = true
				} else if inEnvironmentDictionary && environmentDepth == 1 {
					currentKey = key
				}
			case "dict":
				if pendingEnvironmentDictionary {
					pendingEnvironmentDictionary = false
					inEnvironmentDictionary = true
					environmentDepth = 1
				} else if inEnvironmentDictionary {
					environmentDepth++
				}
			case "string":
				if !inEnvironmentDictionary || environmentDepth != 1 || currentKey == "" {
					continue
				}
				var value string
				if err := decoder.DecodeElement(&value, &element); err != nil {
					return nil, fmt.Errorf("parse service plist value: %w", err)
				}
				for _, allowed := range serviceEnvironmentKeys {
					if currentKey == allowed {
						result[currentKey] = value
						break
					}
				}
				currentKey = ""
			}
		case xml.EndElement:
			if inEnvironmentDictionary && element.Name.Local == "dict" {
				environmentDepth--
				if environmentDepth == 0 {
					return result, nil
				}
			}
		}
	}
}

func preserveServiceEnvironment(home string, sc serviceConfig) (serviceConfig, error) {
	raw, err := os.ReadFile(plistPath(home, sc.Label))
	if os.IsNotExist(err) {
		return sc, nil
	}
	if err != nil {
		return serviceConfig{}, fmt.Errorf("read existing service plist: %w", err)
	}
	existing, err := parseServiceEnvironment(raw)
	if err != nil {
		return serviceConfig{}, err
	}
	for _, key := range serviceEnvironmentKeys {
		if sc.environmentExplicit(key) {
			continue
		}
		if value, ok := existing[key]; ok {
			sc.setEnvironmentValue(key, value)
		}
	}
	return sc, nil
}

func prepareServiceDefinition(home string, sc serviceConfig) error {
	preserved, err := preserveServiceEnvironment(home, sc)
	if err != nil {
		return err
	}
	if err := validateServiceDisplayTimezone(preserved); err != nil {
		return err
	}
	return writePlist(home, preserved)
}

func prepareServiceInstall(home string, sc serviceConfig) error {
	if err := prepareServiceDefinition(home, sc); err != nil {
		return fmt.Errorf("service install: refreshing plist: %w", err)
	}
	return nil
}

// prepareServiceRestart refreshes the installed definition before kickstart.
// Explicit new values win; otherwise the prior closed operational environment
// survives while the bundled binary path advances.
func prepareServiceRestart(home string, sc serviceConfig) error {
	if err := prepareServiceDefinition(home, sc); err != nil {
		return fmt.Errorf("service restart: refreshing plist: %w", err)
	}
	return nil
}

func svcInstall(home string, sc serviceConfig) error {
	if err := prepareServiceInstall(home, sc); err != nil {
		return err
	}
	if runtime.GOOS != "darwin" {
		fmt.Println("service install: plist written; launchctl unsupported on", runtime.GOOS)
		return nil
	}
	out, err := exec.Command("launchctl", "bootstrap", guiTarget(), plistPath(home, sc.Label)).CombinedOutput()
	if err != nil && !strings.Contains(string(out), "already bootstrapped") {
		return fmt.Errorf("service install: launchctl bootstrap: %w: %s", err, strings.TrimSpace(string(out)))
	}
	fmt.Println("service install: installed and started", sc.Label)
	return nil
}

func svcStart(sc serviceConfig) error {
	if runtime.GOOS != "darwin" {
		return unsupportedOS("start")
	}
	// `stop` boots the agent OUT of the domain, so kickstart alone then fails with
	// "Could not find service ... in domain". Re-bootstrap the plist first (a
	// no-op when it is already loaded), then kickstart.
	if home, err := os.UserHomeDir(); err == nil {
		out, err := exec.Command("launchctl", "bootstrap", guiTarget(), plistPath(home, sc.Label)).CombinedOutput()
		if err != nil && !strings.Contains(string(out), "already bootstrapped") && !strings.Contains(string(out), "Bootstrap failed: 37") {
			// Not fatal: the agent may already be loaded. kickstart below decides.
			_ = out
		}
	}
	out, err := exec.Command("launchctl", "kickstart", guiLabelTarget(sc.Label)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("service start: launchctl kickstart: %w: %s (is it installed? run `fort service install`)", err, strings.TrimSpace(string(out)))
	}
	fmt.Println("service start: started", sc.Label)
	return nil
}

func svcStop(sc serviceConfig) error {
	if runtime.GOOS != "darwin" {
		return unsupportedOS("stop")
	}
	out, err := exec.Command("launchctl", "bootout", guiLabelTarget(sc.Label)).CombinedOutput()
	if err != nil && !strings.Contains(string(out), "not loaded") && !strings.Contains(string(out), "Could not find") {
		return fmt.Errorf("service stop: launchctl bootout: %w: %s", err, strings.TrimSpace(string(out)))
	}
	fmt.Println("service stop: stopped", sc.Label)
	return nil
}

func serviceRestartCommands(home string, sc serviceConfig) [][]string {
	return [][]string{
		{"launchctl", "bootout", guiLabelTarget(sc.Label)},
		{"launchctl", "bootstrap", guiTarget(), plistPath(home, sc.Label)},
		{"launchctl", "kickstart", guiLabelTarget(sc.Label)},
	}
}

type serviceCommandRunner func(command []string) ([]byte, error)

// runServiceRestart tolerates launchd's short teardown window after bootout.
// On macOS, an immediate bootstrap can transiently fail with errno 5 while the
// previous job is still leaving the GUI domain. Only that exact bootstrap
// failure is retried; every other launchctl error still fails closed.
func runServiceRestart(home string, sc serviceConfig, run serviceCommandRunner, wait func(time.Duration)) error {
	for index, command := range serviceRestartCommands(home, sc) {
		for attempt := 1; ; attempt++ {
			out, err := run(command)
			if index == 0 && err != nil && (strings.Contains(string(out), "not loaded") || strings.Contains(string(out), "Could not find")) {
				break
			}
			if err == nil {
				break
			}
			if index == 1 &&
				strings.Contains(string(out), "Bootstrap failed: 5: Input/output error") &&
				attempt < serviceRestartBootstrapAttempts {
				wait(serviceRestartRetryDelay)
				continue
			}
			return fmt.Errorf("service restart: %s: %w: %s", strings.Join(command, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func svcRestart(home string, sc serviceConfig) error {
	if runtime.GOOS != "darwin" {
		return unsupportedOS("restart")
	}
	if err := runServiceRestart(home, sc, func(command []string) ([]byte, error) {
		return exec.Command(command[0], command[1:]...).CombinedOutput()
	}, time.Sleep); err != nil {
		return err
	}
	fmt.Println("service restart: restarted", sc.Label)
	return nil
}

func svcUninstall(home string, sc serviceConfig) error {
	if err := svcStop(sc); err != nil {
		// stop's job is best-effort here; uninstall should still remove the
		// plist even if the agent was never running.
		fmt.Println("service uninstall:", err)
	}
	if err := removePlist(home, sc.Label); err != nil {
		return fmt.Errorf("service uninstall: removing plist: %w", err)
	}
	fmt.Println("service uninstall: removed", sc.Label)
	return nil
}

// svcStatus reports "running" or "stopped" (grepped verbatim by the FortKit
// ServiceController) plus the configured address. It prefers launchctl on
// darwin, and falls back to an HTTP probe of the daemon's /api/summary — the
// only signal available on other platforms or when launchctl's answer is
// inconclusive (e.g. the agent isn't installed but the daemon is running
// some other way).
func svcStatus(sc serviceConfig) error {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("launchctl", "print", guiLabelTarget(sc.Label)).CombinedOutput()
		if err == nil && strings.Contains(string(out), "state = running") {
			fmt.Printf("running (%s, launchctl)\n", sc.Addr)
			return nil
		}
	}
	if httpProbeRunning(sc.Addr) {
		fmt.Printf("running (%s, http)\n", sc.Addr)
		return nil
	}
	fmt.Printf("stopped (%s)\n", sc.Addr)
	return nil
}

// httpProbeRunning reports whether a GET of /api/summary against addr
// succeeds — the fallback liveness check when launchctl isn't available or
// doesn't recognize the label.
func httpProbeRunning(addr string) bool {
	if addr == "" {
		return false
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/api/summary")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func unsupportedOS(verb string) error {
	return fmt.Errorf("service %s: unsupported on %s (launchctl is darwin-only)", verb, runtime.GOOS)
}

func guiTarget() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func guiLabelTarget(label string) string {
	return fmt.Sprintf("gui/%d/%s", os.Getuid(), label)
}
