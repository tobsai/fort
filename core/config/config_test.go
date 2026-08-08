package config

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultsAreSane(t *testing.T) {
	c := Default()
	if c.Addr == "" || c.DBPath == "" || c.RulesPath == "" || c.WorkRoot == "" {
		t.Errorf("default config has empty fields: %+v", c)
	}
}

func TestFromEnvOverrides(t *testing.T) {
	env := map[string]string{
		"FORT_ADDR":             "0.0.0.0:9999",
		"FORT_DB":               "/tmp/x.db",
		"FORT_RULES":            "rules/custom.yaml",
		"FORT_WORKROOT":         "/tmp/work",
		"FORT_DISPLAY_TIMEZONE": "America/Chicago",
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
	if c.DisplayTimezone != "America/Chicago" {
		t.Errorf("display timezone = %q", c.DisplayTimezone)
	}
}

func TestFromEnvFallsBackToDefaults(t *testing.T) {
	c := FromEnv(func(string) string { return "" })
	if c != Default() {
		t.Errorf("empty env should equal defaults: %+v vs %+v", c, Default())
	}
}

func TestDisplayLocationUsesOneValidatedIANAZone(t *testing.T) {
	readlink := func(string) (string, error) {
		return "/var/db/timezone/zoneinfo/America/Chicago", nil
	}
	readFile := func(string) ([]byte, error) { return nil, errors.New("unused") }

	location, err := displayLocation("", func(string) string { return "" }, readlink, readFile, time.FixedZone("Local", -6*60*60))
	if err != nil {
		t.Fatal(err)
	}
	if got := location.String(); got != "America/Chicago" {
		t.Fatalf("host display timezone = %q", got)
	}

	location, err = displayLocation("Europe/London", func(string) string { return "" }, func(string) (string, error) {
		return "", errors.New("unused")
	}, readFile, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if got := location.String(); got != "Europe/London" {
		t.Fatalf("configured display timezone = %q", got)
	}

	if _, err := displayLocation("Not/A_Zone", func(string) string { return "" }, readlink, readFile, time.Local); err == nil {
		t.Fatal("invalid configured display timezone was accepted")
	}
	if _, err := displayLocation("Local", func(string) string { return "" }, readlink, readFile, time.Local); err == nil {
		t.Fatal("process-local pseudo-zone was accepted as a shared IANA timezone")
	}
	if _, err := displayLocation("", func(string) string { return "" }, func(string) (string, error) {
		return "", errors.New("missing")
	}, readFile, time.FixedZone("Local", -6*60*60)); err == nil {
		t.Fatal("unresolved host timezone silently chose a different day boundary")
	}
}

func TestDisplayLocationMatchesCopiedUTCLocaltime(t *testing.T) {
	readFile := func(path string) ([]byte, error) {
		switch path {
		case "/etc/localtime", "/usr/share/zoneinfo/Etc/UTC":
			return []byte("copied UTC TZif"), nil
		default:
			return nil, errors.New("missing")
		}
	}
	location, err := displayLocation("", func(string) string { return "" }, func(string) (string, error) {
		return "", errors.New("not a symlink")
	}, readFile, time.FixedZone("Local", 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := location.String(); got != "Etc/UTC" {
		t.Fatalf("copied UTC display timezone = %q", got)
	}
}

func TestDisplayLocationMatchesCopiedLocaltimeToCanonicalIANAZone(t *testing.T) {
	readFile := func(path string) ([]byte, error) {
		switch path {
		case "/etc/localtime":
			return []byte("copied Chicago TZif"), nil
		case "/usr/share/zoneinfo/zone1970.tab":
			return []byte("# country coordinates zone\nUS\t+415100-0873900\tAmerica/Chicago\n"), nil
		case "/usr/share/zoneinfo/America/Chicago":
			return []byte("copied Chicago TZif"), nil
		default:
			return nil, errors.New("missing")
		}
	}
	location, err := displayLocation("", func(string) string { return "" }, func(string) (string, error) {
		return "", errors.New("not a symlink")
	}, readFile, time.FixedZone("Local", -6*60*60))
	if err != nil {
		t.Fatal(err)
	}
	if got := location.String(); got != "America/Chicago" {
		t.Fatalf("copied localtime display timezone = %q", got)
	}
}

func TestDisplayLocationMatchesCopiedZoneAcrossSplitZoneinfoRoots(t *testing.T) {
	readFile := func(path string) ([]byte, error) {
		switch path {
		case "/etc/localtime":
			return []byte("copied Chicago TZif"), nil
		case "/usr/share/zoneinfo/zone1970.tab", "/usr/share/lib/zoneinfo/zone1970.tab":
			return []byte("US\t+415100-0873900\tAmerica/Chicago\n"), nil
		case "/usr/share/lib/zoneinfo/America/Chicago":
			return []byte("copied Chicago TZif"), nil
		default:
			return nil, errors.New("missing")
		}
	}
	location, err := displayLocation("", func(string) string { return "" }, func(string) (string, error) {
		return "", errors.New("not a symlink")
	}, readFile, time.FixedZone("Local", -6*60*60))
	if err != nil {
		t.Fatal(err)
	}
	if got := location.String(); got != "America/Chicago" {
		t.Fatalf("split-root display timezone = %q", got)
	}
}
