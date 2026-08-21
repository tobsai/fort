package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tobsai/fort/core/conversation"
)

type AgentChannelMigrationSkip struct {
	ConversationID string `json:"conversation_id"`
	Reason         string `json:"reason"`
}

type AgentChannelMigrationReport struct {
	Channels      []conversation.AgentChannel             `json:"channels"`
	Conversations []conversation.AgentChannelConversation `json:"conversations"`
	Pins          []conversation.AgentConversationPin     `json:"pins"`
	Skipped       []AgentChannelMigrationSkip             `json:"skipped"`
}

type agentChannelMigrationQueryer interface {
	rowsQueryer
	QueryRow(query string, args ...any) *sql.Row
}

func (s *Store) PreviewPrimaryAgentChannelMigration() (AgentChannelMigrationReport, error) {
	return buildPrimaryAgentChannelMigration(s.db)
}

func (s *Store) MigratePrimaryAgentChannels() (AgentChannelMigrationReport, error) {
	tx, err := s.beginConversationTurnTransaction(true)
	if err != nil {
		return AgentChannelMigrationReport{}, err
	}
	defer tx.Rollback()
	report, err := buildPrimaryAgentChannelMigration(tx)
	if err != nil {
		return AgentChannelMigrationReport{}, err
	}
	for _, channel := range report.Channels {
		bindingKey, err := conversation.AgentChannelID(channel.Binding)
		if err != nil {
			return AgentChannelMigrationReport{}, err
		}
		bindingJSON, err := json.Marshal(channel.Binding)
		if err != nil {
			return AgentChannelMigrationReport{}, err
		}
		if _, err := tx.Exec(`INSERT INTO agent_channel(id,name,state,option_id,binding_key,binding_json,created_at)
VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, channel.ID, channel.Name, channel.State, channel.OptionID,
			bindingKey, string(bindingJSON), nowOr(channel.CreatedAt)); err != nil {
			return AgentChannelMigrationReport{}, err
		}
		var persistedBinding string
		if err := tx.QueryRow(`SELECT binding_json FROM agent_channel WHERE id=?`, channel.ID).Scan(&persistedBinding); err != nil {
			return AgentChannelMigrationReport{}, err
		}
		if persistedBinding != string(bindingJSON) {
			return AgentChannelMigrationReport{}, fmt.Errorf("Agent Channel migration identity conflict for %s", channel.ID)
		}
	}
	for _, link := range report.Conversations {
		if _, err := tx.Exec(`INSERT INTO agent_channel_conversation(agent_channel_id,conversation_id,created_at)
VALUES(?,?,?) ON CONFLICT(conversation_id) DO NOTHING`, link.AgentChannelID, link.ConversationID, nowOr(link.CreatedAt)); err != nil {
			return AgentChannelMigrationReport{}, err
		}
		var owner string
		if err := tx.QueryRow(`SELECT agent_channel_id FROM agent_channel_conversation WHERE conversation_id=?`, link.ConversationID).Scan(&owner); err != nil {
			return AgentChannelMigrationReport{}, err
		}
		if owner != link.AgentChannelID {
			return AgentChannelMigrationReport{}, fmt.Errorf("Agent Channel migration ownership conflict for %s", link.ConversationID)
		}
	}
	for _, pin := range report.Pins {
		if _, err := tx.Exec(`INSERT INTO agent_conversation_pin(conversation_id,pinned_at) VALUES(?,?)
ON CONFLICT(conversation_id) DO NOTHING`, pin.ConversationID, nowOr(pin.PinnedAt)); err != nil {
			return AgentChannelMigrationReport{}, err
		}
		var persisted string
		if err := tx.QueryRow(`SELECT pinned_at FROM agent_conversation_pin WHERE conversation_id=?`, pin.ConversationID).Scan(&persisted); err != nil {
			return AgentChannelMigrationReport{}, err
		}
		if !parseTime(persisted).Equal(pin.PinnedAt) {
			return AgentChannelMigrationReport{}, fmt.Errorf("Agent Channel migration pin conflict for %s", pin.ConversationID)
		}
	}
	if err := tx.Commit(); err != nil {
		return AgentChannelMigrationReport{}, err
	}
	return report, nil
}

func buildPrimaryAgentChannelMigration(queryer agentChannelMigrationQueryer) (AgentChannelMigrationReport, error) {
	report := AgentChannelMigrationReport{
		Channels: []conversation.AgentChannel{}, Conversations: []conversation.AgentChannelConversation{},
		Pins: []conversation.AgentConversationPin{}, Skipped: []AgentChannelMigrationSkip{},
	}
	primaryCreated := sqliteRFC3339NanoOrder("created_at")
	rows, err := queryer.Query(fmt.Sprintf(`SELECT conversation_id FROM primary_channel ORDER BY %s,conversation_id`, primaryCreated))
	if err != nil {
		return AgentChannelMigrationReport{}, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return AgentChannelMigrationReport{}, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return AgentChannelMigrationReport{}, err
	}
	if err := rows.Close(); err != nil {
		return AgentChannelMigrationReport{}, err
	}

	channels := map[string]conversation.AgentChannel{}
	for _, id := range ids {
		primary, err := primaryChannelForMigration(queryer, id)
		if err != nil {
			report.Skipped = append(report.Skipped, AgentChannelMigrationSkip{ConversationID: id, Reason: err.Error()})
			continue
		}
		participants, err := conversationParticipantsQuery(queryer, id)
		if err != nil {
			return AgentChannelMigrationReport{}, err
		}
		participant, err := primaryChannelParticipant(&primary, participants)
		if err != nil {
			report.Skipped = append(report.Skipped, AgentChannelMigrationSkip{ConversationID: id, Reason: err.Error()})
			continue
		}
		binding := agentBindingFromPrimary(participant, primary)
		channelID, err := conversation.AgentChannelID(binding)
		if err != nil {
			report.Skipped = append(report.Skipped, AgentChannelMigrationSkip{ConversationID: id, Reason: err.Error()})
			continue
		}
		if _, exists := channels[channelID]; !exists {
			name := strings.TrimSpace(participant.DisplayName)
			if name == "" {
				name = participant.Agent
			}
			channels[channelID] = conversation.AgentChannel{
				ID: channelID, Name: name, State: conversation.AgentChannelOpen,
				OptionID: "migrated-primary:v1:" + strings.TrimPrefix(channelID, "agent-channel:v1:"),
				Binding:  binding, CreatedAt: primary.CreatedAt,
			}
		}
		report.Conversations = append(report.Conversations, conversation.AgentChannelConversation{
			AgentChannelID: channelID, ConversationID: id, CreatedAt: primary.CreatedAt,
		})
		var existingOwner string
		ownershipErr := queryer.QueryRow(`SELECT agent_channel_id FROM agent_channel_conversation WHERE conversation_id=?`, id).Scan(&existingOwner)
		if ownershipErr == nil {
			continue
		}
		if !errors.Is(ownershipErr, sql.ErrNoRows) {
			return AgentChannelMigrationReport{}, ownershipErr
		}
		var pinned string
		err = queryer.QueryRow(`SELECT pinned_at FROM primary_channel_pin WHERE conversation_id=?`, id).Scan(&pinned)
		if err == nil {
			report.Pins = append(report.Pins, conversation.AgentConversationPin{ConversationID: id, PinnedAt: parseTime(pinned)})
		} else if !errors.Is(err, sql.ErrNoRows) {
			return AgentChannelMigrationReport{}, err
		}
	}
	for _, channel := range channels {
		report.Channels = append(report.Channels, channel)
	}
	sort.Slice(report.Channels, func(i, j int) bool { return report.Channels[i].ID < report.Channels[j].ID })
	sort.Slice(report.Conversations, func(i, j int) bool {
		return report.Conversations[i].ConversationID < report.Conversations[j].ConversationID
	})
	sort.Slice(report.Pins, func(i, j int) bool { return report.Pins[i].ConversationID < report.Pins[j].ConversationID })
	sort.Slice(report.Skipped, func(i, j int) bool { return report.Skipped[i].ConversationID < report.Skipped[j].ConversationID })
	return report, nil
}

func primaryChannelForMigration(queryer agentChannelMigrationQueryer, id string) (conversation.PrimaryChannel, error) {
	var primary conversation.PrimaryChannel
	var created string
	dest := []any{&primary.ConversationID, &primary.ParticipantID, &primary.Authority}
	dest = append(dest, subscriptionPolicyScanDest(&primary.Policy)...)
	dest = append(dest, &created)
	if err := queryer.QueryRow(`SELECT conversation_id,participant_id,authority,`+subscriptionPolicyColumns+`,created_at
FROM primary_channel WHERE conversation_id=?`, id).Scan(dest...); err != nil {
		return conversation.PrimaryChannel{}, err
	}
	primary.CreatedAt = parseTime(created)
	if err := primary.Validate(); err != nil {
		return conversation.PrimaryChannel{}, err
	}
	return primary, nil
}

func agentBindingFromPrimary(participant conversation.Participant, primary conversation.PrimaryChannel) conversation.AgentBinding {
	policy := primary.Policy
	return conversation.AgentBinding{
		Seat: conversation.AgentSeatIdentity{
			ID: participant.SeatID, Profile: participant.Profile, Agent: participant.Agent,
			Model: participant.Model, Machine: participant.Machine,
		},
		Authority: conversation.AgentAuthoritySnapshot{
			RequestedModel: participant.Model, ResolvedModel: conversation.UnknownProviderIdentity,
			Authority: primary.Authority, PolicyID: policy.PolicyID, PolicyRevision: policy.PolicyRevision,
			AdapterID: policy.AdapterID, AdapterRevision: policy.AdapterRevision,
			RuntimeContract: policy.RuntimeContract, SessionMode: policy.ThreadMode,
			MemoryMode: conversation.AgentMemoryEphemeral,
			ExecutionPolicy: map[string]string{
				"account_type": policy.AccountType, "account_plan": policy.AccountPlan,
				"reasoning_effort": policy.ReasoningEffort, "reasoning_context": policy.ReasoningContext,
				"sandbox_mode": policy.SandboxMode, "approval_policy": policy.ApprovalPolicy,
				"workdir_mode": policy.WorkdirMode, "dynamic_tools_mode": policy.DynamicToolsMode,
				"mcp_mode": policy.MCPMode, "command_policy": policy.CommandPolicy,
				"file_read_policy": policy.FileReadPolicy, "isolation_revision": policy.IsolationRevision,
				"codex_version":                  policy.CodexVersion,
				"codex_executable_revision":      policy.CodexExecutableRevision,
				"codex_schema_revision":          policy.CodexSchemaRevision,
				"developer_instruction_revision": policy.DeveloperInstructionRevision,
				"request_timeout_millis":         strconv.Itoa(policy.RequestTimeoutMillis),
			},
		},
	}
}
