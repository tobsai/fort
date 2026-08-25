package hermesbot_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/hermesbot"
)

func TestInventoryReturnsDiscoveredProfileAsUnreadySourceAgent(t *testing.T) {
	t.Parallel()

	response, err := os.ReadFile("testdata/profiles-list-valid.json")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 22, 18, 0, 0, 0, time.UTC)
	transport := &fakeLocalProfileRosterTransport{
		exchange: hermesbot.VerifiedRPCExchange{
			ConnectionIdentity: "hermes-dashboard:mini:9119",
			LauncherDigest:     strings.Repeat("a", 64),
			CodeRootDigest:     strings.Repeat("b", 64),
			HermesVersion:      hermesbot.HermesVersion,
			HermesRevision:     hermesbot.HermesRevision,
			Body:               io.NopCloser(strings.NewReader(string(response))),
		},
	}
	source := hermesExecutionSource()
	inventory, err := hermesbot.NewInventory(hermesbot.InventoryOptions{
		ExecutionSource: source,
		AcceptedSource: hermesbot.LocalSourceContract{
			ConnectionIdentity: "hermes-dashboard:mini:9119",
			LauncherDigest:     strings.Repeat("a", 64),
			CodeRootDigest:     strings.Repeat("b", 64),
			HermesVersion:      hermesbot.HermesVersion,
			HermesRevision:     hermesbot.HermesRevision,
		},
		Transport: transport,
		Clock:     func() time.Time { return now },
		RequestID: func() string { return "inventory-request-1" },
	})
	if err != nil {
		t.Fatalf("NewInventory: %v", err)
	}
	var seam runtime.AgentSourceInventory = inventory

	got, err := seam.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{
		ExecutionSourceID: source.ID,
	})
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	wantSource := source
	wantSource.LastSeenAt = now
	want := runtime.AgentSourceInventorySnapshot{
		ExecutionSource: wantSource,
		Agents: []runtime.SourceAgentInventory{{
			SourceAgent: conversation.SourceAgent{
				ID:                "source-agent:hermes:v1:691db9e0d4321588be24f554891d2aae11dd2738b449efa63c9aa52d8807519a",
				ExecutionSourceID: source.ID, OpaqueSourceAgentID: "researcher",
				DisplayName: "Researcher", LastSeenAt: now,
			},
			Capabilities: []string{},
			Readiness: runtime.SourceAgentReadiness{
				Ready: false, ContractID: hermesbot.InventoryContractID,
				ContractRevision: hermesbot.InventoryContractRevision,
				Evidence:         []string{"profile_discovered", "execution_adapter_not_approved"},
			},
		}},
		ObservedAt: now,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Inventory snapshot = %#v, want %#v", got, want)
	}
	if gotErr := got.Validate(); gotErr != nil {
		t.Fatalf("Inventory snapshot validation: %v", gotErr)
	}
	if request := string(transport.request); request != `{"jsonrpc":"2.0","id":"inventory-request-1","method":"profiles.list","params":{"include_sessions":false}}` {
		t.Fatalf("profiles.list request = %q", request)
	}
}

func TestInventoryUsesSourceQualifiedIdentityAndDeterministicAllocatedRosters(t *testing.T) {
	t.Parallel()

	reversed, err := os.ReadFile("testdata/profiles-list-reversed.json")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := os.ReadFile("testdata/profiles-list-empty.json")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 22, 18, 30, 0, 0, time.UTC)

	firstSource := hermesExecutionSource()
	first := mustInventory(t, firstSource, reversed, now)
	firstSnapshot, err := first.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: firstSource.ID})
	if err != nil {
		t.Fatalf("first Inventory: %v", err)
	}
	if got := []string{
		firstSnapshot.Agents[0].SourceAgent.OpaqueSourceAgentID,
		firstSnapshot.Agents[1].SourceAgent.OpaqueSourceAgentID,
	}; !reflect.DeepEqual(got, []string{"researcher", "writer"}) {
		t.Fatalf("sorted profile IDs = %v", got)
	}

	secondSource := hermesExecutionSource()
	secondSource.ID = "source:studio"
	secondSource.InstanceID = "instance:studio"
	secondSource.GatewayID = "gateway:studio"
	secondSource.DisplayName = "Hermes · Studio"
	second := mustInventory(t, secondSource, reversed, now)
	secondSnapshot, err := second.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: secondSource.ID})
	if err != nil {
		t.Fatalf("second Inventory: %v", err)
	}
	if firstSnapshot.Agents[0].SourceAgent.ID == secondSnapshot.Agents[0].SourceAgent.ID {
		t.Fatal("same-named profiles on different Execution Sources shared a Source Agent ID")
	}
	firstIdentity, err := firstSnapshot.Agents[0].SourceAgent.Identity()
	if err != nil {
		t.Fatal(err)
	}
	secondIdentity, err := secondSnapshot.Agents[0].SourceAgent.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if firstIdentity == secondIdentity {
		t.Fatal("same-named profiles on different Execution Sources shared an identity")
	}

	emptyInventory := mustInventory(t, firstSource, empty, now)
	emptySnapshot, err := emptyInventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: firstSource.ID})
	if err != nil {
		t.Fatalf("empty Inventory: %v", err)
	}
	if emptySnapshot.Agents == nil || len(emptySnapshot.Agents) != 0 {
		t.Fatalf("empty roster = %#v, want allocated empty list", emptySnapshot.Agents)
	}
	if err := emptySnapshot.Validate(); err != nil {
		t.Fatalf("empty snapshot validation: %v", err)
	}
}

