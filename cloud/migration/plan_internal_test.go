package migration

import "testing"

func TestEveryReadyImportRuleHasCanonicalParityProjection(t *testing.T) {
	t.Parallel()

	projected := make(map[string]bool, len(parityMappings))
	for _, mapping := range parityMappings {
		projected[mapping.source] = true
	}
	for table, rule := range postgresImportRules {
		if rule.class == MappingReady && !projected[table] {
			t.Errorf("ready table %q has no canonical parity projection", table)
		}
	}
}
