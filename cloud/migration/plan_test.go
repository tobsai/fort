package migration_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tobsai/fort/cloud/migration"
)

func TestDryRunClassifiesEveryRowAndFailsClosedOnChoicesOrIncompatibility(t *testing.T) {
	t.Parallel()

	key := bytes.Repeat([]byte{0x42}, migration.ArchiveKeyBytes)
	archive := migrationArchiveFixture(t, key, []migration.Table{
		oneTextRow("stable_agent", "agent:1"),
		oneTextRow("schedule", "schedule:1"),
		oneTextRow("run", "run:legacy"),
		oneTextRow("surprise_table", "unknown:1"),
	})
	report := migration.PlanPostgresImport(archive)
	if report.TotalRows != 4 || len(report.Rows) != 4 {
		t.Fatalf("dry-run coverage = total %d decisions %d, want 4/4", report.TotalRows, len(report.Rows))
	}
	if report.Counts[migration.MappingReady] != 1 || report.Counts[migration.MappingNeedsExplicitChoice] != 1 ||
		report.Counts[migration.MappingLegacyRetained] != 1 || report.Counts[migration.MappingIncompatible] != 1 {
		t.Fatalf("classification counts = %#v", report.Counts)
	}
	if report.Resolved {
		t.Fatal("dry-run resolved despite explicit and incompatible mappings")
	}
	if err := report.RequireResolved(); !errors.Is(err, migration.ErrUnresolvedMappings) {
		t.Fatalf("RequireResolved error = %v, want unresolved mappings", err)
	}
	for _, decision := range report.Rows {
		if decision.RecordDigest == "" || decision.SourceTable == "" || decision.Class == "" || decision.Reason == "" {
			t.Fatalf("incomplete row decision = %+v", decision)
		}
	}
}

func TestDryRunDoesNotCallRowsReadyWhenCloudRequiresMissingEvidence(t *testing.T) {
	t.Parallel()

	key := bytes.Repeat([]byte{0x42}, migration.ArchiveKeyBytes)
	archive := migrationArchiveFixture(t, key, []migration.Table{
		oneTextRow("agent_profile_revision", "profile:1"),
		oneTextRow("agent_behavior_revision", "behavior:1"),
	})
	report := migration.PlanPostgresImport(archive)
	for _, row := range report.Rows {
		if row.Class != migration.MappingNeedsExplicitChoice {
			t.Errorf("%s classification = %s, want missing cloud evidence to require an explicit choice", row.SourceTable, row.Class)
		}
	}
	if report.Resolved {
		t.Fatal("profile/behavior rows resolved without created-by and behavior-digest evidence")
	}
}

func TestDryRunFailsClosedOnAnUnknownEmptyTable(t *testing.T) {
	t.Parallel()

	key := bytes.Repeat([]byte{0x42}, migration.ArchiveKeyBytes)
	archive := migrationArchiveFixture(t, key, []migration.Table{{
		Name: "future_unknown_table", Columns: []migration.Column{{Name: "id", SourceType: "TEXT"}}, Rows: []migration.Row{},
	}})
	report := migration.PlanPostgresImport(archive)
	if report.Resolved || len(report.Tables) != 1 || report.Tables[0].Class != migration.MappingIncompatible {
		t.Fatalf("unknown empty table report = %+v", report)
	}
	if err := report.RequireResolved(); !errors.Is(err, migration.ErrUnresolvedMappings) {
		t.Fatalf("unknown empty table error = %v", err)
	}
}