func TestInventorySourceAgentIDUsesUTF8ByteLengthsAndAllowsDefaultProfile(t *testing.T) {
	t.Parallel()

	response, err := os.ReadFile("testdata/profiles-list-valid.json")
	if err != nil {
		t.Fatal(err)
	}
	response = bytes.Replace(response, []byte(`"name": "researcher"`), []byte(`"name": "default"`), 1)
	source := hermesExecutionSource()
	source.ID = "source:méta"
	inventory := mustInventory(t, source, response, time.Now())
	snapshot, err := inventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: source.ID})
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if got, want := snapshot.Agents[0].SourceAgent.ID, "source-agent:hermes:v1:0649558dd77a6a28ac100c699cbc03f9ed82484ca073b9986205558a4d47f9d4"; got != want {
		t.Fatalf("Source Agent ID = %s, want %s", got, want)
	}
}

func TestInventoryRejectsProfileRowsMissingFrozenRequiredFields(t *testing.T) {
	t.Parallel()

	response, err := os.ReadFile("testdata/profiles-list-valid.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		line        string
		replacement string
	}{
		{name: "name", line: "        \"name\": \"researcher\",\n"},
		{name: "path", line: "        \"path\": \"/private/discarded/hermes/profiles/researcher\",\n"},
		{name: "is_default", line: "        \"is_default\": false,\n"},
		{name: "model", line: "        \"model\": null,\n"},
		{name: "provider", line: "        \"provider\": null,\n"},
		{name: "description", line: "        \"description\": \"Private upstream description\",\n"},
		{name: "display_name", line: "        \"display_name\": \"Researcher\",\n"},
		{name: "skill_count", line: "        \"skill_count\": 12,\n"},
		{
			name:        "has_avatar",
			line:        "        \"ui_meta_revisions\": {},\n        \"has_avatar\": true\n",
			replacement: "        \"ui_meta_revisions\": {}\n",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			withoutField := bytes.Replace(response, []byte(test.line), []byte(test.replacement), 1)
			if bytes.Equal(withoutField, response) {
				t.Fatalf("fixture did not contain required %s field", test.name)
			}
			inventory := mustInventory(t, hermesExecutionSource(), withoutField, time.Date(2026, 8, 22, 19, 0, 0, 0, time.UTC))

			if _, err := inventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"}); err == nil {
				t.Fatalf("Inventory accepted a profile row missing the frozen %s field", test.name)
			}
		})
	}
}

func TestNewInventoryRequiresExactConservativeHermesResourceDisclosure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*conversation.ResourceSharingDisclosure)
	}{
		{name: "provider credentials", mutate: func(disclosure *conversation.ResourceSharingDisclosure) {
			disclosure.ProviderCredentials = conversation.ResourceProfileScoped
		}},
		{name: "filesystem", mutate: func(disclosure *conversation.ResourceSharingDisclosure) {
			disclosure.Filesystem = conversation.ResourceProfileScoped
		}},
		{name: "browser sessions", mutate: func(disclosure *conversation.ResourceSharingDisclosure) {
			disclosure.BrowserSessions = conversation.ResourceProfileScoped
		}},
		{name: "framework sessions", mutate: func(disclosure *conversation.ResourceSharingDisclosure) {
			disclosure.FrameworkSessions = conversation.ResourceMachineShared
		}},
		{name: "source memory", mutate: func(disclosure *conversation.ResourceSharingDisclosure) {
			disclosure.SourceMemory = conversation.ResourceProfileScoped
		}},
		{name: "tool configuration", mutate: func(disclosure *conversation.ResourceSharingDisclosure) {
			disclosure.ToolConfiguration = conversation.ResourceMachineShared
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			source := hermesExecutionSource()
			test.mutate(&source.ResourceSharing)
			_, err := hermesbot.NewInventory(hermesbot.InventoryOptions{
				ExecutionSource: source,
				AcceptedSource:  acceptedLocalSource(),
				Transport:       &fakeLocalProfileRosterTransport{},
				Clock:           time.Now,
				RequestID:       func() string { return "inventory-request-1" },
			})
			if err == nil {
				t.Fatalf("NewInventory accepted drifted %s disclosure", test.name)
			}
		})
	}
}

