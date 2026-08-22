package handler

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestChatEndpointDispatchesOnlyExactParentScopedResources(t *testing.T) {
	t.Parallel()
	key := []byte("0123456789abcdef0123456789abcdef")
	values := map[string]string{
		"DATABASE_URL":                     "postgresql://runtime.test/fort?sslmode=require",
		"FORT_CONTROL_ASSERTION_KEYS_JSON": `{"service-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_BODY_ACTIVE_KID":             "body-2026-08",
		"FORT_BODY_KEYS_JSON":              `{"body-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_AUTHORITY_MODE":              "cloud_v2_write",
		"FORT_AUTHORITY_EPOCH":             "8",
	}
	store := &fakeChatStore{claimed: make(map[string]struct{})}
	opens := 0
	handler := newChatEndpoint(func(key string) string { return values[key] }, func(context.Context, string) (chatControlStore, error) {
		opens++
		return store, nil
	})
	now := time.Now().UTC()
	deadline := now.Add(10 * time.Minute).Format(time.RFC3339Nano)
	tests := []struct {
		name, method, target, routeClass, body, want string
	}{
		{"conversation", http.MethodGet, "/api/v2/chat?resource=conversation&agent_id=agent%3Aresearch&conversation_id=conversation%3Ahome", "owner.agent_conversations.read", "", `"messages":[]`},
		{"conversation mutation", http.MethodPatch, "/api/v2/chat?resource=conversation&agent_id=agent%3Aresearch&conversation_id=conversation%3Asecondary", "owner.agent_conversations.mutate", `{"idempotency_key":"pin:one","action":"pin"}`, `"pinned":true`},
		{"turn", http.MethodPost, "/api/v2/chat?resource=turns&agent_id=agent%3Aresearch&conversation_id=conversation%3Ahome", "owner.agent_turns.send", `{"idempotency_key":"send:one","client_turn_id":"client:one","text":"hello","hard_deadline":"` + deadline + `"}`, `"created":true`},
		{"retry", http.MethodPost, "/api/v2/chat?resource=retry&agent_id=agent%3Aresearch&conversation_id=conversation%3Ahome&target_id=target%3Aone", "owner.agent_targets.retry", `{"idempotency_key":"retry:one"}`, `"state":"queued"`},
		{"cancel", http.MethodPost, "/api/v2/chat?resource=cancel&agent_id=agent%3Aresearch&conversation_id=conversation%3Ahome&target_id=target%3Aone", "owner.agent_targets.cancel", `{"idempotency_key":"cancel:one"}`, `"state":"canceled"`},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			request.Header.Set(controlapi.ServiceAssertionHeader, chatAssertion(t, key, test.routeClass, test.body, "chat-endpoint-nonce-00000000000000"+string(rune('a'+index))))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code < 200 || recorder.Code > 299 || !strings.Contains(recorder.Body.String(), test.want) {
				t.Fatalf("response = %d %q, want body containing %q", recorder.Code, recorder.Body.String(), test.want)
			}
		})
	}
	if opens != 1 || store.accountID != chatAccountID || store.agentID != "agent:research" ||
		store.conversationID != "conversation:home" || store.targetID != "target:one" {
		t.Fatalf("opens/scope = %d %q/%q/%q/%q", opens, store.accountID, store.agentID, store.conversationID, store.targetID)
	}
}

