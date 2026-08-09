package ui_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/ui"
)

var _ ui.PrimaryChannelPort = (*control.PrimaryChannelService)(nil)

type primaryAPIFake struct {
	agent       ui.PrimaryAgentView
	channels    []conversation.PrimaryChannelSummary
	details     map[string]ui.PrimaryChannelDetail
	needs       []ui.PrimaryNeedsYouItem
	errByCall   map[string]error
	calls       []string
	lastState   string
	lastOption  string
	lastName    string
	lastPinned  bool
	lastTurnID  string
	lastText    string
	lastTarget  string
	createdID   string
	retryTarget conversation.Target
}

func (f *primaryAPIFake) call(name string) error {
	f.calls = append(f.calls, name)
	return f.errByCall[name]
}

func (f *primaryAPIFake) PrimaryAgent(context.Context) (ui.PrimaryAgentView, error) {
	return f.agent, f.call("primary-agent")
}

func (f *primaryAPIFake) SetPrimaryAgent(_ context.Context, optionID string) (ui.PrimaryAgentView, error) {
	f.lastOption = optionID
	return f.agent, f.call("set-primary-agent")
}

func (f *primaryAPIFake) ClearPrimaryAgent(context.Context) error {
	return f.call("clear-primary-agent")
}

func (f *primaryAPIFake) RecheckPrimaryAgent(context.Context) (ui.PrimaryAgentView, error) {
	return f.agent, f.call("recheck-primary-agent")
}

func (f *primaryAPIFake) ListChannels(_ context.Context, state string) ([]conversation.PrimaryChannelSummary, error) {
	f.lastState = state
	return f.channels, f.call("list-channels")
}

func (f *primaryAPIFake) GetChannel(_ context.Context, id string) (ui.PrimaryChannelDetail, error) {
	if err := f.call("get-channel"); err != nil {
		return ui.PrimaryChannelDetail{}, err
	}
	item, ok := f.details[id]
	if !ok {
		return ui.PrimaryChannelDetail{}, sql.ErrNoRows
	}
	return item, nil
}

func (f *primaryAPIFake) CreateChannel(_ context.Context, name string) (ui.PrimaryChannelDetail, error) {
	f.lastName = name
	if err := f.call("create-channel"); err != nil {
		return ui.PrimaryChannelDetail{}, err
	}
	return f.details[f.createdID], nil
}

func (f *primaryAPIFake) RenameChannel(_ context.Context, id, name string) error {
	f.lastName = name
	return f.call("rename-channel")
}

func (f *primaryAPIFake) SetChannelState(_ context.Context, id string, state conversation.ConversationState) error {
	f.lastState = string(state)
	return f.call("set-channel-state")
}

func (f *primaryAPIFake) SetChannelPinned(_ context.Context, id string, pinned bool) error {
	f.lastPinned = pinned
	return f.call("set-channel-pinned")
}

func (f *primaryAPIFake) PostTurn(_ context.Context, channelID, clientTurnID, text string) (conversation.TurnResult, error) {
	f.lastTurnID, f.lastText = clientTurnID, text
	if err := f.call("post-turn"); err != nil {
		return conversation.TurnResult{}, err
	}
	return conversation.TurnResult{Turn: conversation.Turn{ID: "turn-1"}}, nil
}

func (f *primaryAPIFake) CancelTarget(_ context.Context, channelID, targetID string) error {
	f.lastTarget = targetID
	return f.call("cancel-target")
}

func (f *primaryAPIFake) RetryTarget(_ context.Context, channelID, targetID string) (conversation.Target, error) {
	f.lastTarget = targetID
	return f.retryTarget, f.call("retry-target")
}

func (f *primaryAPIFake) RecheckAndRetryTarget(_ context.Context, channelID, targetID string) (conversation.Target, error) {
	f.lastTarget = targetID
	return f.retryTarget, f.call("recheck-and-retry-target")
}