func TestInventoryNeverPublishesAfterCallerCancellation(t *testing.T) {
	t.Parallel()

	response, err := os.ReadFile("testdata/profiles-list-valid.json")
	if err != nil {
		t.Fatal(err)
	}
	inventory := mustInventory(t, hermesExecutionSource(), response, time.Date(2026, 8, 22, 19, 30, 0, 0, time.UTC))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := inventory.Inventory(ctx, runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"}); err == nil {
		t.Fatal("Inventory published a snapshot after caller cancellation")
	}
}

func TestInventoryNeverPublishesIfCancellationWinsDuringProjection(t *testing.T) {
	t.Parallel()

	response, err := os.ReadFile("testdata/profiles-list-valid.json")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	transport := &fakeLocalProfileRosterTransport{
		exchange: verifiedExchange(nil),
		response: response,
	}
	inventory := mustInventoryWithTransport(t, transport, func() time.Time {
		cancel()
		return time.Date(2026, 8, 22, 19, 45, 0, 0, time.UTC)
	})

	if _, err := inventory.Inventory(ctx, runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"}); err == nil {
		t.Fatal("Inventory published a snapshot after cancellation won during projection")
	}
}

func TestInventoryDeadlineInterruptsResponseBodyRead(t *testing.T) {
	t.Parallel()

	body := newBlockingReadCloser()
	transport := &fakeLocalProfileRosterTransport{exchange: verifiedExchange(body)}
	inventory, err := hermesbot.NewInventory(hermesbot.InventoryOptions{
		ExecutionSource: hermesExecutionSource(),
		AcceptedSource:  acceptedLocalSource(),
		Transport:       transport,
		Clock:           time.Now,
		RequestID:       func() string { return "inventory-request-1" },
	})
	if err != nil {
		t.Fatalf("NewInventory: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, inventoryErr := inventory.Inventory(ctx, runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"})
		done <- inventoryErr
	}()

	select {
	case inventoryErr := <-done:
		if inventoryErr == nil {
			t.Fatal("Inventory published a snapshot after its response deadline")
		}
	case <-time.After(time.Second):
		_ = body.Close()
		<-done
		t.Fatal("Inventory did not interrupt a blocked response body read")
	}
	select {
	case <-body.closed:
	default:
		t.Fatal("Inventory did not close the response body at its deadline")
	}
	if got := body.CloseCount(); got != 1 {
		t.Fatalf("response body close count = %d, want 1", got)
	}
}

func TestInventoryDeadlineJoinsInterruptedResponseRead(t *testing.T) {
	t.Parallel()

	body := newJoiningReadCloser()
	transport := &fakeLocalProfileRosterTransport{exchange: verifiedExchange(body)}
	inventory := mustInventoryWithTransport(t, transport, time.Now)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, inventoryErr := inventory.Inventory(ctx, runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"})
		done <- inventoryErr
	}()

	select {
	case <-body.closed:
	case <-time.After(time.Second):
		close(body.release)
		t.Fatal("Inventory did not interrupt the response body read")
	}
	select {
	case inventoryErr := <-done:
		close(body.release)
		if inventoryErr == nil {
			t.Fatal("Inventory published a snapshot after its deadline")
		}
		t.Fatal("Inventory returned before the interrupted body read exited")
	case <-time.After(50 * time.Millisecond):
	}
	close(body.release)
	select {
	case inventoryErr := <-done:
		if inventoryErr == nil {
			t.Fatal("Inventory published a snapshot after its deadline")
		}
	case <-time.After(time.Second):
		t.Fatal("Inventory did not join the interrupted body read")
	}
}

