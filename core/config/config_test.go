package config

import "testing"

func TestDefaultsAreSane(t *testing.T) {
	c := Default()
	if c.Addr == "" || c.DBPath == "" || c.RulesPath == "" || c.WorkRoot == "" {
		t.Errorf("default config has empty fields: %+v", c)
	}
}

func TestFromEnvOverrides(t *testing.T) {
	env := map[string]string{
		"FORT_ADDR":     "0.0.0.0:9999",
		"FORT_DB":       "/tmp/x.db",
		"FORT_RULES":    "rules/custom.yaml",
		"FORT_WORKROOT": "/tmp/work",
	}
	c := FromEnv(func(k string) string { return env[k] })
	if c.Addr != "0.0.0.0:9999" {
		t.Errorf("addr = %q", c.Addr)
	}
	if c.DBPath != "/tmp/x.db" {
		t.Errorf("db = %q", c.DBPath)
	}
	if c.RulesPath != "rules/custom.yaml" {
		t.Errorf("rules = %q", c.RulesPath)
	}
	if c.WorkRoot != "/tmp/work" {
		t.Errorf("workroot = %q", c.WorkRoot)
	}
}

func TestFromEnvFallsBackToDefaults(t *testing.T) {
	c := FromEnv(func(string) string { return "" })
	if c != Default() {
		t.Errorf("empty env should equal defaults: %+v vs %+v", c, Default())
	}
}
