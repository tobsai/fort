// fort service manages the fort daemon as a launchd user agent on macOS
// (spec 032): install/start/stop/restart/status/uninstall, so the Mac app can
// shell out to a single Go-tested subcommand rather than reimplementing
// launchd plumbing in Swift.
package main

import (
	"errors"
	"fmt"
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

// serviceConfig is the launchd user-agent definition for the Fort daemon.
type serviceConfig struct {
	Label   string
	BinPath string
	Args    []string
	Addr    string
	DBPath  string
	LogDir  string
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
	if sc.Addr != "" {
		b.WriteString("    <key>FORT_ADDR</key>\n    <string>" + xmlEscape(sc.Addr) + "</string>\n")
	}
	if sc.DBPath != "" {
		b.WriteString("    <key>FORT_DB</key>\n    <string>" + xmlEscape(sc.DBPath) + "</string>\n")
	}
	b.WriteString("  </dict>\n")
	b.WriteString("  <key>RunAtLoad</key>\n  <true/>\n")
	b.WriteString("  <key>KeepAlive</key>\n  <true/>\n")
	if sc.LogDir != "" {
		b.WriteString("  <key>StandardOutPath</key>\n  <string>" + xmlEscape(filepath.Join(sc.LogDir, "fort.out.log")) + "</string>\n")
		b.WriteString("  <key>StandardErrorPath</key>\n  <string>" + xmlEscape(filepath.Join(sc.LogDir, "fort.err.log")) + "</string>\n")
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.String()
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
	return serviceConfig{
		Label:   defaultServiceLabel,
		BinPath: bin,
		Args:    []string{"serve"},
		Addr:    cfg.Addr,
		DBPath:  cfg.DBPath,
		LogDir:  filepath.Join(home, "Library", "Logs", "Fort"),
	}, nil
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
		return svcInstall(home, sc)
	case "start":
		return svcStart(sc)
	case "stop":
		return svcStop(sc)
	case "restart":
		return svcRestart(sc)
	case "status":
		return svcStatus(sc)
	case "uninstall":
		return svcUninstall(home, sc)
	default:
		return fmt.Errorf("usage: fort service <install|start|stop|restart|status|uninstall>; unknown subcommand %q", args[0])
	}
}

func svcInstall(home string, sc serviceConfig) error {
	if err := writePlist(home, sc); err != nil {
		return fmt.Errorf("service install: writing plist: %w", err)
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
	out, err := exec.Command("launchctl", "kickstart", guiLabelTarget(sc.Label)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("service start: launchctl kickstart: %w: %s", err, strings.TrimSpace(string(out)))
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

func svcRestart(sc serviceConfig) error {
	if runtime.GOOS != "darwin" {
		return unsupportedOS("restart")
	}
	out, err := exec.Command("launchctl", "kickstart", "-k", guiLabelTarget(sc.Label)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("service restart: launchctl kickstart -k: %w: %s", err, strings.TrimSpace(string(out)))
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