func TestInventoryRejectsAmbiguousDuplicateJSONKeys(t *testing.T) {
	t.Parallel()

	response, err := os.ReadFile("testdata/profiles-list-valid.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		old         string
		replacement string
	}{
		{
			name:        "envelope request ID",
			old:         `  "id": "inventory-request-1",`,
			replacement: "  \"id\": \"wrong\",\n  \"id\": \"inventory-request-1\",",
		},
		{
			name:        "profile name",
			old:         `        "name": "researcher",`,
			replacement: "        \"name\": \"root\",\n        \"name\": \"researcher\",",
		},
		{
			name:        "Bot Mode protocol marker",
			old:         `    "bot_mode_protocol": true`,
			replacement: "    \"bot_mode_protocol\": false,\n    \"bot_mode_protocol\": true",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			ambiguous := bytes.Replace(response, []byte(test.old), []byte(test.replacement), 1)
			if bytes.Equal(ambiguous, response) {
				t.Fatal("fixture replacement did not apply")
			}
			inventory := mustInventory(t, hermesExecutionSource(), ambiguous, time.Now())
			if _, err := inventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"}); err == nil {
				t.Fatal("Inventory accepted an object with duplicate keys")
			}
		})
	}
}

func TestInventoryRejectsExplicitNullErrorAlongsideResult(t *testing.T) {
	t.Parallel()

	response, err := os.ReadFile("testdata/profiles-list-valid.json")
	if err != nil {
		t.Fatal(err)
	}
	ambiguous := bytes.Replace(response, []byte(`  "result": {`), []byte("  \"error\": null,\n  \"result\": {"), 1)
	if bytes.Equal(ambiguous, response) {
		t.Fatal("fixture replacement did not apply")
	}
	inventory := mustInventory(t, hermesExecutionSource(), ambiguous, time.Now())
	if _, err := inventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"}); err == nil {
		t.Fatal("Inventory accepted an explicit error member alongside a result")
	}
}

func TestInventoryFallsBackFromInvalidUnicodeDisplayName(t *testing.T) {
	t.Parallel()

	response, err := os.ReadFile("testdata/profiles-list-valid.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "unpaired high surrogate", raw: `\ud800`, want: "researcher"},
		{name: "unpaired low surrogate", raw: `\udfff`, want: "researcher"},
		{name: "valid surrogate pair", raw: `\ud83d\ude80`, want: "🚀"},
		{name: "leading whitespace", raw: ` Researcher`, want: "researcher"},
		{name: "control character", raw: `\u0001`, want: "researcher"},
		{name: "more than 64 scalars", raw: strings.Repeat("a", 65), want: "researcher"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			candidate := bytes.Replace(
				response,
				[]byte(`"display_name": "Researcher"`),
				[]byte(`"display_name": "`+test.raw+`"`),
				1,
			)
			if bytes.Equal(candidate, response) {
				t.Fatal("fixture replacement did not apply")
			}
			inventory := mustInventory(t, hermesExecutionSource(), candidate, time.Now())
			snapshot, err := inventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"})
			if err != nil {
				t.Fatalf("Inventory: %v", err)
			}
			if got := snapshot.Agents[0].SourceAgent.DisplayName; got != test.want {
				t.Fatalf("display name = %q, want %q", got, test.want)
			}
		})
	}
}

func TestInventoryRejectsDriftedProfileMetadataTypes(t *testing.T) {
	t.Parallel()

	response, err := os.ReadFile("testdata/profiles-list-valid.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		old         string
		replacement string
	}{
		{name: "ui_meta is not an object", old: `"ui_meta": {"accent": "private"}`, replacement: `"ui_meta": "private"`},
		{name: "ui_meta_revisions is not an object", old: `"ui_meta_revisions": {}`, replacement: `"ui_meta_revisions": []`},
		{name: "skill count is negative", old: `"skill_count": 12`, replacement: `"skill_count": -1`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			drifted := bytes.Replace(response, []byte(test.old), []byte(test.replacement), 1)
			if bytes.Equal(drifted, response) {
				t.Fatal("fixture replacement did not apply")
			}
			inventory := mustInventory(t, hermesExecutionSource(), drifted, time.Now())
			if _, err := inventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"}); err == nil {
				t.Fatal("Inventory accepted drifted metadata")
			}
		})
	}
}