func (f *primaryAPIFake) NeedsYou(context.Context) ([]ui.PrimaryNeedsYouItem, error) {
	return f.needs, f.call("needs-you")
}

func newPrimaryAPIFake() *primaryAPIFake {
	detail := ui.PrimaryChannelDetail{
		Conversation: conversation.Conversation{ID: "channel-1", Title: "Private", State: conversation.ConversationOpen},
		Targets: []conversation.Target{
			{ID: "failed-target", State: conversation.TargetFailed, Attempt: 1},
		},
	}
	return &primaryAPIFake{
		details:     map[string]ui.PrimaryChannelDetail{"channel-1": detail},
		errByCall:   map[string]error{},
		createdID:   "channel-1",
		retryTarget: conversation.Target{ID: "retry-target", State: conversation.TargetQueued, Attempt: 2},
	}
}

func primaryMux(port ui.PrimaryChannelPort) *http.ServeMux {
	mux := http.NewServeMux()
	ui.New(ui.Deps{Primary: port}).RegisterPrimaryRoutes(mux)
	return mux
}

func primaryRequest(t *testing.T, handler http.Handler, method, path string, body any, want int) []byte {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	return primaryRawRequest(t, handler, method, path, data, want)
}

func primaryRawRequest(t *testing.T, handler http.Handler, method, path string, body []byte, want int) []byte {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:4000"
	req.Host = "127.0.0.1:4087"
	if len(body) != 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, rec.Code, want, rec.Body.String())
	}
	return rec.Body.Bytes()
}

func assertJSONArrayField(t *testing.T, body []byte, field string) {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		t.Fatalf("decode object: %v body=%s", err, body)
	}
	if got := strings.TrimSpace(string(object[field])); got != "[]" {
		t.Fatalf("%s=%s, want []", field, got)
	}
}

func TestPrimaryRoutesExposeOnlyServerSelectedChannelOperations(t *testing.T) {
	fake := newPrimaryAPIFake()
	mux := primaryMux(fake)

	assertJSONArrayField(t, primaryRequest(t, mux, http.MethodGet, "/api/settings/primary-agent", nil, http.StatusOK), "options")
	primaryRequest(t, mux, http.MethodPut, "/api/settings/primary-agent", map[string]any{"option_id": "primary-option:v1:abc"}, http.StatusOK)
	if fake.lastOption != "primary-option:v1:abc" {
		t.Fatalf("selected option=%q", fake.lastOption)
	}
	primaryRequest(t, mux, http.MethodPost, "/api/settings/primary-agent/recheck", nil, http.StatusOK)
	primaryRequest(t, mux, http.MethodDelete, "/api/settings/primary-agent", nil, http.StatusNoContent)

	if got := strings.TrimSpace(string(primaryRequest(t, mux, http.MethodGet, "/api/channels", nil, http.StatusOK))); got != "[]" {
		t.Fatalf("empty Channels=%s, want []", got)
	}
	if fake.lastState != "open" {
		t.Fatalf("default state=%q, want open", fake.lastState)
	}
	primaryRequest(t, mux, http.MethodGet, "/api/channels?state=all", nil, http.StatusOK)
	if fake.lastState != "all" {
		t.Fatalf("explicit state=%q, want all", fake.lastState)
	}

	created := primaryRequest(t, mux, http.MethodPost, "/api/channels", map[string]any{"name": "  Private  "}, http.StatusCreated)
	if fake.lastName != "Private" {
		t.Fatalf("created name=%q, want trimmed", fake.lastName)
	}
	for _, field := range []string{"participants", "messages", "turns"} {
		assertJSONArrayField(t, created, field)
	}
	detail := primaryRequest(t, mux, http.MethodGet, "/api/channels/channel-1", nil, http.StatusOK)
	for _, field := range []string{"participants", "messages", "turns"} {
		assertJSONArrayField(t, detail, field)
	}

	primaryRequest(t, mux, http.MethodPatch, "/api/channels/channel-1", map[string]any{"name": "Renamed"}, http.StatusNoContent)
	primaryRequest(t, mux, http.MethodPatch, "/api/channels/channel-1", map[string]any{"state": "archived"}, http.StatusNoContent)
	primaryRequest(t, mux, http.MethodPatch, "/api/channels/channel-1", map[string]any{"pinned": true}, http.StatusNoContent)
	if fake.lastName != "Renamed" || fake.lastState != "archived" || !fake.lastPinned {
		t.Fatalf("patch projection name=%q state=%q pinned=%v", fake.lastName, fake.lastState, fake.lastPinned)
	}

	turn := primaryRequest(t, mux, http.MethodPost, "/api/channels/channel-1/turns", map[string]any{
		"client_turn_id": "3b241101-e2bb-4255-8caf-4136c566a962", "text": "  Hello  ",
	}, http.StatusAccepted)
	assertJSONArrayField(t, turn, "targets")
	if fake.lastText != "Hello" || fake.lastTurnID != "3b241101-e2bb-4255-8caf-4136c566a962" {
		t.Fatalf("turn id=%q text=%q", fake.lastTurnID, fake.lastText)
	}

	primaryRequest(t, mux, http.MethodPost, "/api/channels/channel-1/targets/failed-target/retry", nil, http.StatusAccepted)
	primaryRequest(t, mux, http.MethodPost, "/api/channels/channel-1/targets/failed-target/recheck-and-retry", nil, http.StatusAccepted)
	channel := fake.details["channel-1"]
	channel.Targets = append(channel.Targets, conversation.Target{ID: "active-target", State: conversation.TargetWorking, Attempt: 1})
	fake.details["channel-1"] = channel
	primaryRequest(t, mux, http.MethodPost, "/api/channels/channel-1/targets/active-target/cancel", nil, http.StatusNoContent)

	if got := strings.TrimSpace(string(primaryRequest(t, mux, http.MethodGet, "/api/needs-you", nil, http.StatusOK))); got != "[]" {
		t.Fatalf("empty needs-you=%s, want []", got)
	}
}