func TestVerifyMigrationComparesOnlyDefinedCanonicalLogicalParity(t *testing.T) {
	t.Parallel()

	key := bytes.Repeat([]byte{0x42}, migration.ArchiveKeyBytes)
	source := migrationArchiveFixture(t, key, []migration.Table{{
		Name: "stable_agent",
		Columns: []migration.Column{
			{Name: "id", SourceType: "TEXT", PrimaryKeyPosition: 1},
			{Name: "account_id", SourceType: "TEXT"},
			{Name: "state", SourceType: "TEXT"},
			{Name: "current_profile_revision_id", SourceType: "TEXT"},
			{Name: "current_behavior_revision_id", SourceType: "TEXT"},
			{Name: "current_binding_revision_id", SourceType: "TEXT"},
			{Name: "canonical_conversation_id", SourceType: "TEXT"},
			{Name: "canonical_participant_id", SourceType: "TEXT"},
			{Name: "created_at", SourceType: "TEXT"},
		},
		Rows: []migration.Row{{Values: []migration.Value{
			{Kind: migration.ValueText, Text: "agent:1"},
			{Kind: migration.ValueText, Text: "4af424a4-d81a-47d5-a495-400868883b86"},
			{Kind: migration.ValueText, Text: "open"},
			{Kind: migration.ValueText, Text: "profile:1"},
			{Kind: migration.ValueText, Text: "behavior:1"},
			{Kind: migration.ValueText, Text: "binding:1"},
			{Kind: migration.ValueText, Text: "conversation:1"},
			{Kind: migration.ValueText, Text: "participant:1"},
			{Kind: migration.ValueText, Text: "2026-08-21T20:00:00Z"},
		}}},
	}})
	target := migrationArchiveFixtureForEngine(t, key, migration.SourcePostgres, []migration.Table{{
		Name: "stable_agent",
		Columns: []migration.Column{
			{Name: "account_id", SourceType: "uuid", PrimaryKeyPosition: 1},
			{Name: "agent_id", SourceType: "text", PrimaryKeyPosition: 2},
			{Name: "state", SourceType: "text"},
			{Name: "current_profile_revision_id", SourceType: "text"},
			{Name: "current_behavior_revision_id", SourceType: "text"},
			{Name: "current_binding_revision_id", SourceType: "text"},
			{Name: "canonical_conversation_id", SourceType: "text"},
			{Name: "created_at", SourceType: "timestamp with time zone"},
		},
		Rows: []migration.Row{{Values: []migration.Value{
			{Kind: migration.ValueText, Text: "4af424a4-d81a-47d5-a495-400868883b86"},
			{Kind: migration.ValueText, Text: "agent:1"},
			{Kind: migration.ValueText, Text: "open"},
			{Kind: migration.ValueText, Text: "profile:1"},
			{Kind: migration.ValueText, Text: "behavior:1"},
			{Kind: migration.ValueText, Text: "binding:1"},
			{Kind: migration.ValueText, Text: "conversation:1"},
			{Kind: migration.ValueTimestamp, Text: "2026-08-21T20:00:00Z"},
		}}},
	}})

	verified := migration.VerifyMigration(source, target, key)
	if !verified.Verified || len(verified.Tables) != 1 || !verified.Tables[0].Matched {
		t.Fatalf("verification = %+v, want stable_agent parity", verified)
	}
	target.Tables[0].Rows[0].Values[1].Text = "agent:other"
	mismatch := migration.VerifyMigration(source, target, key)
	if mismatch.Verified || mismatch.Tables[0].Matched {
		t.Fatalf("mismatch verification = %+v, want fail closed", mismatch)
	}
}

func migrationArchiveFixture(t *testing.T, key []byte, tables []migration.Table) migration.Archive {
	t.Helper()
	return migrationArchiveFixtureForEngine(t, key, migration.SourceSQLite, tables)
}

func migrationArchiveFixtureForEngine(t *testing.T, key []byte, engine migration.SourceEngine, tables []migration.Table) migration.Archive {
	t.Helper()
	archive, err := migration.NewArchive(engine, string(engine)+":schema:1", strings.Repeat("a", 64), tables, key)
	if err != nil {
		t.Fatal(err)
	}
	return archive
}

func oneTextRow(name, value string) migration.Table {
	return migration.Table{
		Name: name, Columns: []migration.Column{{Name: "id", SourceType: "TEXT", PrimaryKeyPosition: 1}},
		Rows: []migration.Row{{Values: []migration.Value{{Kind: migration.ValueText, Text: value}}}},
	}
}
