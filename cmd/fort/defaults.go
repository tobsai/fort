package main

import _ "embed"

// defaultRulesYAML is a build-time copy of rules/v1.yaml embedded into the
// binary so `brew install fort && fort serve` works from any directory, with no
// checked-out repo. It is used only when the configured ruleset file is missing
// AND the path is the default (an explicit FORT_RULES pointing at a missing file
// is still an error). TestEmbeddedRulesMatchCanonical guards against drift.
//
//go:embed defaults/rules.yaml
var defaultRulesYAML []byte
