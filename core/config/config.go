// Package config loads fort-core configuration from the environment with sane
// defaults (backlog AO-011).
package config

// Config is the fort-core runtime configuration.
type Config struct {
	Addr      string // HTTP/WS bind address
	DBPath    string // SQLite state-store path
	RulesPath string // active routing ruleset
	WorkRoot  string // scoped root all native runtimes execute under
	// DisplayTimezone is the optional IANA timezone shared by every Today
	// client. Empty resolves from the daemon host at startup.
	DisplayTimezone string

	// Multi-machine orchestration (spec 022). All optional: with MachinesPath
	// empty, Fort is single-machine and behaves exactly as before.
	NodeName     string // this machine's identity in the registry (default: hostname)
	MachinesPath string // path to machines.yaml ("" = single-machine)
	NodeToken    string // shared bearer token for inter-Fort /api/exec calls

	// MachinesManaged is true when MachinesPath points at the Fort-managed
	// registry in the data dir (spec 024). Enrollment only ever writes the
	// managed file; an operator-set FORT_MACHINES is never touched.
	MachinesManaged bool
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{
		Addr:      "127.0.0.1:4087",
		DBPath:    ".fort-native/fort.db",
		RulesPath: "rules/v1.yaml",
		WorkRoot:  ".fort-native/work",
	}
}

// FromEnv layers FORT_* environment overrides over the defaults. getenv is
// injectable for testing (pass os.Getenv in production).
func FromEnv(getenv func(string) string) Config {
	c := Default()
	if v := getenv("FORT_ADDR"); v != "" {
		c.Addr = v
	}
	if v := getenv("FORT_DB"); v != "" {
		c.DBPath = v
	}
	if v := getenv("FORT_RULES"); v != "" {
		c.RulesPath = v
	}
	if v := getenv("FORT_WORKROOT"); v != "" {
		c.WorkRoot = v
	}
	if v := getenv("FORT_DISPLAY_TIMEZONE"); v != "" {
		c.DisplayTimezone = v
	}
	// NodeName defaults to the OS hostname, resolved at the composition root
	// (cmd/fort) so config stays a pure function of getenv (and testable).
	if v := getenv("FORT_NODE_NAME"); v != "" {
		c.NodeName = v
	}
	if v := getenv("FORT_MACHINES"); v != "" {
		c.MachinesPath = v
	}
	if v := getenv("FORT_NODE_TOKEN"); v != "" {
		c.NodeToken = v
	}
	return c
}