func TestPrimaryTurnIdempotencyReachesServiceBeforeActiveStateProjection(t *testing.T) {
	fake := newPrimaryAPIFake()
	detail := fake.details["channel-1"]
	detail.Targets = append(detail.Targets, conversation.Target{ID: "working-target", State: conversation.TargetWorking, Attempt: 1})
	fake.details["channel-1"] = detail
	body := primaryRequest(t, primaryMux(fake), http.MethodPost, "/api/channels/channel-1/turns", map[string]any{
		"client_turn_id": "3b241101-e2bb-4255-8caf-4136c566a962", "text": "Hello",
	}, http.StatusAccepted)
	if len(body) == 0 || fake.lastTurnID == "" {
		t.Fatalf("idempotent turn did not reach the authoritative service: calls=%v body=%s", fake.calls, body)
	}
}

func TestPrimaryRoutesRejectUnknownQueriesAndUnboundedJSON(t *testing.T) {
	tests := []struct {
		method string
		path   string
		body   []byte
	}{
		{http.MethodGet, "/api/settings/primary-agent?probe=true", nil},
		{http.MethodGet, "/api/channels?unknown=x", nil},
		{http.MethodGet, "/api/channels?state=open&state=all", nil},
		{http.MethodGet, "/api/channels?state=closed", nil},
		{http.MethodGet, "/api/channels/channel-1?extra=1", nil},
		{http.MethodGet, "/api/needs-you?extra=1", nil},
		{http.MethodPut, "/api/settings/primary-agent", []byte(`{"option_id":"x","authority":"caller-chosen"}`)},
		{http.MethodPut, "/api/settings/primary-agent", []byte(`{"option_id":"x"} {}`)},
		{http.MethodPut, "/api/settings/primary-agent", []byte{'{', '"', 'o', 'p', 't', 'i', 'o', 'n', '_', 'i', 'd', '"', ':', '"', 0xff, '"', '}'}},
		{http.MethodPost, "/api/channels", []byte(`{"name":"x","participant_ids":["forbidden"]}`)},
		{http.MethodPatch, "/api/channels/channel-1", []byte(`{}`)},
		{http.MethodPatch, "/api/channels/channel-1", []byte(`{"name":"x","pinned":true}`)},
		{http.MethodPatch, "/api/channels/channel-1", []byte(`{"state":"closed"}`)},
		{http.MethodPost, "/api/channels/channel-1/turns", []byte(`{"client_turn_id":"not-a-uuid","text":"hello"}`)},
		{http.MethodPost, "/api/channels/channel-1/turns", []byte(`{"client_turn_id":"3b241101-e2bb-4255-8caf-4136c566a962","text":" "}`)},
		{http.MethodPost, "/api/channels/channel-1/targets/failed-target/retry?force=true", nil},
		{http.MethodPost, "/api/channels/channel-1/targets/failed-target/retry", []byte(`{}`)},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			fake := newPrimaryAPIFake()
			body := primaryRawRequest(t, primaryMux(fake), test.method, test.path, test.body, http.StatusBadRequest)
			var response map[string]string
			if err := json.Unmarshal(body, &response); err != nil || response["error"] == "" {
				t.Fatalf("error response=%s decode=%v", body, err)
			}
			if response["code"] != "" {
				t.Fatalf("validation invented non-domain code %q", response["code"])
			}
		})
	}
}

