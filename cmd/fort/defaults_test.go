package main

import (
	"os"
	"testing"

	"github.com/tobsai/fort/core/rules"
)

// TestEmbeddedRulesMatchCanonical keeps cmd/fort/defaults/rules.yaml (embedded
// for brew installs) byte-identical to the canonical rules/v1.yaml. If this
// fails, re-copy: `cp rules/v1.yaml cmd/fort/defaults/rules.yaml`.
func TestEmbeddedRulesMatchCanonical(t *testing.T) {
	canonical, err := os.ReadFile("../../rules/v1.yaml")
	if err != nil {
		t.Fatalf("read canonical rules: %v", err)
	}
	if string(defaultRulesYAML) != string(canonical) {
		t.Fatalf("embedded default rules drifted from rules/v1.yaml; re-copy it")
	}
}

// TestEmbeddedRulesParse proves the embedded fallback is a valid ruleset.
func TestEmbeddedRulesParse(t *testing.T) {
	if _, err := rules.Parse(defaultRulesYAML); err != nil {
		t.Fatalf("embedded default ruleset is invalid: %v", err)
	}
}
