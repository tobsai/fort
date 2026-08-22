package migration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type MappingClass string

const (
	MappingReady               MappingClass = "ready"
	MappingNeedsExplicitChoice MappingClass = "needs_explicit_choice"
	MappingLegacyRetained      MappingClass = "legacy_retained"
	MappingIncompatible        MappingClass = "incompatible"
)

type RowMapping struct {
	SourceTable  string       `json:"source_table"`
	TargetTable  string       `json:"target_table,omitempty"`
	RecordDigest string       `json:"record_digest"`
	Class        MappingClass `json:"class"`
	Reason       string       `json:"reason"`
}

type TableMapping struct {
	SourceTable string       `json:"source_table"`
	TargetTable string       `json:"target_table,omitempty"`
	Count       int64        `json:"count"`
	Digest      string       `json:"digest"`
	Class       MappingClass `json:"class"`
	Reason      string       `json:"reason"`
}

type ImportPlan struct {
	SourceEngine  SourceEngine           `json:"source_engine"`
	SchemaVersion string                 `json:"schema_version"`
	ManifestMAC   string                 `json:"manifest_mac"`
	TotalRows     int64                  `json:"total_rows"`
	Counts        map[MappingClass]int64 `json:"counts"`
	Tables        []TableMapping         `json:"tables"`
	Rows          []RowMapping           `json:"rows"`
	Resolved      bool                   `json:"resolved"`
}

type mappingRule struct {
	class  MappingClass
	target string
	reason string
}