func TestInventoryFailsClosedOnSourceAttestationAndRosterDrift(t *testing.T) {
	t.Parallel()

	valid, err := os.ReadFile("testdata/profiles-list-valid.json")
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := os.ReadFile("testdata/profiles-list-reversed.json")
	if err != nil {
		t.Fatal(err)
	}
	duplicate := bytes.Replace(reversed, []byte(`"name": "writer"`), []byte(`"name": "researcher"`), 1)
	tests := []struct {
		name           string
		response       []byte
		requestSource  string
		transportError error
		mutateExchange func(*hermesbot.VerifiedRPCExchange)
	}{
		{name: "wrong requested source", response: valid, requestSource: "source:other"},
		{name: "wrong connection identity", response: valid, mutateExchange: func(exchange *hermesbot.VerifiedRPCExchange) {
			exchange.ConnectionIdentity = "hermes-dashboard:other:9119"
		}},
		{name: "wrong launcher digest", response: valid, mutateExchange: func(exchange *hermesbot.VerifiedRPCExchange) { exchange.LauncherDigest = strings.Repeat("c", 64) }},
		{name: "wrong code-root digest", response: valid, mutateExchange: func(exchange *hermesbot.VerifiedRPCExchange) { exchange.CodeRootDigest = strings.Repeat("c", 64) }},
		{name: "wrong Hermes version", response: valid, mutateExchange: func(exchange *hermesbot.VerifiedRPCExchange) { exchange.HermesVersion = "0.20.6" }},
		{name: "wrong Hermes revision", response: valid, mutateExchange: func(exchange *hermesbot.VerifiedRPCExchange) { exchange.HermesRevision = "drifted-private-revision" }},
		{name: "invalid canonical profile", response: bytes.Replace(valid, []byte(`"name": "researcher"`), []byte(`"name": "Researcher"`), 1)},
		{name: "reserved canonical profile", response: bytes.Replace(valid, []byte(`"name": "researcher"`), []byte(`"name": "root"`), 1)},
		{name: "duplicate canonical profile", response: duplicate},
		{name: "mismatched response ID", response: bytes.Replace(valid, []byte(`"id": "inventory-request-1"`), []byte(`"id": "another-request"`), 1)},
		{name: "wrong JSON-RPC version", response: bytes.Replace(valid, []byte(`"jsonrpc": "2.0"`), []byte(`"jsonrpc": "1.0"`), 1)},
		{name: "false Bot Mode marker", response: bytes.Replace(valid, []byte(`"bot_mode_protocol": true`), []byte(`"bot_mode_protocol": false`), 1)},
		{name: "null profiles", response: []byte(`{"jsonrpc":"2.0","id":"inventory-request-1","result":{"profiles":null,"bot_mode_protocol":true}}`)},
		{name: "unknown profile field", response: bytes.Replace(valid, []byte(`"has_avatar": true`), []byte(`"has_avatar": true, "private_unknown": true`), 1)},
		{name: "malformed response", response: []byte(`{"jsonrpc":`)},
		{name: "oversized response", response: append(append([]byte(nil), valid...), bytes.Repeat([]byte(" "), 1<<20)...)},
		{name: "authentication failure", response: valid, transportError: errors.New("401 dashboard token private-token")},
		{name: "unavailable transport", response: valid, transportError: errors.New("dial private-host failed")},
		{name: "missing response body", response: nil},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			exchange := verifiedExchange(nil)
			if test.mutateExchange != nil {
				test.mutateExchange(&exchange)
			}
			transport := &fakeLocalProfileRosterTransport{
				exchange: exchange,
				response: append([]byte(nil), test.response...),
				err:      test.transportError,
			}
			inventory := mustInventoryWithTransport(t, transport, time.Now)
			requestSource := test.requestSource
			if requestSource == "" {
				requestSource = "source:mini"
			}
			snapshot, inventoryErr := inventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: requestSource})
			if inventoryErr == nil {
				t.Fatal("Inventory accepted untrusted source or roster drift")
			}
			if !reflect.DeepEqual(snapshot, runtime.AgentSourceInventorySnapshot{}) {
				t.Fatalf("failed observation published partial snapshot: %#v", snapshot)
			}
		})
	}
}

func TestInventoryAcceptsAtMost256Profiles(t *testing.T) {
	t.Parallel()

	accepted := mustInventory(t, hermesExecutionSource(), profileRosterResponse(256), time.Now())
	snapshot, err := accepted.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"})
	if err != nil {
		t.Fatalf("256-profile Inventory: %v", err)
	}
	if len(snapshot.Agents) != 256 {
		t.Fatalf("profile count = %d, want 256", len(snapshot.Agents))
	}

	rejected := mustInventory(t, hermesExecutionSource(), profileRosterResponse(257), time.Now())
	if _, err := rejected.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"}); err == nil {
		t.Fatal("Inventory accepted 257 profiles")
	}
}

