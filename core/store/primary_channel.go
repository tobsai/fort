package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	modernsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const subscriptionPolicyColumns = `policy_id,policy_revision,adapter_id,adapter_revision,
codex_version,codex_executable_revision,codex_schema_revision,runtime_contract,
reasoning_effort,reasoning_context,request_timeout_millis,developer_instruction_revision,
account_type,account_plan,thread_mode,sandbox_mode,approval_policy,workdir_mode,
dynamic_tools_mode,mcp_mode,command_policy,file_read_policy,isolation_revision`

const conversationTargetColumns = `id,turn_id,participant_id,run_id,attempt,state,error_code,error,
authority,policy_id,policy_revision,selected_adapter_id,selected_adapter_revision,
selected_codex_version,selected_codex_executable_revision,selected_codex_schema_revision,
runtime_contract,requested_model,reasoning_effort,reasoning_context,request_timeout_millis,
developer_instruction_revision,account_type,account_plan,thread_mode,sandbox_mode,approval_policy,
workdir_mode,dynamic_tools_mode,mcp_mode,command_policy,file_read_policy,isolation_revision,
observed_adapter_id,observed_adapter_revision,observed_codex_version,
observed_codex_executable_revision,observed_codex_schema_revision,resolved_model,provider_thread_id,
provider_terminal_status,usage_source,input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,
created_at,updated_at`

func qualifiedTargetColumns(alias string) string {
	columns := strings.Split(conversationTargetColumns, ",")
	for index := range columns {
		columns[index] = alias + "." + strings.TrimSpace(columns[index])
	}
	return strings.Join(columns, ",")
}

func (s *Store) UpsertPrimaryAgentSetting(setting conversation.PrimaryAgentSetting) error {
	if err := setting.Validate(); err != nil {
		return err
	}
	values := []any{
		1, setting.OptionID, setting.Seat.ID, setting.Seat.Profile, setting.Seat.Agent,
		setting.Seat.Model, setting.Seat.Machine, setting.Seat.DisplayName, setting.Authority,
	}
	values = append(values, subscriptionPolicyValues(setting.Policy)...)
	values = append(values, nowOr(setting.UpdatedAt))
	_, err := s.db.Exec(`INSERT INTO primary_agent_setting(
singleton,option_id,seat_id,profile,agent,model,machine,display_name,authority,`+subscriptionPolicyColumns+`,updated_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(singleton) DO UPDATE SET
option_id=excluded.option_id,seat_id=excluded.seat_id,profile=excluded.profile,agent=excluded.agent,
model=excluded.model,machine=excluded.machine,display_name=excluded.display_name,authority=excluded.authority,
policy_id=excluded.policy_id,policy_revision=excluded.policy_revision,adapter_id=excluded.adapter_id,
adapter_revision=excluded.adapter_revision,codex_version=excluded.codex_version,
codex_executable_revision=excluded.codex_executable_revision,codex_schema_revision=excluded.codex_schema_revision,
runtime_contract=excluded.runtime_contract,reasoning_effort=excluded.reasoning_effort,
reasoning_context=excluded.reasoning_context,request_timeout_millis=excluded.request_timeout_millis,
developer_instruction_revision=excluded.developer_instruction_revision,account_type=excluded.account_type,
account_plan=excluded.account_plan,thread_mode=excluded.thread_mode,sandbox_mode=excluded.sandbox_mode,
approval_policy=excluded.approval_policy,workdir_mode=excluded.workdir_mode,
dynamic_tools_mode=excluded.dynamic_tools_mode,mcp_mode=excluded.mcp_mode,
command_policy=excluded.command_policy,file_read_policy=excluded.file_read_policy,
isolation_revision=excluded.isolation_revision,updated_at=excluded.updated_at`, values...)
	return err
}

func (s *Store) GetPrimaryAgentSetting() (conversation.PrimaryAgentSetting, error) {
	return scanPrimaryAgentSetting(s.db.QueryRow(`SELECT
singleton,option_id,seat_id,profile,agent,model,machine,display_name,authority,` + subscriptionPolicyColumns + `,updated_at
FROM primary_agent_setting WHERE singleton=1`))
}

func (s *Store) ClearPrimaryAgentSetting() error {
	_, err := s.db.Exec(`DELETE FROM primary_agent_setting WHERE singleton=1`)
	return err
}