var postgresImportRules = map[string]mappingRule{
	// These tables have a defined canonical parity projection. "ready" means
	// their rows need no product choice; it does not authorize a remote apply.
	"stable_agent":                    {MappingReady, "stable_agent", "stable Agent identity has defined canonical parity"},
	"agent_binding_transition":        {MappingReady, "agent_binding_transition", "accepted Binding transition has defined canonical parity"},
	"agent_conversation_pin_revision": {MappingReady, "agent_conversation_pin", "pin revisions have defined canonical parity"},
	"stable_group":                    {MappingReady, "group_conversation", "stable Group identity has defined canonical parity"},
	"source_routine_projection":       {MappingReady, "source_routine_projection", "source Routine projection has defined canonical parity"},
	"routine_import_receipt":          {MappingReady, "routine_import_receipt", "Routine fencing receipt has defined canonical parity"},

	// These require an explicit approved semantic mapping. They intentionally
	// block a migration plan while non-empty.
	"execution_source":                      {MappingNeedsExplicitChoice, "execution_source", "local computer identity must be matched to an enrolled cloud worker"},
	"source_agent":                          {MappingNeedsExplicitChoice, "source_agent", "source inventory evidence needs an approved worker mapping"},
	"agent_binding_revision":                {MappingNeedsExplicitChoice, "agent_binding_revision", "Binding computer and capability evidence need an enrolled worker choice"},
	"agent_profile_revision":                {MappingNeedsExplicitChoice, "agent_profile_revision", "cloud created-by evidence is absent from the local profile revision"},
	"agent_behavior_revision":               {MappingNeedsExplicitChoice, "agent_behavior_revision", "cloud created-by and behavior-digest evidence need an approved deterministic mapping"},
	"agent_conversation":                    {MappingNeedsExplicitChoice, "agent_conversation", "canonical and secondary Conversation ownership must be explicitly accepted"},
	"conversation":                          {MappingNeedsExplicitChoice, "conversation", "Conversation kind, membership, and Home/Group ownership must be explicit"},
	"conversation_participant":              {MappingNeedsExplicitChoice, "conversation_participant", "participant seat and authority snapshots require explicit v2 evidence"},
	"conversation_message":                  {MappingNeedsExplicitChoice, "conversation_message", "plaintext messages require approved encryption and logical digest mapping"},
	"conversation_turn":                     {MappingNeedsExplicitChoice, "conversation_turn", "turn context, grant, membership, and policies require explicit v2 snapshots"},
	"conversation_target":                   {MappingNeedsExplicitChoice, "conversation_target", "target pins and execution evidence require explicit v2 reconstruction"},
	"agent_channel":                         {MappingNeedsExplicitChoice, "stable_agent", "legacy Agent Channel needs explicit stable Agent and canonical Conversation selection"},
	"agent_channel_conversation":            {MappingNeedsExplicitChoice, "agent_conversation", "legacy Agent Conversation ownership needs explicit canonical selection"},
	"agent_conversation_pin":                {MappingNeedsExplicitChoice, "agent_conversation_pin", "legacy pin must be reconciled with revisioned pin evidence"},
	"agent_channel_created_conversation":    {MappingNeedsExplicitChoice, "agent_conversation", "legacy created Conversation needs explicit ownership"},
	"primary_channel":                       {MappingNeedsExplicitChoice, "stable_agent", "Primary Channel needs explicit stable Agent canonical mapping"},
	"primary_channel_pin":                   {MappingNeedsExplicitChoice, "agent_conversation_pin", "Primary Channel pin needs explicit Agent ownership"},
	"stable_group_turn":                     {MappingNeedsExplicitChoice, "conversation_turn", "Group Turn needs exact v2 membership, grant, and policy snapshots"},
	"stable_group_turn_recipient":           {MappingNeedsExplicitChoice, "conversation_target_binding", "Group recipients require exact v2 target pins"},
	"stable_group_initial_target":           {MappingNeedsExplicitChoice, "conversation_target", "Group target state requires v2 target reconstruction"},
	"stable_group_lifecycle_event":          {MappingNeedsExplicitChoice, "ledger_event", "Group lifecycle audit events need an approved cloud event ordering projection"},
	"group_membership_revision":             {MappingNeedsExplicitChoice, "conversation_membership_revision", "Group revision must be attached to its Conversation"},
	"group_member_revision":                 {MappingNeedsExplicitChoice, "conversation_member_revision", "Group member revision needs v2 Conversation membership evidence"},
	"group_member_binding":                  {MappingNeedsExplicitChoice, "conversation_participant", "Group binding needs immutable v2 participant evidence"},
	"stable_handoff":                        {MappingNeedsExplicitChoice, "handoff", "Handoff command and authority bodies require approved encrypted mapping"},
	"stable_handoff_target":                 {MappingNeedsExplicitChoice, "conversation_target", "Handoff target requires exact v2 execution pins"},
	"stable_handoff_cancellation":           {MappingNeedsExplicitChoice, "worker_command", "Handoff cancellation must be reconciled with the exact cloud attempt, lease, and worker acknowledgement"},
	"stable_handoff_projection":             {MappingNeedsExplicitChoice, "handoff_projection", "Handoff projection kind must be reconstructed explicitly"},
	"stable_handoff_attempt":                {MappingNeedsExplicitChoice, "handoff_attempt", "legacy lease and fence evidence needs exact worker lease reconciliation"},
	"stable_handoff_completion":             {MappingNeedsExplicitChoice, "handoff_emitter_receipt", "completion needs exact terminal receipt and emitter evidence"},
	"routine":                               {MappingNeedsExplicitChoice, "routine", "Routine state and current revision require accepted v2 policy mapping"},
	"routine_revision":                      {MappingNeedsExplicitChoice, "routine_revision", "Routine policy strings require explicit structured v2 policies"},
	"routine_occurrence":                    {MappingNeedsExplicitChoice, "routine_occurrence", "occurrence state and approval evidence require explicit v2 mapping"},
	"routine_run":                           {MappingNeedsExplicitChoice, "routine_run", "Routine run attempts and target identity require explicit v2 reconstruction"},
	"routine_run_activity":                  {MappingNeedsExplicitChoice, "ledger_event", "Routine activity needs an approved durable event projection"},
	"routine_result":                        {MappingNeedsExplicitChoice, "conversation_message", "Routine result requires encrypted authoritative message mapping"},
	"schedule":                              {MappingNeedsExplicitChoice, "routine", "legacy schedule must be explicitly retained or promoted to an Agent-owned Routine"},
	"schedule_channel_link":                 {MappingNeedsExplicitChoice, "routine_revision", "legacy schedule result Conversation requires an explicit Routine choice"},
	"schedule_occurrence":                   {MappingNeedsExplicitChoice, "routine_occurrence", "legacy occurrence history requires an explicit Routine choice"},
	"execution_source_config_observation":   {MappingNeedsExplicitChoice, "execution_source_config_observation", "local source observation needs an enrolled worker and encrypted evidence mapping"},
	"stable_agent_context_manifest":         {MappingNeedsExplicitChoice, "context_manifest", "local context manifest needs approved logical artifact mapping"},
	"stable_agent_context_manifest_message": {MappingNeedsExplicitChoice, "context_manifest_message", "local context membership needs cloud message identity mapping"},
	"stable_agent_direct_turn":              {MappingNeedsExplicitChoice, "conversation_turn", "direct turn needs exact v2 grant and policy snapshots"},
	"stable_agent_direct_target_binding":    {MappingNeedsExplicitChoice, "conversation_target_binding", "direct target needs immutable v2 pin evidence"},

	// v1 operational evidence stays recoverable in the encrypted archive but is
	// not projected into the cloud-v2 product ledger.
	"route_decision":                     {MappingLegacyRetained, "", "legacy deterministic routing evidence remains in the encrypted rollback archive"},
	"run":                                {MappingLegacyRetained, "", "legacy execution run remains in the encrypted rollback archive"},
	"node_run":                           {MappingLegacyRetained, "", "legacy DAG node run remains in the encrypted rollback archive"},
	"event":                              {MappingLegacyRetained, "", "legacy event stream remains in the encrypted rollback archive"},
	"invite":                             {MappingLegacyRetained, "", "expired local enrollment material is retained only for rollback evidence"},
	"backlog_item":                       {MappingLegacyRetained, "", "legacy backlog item remains in the encrypted rollback archive"},
	"playbook_revision":                  {MappingLegacyRetained, "", "legacy playbook revision remains in the encrypted rollback archive"},
	"project":                            {MappingLegacyRetained, "", "legacy project grouping remains in the encrypted rollback archive"},
	"primary_agent_setting":              {MappingLegacyRetained, "", "legacy primary-Agent setting is superseded by immutable Agent Binding evidence"},
	"stable_agent_participant_evidence":  {MappingLegacyRetained, "", "local participant evidence is retained while v2 snapshots are explicitly rebuilt"},
	"stable_agent_create_idempotency":    {MappingLegacyRetained, "", "local idempotency record remains rollback-only evidence"},
	"stable_agent_lifecycle_idempotency": {MappingLegacyRetained, "", "local lifecycle idempotency remains rollback-only evidence"},
	"agent_profile_revision_acceptance":  {MappingLegacyRetained, "", "local acceptance receipt remains rollback-only evidence"},
	"stable_agent_lifecycle_event":       {MappingLegacyRetained, "", "local lifecycle event remains rollback-only evidence"},
	"stable_group_create_idempotency":    {MappingLegacyRetained, "", "local Group idempotency remains rollback-only evidence"},
	"routine_create_idempotency":         {MappingLegacyRetained, "", "local Routine idempotency remains rollback-only evidence"},
	"routine_revalidate_idempotency":     {MappingLegacyRetained, "", "local Routine revalidation idempotency remains rollback-only evidence"},
}