func TestInventoryDiscardsPrivateUpstreamFieldsAndSanitizesErrors(t *testing.T) {
	t.Parallel()

	response, err := os.ReadFile("testdata/profiles-list-reversed.json")
	if err != nil {
		t.Fatal(err)
	}
	inventory := mustInventory(t, hermesExecutionSource(), response, time.Now())
	snapshot, err := inventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"})
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	projected := fmt.Sprintf("%#v", snapshot)
	for _, private := range []string{
		"/private/discarded", "discarded writer description", "model-b", "provider-b", `map[private:true]`,
	} {
		if strings.Contains(projected, private) {
			t.Fatalf("snapshot leaked private upstream value %q", private)
		}
	}

	upstreamError := []byte(`{"jsonrpc":"2.0","id":"inventory-request-1","error":{"code":401,"message":"private-token","data":{"path":"/private/hermes"}}}`)
	errorInventory := mustInventory(t, hermesExecutionSource(), upstreamError, time.Now())
	if _, err := errorInventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"}); err == nil {
		t.Fatal("Inventory accepted upstream error envelope")
	} else if strings.Contains(err.Error(), "private-token") || strings.Contains(err.Error(), "/private/hermes") {
		t.Fatalf("public error leaked upstream body: %v", err)
	}

	returnedBody := &closeSpy{Reader: strings.NewReader("private-body")}
	transport := &fakeLocalProfileRosterTransport{
		exchange: verifiedExchange(returnedBody),
		err:      errors.New("private-token at /private/hermes"),
	}
	transportInventory := mustInventoryWithTransport(t, transport, time.Now)
	if _, err := transportInventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"}); err == nil {
		t.Fatal("Inventory accepted transport failure")
	} else if strings.Contains(err.Error(), "private-token") || strings.Contains(err.Error(), "/private/hermes") {
		t.Fatalf("public error leaked transport details: %v", err)
	}
	if !returnedBody.closed {
		t.Fatal("Inventory did not close a body returned alongside a transport error")
	}
}

func TestInventoryContractRevisionMatchesFrozenManifest(t *testing.T) {
	t.Parallel()

	digest := sha256.Sum256([]byte(hermesbot.InventoryContractManifest))
	if got := fmt.Sprintf("%x", digest); got != hermesbot.InventoryContractRevision {
		t.Fatalf("inventory contract revision = %s, want %s", hermesbot.InventoryContractRevision, got)
	}
	for _, required := range []string{
		"identity=source-qualified-length-prefixed-sha256",
		"limits=response_bytes:1048576,profiles:256,deadline_ms:5000,display_scalars:64",
		"evidence=profile_discovered,execution_adapter_not_approved",
		"resource_sharing=unknown,machine_shared,machine_shared,profile_scoped,unknown,profile_scoped",
		"attestation=connection_identity,launcher_sha256,code_root_sha256,hermes_version,hermes_revision",
	} {
		if !strings.Contains(hermesbot.InventoryContractManifest, required) {
			t.Fatalf("inventory contract manifest is missing %q", required)
		}
	}
}

func TestNewInventoryRejectsDriftedContractAndMissingDependencies(t *testing.T) {
	t.Parallel()

	valid := hermesbot.InventoryOptions{
		ExecutionSource: hermesExecutionSource(),
		AcceptedSource:  acceptedLocalSource(),
		Transport:       &fakeLocalProfileRosterTransport{},
		Clock:           time.Now,
		RequestID:       func() string { return "inventory-request-1" },
	}
	tests := []struct {
		name   string
		mutate func(*hermesbot.InventoryOptions)
	}{
		{name: "invalid Execution Source", mutate: func(options *hermesbot.InventoryOptions) { options.ExecutionSource.ID = "" }},
		{name: "wrong framework", mutate: func(options *hermesbot.InventoryOptions) { options.ExecutionSource.Framework = "other" }},
		{name: "missing connection identity", mutate: func(options *hermesbot.InventoryOptions) { options.AcceptedSource.ConnectionIdentity = "" }},
		{name: "unnormalized connection identity", mutate: func(options *hermesbot.InventoryOptions) { options.AcceptedSource.ConnectionIdentity = " source " }},
		{name: "invalid launcher digest", mutate: func(options *hermesbot.InventoryOptions) {
			options.AcceptedSource.LauncherDigest = strings.Repeat("A", 64)
		}},
		{name: "invalid code-root digest", mutate: func(options *hermesbot.InventoryOptions) { options.AcceptedSource.CodeRootDigest = "short" }},
		{name: "wrong accepted version", mutate: func(options *hermesbot.InventoryOptions) { options.AcceptedSource.HermesVersion = "0.20.6" }},
		{name: "wrong accepted revision", mutate: func(options *hermesbot.InventoryOptions) { options.AcceptedSource.HermesRevision = "drift" }},
		{name: "missing transport", mutate: func(options *hermesbot.InventoryOptions) { options.Transport = nil }},
		{name: "missing clock", mutate: func(options *hermesbot.InventoryOptions) { options.Clock = nil }},
		{name: "missing request ID source", mutate: func(options *hermesbot.InventoryOptions) { options.RequestID = nil }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			options := valid
			test.mutate(&options)
			if _, err := hermesbot.NewInventory(options); err == nil {
				t.Fatal("NewInventory accepted drifted construction")
			}
		})
	}
}

