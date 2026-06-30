package rules

import (
	"strings"
	"testing"
)

const validRuleset = `
version: 1
defaults:
  route: claude
rules:
  - id: dev-to-codex
    when:
      any:
        - label: [dev, feature, bug]
    route: codex
  - id: design-to-claude
    when:
      label: [design, chat]
    route: claude
  - id: night-errands
    when:
      all:
        - label: [errand]
        - time:
            after: "18:00"
            before: "06:00"
    route: openclaw
`

func TestParseValidRuleset(t *testing.T) {
	rs, err := Parse([]byte(validRuleset))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if rs.Version != 1 {
		t.Errorf("version = %d, want 1", rs.Version)
	}
	if rs.Defaults.Route != "claude" {
		t.Errorf("defaults.route = %q, want claude", rs.Defaults.Route)
	}
	if len(rs.Rules) != 3 {
		t.Fatalf("len(rules) = %d, want 3", len(rs.Rules))
	}
	if rs.Rules[0].ID != "dev-to-codex" || rs.Rules[0].Route != "codex" {
		t.Errorf("rule[0] = %+v", rs.Rules[0])
	}
	if len(rs.Rules[0].When.Any) != 1 {
		t.Errorf("rule[0].when.any len = %d, want 1", len(rs.Rules[0].When.Any))
	}
	if got := rs.Rules[0].When.Any[0].Label; len(got) != 3 {
		t.Errorf("rule[0].when.any[0].label = %v, want 3 labels", got)
	}
	// Leaf matcher (no any/all wrapper) parses too.
	if got := rs.Rules[1].When.Label; len(got) != 2 || got[0] != "design" {
		t.Errorf("rule[1].when.label = %v, want [design chat]", got)
	}
	// Nested all + time window.
	all := rs.Rules[2].When.All
	if len(all) != 2 {
		t.Fatalf("rule[2].when.all len = %d, want 2", len(all))
	}
	if all[1].Time == nil || all[1].Time.After != "18:00" || all[1].Time.Before != "06:00" {
		t.Errorf("rule[2] time window = %+v", all[1].Time)
	}
}

func TestParseRejectsInvalidYAML(t *testing.T) {
	_, err := Parse([]byte("version: 1\nrules: [oops\n"))
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

func TestParseRejectsMissingRoute(t *testing.T) {
	bad := `
version: 1
defaults:
  route: claude
rules:
  - id: no-route
    when:
      label: [dev]
`
	_, err := Parse([]byte(bad))
	if err == nil {
		t.Fatal("expected error for rule missing route, got nil")
	}
	if !strings.Contains(err.Error(), "no-route") {
		t.Errorf("error should name the offending rule, got: %v", err)
	}
}

func TestParseRejectsMissingDefaultRoute(t *testing.T) {
	bad := `
version: 1
rules:
  - id: r1
    when:
      label: [dev]
    route: codex
`
	_, err := Parse([]byte(bad))
	if err == nil {
		t.Fatal("expected error for missing defaults.route, got nil")
	}
}

func TestParseRejectsUnknownVersion(t *testing.T) {
	bad := `
version: 99
defaults:
  route: claude
rules: []
`
	_, err := Parse([]byte(bad))
	if err == nil {
		t.Fatal("expected error for unknown version, got nil")
	}
}
