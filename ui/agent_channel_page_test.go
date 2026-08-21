package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

type agentRouteModeFake struct{ AgentChannelPort }

func (agentRouteModeFake) AgentOptions(context.Context) ([]AgentOption, error) { return nil, nil }

func TestAgentChannelPagePresentsAgentsAsChannelsAndNestsConversations(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	New(Deps{}).handleAgentChannelPage(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type=%q", got)
	}
	page := response.Body.String()
	for _, required := range []string{
		"<title>Fort · Agent Channels</title>",
		`id="agent-channel-list"`,
		`class="agent-channel-row"`,
		`class="conversation-groups"`,
		"Pinned conversations",
		"Recent conversations",
		"New conversation",
		`id="conversation-composer"`,
		"/api/agent-channels",
		"/conversations",
		"/turns",
		"agent_option_id",
	} {
		if !strings.Contains(page, required) {
			t.Errorf("agent-first page is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"Choose Primary Agent",
		"clear-primary-agent",
		"/api/settings/primary-agent",
		"API.channels='/api/channels'",
	} {
		if strings.Contains(page, forbidden) {
			t.Errorf("agent-first page retained legacy primary-channel contract %q", forbidden)
		}
	}
}

func TestAgentChannelPageFirstSendUsesAtomicParentRoute(t *testing.T) {
	if !strings.Contains(agentChannelPageHTML, "firstTurnURL(state.selectedAgentID)") {
		t.Fatal("empty Agent Channel send does not use the atomic parent first-turn route")
	}
	if !strings.Contains(agentChannelPageHTML, "JSON.stringify({name:conversationName(text),client_turn_id") {
		t.Fatal("atomic first send is missing its conversation name or idempotency key")
	}
}

func TestAgentChannelPageStreamsPersistedConversationReplacements(t *testing.T) {
	for _, required := range []string{
		"new EventSource(conversationURL(agentID,id)+'/events')",
		"state.eventSource.close()",
		"state.detail=JSON.parse(event.data)",
	} {
		if !strings.Contains(agentChannelPageHTML, required) {
			t.Fatalf("Agent Conversation stream is missing %q", required)
		}
	}
}

func TestAgentChannelPageReusesPendingIdempotencyKeyUntilAuthoritativeResponse(t *testing.T) {
	for _, required := range []string{
		"const clientTurnID=pendingTurnIDFor(text)",
		"client_turn_id:clientTurnID",
		"clearPendingTurn(clientTurnID)",
		"localStorage.setItem(pendingTurnKey()",
	} {
		if !strings.Contains(agentChannelPageHTML, required) {
			t.Fatalf("pending turn idempotency is missing %q", required)
		}
	}
	if strings.Contains(agentChannelPageHTML, "client_turn_id:newUUID()") {
		t.Fatal("Send still generates a fresh idempotency key for every retry")
	}
}

func TestAgentChannelPageKeepsRecoveryActionsClosedAndActionable(t *testing.T) {
	for _, required := range []string{
		"function agentRecoveryActions(target)",
		"agentRecoveryActions(target).includes('retry')",
		"list(item.recovery_actions)",
		"openConversation(agentID,conversationID)",
		"retryNeed(item,retry)",
	} {
		if !strings.Contains(agentChannelPageHTML, required) {
			t.Fatalf("Agent recovery UI is missing %q", required)
		}
	}
	if strings.Contains(agentChannelPageHTML, "if(status==='failed'){const retry=") {
		t.Fatal("every failed target still receives an unconditional Retry action")
	}
}

func TestAgentChannelStartupDoesNotInventAFirstSelection(t *testing.T) {
	if strings.Contains(agentChannelPageHTML, "channelID(state.agents[0])") || strings.Contains(agentChannelPageHTML, "conversationID(conversations[0])") {
		t.Fatal("startup still auto-selects the first Agent Channel or Conversation")
	}
	if !strings.Contains(agentChannelPageHTML, "if(!state.selectedAgentID){renderWelcome();return}") {
		t.Fatal("startup does not fall back to the Agent Channel list when restoration is invalid")
	}
}

func TestAgentChannelPageUsesLivingProductMarkWithoutUsingItAsAgentAvatar(t *testing.T) {
	page := agentChannelPageHTML
	for _, required := range []string{
		`class="fort-mark"`,
		"animation:fort-ambient",
		"@keyframes fort-ambient",
		"@keyframes fort-glow",
		"prefers-reduced-motion:reduce",
		"document.visibilityState",
		"IntersectionObserver",
		`data-motion="paused"`,
		`aria-label="Fort"`,
		`class="agent-avatar"`,
	} {
		if !strings.Contains(page, required) {
			t.Errorf("living-mark contract is missing %q", required)
		}
	}
	if strings.Contains(page, `<img class="agent-avatar" src="/fort-icon.png"`) {
		t.Fatal("Fort product mark is being presented as the agent avatar")
	}
	if !strings.Contains(page, "renderWorkingEnergy(null)") {
		t.Fatal("switching to an empty or welcome state can leave the product mark falsely Working")
	}
	for _, required := range []string{
		"byID('fort-product-mark').dataset.energy='ambient'",
		`id="conversation-working"`,
		"Selected conversation is Working",
	} {
		if !strings.Contains(page, required) {
			t.Fatalf("scoped living-mark truth is missing %q", required)
		}
	}
}

func TestAgentChannelPageShowsExactInspectableBinding(t *testing.T) {
	for _, required := range []string{
		"['Seat ID',seat.id]",
		"['Profile',seat.profile]",
		"['Agent / harness',seat.agent]",
		"['Seat model',seat.model]",
		"['Requested model',authority.requested_model]",
		"['Resolved model',first(authority.resolved_model,'unknown')]",
		"['Computer',seat.machine]",
		"['Authority',authority.authority]",
		"['Policy ID',authority.policy_id]",
		"['Policy revision',authority.policy_revision]",
		"['Adapter ID',authority.adapter_id]",
		"['Adapter revision',authority.adapter_revision]",
		"['Runtime contract',authority.runtime_contract]",
		"['Session',authority.session_mode]",
		"['Memory',authority.memory_mode]",
	} {
		if !strings.Contains(agentChannelPageHTML, required) {
			t.Fatalf("exact Agent identity inspector is missing %q", required)
		}
	}
}

func TestAgentChannelPageGatesUnavailableAgentAndOffersBoundedRecheck(t *testing.T) {
	for _, required := range []string{
		"function readinessReason(item)",
		"const canSend=status==='ready'",
		"byID('message-text').disabled=!canSend",
		"byID('send-message').disabled=!canSend",
		"first(current.reason,status)",
		"recheckSelectedAgent(recheck)",
		"request(API.options+'/recheck',{method:'POST'})",
	} {
		if !strings.Contains(agentChannelPageHTML, required) {
			t.Fatalf("bounded Agent readiness UI is missing %q", required)
		}
	}
}

func TestAgentChannelPageCanArchiveListAndReopenConversations(t *testing.T) {
	for _, required := range []string{
		`id="archive-conversation"`,
		`id="archived-conversations"`,
		`id="archived-dialog"`,
		"'/conversations?state=archived'",
		"JSON.stringify({state:'archived'})",
		"JSON.stringify({state:'open'})",
		"reopenConversation(conversation.id,reopen)",
	} {
		if !strings.Contains(agentChannelPageHTML, required) {
			t.Fatalf("Conversation archive/reopen UI is missing %q", required)
		}
	}
}

func TestAgentChannelPageCanRenameArchiveAndReopenAgentChannels(t *testing.T) {
	for _, required := range []string{
		`id="archived-agents"`,
		`id="archived-agents-dialog"`,
		"renameSelectedAgent(rename)",
		"archiveSelectedAgent(archive)",
		"API.agents+'?state=archived'",
		"JSON.stringify({name})",
		"JSON.stringify({state:'archived'})",
		"reopenAgent(channel.id,reopen)",
	} {
		if !strings.Contains(agentChannelPageHTML, required) {
			t.Fatalf("Agent Channel presentation lifecycle UI is missing %q", required)
		}
	}
}

func TestAgentChannelPageJavaScriptParses(t *testing.T) {
	start := strings.LastIndex(agentChannelPageHTML, "<script>")
	end := strings.LastIndex(agentChannelPageHTML, "</script>")
	if start < 0 || end <= start {
		t.Fatal("Agent Channel page script missing")
	}
	if _, err := goja.Compile("agent-channel-page.js", agentChannelPageHTML[start+len("<script>"):end], false); err != nil {
		t.Fatalf("Agent Channel page JavaScript does not parse: %v", err)
	}
}

func TestAgentProductModeServesSeparateAgentFirstRoot(t *testing.T) {
	mux := http.NewServeMux()
	if err := New(Deps{}).RegisterProductMode(mux, ProductMode{
		PrimaryChannels: PrimaryChannelsPreview,
		AgentChannels:   AgentChannelsPrimary,
	}); err != nil {
		t.Fatal(err)
	}
	response := modeResponse(t, mux, http.MethodGet, "/")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Fort · Agent Channels") {
		t.Fatalf("agent product root status=%d body=%s", response.Code, response.Body.String())
	}

	legacyMux := http.NewServeMux()
	if err := New(Deps{}).RegisterMode(legacyMux, PrimaryChannelsPreview); err != nil {
		t.Fatal(err)
	}
	legacy := modeResponse(t, legacyMux, http.MethodGet, "/")
	if legacy.Code != http.StatusOK || !strings.Contains(legacy.Body.String(), "Fort · Private Channels") {
		t.Fatalf("legacy primary root changed: status=%d body=%s", legacy.Code, legacy.Body.String())
	}
}

func TestAgentProductModeMountsAgentAPIAlongsideLegacyPrimaryAPI(t *testing.T) {
	mux := http.NewServeMux()
	server := New(Deps{AgentChannels: agentRouteModeFake{}})
	if err := server.RegisterProductMode(mux, ProductMode{
		PrimaryChannels: PrimaryChannelsPreview,
		AgentChannels:   AgentChannelsPrimary,
	}); err != nil {
		t.Fatal(err)
	}
	response := modeResponse(t, mux, http.MethodGet, "/api/agent-options")
	if response.Code != http.StatusOK || response.Body.String() != "[]\n" {
		t.Fatalf("agent options status=%d body=%q", response.Code, response.Body.String())
	}
	if response := modeResponse(t, mux, http.MethodGet, "/api/channels"); response.Code == http.StatusNotFound {
		t.Fatal("legacy /api/channels was not preserved during the agent-first cutover")
	}
}

func TestAgentProductRequiresPrimaryRollbackSurface(t *testing.T) {
	mux := http.NewServeMux()
	server := New(Deps{AgentChannels: agentRouteModeFake{}})
	err := server.RegisterNativeProductRoutes(mux, ProductMode{
		PrimaryChannels: PrimaryChannelsOff,
		AgentChannels:   AgentChannelsPrimary,
	})
	if err == nil || !strings.Contains(err.Error(), "rollback") {
		t.Fatalf("Agent cutover without Primary rollback surface error = %v", err)
	}
}

func TestAgentChannelsOffExplicitlyTombstonesAtomicFirstTurnRoute(t *testing.T) {
	mux := http.NewServeMux()
	if err := New(Deps{}).RegisterProductMode(mux, ProductMode{
		PrimaryChannels: PrimaryChannelsOff,
		AgentChannels:   AgentChannelsOff,
	}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/agent-channels/agent-1/turns", nil)
	_, pattern := mux.Handler(request)
	if pattern != "POST /api/agent-channels/{channel_id}/turns" {
		t.Fatalf("matched pattern=%q, want explicit atomic-first-turn tombstone", pattern)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=404", response.Code)
	}
}