func TestInventoryAppliesOneUTCObservationAndFiveSecondDeadline(t *testing.T) {
	t.Parallel()

	response, err := os.ReadFile("testdata/profiles-list-valid.json")
	if err != nil {
		t.Fatal(err)
	}
	var deadlineRemaining time.Duration
	transport := &fakeLocalProfileRosterTransport{roundTrip: func(ctx context.Context, _ []byte) (hermesbot.VerifiedRPCExchange, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			return hermesbot.VerifiedRPCExchange{}, errors.New("deadline missing")
		}
		deadlineRemaining = time.Until(deadline)
		return verifiedExchange(io.NopCloser(bytes.NewReader(response))), nil
	}}
	clockCalls := 0
	zone := time.FixedZone("private-local-zone", -5*60*60)
	localObservation := time.Date(2026, 8, 22, 14, 0, 0, 123, zone)
	inventory := mustInventoryWithTransport(t, transport, func() time.Time {
		clockCalls++
		return localObservation
	})

	snapshot, err := inventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"})
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if deadlineRemaining < 4*time.Second || deadlineRemaining > 5*time.Second {
		t.Fatalf("transport deadline remaining = %s, want bounded five-second deadline", deadlineRemaining)
	}
	if clockCalls != 1 {
		t.Fatalf("clock calls = %d, want 1", clockCalls)
	}
	want := localObservation.UTC()
	if !snapshot.ObservedAt.Equal(want) || snapshot.ObservedAt.Location() != time.UTC ||
		!snapshot.ExecutionSource.LastSeenAt.Equal(want) || !snapshot.Agents[0].SourceAgent.LastSeenAt.Equal(want) {
		t.Fatalf("observation timestamps are inconsistent: %#v", snapshot)
	}
}

func TestInventoryRejectsInvalidGeneratedRequestIDBeforeTransport(t *testing.T) {
	t.Parallel()

	transport := &fakeLocalProfileRosterTransport{}
	inventory, err := hermesbot.NewInventory(hermesbot.InventoryOptions{
		ExecutionSource: hermesExecutionSource(),
		AcceptedSource:  acceptedLocalSource(),
		Transport:       transport,
		Clock:           time.Now,
		RequestID:       func() string { return " request-id " },
	})
	if err != nil {
		t.Fatalf("NewInventory: %v", err)
	}
	if _, err := inventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{ExecutionSourceID: "source:mini"}); err == nil {
		t.Fatal("Inventory accepted an unnormalized request ID")
	}
	if transport.request != nil {
		t.Fatalf("transport called with invalid request ID: %q", transport.request)
	}
}

func mustInventory(t *testing.T, source conversation.ExecutionSource, response []byte, now time.Time) *hermesbot.Inventory {
	t.Helper()
	transport := &fakeLocalProfileRosterTransport{
		exchange: hermesbot.VerifiedRPCExchange{
			ConnectionIdentity: "hermes-dashboard:mini:9119",
			LauncherDigest:     strings.Repeat("a", 64),
			CodeRootDigest:     strings.Repeat("b", 64),
			HermesVersion:      hermesbot.HermesVersion,
			HermesRevision:     hermesbot.HermesRevision,
		},
		response: append([]byte(nil), response...),
	}
	inventory, err := hermesbot.NewInventory(hermesbot.InventoryOptions{
		ExecutionSource: source,
		AcceptedSource:  acceptedLocalSource(),
		Transport:       transport,
		Clock:           func() time.Time { return now },
		RequestID:       func() string { return "inventory-request-1" },
	})
	if err != nil {
		t.Fatalf("NewInventory: %v", err)
	}
	return inventory
}

func mustInventoryWithTransport(t *testing.T, transport *fakeLocalProfileRosterTransport, clock func() time.Time) *hermesbot.Inventory {
	t.Helper()
	inventory, err := hermesbot.NewInventory(hermesbot.InventoryOptions{
		ExecutionSource: hermesExecutionSource(),
		AcceptedSource:  acceptedLocalSource(),
		Transport:       transport,
		Clock:           clock,
		RequestID:       func() string { return "inventory-request-1" },
	})
	if err != nil {
		t.Fatalf("NewInventory: %v", err)
	}
	return inventory
}

