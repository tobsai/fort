package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DisplayLocation resolves the one IANA timezone used by every Today client.
// An explicit FORT_DISPLAY_TIMEZONE is authoritative; otherwise Fort derives
// the host's zone from TZ or the standard /etc/localtime link.
func (c Config) DisplayLocation() (*time.Location, error) {
	return displayLocation(c.DisplayTimezone, os.Getenv, os.Readlink, os.ReadFile, time.Local)
}

func displayLocation(configured string, getenv func(string) string, readlink func(string) (string, error), readFile func(string) ([]byte, error), local *time.Location) (*time.Location, error) {
	if name := strings.TrimSpace(configured); name != "" {
		return loadDisplayLocation(name)
	}
	if name := strings.TrimPrefix(strings.TrimSpace(getenv("TZ")), ":"); name != "" {
		if pathName := ianaNameFromZoneinfoPath(name); pathName != "" {
			name = pathName
		}
		if location, err := loadNamedLocation(name); err == nil {
			return location, nil
		}
	}
	if target, err := readlink("/etc/localtime"); err == nil {
		if name := ianaNameFromZoneinfoPath(target); name != "" {
			if location, loadErr := loadNamedLocation(name); loadErr == nil {
				return location, nil
			}
		}
	}
	if local != nil && local.String() != "" && local.String() != "Local" {
		if location, err := loadNamedLocation(local.String()); err == nil {
			return location, nil
		}
	}
	if name := matchCopiedLocaltime(readFile); name != "" {
		if location, err := loadNamedLocation(name); err == nil {
			return location, nil
		}
	}
	if data, err := readFile("/etc/timezone"); err == nil {
		if name := strings.TrimSpace(string(data)); name != "" {
			if location, loadErr := loadNamedLocation(name); loadErr == nil {
				return location, nil
			}
		}
	}
	return nil, fmt.Errorf("config: cannot resolve host IANA display timezone; set FORT_DISPLAY_TIMEZONE")
}

func ianaNameFromZoneinfoPath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if marker := strings.LastIndex(path, "/zoneinfo/"); marker >= 0 {
		return strings.TrimPrefix(path[marker+len("/zoneinfo/"):], "posix/")
	}
	return ""
}

var zoneinfoRoots = []string{
	"/usr/share/zoneinfo",
	"/usr/share/lib/zoneinfo",
	"/usr/lib/locale/TZ",
	"/etc/zoneinfo",
}

// matchCopiedLocaltime handles hosts that copy rather than symlink the TZif
// file. The standard zone tables bound the search to canonical IANA names.
func matchCopiedLocaltime(readFile func(string) ([]byte, error)) string {
	localData, err := readFile("/etc/localtime")
	if err != nil || len(localData) == 0 {
		return ""
	}
	seen := map[string]bool{}
	for _, root := range zoneinfoRoots {
		if utcData, utcErr := readFile(filepath.Join(root, "Etc", "UTC")); utcErr == nil && bytes.Equal(localData, utcData) {
			return "Etc/UTC"
		}
		for _, table := range []string{"zone1970.tab", "zone.tab"} {
			data, tableErr := readFile(filepath.Join(root, table))
			if tableErr != nil {
				continue
			}
			for _, line := range strings.Split(string(data), "\n") {
				fields := strings.Fields(line)
				if len(fields) < 3 || strings.HasPrefix(fields[0], "#") {
					continue
				}
				name := fields[2]
				zonePath := filepath.Join(root, filepath.FromSlash(name))
				if seen[zonePath] {
					continue
				}
				seen[zonePath] = true
				zoneData, zoneErr := readFile(zonePath)
				if zoneErr == nil && bytes.Equal(localData, zoneData) {
					return name
				}
			}
		}
	}
	return ""
}

func loadDisplayLocation(name string) (*time.Location, error) {
	location, err := loadNamedLocation(name)
	if err != nil {
		return nil, fmt.Errorf("config: display timezone %q: %w", name, err)
	}
	return location, nil
}

func loadNamedLocation(name string) (*time.Location, error) {
	if strings.TrimSpace(name) == "Local" {
		return nil, fmt.Errorf("Local is process-relative, not an IANA timezone")
	}
	return time.LoadLocation(name)
}
