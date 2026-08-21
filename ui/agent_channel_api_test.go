package ui_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/ui"
)

var _ ui.AgentChannelPort = (*control.AgentChannelService)(nil)

type agentChannelAPIFake struct {
	ui.AgentChannelPort
	options         []ui.AgentOption
	channels        []ui.AgentChannelSummary
	details         map[string]ui.AgentChannelDetail
	conversation    ui.AgentConversationDetail
	first           ui.AgentFirstTurnResult
	createdOptionID string
	createdName     string
	firstChannelID  string
	firstName       string
	firstClientTurn string
	firstText       string
}

func (f *agentChannelAPIFake) AgentOptions(context.Context) ([]ui.AgentOption, error) {
	return f.options, nil
}

func (f *agentChannelAPIFake) RecheckAgentOptions(context.Context) ([]ui.AgentOption, error) {
	return f.options, nil
}

func (f *agentChannelAPIFake) ListAgentChannels(context.Context, string) ([]ui.AgentChannelSummary, error) {
	return f.channels, nil
}

func (f *agentChannelAPIFake) GetAgentChannel(_ context.Context, id string) (ui.AgentChannelDetail, error) {
	if item, ok := f.details[id]; ok {
		return item, nil
	}
	return ui.AgentChannelDetail{}, sql.ErrNoRows
}

func (f *agentChannelAPIFake) CreateAgentChannel(_ context.Context, optionID, name string) (ui.AgentChannelDetail, error) {
	f.createdOptionID, f.createdName = optionID, name
	return f.details["agent-1"], nil
}

func (f *agentChannelAPIFake) PostFirstAgentTurn(_ context.Context, channelID, name, clientTurnID, text string) (ui.AgentFirstTurnResult, error) {
	f.firstChannelID, f.firstName, f.firstClientTurn, f.firstText = channelID, name, clientTurnID, text
	return f.first, nil
}

func (f *agentChannelAPIFake) GetAgentConversation(_ context.Context, channelID, conversationID string) (ui.AgentConversationDetail, error) {
	if channelID != f.conversation.ChannelID || conversationID != f.conversation.Conversation.ID {
		return ui.AgentConversationDetail{}, sql.ErrNoRows
	}
	return f.conversation, nil
}

func TestAgentChannelAPIUsesSeparateNestedContract(t *testing.T) {
	clientTurnID := uuid.NewString()
	channel := conversation.AgentChannel{ID: "agent-1", Name: "OpenClaw — Personal", State: conversation.AgentChannelOpen}
	conversationItem := conversation.Conversation{ID: "conversation-1", Title: "Product direction", State: conversation.ConversationOpen}
	detail := ui.AgentChannelDetail{Channel: channel, Conversations: []conversation.AgentConversationSummary{}}
	child := ui.AgentConversationDetail{
		ChannelID: "agent-1", Conversation: conversationItem,
		Messages: []conversation.Message{}, Turns: []conversation.Turn{}, Targets: []conversation.Target{
			{ID: "target-done", TurnID: "turn-done", ParticipantID: "participant-1", State: conversation.TargetAnswered},
		},
	}
	fake := &agentChannelAPIFake{
		options: []ui.AgentOption{}, channels: []ui.AgentChannelSummary{detail},
		details: map[string]ui.AgentChannelDetail{"agent-1": detail}, conversation: child,
		first: ui.AgentFirstTurnResult{
			Conversation: child,
			Turn:         conversation.Turn{ID: "turn-1", ConversationID: "conversation-1", ClientTurnID: clientTurnID},
			Targets:      []conversation.Target{},
		},
	}
	server := ui.New(ui.Deps{AgentChannels: fake})
	mux := http.NewServeMux()
	server.RegisterAgentChannelRoutes(mux)

	body := primaryRequest(t, mux, http.MethodGet, "/api/agent-options", nil, http.StatusOK)
	if string(body) != "[]\n" {
		t.Fatalf("empty Agent options JSON = %q", body)
	}

	body = primaryRequest(t, mux, http.MethodPost, "/api/agent-channels", map[string]string{
		"agent_option_id": "option-openclaw", "name": "OpenClaw — Personal",
	}, http.StatusCreated)
	var gotChannel ui.AgentChannelDetail
	if err := json.Unmarshal(body, &gotChannel); err != nil || gotChannel.Channel.ID != "agent-1" {
		t.Fatalf("create Agent Channel = %+v, %v", gotChannel, err)
	}
	if fake.createdOptionID != "option-openclaw" || fake.createdName != "OpenClaw — Personal" {
		t.Fatalf("create inputs = %q, %q", fake.createdOptionID, fake.createdName)
	}

	primaryRequest(t, mux, http.MethodPost, "/api/agent-channels/agent-1/turns", map[string]string{
		"name": "Product direction", "client_turn_id": clientTurnID, "text": "hello",
	}, http.StatusAccepted)
	if fake.firstChannelID != "agent-1" || fake.firstName != "Product direction" || fake.firstClientTurn != clientTurnID || fake.firstText != "hello" {
		t.Fatalf("atomic first Send inputs = %+v", fake)
	}

	primaryRequest(t, mux, http.MethodGet, "/api/agent-channels/foreign/conversations/conversation-1", nil, http.StatusNotFound)
	primaryRequest(t, mux, http.MethodPost, "/api/agent-channels/agent-1/conversations/conversation-1/targets/target-done/retry", nil, http.StatusConflict)
	primaryRawRequest(t, mux, http.MethodPost, "/api/agent-channels", []byte(`{"agent_option_id":"option-openclaw","name":"OpenClaw","seat":"forged"}`), http.StatusBadRequest)
}