func profileRosterResponse(count int) []byte {
	var response strings.Builder
	response.WriteString(`{"jsonrpc":"2.0","id":"inventory-request-1","result":{"profiles":[`)
	for index := 0; index < count; index++ {
		if index > 0 {
			response.WriteByte(',')
		}
		fmt.Fprintf(&response, `{"name":"p%03d","path":"/private/discarded/%03d","is_default":false,"model":null,"provider":null,"description":"private","display_name":"Profile %03d","skill_count":0,"has_avatar":false}`, index, index, index)
	}
	response.WriteString(`],"bot_mode_protocol":true}}`)
	return []byte(response.String())
}

func acceptedLocalSource() hermesbot.LocalSourceContract {
	return hermesbot.LocalSourceContract{
		ConnectionIdentity: "hermes-dashboard:mini:9119",
		LauncherDigest:     strings.Repeat("a", 64),
		CodeRootDigest:     strings.Repeat("b", 64),
		HermesVersion:      hermesbot.HermesVersion,
		HermesRevision:     hermesbot.HermesRevision,
	}
}

func verifiedExchange(body io.ReadCloser) hermesbot.VerifiedRPCExchange {
	return hermesbot.VerifiedRPCExchange{
		ConnectionIdentity: "hermes-dashboard:mini:9119",
		LauncherDigest:     strings.Repeat("a", 64),
		CodeRootDigest:     strings.Repeat("b", 64),
		HermesVersion:      hermesbot.HermesVersion,
		HermesRevision:     hermesbot.HermesRevision,
		Body:               body,
	}
}

type blockingReadCloser struct {
	closed     chan struct{}
	once       sync.Once
	mu         sync.Mutex
	closeCount int
}

type joiningReadCloser struct {
	closed  chan struct{}
	release chan struct{}
	once    sync.Once
}

type closeSpy struct {
	io.Reader
	closed bool
}

func (body *closeSpy) Close() error {
	body.closed = true
	return nil
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{closed: make(chan struct{})}
}

func newJoiningReadCloser() *joiningReadCloser {
	return &joiningReadCloser{closed: make(chan struct{}), release: make(chan struct{})}
}

func (body *joiningReadCloser) Read([]byte) (int, error) {
	<-body.closed
	<-body.release
	return 0, errors.New("private interrupted body")
}

func (body *joiningReadCloser) Close() error {
	body.once.Do(func() { close(body.closed) })
	return nil
}

func (body *blockingReadCloser) Read([]byte) (int, error) {
	<-body.closed
	return 0, errors.New("private blocked body")
}

func (body *blockingReadCloser) Close() error {
	body.mu.Lock()
	body.closeCount++
	body.mu.Unlock()
	body.once.Do(func() { close(body.closed) })
	return nil
}

func (body *blockingReadCloser) CloseCount() int {
	body.mu.Lock()
	defer body.mu.Unlock()
	return body.closeCount
}

type fakeLocalProfileRosterTransport struct {
	request   []byte
	exchange  hermesbot.VerifiedRPCExchange
	response  []byte
	err       error
	roundTrip func(context.Context, []byte) (hermesbot.VerifiedRPCExchange, error)
}

func (transport *fakeLocalProfileRosterTransport) RoundTrip(ctx context.Context, request []byte) (hermesbot.VerifiedRPCExchange, error) {
	transport.request = append([]byte(nil), request...)
	if transport.roundTrip != nil {
		return transport.roundTrip(ctx, request)
	}
	exchange := transport.exchange
	if transport.response != nil {
		exchange.Body = io.NopCloser(bytes.NewReader(transport.response))
	}
	return exchange, transport.err
}

func hermesExecutionSource() conversation.ExecutionSource {
	return conversation.ExecutionSource{
		ID: "source:mini", AccountID: "account:one", Framework: "hermes",
		InstanceID: "instance:mini", GatewayID: "gateway:mini", DisplayName: "Hermes · Mac mini",
		ResourceSharing: conversation.ResourceSharingDisclosure{
			ProviderCredentials: conversation.ResourceUnknown,
			Filesystem:          conversation.ResourceMachineShared,
			BrowserSessions:     conversation.ResourceMachineShared,
			FrameworkSessions:   conversation.ResourceProfileScoped,
			SourceMemory:        conversation.ResourceUnknown,
			ToolConfiguration:   conversation.ResourceProfileScoped,
		},
	}
}