func PlanPostgresImport(archive Archive) ImportPlan {
	report := ImportPlan{
		SourceEngine: archive.SourceEngine, SchemaVersion: archive.SchemaVersion,
		ManifestMAC: archive.ManifestMAC, TotalRows: archive.RowCount,
		Counts: make(map[MappingClass]int64), Tables: make([]TableMapping, 0, len(archive.Tables)),
		Rows: make([]RowMapping, 0, archive.RowCount), Resolved: archive.SourceEngine == SourceSQLite,
	}
	for _, table := range archive.Tables {
		rule, known := postgresImportRules[table.Name]
		if !known {
			rule = mappingRule{MappingIncompatible, "", "table has no approved cloud-v2 retention or mapping rule"}
		}
		report.Tables = append(report.Tables, TableMapping{
			SourceTable: table.Name, TargetTable: rule.target, Count: table.Count,
			Digest: table.Digest, Class: rule.class, Reason: rule.reason,
		})
		for _, row := range table.Rows {
			report.Rows = append(report.Rows, RowMapping{
				SourceTable: table.Name, TargetTable: rule.target, RecordDigest: row.Digest,
				Class: rule.class, Reason: rule.reason,
			})
			report.Counts[rule.class]++
		}
		if rule.class == MappingIncompatible || (table.Count > 0 && rule.class == MappingNeedsExplicitChoice) {
			report.Resolved = false
		}
	}
	return report
}

func (report ImportPlan) RequireResolved() error {
	if !report.Resolved || report.SourceEngine != SourceSQLite || int64(len(report.Rows)) != report.TotalRows {
		return fmt.Errorf("%w: needs_explicit_choice=%d incompatible=%d", ErrUnresolvedMappings,
			report.Counts[MappingNeedsExplicitChoice], report.Counts[MappingIncompatible])
	}
	return nil
}