func TestPrimaryMutationsRequireLocalSameOriginAndJSONMediaType(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		host       string
		origin     string
		fetchSite  string
		mediaType  string
		want       int
	}{
		{name: "remote client", remoteAddr: "203.0.113.8:4000", host: "127.0.0.1:4087", mediaType: "application/json", want: http.StatusForbidden},
		{name: "cross origin", remoteAddr: "127.0.0.1:4000", host: "127.0.0.1:4087", origin: "https://attacker.example", fetchSite: "cross-site", mediaType: "application/json", want: http.StatusForbidden},
		{name: "dns rebinding host", remoteAddr: "127.0.0.1:4000", host: "attacker.example", origin: "http://attacker.example", fetchSite: "same-origin", mediaType: "application/json", want: http.StatusForbidden},
		{name: "simple text request", remoteAddr: "127.0.0.1:4000", host: "127.0.0.1:4087", origin: "http://127.0.0.1:4087", fetchSite: "same-origin", mediaType: "text/plain", want: http.StatusUnsupportedMediaType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := newPrimaryAPIFake()
			request := httptest.NewRequest(http.MethodPost, "/api/channels", strings.NewReader(`{"name":"Private"}`))
			request.RemoteAddr, request.Host = test.remoteAddr, test.host
			request.Header.Set("Content-Type", test.mediaType)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.fetchSite != "" {
				request.Header.Set("Sec-Fetch-Site", test.fetchSite)
			}
			result := httptest.NewRecorder()
			primaryMux(fake).ServeHTTP(result, request)
			if result.Code != test.want || len(fake.calls) != 0 {
				t.Fatalf("status=%d want=%d calls=%v body=%s", result.Code, test.want, fake.calls, result.Body.String())
			}
		})
	}
}

func TestPrimaryReadsRequireLoopbackPeerAndHost(t *testing.T) {
	for _, path := range []string{
		"/api/settings/primary-agent",
		"/api/channels",
		"/api/channels/channel-1",
		"/api/channels/channel-1/events",
		"/api/needs-you",
	} {
		for _, access := range []struct {
			name       string
			remoteAddr string
			host       string
		}{
			{name: "remote peer", remoteAddr: "192.0.2.25:4000", host: "127.0.0.1:4087"},
			{name: "LAN host", remoteAddr: "127.0.0.1:4000", host: "192.168.1.25:4087"},
		} {
			t.Run(access.name+" "+path, func(t *testing.T) {
				fake := newPrimaryAPIFake()
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				request := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
				request.RemoteAddr, request.Host = access.remoteAddr, access.host
				result := httptest.NewRecorder()
				primaryMux(fake).ServeHTTP(result, request)
				if result.Code != http.StatusForbidden || len(fake.calls) != 0 {
					t.Fatalf("status=%d calls=%v body=%s", result.Code, fake.calls, result.Body.String())
				}
			})
		}
	}
}