func TestChatEndpointRejectsMissingBodyKeyRingBeforeDatabaseOpen(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"DATABASE_URL":                     "postgresql://runtime.test/fort?sslmode=require",
		"FORT_CONTROL_ASSERTION_KEYS_JSON": `{"service-2026-08":"0123456789abcdef0123456789abcdef"}`,
	}
	opens := 0
	handler := newChatEndpoint(func(key string) string { return values[key] }, func(context.Context, string) (chatControlStore, error) {
		opens++
		return &fakeChatStore{}, nil
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/api/v2/chat?resource=conversation&agent_id=agent%3Aresearch&conversation_id=conversation%3Ahome", nil))
	if recorder.Code != http.StatusServiceUnavailable || opens != 0 {
		t.Fatalf("status/opens = %d/%d, want 503/0", recorder.Code, opens)
	}
}

func TestChatEndpointLeavesReadsAvailableButFencesMutationsOutsideCloudAuthority(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"DATABASE_URL":         "postgresql://runtime.test/fort?sslmode=require",
		"FORT_AUTHORITY_MODE":  "legacy_v1_write",
		"FORT_AUTHORITY_EPOCH": "7",
	}
	opens := 0
	handler := newChatEndpoint(func(key string) string { return values[key] }, func(context.Context, string) (chatControlStore, error) {
		opens++
		return &fakeChatStore{}, nil
	})
	request := httptest.NewRequest(http.MethodPost,
		"/api/v2/chat?resource=turns&agent_id=agent%3Aresearch&conversation_id=conversation%3Ahome",
		strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"code":"write_authority_inactive"`) || opens != 0 {
		t.Fatalf("status/body/opens = %d/%q/%d, want 409 authority fence before database", recorder.Code, recorder.Body.String(), opens)
	}
}

func TestChatEndpointRejectsAmbiguousRouteBeforeDatabase(t *testing.T) {
	t.Parallel()
	opens := 0
	handler := newChatEndpoint(func(string) string { return "" }, func(context.Context, string) (chatControlStore, error) {
		opens++
		return nil, nil
	})
	for _, test := range []struct{ method, target string }{
		{http.MethodGet, "/api/v2/chat?resource=turns&agent_id=a&conversation_id=c"},
		{http.MethodPost, "/api/v2/chat?resource=conversation&agent_id=a&conversation_id=c"},
		{http.MethodPost, "/api/v2/chat?resource=retry&agent_id=a&conversation_id=c"},
		{http.MethodPost, "/api/v2/chat?resource=cancel&agent_id=a&conversation_id=c&target_id=t&account_id=forged"},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.target, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d, want 404", test.method, test.target, recorder.Code)
		}
	}
	if opens != 0 {
		t.Fatalf("database opens = %d, want zero", opens)
	}
}

const chatAccountID = "4af424a4-d81a-47d5-a495-400868883b86"

type fakeChatStore struct {
	claimed                            map[string]struct{}
	accountID, agentID, conversationID string
	targetID                           string
	mutation                           ledger.SetAgentConversationPinCommand
}

func (store *fakeChatStore) RenameAgentConversation(_ context.Context, command ledger.RenameAgentConversationCommand) (ledger.AgentConversationRecord, error) {
	store.accountID, store.agentID, store.conversationID = command.AccountID, command.AgentID, command.ConversationID
	return ledger.AgentConversationRecord{Conversation: conversation.Conversation{ID: command.ConversationID, Title: command.Title, State: conversation.ConversationOpen},
		Link: conversation.AgentConversation{AgentID: command.AgentID, ConversationID: command.ConversationID, Kind: conversation.AgentConversationSecondary}}, nil
}

func (store *fakeChatStore) SetAgentConversationPin(_ context.Context, command ledger.SetAgentConversationPinCommand) (ledger.AgentConversationRecord, error) {
	store.accountID, store.agentID, store.conversationID, store.mutation = command.AccountID, command.AgentID, command.ConversationID, command
	return ledger.AgentConversationRecord{Conversation: conversation.Conversation{ID: command.ConversationID, Title: "Secondary", State: conversation.ConversationOpen},
		Link: conversation.AgentConversation{AgentID: command.AgentID, ConversationID: command.ConversationID, Kind: conversation.AgentConversationSecondary}, Pinned: command.Pinned}, nil
}

func (store *fakeChatStore) SetAgentConversationState(_ context.Context, command ledger.SetAgentConversationStateCommand) (ledger.AgentConversationRecord, error) {
	store.accountID, store.agentID, store.conversationID = command.AccountID, command.AgentID, command.ConversationID
	return ledger.AgentConversationRecord{Conversation: conversation.Conversation{ID: command.ConversationID, Title: "Secondary", State: command.State},
		Link: conversation.AgentConversation{AgentID: command.AgentID, ConversationID: command.ConversationID, Kind: conversation.AgentConversationSecondary}}, nil
}

func (store *fakeChatStore) Claim(_ context.Context, accountID, keyID, nonce string, _ time.Time) (bool, error) {
	claim := accountID + ":" + keyID + ":" + nonce
	if _, found := store.claimed[claim]; found {
		return false, nil
	}
	store.claimed[claim] = struct{}{}
	return true, nil
}

func (store *fakeChatStore) ReadAgentConversation(_ context.Context, accountID, agentID, conversationID string) (ledger.AgentConversationProjection, error) {
	store.accountID, store.agentID, store.conversationID = accountID, agentID, conversationID
	return ledger.AgentConversationProjection{Messages: []ledger.AgentConversationMessage{}, Turns: []ledger.AgentConversationTurn{}, Targets: []ledger.AgentConversationTarget{}}, nil
}

func (store *fakeChatStore) SendAgentTurn(_ context.Context, command ledger.SendAgentTurnCommand) (ledger.AgentTurnDispatch, error) {
	store.accountID, store.agentID, store.conversationID = command.AccountID, command.AgentID, command.ConversationID
	return ledger.AgentTurnDispatch{Created: true}, nil
}

func (store *fakeChatStore) RetryAgentTarget(_ context.Context, command ledger.RetryAgentTargetCommand) (ledger.AgentConversationTarget, error) {
	store.accountID, store.agentID, store.conversationID, store.targetID = command.AccountID, command.AgentID, command.ConversationID, command.TargetID
	return ledger.AgentConversationTarget{ID: command.TargetID, State: "queued"}, nil
}

func (store *fakeChatStore) CancelAgentTarget(_ context.Context, command ledger.CancelAgentTargetCommand) (ledger.AgentConversationTarget, error) {
	store.accountID, store.agentID, store.conversationID, store.targetID = command.AccountID, command.AgentID, command.ConversationID, command.TargetID
	return ledger.AgentConversationTarget{ID: command.TargetID, State: "canceled"}, nil
}

func (store *fakeChatStore) Close() error { return nil }

func chatAssertion(t *testing.T, key []byte, routeClass, body, nonce string) string {
	t.Helper()
	now := time.Now().UTC()
	digest := sha256.Sum256([]byte(body))
	token, err := controlapi.IssueServiceAssertion(key, controlapi.ServiceAssertion{
		KeyID: "service-2026-08", AccountID: chatAccountID, RouteClass: routeClass,
		Audience: "fort-control", RequestDigest: hex.EncodeToString(digest[:]),
		IssuedAt: now.Add(-time.Second), ExpiresAt: now.Add(30 * time.Second), Nonce: nonce,
	})
	if err != nil {
		t.Fatalf("issue assertion: %v", err)
	}
	return token
}
