// Package rules defines Fort's deterministic routing-rule schema and a strict
// parser for it (backlog AO-012). A ruleset is ordered; the matcher engine
// (package router) applies first-match-wins with a default fallback.
//
// No model/LLM calls ever happen here — routing reads only deterministic
// signals on a task.
package rules

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// SupportedVersion is the only ruleset schema version this build understands.
const SupportedVersion = 1

// Ruleset is a parsed, validated routing ruleset.
type Ruleset struct {
	Version  int      `yaml:"version"`
	Defaults Defaults `yaml:"defaults"`
	Rules    []Rule   `yaml:"rules"`
}

// Defaults holds the fallback route used when no rule matches.
type Defaults struct {
	Route string `yaml:"route"`
}

// Rule is one ordered routing rule: when When matches a task, route to Route.
type Rule struct {
	ID    string  `yaml:"id"`
	When  Matcher `yaml:"when"`
	Route string  `yaml:"route"`
}

// Matcher is either a leaf (one or more of the signal fields, ANDed together)
// or a composition (Any / All of sub-matchers). A leaf with multiple fields
// requires all of them to hold.
type Matcher struct {
	// Leaf signals.
	Label []string    `yaml:"label,omitempty"`
	Path  []string    `yaml:"path,omitempty"`
	Repo  string      `yaml:"repo,omitempty"`
	Agent string      `yaml:"agent,omitempty"`
	Size  []string    `yaml:"size,omitempty"`
	Time  *TimeWindow `yaml:"time,omitempty"`

	// Composition.
	Any []Matcher `yaml:"any,omitempty"`
	All []Matcher `yaml:"all,omitempty"`
}

// TimeWindow is a time-of-day window [After, Before) in 24h "HH:MM". When
// After > Before the window wraps midnight (e.g. 18:00–06:00).
type TimeWindow struct {
	After  string `yaml:"after"`
	Before string `yaml:"before"`
}

// Parse decodes and validates a ruleset. YAML syntax errors are returned with
// line information from the underlying decoder; semantic errors name the
// offending rule.
func Parse(data []byte) (*Ruleset, error) {
	var rs Ruleset
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return nil, fmt.Errorf("ruleset: invalid YAML: %w", err)
	}
	if err := rs.validate(); err != nil {
		return nil, err
	}
	return &rs, nil
}

func (rs *Ruleset) validate() error {
	if rs.Version != SupportedVersion {
		return fmt.Errorf("ruleset: unsupported version %d (want %d)", rs.Version, SupportedVersion)
	}
	if rs.Defaults.Route == "" {
		return fmt.Errorf("ruleset: defaults.route is required")
	}
	for i, r := range rs.Rules {
		who := r.ID
		if who == "" {
			who = fmt.Sprintf("rules[%d]", i)
		}
		if r.Route == "" {
			return fmt.Errorf("ruleset: rule %q is missing a route", who)
		}
		if err := r.When.validate(who); err != nil {
			return err
		}
	}
	return nil
}

// validate ensures a matcher has at least one condition.
func (m *Matcher) validate(rule string) error {
	if m.empty() {
		return fmt.Errorf("ruleset: rule %q has an empty when matcher", rule)
	}
	for _, sub := range m.Any {
		if err := sub.validate(rule); err != nil {
			return err
		}
	}
	for _, sub := range m.All {
		if err := sub.validate(rule); err != nil {
			return err
		}
	}
	return nil
}

func (m *Matcher) empty() bool {
	return len(m.Label) == 0 && len(m.Path) == 0 && m.Repo == "" &&
		m.Agent == "" && len(m.Size) == 0 && m.Time == nil &&
		len(m.Any) == 0 && len(m.All) == 0
}