type primaryCodedError struct {
	code string
}

func (e primaryCodedError) Error() string              { return e.code }
func (e primaryCodedError) PrimaryChannelCode() string { return e.code }

func TestPrimaryRouteStatusAndClosedCodeMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "missing", err: sql.ErrNoRows, wantStatus: http.StatusNotFound},
		{name: "active", err: conversation.NewBoundedError(conversation.ErrorConversationActive, errors.New("active")), wantStatus: http.StatusConflict, wantCode: "conversation_active"},
		{name: "context", err: conversation.ErrContextTooLarge, wantStatus: http.StatusConflict, wantCode: "conversation_context_limit"},
		{name: "not configured", err: primaryCodedError{"primary_agent_not_configured"}, wantStatus: http.StatusConflict, wantCode: "primary_agent_not_configured"},
		{name: "unready", err: primaryCodedError{"primary_agent_unready"}, wantStatus: http.StatusConflict, wantCode: "primary_agent_unready"},
		{name: "drift", err: primaryCodedError{"primary_agent_drift"}, wantStatus: http.StatusConflict, wantCode: "primary_agent_drift"},
		{name: "policy unavailable", err: primaryCodedError{"chat_policy_unavailable"}, wantStatus: http.StatusServiceUnavailable, wantCode: "chat_policy_unavailable"},
		{name: "authority", err: primaryCodedError{"chat_authority_violation"}, wantStatus: http.StatusConflict, wantCode: "chat_authority_violation"},
		{name: "invariant", err: primaryCodedError{"primary_channel_invariant"}, wantStatus: http.StatusConflict, wantCode: "primary_channel_invariant"},
		{name: "provider incomplete", err: primaryCodedError{"provider_incomplete"}, wantStatus: http.StatusConflict, wantCode: "provider_incomplete"},
		{name: "unknown backend", err: errors.New("database offline"), wantStatus: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := newPrimaryAPIFake()
			fake.errByCall["primary-agent"] = test.err
			body := primaryRequest(t, primaryMux(fake), http.MethodGet, "/api/settings/primary-agent", nil, test.wantStatus)
			var response map[string]string
			if err := json.Unmarshal(body, &response); err != nil {
				t.Fatal(err)
			}
			if response["error"] == "" || response["code"] != test.wantCode {
				t.Fatalf("response=%+v, want code %q", response, test.wantCode)
			}
		})
	}

	body := primaryRequest(t, primaryMux(nil), http.MethodGet, "/api/channels", nil, http.StatusServiceUnavailable)
	var unavailable map[string]string
	if err := json.Unmarshal(body, &unavailable); err != nil || unavailable["error"] == "" || unavailable["code"] != "" {
		t.Fatalf("unavailable response=%s decode=%v", body, err)
	}
}

func TestPrimaryTargetRoutesValidateNestedIdentityAndStateBeforeMutation(t *testing.T) {
	fake := newPrimaryAPIFake()
	channel := fake.details["channel-1"]
	channel.Targets = append(channel.Targets, conversation.Target{ID: "active-target", State: conversation.TargetWorking, Attempt: 1})
	fake.details["channel-1"] = channel
	mux := primaryMux(fake)

	primaryRequest(t, mux, http.MethodPost, "/api/channels/channel-1/targets/foreign/retry", nil, http.StatusNotFound)
	primaryRequest(t, mux, http.MethodPost, "/api/channels/missing/targets/failed-target/retry", nil, http.StatusNotFound)
	primaryRequest(t, mux, http.MethodPost, "/api/channels/channel-1/targets/active-target/retry", nil, http.StatusConflict)
	primaryRequest(t, mux, http.MethodPost, "/api/channels/channel-1/targets/failed-target/cancel", nil, http.StatusConflict)

	for _, call := range fake.calls {
		if call == "retry-target" || call == "cancel-target" {
			t.Fatalf("invalid nested action reached mutation: calls=%v", fake.calls)
		}
	}
}

type primaryEventWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
	cancel context.CancelFunc
}

func (w *primaryEventWriter) Header() http.Header { return w.header }
func (w *primaryEventWriter) WriteHeader(status int) {
	w.status = status
}
func (w *primaryEventWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}
func (w *primaryEventWriter) Flush() { w.cancel() }

func TestPrimaryChannelEventsStreamCanonicalNonNullDetail(t *testing.T) {
	fake := newPrimaryAPIFake()
	mux := primaryMux(fake)
	ctx, cancel := context.WithCancel(context.Background())
	writer := &primaryEventWriter{header: http.Header{}, cancel: cancel}
	req := httptest.NewRequest(http.MethodGet, "/api/channels/channel-1/events", nil).WithContext(ctx)
	req.RemoteAddr, req.Host = "127.0.0.1:4000", "127.0.0.1:4087"
	mux.ServeHTTP(writer, req)

	if writer.status != http.StatusOK || writer.header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status=%d headers=%v body=%s", writer.status, writer.header, writer.body.String())
	}
	line := strings.TrimPrefix(strings.TrimSpace(writer.body.String()), "data: ")
	var detail map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &detail); err != nil {
		t.Fatalf("decode SSE detail: %v stream=%q", err, writer.body.String())
	}
	for _, field := range []string{"participants", "messages", "turns"} {
		if strings.TrimSpace(string(detail[field])) != "[]" {
			t.Fatalf("SSE %s=%s, want []", field, detail[field])
		}
	}
}

func TestPrimaryNeedsYouNormalizesRecoveryActions(t *testing.T) {
	fake := newPrimaryAPIFake()
	fake.needs = []ui.PrimaryNeedsYouItem{{Target: conversation.Target{ID: "failed", State: conversation.TargetFailed}}}
	body := primaryRequest(t, primaryMux(fake), http.MethodGet, "/api/needs-you", nil, http.StatusOK)
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil || len(items) != 1 {
		t.Fatalf("decode needs-you: %v body=%s", err, body)
	}
	if got := strings.TrimSpace(string(items[0]["recovery_actions"])); got != "[]" {
		t.Fatalf("recovery_actions=%s, want []", got)
	}
}

func TestPrimaryChannelEventMissingReturnsJSONBeforeSSEHeaders(t *testing.T) {
	body := primaryRequest(t, primaryMux(newPrimaryAPIFake()), http.MethodGet, "/api/channels/missing/events", nil, http.StatusNotFound)
	var response map[string]string
	if err := json.Unmarshal(body, &response); err != nil || response["error"] == "" {
		t.Fatalf("missing event response=%s decode=%v", body, err)
	}
}

func TestPrimarySettingsDeleteRejectsBodies(t *testing.T) {
	body := primaryRawRequest(t, primaryMux(newPrimaryAPIFake()), http.MethodDelete, "/api/settings/primary-agent", []byte(`{}`), http.StatusBadRequest)
	if !bytes.Contains(body, []byte(`"error"`)) {
		t.Fatalf("response=%s", body)
	}
}

func TestPrimaryEventsRejectQueries(t *testing.T) {
	primaryRequest(t, primaryMux(newPrimaryAPIFake()), http.MethodGet, "/api/channels/channel-1/events?since=1", nil, http.StatusBadRequest)
}

func TestPrimaryEventLoopStopsOnCanceledRequest(t *testing.T) {
	fake := newPrimaryAPIFake()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/channels/channel-1/events", nil).WithContext(ctx)
	req.RemoteAddr, req.Host = "127.0.0.1:4000", "127.0.0.1:4087"
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		primaryMux(fake).ServeHTTP(rec, req)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled event request did not stop")
	}
}