type TableVerification struct {
	LogicalTable string `json:"logical_table"`
	SourceTable  string `json:"source_table"`
	TargetTable  string `json:"target_table"`
	SourceCount  int64  `json:"source_count"`
	TargetCount  int64  `json:"target_count"`
	SourceDigest string `json:"source_digest"`
	TargetDigest string `json:"target_digest"`
	Matched      bool   `json:"matched"`
	Reason       string `json:"reason,omitempty"`
}

type VerificationReport struct {
	SourceManifestMAC string              `json:"source_manifest_mac"`
	TargetManifestMAC string              `json:"target_manifest_mac"`
	Tables            []TableVerification `json:"tables"`
	Verified          bool                `json:"verified"`
}

type parityColumn struct {
	logical    string
	source     string
	target     string
	normalizer valueNormalizer
}

type valueNormalizer int

const (
	normalizeText valueNormalizer = iota
	normalizeBoolean
	normalizeJSON
	normalizeTimestamp
)

type parityMapping struct {
	logical, source, target string
	columns                 []parityColumn
}

var parityMappings = []parityMapping{
	{
		logical: "stable_agent", source: "stable_agent", target: "stable_agent",
		columns: []parityColumn{
			{"account_id", "account_id", "account_id", normalizeText},
			{"agent_id", "id", "agent_id", normalizeText},
			{"state", "state", "state", normalizeText},
			{"current_profile_revision_id", "current_profile_revision_id", "current_profile_revision_id", normalizeText},
			{"current_behavior_revision_id", "current_behavior_revision_id", "current_behavior_revision_id", normalizeText},
			{"current_binding_revision_id", "current_binding_revision_id", "current_binding_revision_id", normalizeText},
			{"canonical_conversation_id", "canonical_conversation_id", "canonical_conversation_id", normalizeText},
			{"created_at", "created_at", "created_at", normalizeTimestamp},
		},
	},
	{
		logical: "agent_profile_revision", source: "agent_profile_revision", target: "agent_profile_revision",
		columns: []parityColumn{
			{"profile_revision_id", "id", "profile_revision_id", normalizeText},
			{"agent_id", "agent_id", "agent_id", normalizeText},
			{"revision", "revision", "revision", normalizeText},
			{"name", "name", "name", normalizeText},
			{"title", "title", "title", normalizeText},
			{"avatar_url", "avatar_url", "avatar_url", normalizeText},
			{"hidden", "hidden", "hidden", normalizeBoolean},
			{"pinned", "pinned", "pinned", normalizeBoolean},
			{"sort_order", "sort_order", "sort_order", normalizeText},
			{"created_at", "created_at", "created_at", normalizeTimestamp},
		},
	},
	{
		logical: "agent_behavior_revision", source: "agent_behavior_revision", target: "agent_behavior_revision",
		columns: []parityColumn{
			{"behavior_revision_id", "id", "behavior_revision_id", normalizeText},
			{"agent_id", "agent_id", "agent_id", normalizeText},
			{"revision", "revision", "revision", normalizeText},
			{"role", "role", "role", normalizeText},
			{"standing_instructions", "standing_instructions", "standing_instructions", normalizeText},
			{"enabled_skills", "enabled_skills_json", "enabled_skills", normalizeJSON},
			{"enabled_tools", "enabled_tools_json", "enabled_tools", normalizeJSON},
			{"prompt_material", "prompt_material", "prompt_material", normalizeText},
			{"created_at", "created_at", "created_at", normalizeTimestamp},
		},
	},
	{
		logical: "agent_conversation_pin", source: "agent_conversation_pin_revision", target: "agent_conversation_pin",
		columns: []parityColumn{
			{"account_id", "account_id", "account_id", normalizeText},
			{"agent_id", "agent_id", "agent_id", normalizeText},
			{"conversation_id", "conversation_id", "conversation_id", normalizeText},
			{"revision", "revision", "revision", normalizeText},
			{"pinned", "pinned", "pinned", normalizeBoolean},
			{"changed_by", "changed_by", "changed_by", normalizeText},
			{"changed_at", "changed_at", "changed_at", normalizeTimestamp},
		},
	},
	{
		logical: "agent_binding_transition", source: "agent_binding_transition", target: "agent_binding_transition",
		columns: []parityColumn{
			{"agent_id", "agent_id", "agent_id", normalizeText},
			{"kind", "kind", "kind", normalizeText},
			{"previous_behavior_revision_id", "previous_behavior_revision_id", "previous_behavior_revision_id", normalizeText},
			{"successor_behavior_revision_id", "successor_behavior_revision_id", "successor_behavior_revision_id", normalizeText},
			{"previous_binding_revision_id", "previous_binding_revision_id", "previous_binding_revision_id", normalizeText},
			{"successor_binding_revision_id", "successor_binding_revision_id", "successor_binding_revision_id", normalizeText},
			{"preview_digest", "preview_digest", "preview_digest", normalizeText},
			{"non_transferable_resources", "non_transferable_resources_json", "non_transferable_resources", normalizeJSON},
			{"readiness_evidence", "readiness_evidence_json", "readiness_evidence", normalizeJSON},
			{"authority_evidence", "authority_evidence_json", "authority_evidence", normalizeJSON},
			{"accepted_by", "accepted_by", "accepted_by", normalizeText},
			{"accepted_at", "accepted_at", "accepted_at", normalizeTimestamp},
		},
	},
	{
		logical: "group_conversation_identity", source: "stable_group", target: "group_conversation",
		columns: []parityColumn{
			{"group_id", "id", "group_id", normalizeText},
			{"conversation_id", "conversation_id", "conversation_id", normalizeText},
			{"created_at", "created_at", "created_at", normalizeTimestamp},
		},
	},
	{
		logical: "source_routine_projection", source: "source_routine_projection", target: "source_routine_projection",
		columns: []parityColumn{
			{"account_id", "account_id", "account_id", normalizeText},
			{"source_routine_projection_id", "id", "source_routine_projection_id", normalizeText},
			{"execution_source_id", "execution_source_id", "execution_source_id", normalizeText},
			{"opaque_source_routine_id", "opaque_source_routine_id", "opaque_source_routine_id", normalizeText},
			{"projection_revision", "projection_revision", "projection_revision", normalizeText},
			{"authority", "authority", "authority", normalizeText},
			{"schedule_snapshot", "schedule_snapshot", "schedule_snapshot", normalizeJSON},
			{"projection_digest", "projection_digest", "projection_digest", normalizeText},
			{"last_occurrence_at", "last_occurrence_at", "last_occurrence_at", normalizeTimestamp},
			{"next_occurrence_at", "next_occurrence_at", "next_occurrence_at", normalizeTimestamp},
			{"observed_at", "observed_at", "observed_at", normalizeTimestamp},
		},
	},
	{
		logical: "routine_import_receipt", source: "routine_import_receipt", target: "routine_import_receipt",
		columns: []parityColumn{
			{"account_id", "account_id", "account_id", normalizeText},
			{"routine_import_receipt_id", "id", "routine_import_receipt_id", normalizeText},
			{"source_routine_projection_id", "source_routine_projection_id", "source_routine_projection_id", normalizeText},
			{"routine_id", "routine_id", "routine_id", normalizeText},
			{"routine_revision_id", "routine_revision_id", "routine_revision_id", normalizeText},
			{"source_disabled_at", "source_disabled_at", "source_disabled_at", normalizeTimestamp},
			{"exact_last_source_occurrence_at", "exact_last_source_occurrence_at", "exact_last_source_occurrence_at", normalizeTimestamp},
			{"exact_next_source_occurrence_at", "exact_next_source_occurrence_at", "exact_next_source_occurrence_at", normalizeTimestamp},
			{"fencing_receipt_ciphertext", "fencing_receipt_ciphertext", "fencing_receipt_ciphertext", normalizeText},
			{"fencing_receipt_key_id", "fencing_receipt_key_id", "fencing_receipt_key_id", normalizeText},
			{"fencing_receipt_nonce", "fencing_receipt_nonce", "fencing_receipt_nonce", normalizeText},
			{"fencing_receipt_digest", "fencing_receipt_digest", "fencing_receipt_digest", normalizeText},
			{"imported_at", "imported_at", "imported_at", normalizeTimestamp},
		},
	},
}