func scanPrimaryAgentSetting(row scanner) (conversation.PrimaryAgentSetting, error) {
	var setting conversation.PrimaryAgentSetting
	var singleton int
	var updated string
	dest := []any{
		&singleton, &setting.OptionID, &setting.Seat.ID, &setting.Seat.Profile, &setting.Seat.Agent,
		&setting.Seat.Model, &setting.Seat.Machine, &setting.Seat.DisplayName, &setting.Authority,
	}
	dest = append(dest, subscriptionPolicyScanDest(&setting.Policy)...)
	dest = append(dest, &updated)
	if err := row.Scan(dest...); err != nil {
		return conversation.PrimaryAgentSetting{}, err
	}
	setting.UpdatedAt = parseTime(updated)
	if err := setting.Validate(); err != nil {
		return conversation.PrimaryAgentSetting{}, err
	}
	return setting, nil
}

// CreatePrimaryChannel snapshots the current singleton setting and creates the
// conversation, its sole participant, and the immutable marker in one SQLite
// transaction.
func (s *Store) CreatePrimaryChannel(item conversation.Conversation, participantID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	setting, err := scanPrimaryAgentSetting(tx.QueryRow(`SELECT
singleton,option_id,seat_id,profile,agent,model,machine,display_name,authority,` + subscriptionPolicyColumns + `,updated_at
FROM primary_agent_setting WHERE singleton=1`))
	if err != nil {
		return err
	}
	if participantID == "" {
		return fmt.Errorf("primary Channel participant id is required")
	}
	if item.State == "" {
		item.State = conversation.ConversationOpen
	}
	created := nowOr(item.CreatedAt)
	updated := created
	if !item.UpdatedAt.IsZero() {
		updated = nowOr(item.UpdatedAt)
	}
	if _, err := tx.Exec(`INSERT INTO conversation(id,project_id,title,state,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		item.ID, nullableString(item.ProjectID), item.Title, item.State, created, updated); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO conversation_participant(
id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at
) VALUES(?,?,?,?,?,?,?,?,0,?, ?,NULL)`, participantID, item.ID, setting.Seat.ID, setting.Seat.Profile,
		setting.Seat.Agent, setting.Seat.Model, setting.Seat.Machine, setting.Seat.DisplayName,
		conversation.ParticipantActive, created); err != nil {
		return err
	}
	values := []any{item.ID, participantID, setting.Authority}
	values = append(values, subscriptionPolicyValues(setting.Policy)...)
	values = append(values, created)
	if _, err := tx.Exec(`INSERT INTO primary_channel(conversation_id,participant_id,authority,`+subscriptionPolicyColumns+`,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, values...); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) getPrimaryChannel(id string) (conversation.PrimaryChannel, error) {
	var channel conversation.PrimaryChannel
	var created string
	dest := []any{&channel.ConversationID, &channel.ParticipantID, &channel.Authority}
	dest = append(dest, subscriptionPolicyScanDest(&channel.Policy)...)
	dest = append(dest, &created)
	err := s.db.QueryRow(`SELECT conversation_id,participant_id,authority,`+subscriptionPolicyColumns+`,created_at
FROM primary_channel WHERE conversation_id=?`, id).Scan(dest...)
	if err != nil {
		return conversation.PrimaryChannel{}, err
	}
	channel.CreatedAt = parseTime(created)
	if err := channel.Validate(); err != nil {
		return conversation.PrimaryChannel{}, err
	}
	return channel, nil
}

func (s *Store) SetPrimaryChannelPinned(id string, pinned bool, pinnedAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRow(`SELECT 1 FROM primary_channel WHERE conversation_id=?`, id).Scan(&exists); err != nil {
		return err
	}
	if pinned {
		_, err = tx.Exec(`INSERT INTO primary_channel_pin(conversation_id,pinned_at) VALUES(?,?) ON CONFLICT(conversation_id) DO NOTHING`, id, nowOr(pinnedAt))
	} else {
		_, err = tx.Exec(`DELETE FROM primary_channel_pin WHERE conversation_id=?`, id)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListPrimaryChannels(state string) ([]conversation.PrimaryChannelSummary, error) {
	if state == "" {
		state = string(conversation.ConversationOpen)
	}
	if state != string(conversation.ConversationOpen) && state != string(conversation.ConversationArchived) && state != "all" {
		return nil, fmt.Errorf("invalid primary Channel state %q", state)
	}
	messageActivity := sqliteRFC3339NanoOrder("message.created_at")
	conversationCreated := sqliteRFC3339NanoOrder("channel_conversation.created_at")
	pinnedOrder := sqliteRFC3339NanoOrder("pin.pinned_at")
	query := `SELECT channel_conversation.id,pin.pinned_at
FROM primary_channel channel
JOIN conversation channel_conversation ON channel_conversation.id=channel.conversation_id
LEFT JOIN primary_channel_pin pin ON pin.conversation_id=channel.conversation_id`
	args := []any{}
	if state != "all" {
		query += ` WHERE channel_conversation.state=?`
		args = append(args, state)
	}
	query += fmt.Sprintf(` ORDER BY CASE WHEN pin.pinned_at IS NULL THEN 1 ELSE 0 END,
CASE WHEN pin.pinned_at IS NULL THEN '' ELSE %s END DESC,
COALESCE((SELECT MAX(%s) FROM conversation_message message WHERE message.conversation_id=channel.conversation_id),%s) DESC,
channel.conversation_id`, pinnedOrder, messageActivity, conversationCreated)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	type listKey struct {
		id       string
		pinnedAt sql.NullString
	}
	keys := []listKey{}
	for rows.Next() {
		var key listKey
		if err := rows.Scan(&key.id, &key.pinnedAt); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	items := make([]conversation.PrimaryChannelSummary, 0, len(keys))
	for _, key := range keys {
		detail, err := s.GetConversation(key.id)
		if err != nil {
			return nil, err
		}
		if detail.PrimaryChannel == nil {
			return nil, fmt.Errorf("primary_channel_invariant: %s", key.id)
		}
		participant, err := primaryChannelParticipant(detail.PrimaryChannel, detail.Participants)
		if err != nil {
			return nil, err
		}
		items = append(items, conversation.PrimaryChannelSummary{
			Conversation: detail.Conversation,
			Participant:  participant,
			Identity:     *detail.PrimaryChannel,
			Pinned:       key.pinnedAt.Valid,
			PinnedAt:     parseTime(key.pinnedAt.String),
		})
	}
	return items, nil
}

func primaryChannelParticipant(identity *conversation.PrimaryChannel, participants []conversation.Participant) (conversation.Participant, error) {
	var selected conversation.Participant
	active := 0
	for _, participant := range participants {
		if participant.State == conversation.ParticipantActive {
			active++
		}
		if participant.ID == identity.ParticipantID {
			selected = participant
		}
	}
	if active != 1 || selected.ID == "" || selected.State != conversation.ParticipantActive ||
		selected.ConversationID != identity.ConversationID || selected.Agent != "codex-subscription" ||
		selected.Model == "" || selected.Profile != "codex-subscription:"+selected.Model {
		return conversation.Participant{}, fmt.Errorf("primary_channel_invariant: %s", identity.ConversationID)
	}
	return selected, nil
}

func (s *Store) CreateScheduleChannelLink(link conversation.ScheduleChannelLink) error {
	_, err := s.db.Exec(`INSERT INTO schedule_channel_link(schedule_id,conversation_id,created_at) VALUES(?,?,?)`,
		link.ScheduleID, link.ConversationID, nowOr(link.CreatedAt))
	return err
}

func (s *Store) GetScheduleChannelLink(scheduleID string) (conversation.ScheduleChannelLink, error) {
	var link conversation.ScheduleChannelLink
	var created string
	err := s.db.QueryRow(`SELECT schedule_id,conversation_id,created_at FROM schedule_channel_link WHERE schedule_id=?`, scheduleID).Scan(
		&link.ScheduleID, &link.ConversationID, &created,
	)
	if err != nil {
		return conversation.ScheduleChannelLink{}, err
	}
	link.CreatedAt = parseTime(created)
	return link, nil
}

func subscriptionPolicyValues(policy conversation.SubscriptionPolicy) []any {
	return []any{
		policy.PolicyID, policy.PolicyRevision, policy.AdapterID, policy.AdapterRevision,
		policy.CodexVersion, policy.CodexExecutableRevision, policy.CodexSchemaRevision, policy.RuntimeContract,
		policy.ReasoningEffort, policy.ReasoningContext, policy.RequestTimeoutMillis, policy.DeveloperInstructionRevision,
		policy.AccountType, policy.AccountPlan, policy.ThreadMode, policy.SandboxMode, policy.ApprovalPolicy,
		policy.WorkdirMode, policy.DynamicToolsMode, policy.MCPMode, policy.CommandPolicy, policy.FileReadPolicy,
		policy.IsolationRevision,
	}
}

func subscriptionPolicyScanDest(policy *conversation.SubscriptionPolicy) []any {
	return []any{
		&policy.PolicyID, &policy.PolicyRevision, &policy.AdapterID, &policy.AdapterRevision,
		&policy.CodexVersion, &policy.CodexExecutableRevision, &policy.CodexSchemaRevision, &policy.RuntimeContract,
		&policy.ReasoningEffort, &policy.ReasoningContext, &policy.RequestTimeoutMillis, &policy.DeveloperInstructionRevision,
		&policy.AccountType, &policy.AccountPlan, &policy.ThreadMode, &policy.SandboxMode, &policy.ApprovalPolicy,
		&policy.WorkdirMode, &policy.DynamicToolsMode, &policy.MCPMode, &policy.CommandPolicy, &policy.FileReadPolicy,
		&policy.IsolationRevision,
	}
}

type conversationTargetExecer interface {
	Exec(string, ...any) (sql.Result, error)
}

func insertConversationTarget(execer conversationTargetExecer, target conversation.Target) error {
	values := []any{
		target.ID, target.TurnID, target.ParticipantID, target.RunID, target.Attempt, target.State,
		nullableString(target.ErrorCode), nullableString(target.Error),
	}
	values = append(values, targetAuthorityValues(target.Authority)...)
	values = append(values, targetReceiptValues(target.Receipt)...)
	values = append(values, nowOr(target.CreatedAt), nowOr(target.UpdatedAt))
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")
	_, err := execer.Exec(`INSERT INTO conversation_target(`+conversationTargetColumns+`) VALUES(`+placeholders+`)`, values...)
	if isPrimaryChannelActiveTargetConstraint(err) {
		return conversation.NewBoundedError(conversation.ErrorConversationActive, conversation.ErrConversationActive)
	}
	return err
}

func isPrimaryChannelActiveTargetConstraint(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *modernsqlite.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code()&0xff != sqlite3.SQLITE_CONSTRAINT {
		return false
	}
	return strings.Contains(err.Error(), "primary_channel_active_target") ||
		strings.Contains(err.Error(), "UNIQUE constraint failed: conversation_target.participant_id")
}

func targetAuthorityValues(authority *conversation.TargetAuthority) []any {
	if authority == nil {
		return make([]any, 25)
	}
	policy := authority.Policy
	return []any{
		authority.Authority, policy.PolicyID, policy.PolicyRevision, policy.AdapterID, policy.AdapterRevision,
		policy.CodexVersion, policy.CodexExecutableRevision, policy.CodexSchemaRevision, policy.RuntimeContract,
		authority.RequestedModel, policy.ReasoningEffort, policy.ReasoningContext, policy.RequestTimeoutMillis,
		policy.DeveloperInstructionRevision, policy.AccountType, policy.AccountPlan, policy.ThreadMode,
		policy.SandboxMode, policy.ApprovalPolicy, policy.WorkdirMode, policy.DynamicToolsMode, policy.MCPMode,
		policy.CommandPolicy, policy.FileReadPolicy, policy.IsolationRevision,
	}
}

func targetReceiptValues(receipt *conversation.TargetReceipt) []any {
	if receipt == nil {
		return make([]any, 13)
	}
	return []any{
		nullableString(receipt.ObservedAdapterID), nullableString(receipt.ObservedAdapterRevision),
		nullableString(receipt.ObservedCodexVersion), nullableString(receipt.ObservedCodexExecutableRevision),
		nullableString(receipt.ObservedCodexSchemaRevision), nullableString(receipt.ResolvedModel),
		nullableString(receipt.ProviderThreadID), nullableString(receipt.ProviderTerminalStatus), nullableString(receipt.UsageSource),
		receipt.InputTokens, receipt.CachedInputTokens, receipt.OutputTokens, receipt.ReasoningTokens,
	}
}

func cloneTargetAuthority(authority *conversation.TargetAuthority) *conversation.TargetAuthority {
	if authority == nil {
		return nil
	}
	copy := *authority
	return &copy
}

func scanConversationTarget(row scanner) (conversation.Target, error) {
	var target conversation.Target
	var state, created, updated string
	var errorCode, targetError sql.NullString
	selectedStrings := make([]sql.NullString, 24)
	var requestTimeout sql.NullInt64
	receiptStrings := make([]sql.NullString, 9)
	receiptTokens := make([]sql.NullInt64, 4)
	dest := []any{
		&target.ID, &target.TurnID, &target.ParticipantID, &target.RunID, &target.Attempt, &state,
		&errorCode, &targetError,
	}
	for index := 0; index < 12; index++ {
		dest = append(dest, &selectedStrings[index])
	}
	dest = append(dest, &requestTimeout)
	for index := 12; index < len(selectedStrings); index++ {
		dest = append(dest, &selectedStrings[index])
	}
	for index := range receiptStrings {
		dest = append(dest, &receiptStrings[index])
	}
	for index := range receiptTokens {
		dest = append(dest, &receiptTokens[index])
	}
	dest = append(dest, &created, &updated)
	if err := row.Scan(dest...); err != nil {
		return conversation.Target{}, err
	}
	target.State, target.ErrorCode, target.Error = conversation.TargetState(state), errorCode.String, targetError.String
	target.CreatedAt, target.UpdatedAt = parseTime(created), parseTime(updated)
	if selectedStrings[0].Valid && selectedStrings[0].String != "" {
		target.Authority = &conversation.TargetAuthority{
			Authority:      selectedStrings[0].String,
			RequestedModel: selectedStrings[9].String,
			Policy: conversation.SubscriptionPolicy{
				PolicyID:                     selectedStrings[1].String,
				PolicyRevision:               selectedStrings[2].String,
				AdapterID:                    selectedStrings[3].String,
				AdapterRevision:              selectedStrings[4].String,
				CodexVersion:                 selectedStrings[5].String,
				CodexExecutableRevision:      selectedStrings[6].String,
				CodexSchemaRevision:          selectedStrings[7].String,
				RuntimeContract:              selectedStrings[8].String,
				ReasoningEffort:              selectedStrings[10].String,
				ReasoningContext:             selectedStrings[11].String,
				RequestTimeoutMillis:         int(requestTimeout.Int64),
				DeveloperInstructionRevision: selectedStrings[12].String,
				AccountType:                  selectedStrings[13].String,
				AccountPlan:                  selectedStrings[14].String,
				ThreadMode:                   selectedStrings[15].String,
				SandboxMode:                  selectedStrings[16].String,
				ApprovalPolicy:               selectedStrings[17].String,
				WorkdirMode:                  selectedStrings[18].String,
				DynamicToolsMode:             selectedStrings[19].String,
				MCPMode:                      selectedStrings[20].String,
				CommandPolicy:                selectedStrings[21].String,
				FileReadPolicy:               selectedStrings[22].String,
				IsolationRevision:            selectedStrings[23].String,
			},
		}
	}
	receiptPresent := false
	for _, value := range receiptStrings {
		receiptPresent = receiptPresent || value.Valid
	}
	for _, value := range receiptTokens {
		receiptPresent = receiptPresent || value.Valid
	}
	if receiptPresent {
		target.Receipt = &conversation.TargetReceipt{
			ObservedAdapterID:               receiptStrings[0].String,
			ObservedAdapterRevision:         receiptStrings[1].String,
			ObservedCodexVersion:            receiptStrings[2].String,
			ObservedCodexExecutableRevision: receiptStrings[3].String,
			ObservedCodexSchemaRevision:     receiptStrings[4].String,
			ResolvedModel:                   receiptStrings[5].String,
			ProviderThreadID:                receiptStrings[6].String,
			ProviderTerminalStatus:          receiptStrings[7].String,
			UsageSource:                     receiptStrings[8].String,
			InputTokens:                     receiptTokens[0].Int64,
			CachedInputTokens:               receiptTokens[1].Int64,
			OutputTokens:                    receiptTokens[2].Int64,
			ReasoningTokens:                 receiptTokens[3].Int64,
		}
	}
	return target, nil
}

func optionalPrimaryChannel(s *Store, id string) (*conversation.PrimaryChannel, error) {
	channel, err := s.getPrimaryChannel(id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &channel, nil
}
