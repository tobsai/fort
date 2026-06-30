// Package config loads fort-core configuration from the environment with sane
// defaults (backlog AO-011).
package config

// Config is the fort-core runtime configuration.
type Config struct {
	Addr      string // HTTP/WS bind address
	DBPath    string // SQLite state-store path
	RulesPath string // active routing ruleset
	WorkRoot  string // scoped root all native runtimes execute under
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
	return c
}