func VerifyMigration(source, target Archive, key []byte) VerificationReport {
	report := VerificationReport{
		SourceManifestMAC: source.ManifestMAC, TargetManifestMAC: target.ManifestMAC,
		Tables: make([]TableVerification, 0),
	}
	if source.SourceEngine != SourceSQLite || target.SourceEngine != SourcePostgres || len(key) != ArchiveKeyBytes {
		return report
	}
	sourceTables := archiveTableIndex(source)
	targetTables := archiveTableIndex(target)
	allMatched := true
	for _, mapping := range parityMappings {
		sourceTable, exists := sourceTables[mapping.source]
		if !exists {
			continue
		}
		verification := TableVerification{LogicalTable: mapping.logical, SourceTable: mapping.source, TargetTable: mapping.target}
		targetTable, targetExists := targetTables[mapping.target]
		if !targetExists {
			verification.SourceCount = int64(len(sourceTable.Rows))
			verification.Reason = "target table is absent"
			allMatched = false
			report.Tables = append(report.Tables, verification)
			continue
		}
		sourceRows, sourceErr := projectLogicalRows(sourceTable, mapping.columns, true)
		targetRows, targetErr := projectLogicalRows(targetTable, mapping.columns, false)
		verification.SourceCount, verification.TargetCount = int64(len(sourceRows)), int64(len(targetRows))
		if sourceErr != nil || targetErr != nil {
			verification.Reason = "canonical parity columns are absent or invalid"
			allMatched = false
			report.Tables = append(report.Tables, verification)
			continue
		}
		verification.SourceDigest, _ = keyedDigest(key, "logical:"+mapping.logical, sourceRows)
		verification.TargetDigest, _ = keyedDigest(key, "logical:"+mapping.logical, targetRows)
		verification.Matched = verification.SourceCount == verification.TargetCount && verification.SourceDigest == verification.TargetDigest
		if !verification.Matched {
			verification.Reason = "canonical logical count or digest differs"
			allMatched = false
		}
		report.Tables = append(report.Tables, verification)
	}
	report.Verified = len(report.Tables) > 0 && allMatched
	return report
}

