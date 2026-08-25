package control_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/exec/hermesbot"
	"github.com/tobsai/fort/ui"
)

func TestHermesInventoryPersistsUnavailableAgentOptionsAcrossRestart(t *testing.T) {
	response, err := os.ReadFile(filepath.Join("..", "exec", "hermesbot", "testdata", "profiles-list-reversed.json"))
	if err != nil {
		t.Fatal(err)
	}
	observedAt := time.Date(2026, 8, 22, 20, 0, 0, 0, time.UTC)
	transport := &integrationHermesRosterTransport{response: response}
	source := integrationHermesExecutionSource()
	inventory, err := hermesbot.NewInventory(hermesbot.InventoryOptions{
		ExecutionSource: source,
		AcceptedSource:  integrationHermesSourceContract(),
		Transport:       transport,
		Clock:           func() time.Time { return observedAt },
		RequestID:       func() string { return "inventory-request-1" },
	})
	if err != nil {
		t.Fatal(err)
	}

	databasePath := filepath.Join(t.TempDir(), "fort.db")
	firstStore, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	optionSource, err := control.NewSourceInventoryAgentOptionSource(firstStore, []control.SourceInventoryRegistration{{
		AccountID: source.AccountID, ExecutionSourceID: source.ID, Inventory: inventory,
	}})
	if err != nil {
		t.Fatal(err)
	}

	initial, err := optionSource.RecheckAgentOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertUnavailableHermesOptions(t, initial)
	if transport.calls != 1 || !bytes.Contains(transport.request, []byte(`"method":"profiles.list"`)) ||
		!bytes.Contains(transport.request, []byte(`"include_sessions":false`)) {
		t.Fatalf("Hermes transport calls = %d, request = %s", transport.calls, transport.request)
	}
	if err := firstStore.Close(); err != nil {
		t.Fatal(err)
	}

	reopenedStore, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopenedStore.Close()
	persistedSource, err := control.NewSourceInventoryAgentOptionSource(reopenedStore, []control.SourceInventoryRegistration{{
		AccountID: source.AccountID, ExecutionSourceID: source.ID,
	}})
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := persistedSource.AgentOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertUnavailableHermesOptions(t, persisted)
	if !reflect.DeepEqual(persisted, initial) {
		t.Fatalf("persisted options = %+v, want initial options %+v", persisted, initial)
	}

	service := control.NewAgentChannelService(reopenedStore, nil, persistedSource)
	if _, err := service.CreateAgentChannel(context.Background(), persisted[0].ID, "Hermes Researcher"); control.ErrorCode(err) != control.ErrorPrimaryAgentUnready {
		t.Fatalf("disabled Hermes option enrollment error = %v (%q), want %q", err, control.ErrorCode(err), control.ErrorPrimaryAgentUnready)
	}
	channels, err := service.ListAgentChannels(context.Background(), "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 0 {
		t.Fatalf("disabled Hermes option created Agent Channels: %+v", channels)
	}
}

func assertUnavailableHermesOptions(t *testing.T, options []ui.AgentOption) {
	t.Helper()
	if len(options) != 2 {
		t.Fatalf("Hermes Agent options = %+v, want two", options)
	}
	ids := []string{options[0].ID, options[1].ID}
	sortedIDs := append([]string(nil), ids...)
	sort.Strings(sortedIDs)
	if !reflect.DeepEqual(ids, sortedIDs) {
		t.Fatalf("Hermes Agent option ids = %v, want sorted ids %v", ids, sortedIDs)
	}
	displays := []string{options[0].DisplayName, options[1].DisplayName}
	sort.Strings(displays)
	if !reflect.DeepEqual(displays, []string{"Researcher", "Writer"}) {
		t.Fatalf("Hermes Agent option display names = %v", displays)
	}
	for _, option := range options {
		if !strings.HasPrefix(option.ID, "source-agent:hermes:v1:") || option.State != "unavailable" || option.Reason != control.ExecutionAdapterNotApproved {
			t.Fatalf("Hermes Agent option = %+v, want source-qualified unavailable row", option)
		}
		if err := option.Binding.Validate(); err == nil {
			t.Fatalf("Hermes discovery-only binding unexpectedly validates: %+v", option.Binding)
		}
	}
}

type integrationHermesRosterTransport struct {
	response []byte
	request  []byte
	calls    int
}

func (transport *integrationHermesRosterTransport) RoundTrip(_ context.Context, request []byte) (hermesbot.VerifiedRPCExchange, error) {
	transport.calls++
	transport.request = append([]byte(nil), request...)
	contract := integrationHermesSourceContract()
	return hermesbot.VerifiedRPCExchange{
		ConnectionIdentity: contract.ConnectionIdentity,
		LauncherDigest:     contract.LauncherDigest,
		CodeRootDigest:     contract.CodeRootDigest,
		HermesVersion:      contract.HermesVersion,
		HermesRevision:     contract.HermesRevision,
		Body:               io.NopCloser(bytes.NewReader(transport.response)),
	}, nil
}

func integrationHermesSourceContract() hermesbot.LocalSourceContract {
	return hermesbot.LocalSourceContract{
		ConnectionIdentity: "hermes-dashboard:mini:9119",
		LauncherDigest:     strings.Repeat("a", 64),
		CodeRootDigest:     strings.Repeat("b", 64),
		HermesVersion:      hermesbot.HermesVersion,
		HermesRevision:     hermesbot.HermesRevision,
	}
}

func integrationHermesExecutionSource() conversation.ExecutionSource {
	return conversation.ExecutionSource{
		ID: "source:mini", AccountID: "account:one", Framework: "hermes", InstanceID: "mini-local",
		GatewayID: "gateway:mini", DisplayName: "Hermes · Mac mini",
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