func archiveTableIndex(archive Archive) map[string]Table {
	result := make(map[string]Table, len(archive.Tables))
	for _, table := range archive.Tables {
		result[table.Name] = table
	}
	return result
}

func projectLogicalRows(table Table, columns []parityColumn, source bool) ([]json.RawMessage, error) {
	columnIndex := make(map[string]int, len(table.Columns))
	for index, column := range table.Columns {
		columnIndex[column.Name] = index
	}
	projected := make([]json.RawMessage, 0, len(table.Rows))
	for _, row := range table.Rows {
		logical := make([]struct {
			Name  string `json:"name"`
			Value Value  `json:"value"`
		}, len(columns))
		for index, column := range columns {
			name := column.target
			if source {
				name = column.source
			}
			position, exists := columnIndex[name]
			if !exists || position >= len(row.Values) {
				return nil, fmt.Errorf("missing parity column %s.%s", table.Name, name)
			}
			normalized, err := normalizeLogicalValue(row.Values[position], column.normalizer)
			if err != nil {
				return nil, err
			}
			logical[index].Name, logical[index].Value = column.logical, normalized
		}
		encoded, err := json.Marshal(logical)
		if err != nil {
			return nil, err
		}
		projected = append(projected, encoded)
	}
	sort.Slice(projected, func(left, right int) bool { return bytes.Compare(projected[left], projected[right]) < 0 })
	return projected, nil
}

func normalizeLogicalValue(value Value, normalizer valueNormalizer) (Value, error) {
	if value.Kind == ValueNull {
		return value, nil
	}
	switch normalizer {
	case normalizeText:
		return Value{Kind: ValueText, Text: value.Text}, nil
	case normalizeBoolean:
		switch strings.ToLower(value.Text) {
		case "1", "true", "t":
			return Value{Kind: ValueBoolean, Text: "true"}, nil
		case "0", "false", "f":
			return Value{Kind: ValueBoolean, Text: "false"}, nil
		default:
			return Value{}, fmt.Errorf("invalid logical boolean %q", value.Text)
		}
	case normalizeJSON:
		var decoded any
		if err := json.Unmarshal([]byte(value.Text), &decoded); err != nil {
			return Value{}, err
		}
		canonical, err := json.Marshal(decoded)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: ValueJSON, Text: string(canonical)}, nil
	case normalizeTimestamp:
		parsed, err := time.Parse(time.RFC3339Nano, value.Text)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: ValueTimestamp, Text: parsed.UTC().Format(time.RFC3339Nano)}, nil
	default:
		return Value{}, fmt.Errorf("unknown logical normalizer %s", strconv.Itoa(int(normalizer)))
	}
}
