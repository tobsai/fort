package capability

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	corecap "github.com/tobsai/fort/core/capability"
)

type fakeCodexAppServerProcess struct {
	responses *bytes.Reader
	requests  bytes.Buffer
	closed    bool
	digest    string
}

func newFakeCodexAppServerProcess(responses string) *fakeCodexAppServerProcess {
	return &fakeCodexAppServerProcess{responses: bytes.NewReader([]byte(responses)), digest: "fixture-executable"}
}

func (p *fakeCodexAppServerProcess) ExecutableDigest() string { return p.digest }

func (p *fakeCodexAppServerProcess) Read(value []byte) (int, error) {
	return p.responses.Read(value)
}

func (p *fakeCodexAppServerProcess) Write(value []byte) (int, error) {
	return p.requests.Write(value)
}

func (p *fakeCodexAppServerProcess) Close() error {
	p.closed = true
	return nil
}

type fakeCodexAppServerStarter struct {
	process CodexAppServerProcess
	err     error
	starts  int
}

type fakeCodexContractVerifier struct {
	contract CodexAppServerContract
	err      error
	calls    atomic.Int32
}

func (v *fakeCodexContractVerifier) Verify(context.Context) (CodexAppServerContract, error) {
	v.calls.Add(1)
	return v.contract, v.err
}

func (s *fakeCodexAppServerStarter) Start(context.Context) (CodexAppServerProcess, error) {
	s.starts++
	return s.process, s.err
}

func TestVerifiedCodexInspectorDefersAndCachesContractVerification(t *testing.T) {
	process := newFakeCodexAppServerProcess(strings.Join([]string{
		validCodexInitializeResponse,
		`{"id":2,"result":{"account":{"type":"apiKey"},"requiresOpenaiAuth":true}}`,
		`{"id":3,"result":{"config":{"model":"gpt-5.6-sol"},"origins":{}}}`,
		`{"id":4,"result":{"data":[{"model":"gpt-5.6-sol","isDefault":true}],"nextCursor":null}}`,
	}, "\n") + "\n")
	starter := &fakeCodexAppServerStarter{process: process}
	verifier := &fakeCodexContractVerifier{contract: CodexAppServerContract{
		ExecutableDigest:         "fixture-executable",
		NormalSchemaDigest:       codexNormalSchemaDigest,
		NormalSchemaFiles:        codexNormalSchemaFiles,
		ExperimentalSchemaDigest: codexExperimentalSchemaDigest,
		ExperimentalSchemaFiles:  codexExperimentalSchemaFiles,
	}}
	inspector := NewVerifiedCodexAppServerInspector(starter, verifier)
	if verifier.calls.Load() != 0 || starter.starts != 0 {
		t.Fatal("constructor performed live capability work")
	}
	if _, err := inspector.Inspect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if verifier.calls.Load() != 1 || starter.starts != 1 {
		t.Fatalf("verify calls=%d starts=%d", verifier.calls.Load(), starter.starts)
	}
}

func TestVerifiedCodexInspectorFailsClosedBeforeStartingAppServer(t *testing.T) {
	starter := &fakeCodexAppServerStarter{}
	verifier := &fakeCodexContractVerifier{err: &ProbeError{Reason: corecap.ReasonIncompatibleVersion}}
	inspector := NewVerifiedCodexAppServerInspector(starter, verifier)

	for range 2 {
		_, err := inspector.Inspect(context.Background())
		var probeError *ProbeError
		if !errors.As(err, &probeError) || probeError.Reason != corecap.ReasonIncompatibleVersion {
			t.Fatalf("error = %#v", err)
		}
	}
	if verifier.calls.Load() != 1 || starter.starts != 0 {
		t.Fatalf("verify calls=%d starts=%d", verifier.calls.Load(), starter.starts)
	}
}

func TestCodexAppServerInspectorReadsAuthenticatedPaginatedCatalogWithoutTurn(t *testing.T) {
	process := newFakeCodexAppServerProcess(strings.Join([]string{
		validCodexInitializeResponse,
		`{"method":"account/updated","params":{}}`,
		`{"id":2,"result":{"account":{"type":"chatgpt","email":"private@example.com","planType":"pro"},"requiresOpenaiAuth":true}}`,
		`{"id":3,"result":{"config":{"model":"gpt-5.6-sol"},"origins":{"model":"PRIVATE-CONFIG-ORIGIN"}}}`,
		`{"id":4,"result":{"data":[{"id":"row-1","model":"gpt-5.6-sol","isDefault":false},{"id":"row-2","model":"gpt-5.6-terra","isDefault":false}],"nextCursor":"private-cursor"}}`,
		`{"id":5,"result":{"data":[{"id":"row-3","model":"gpt-5.6-luna","isDefault":false},{"id":"row-4","model":"gpt-5.5","isDefault":true}],"nextCursor":null}}`,
	}, "\n") + "\n")
	starter := &fakeCodexAppServerStarter{process: process}
	inspector := NewCodexAppServerInspector(starter, CodexAppServerContract{
		ExecutableDigest:         "fixture-executable",
		NormalSchemaDigest:       codexNormalSchemaDigest,
		NormalSchemaFiles:        codexNormalSchemaFiles,
		ExperimentalSchemaDigest: codexExperimentalSchemaDigest,
		ExperimentalSchemaFiles:  codexExperimentalSchemaFiles,
		GmailIsolationReady:      true,
	})

	inspection, err := inspector.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.AccountReady || inspection.AccountHandle != "authenticated" {
		t.Fatalf("account = %#v", inspection)
	}
	if len(inspection.Models) != 4 || !inspection.Models["gpt-5.6-sol"] ||
		!inspection.Models["gpt-5.6-terra"] || !inspection.Models["gpt-5.6-luna"] ||
		!inspection.Models["gpt-5.5"] {
		t.Fatalf("models = %#v", inspection.Models)
	}
	if inspection.DefaultModel != "gpt-5.6-sol" {
		t.Fatalf("default model = %q", inspection.DefaultModel)
	}
	if inspection.NormalSchemaDigest != codexNormalSchemaDigest || inspection.NormalSchemaFiles != codexNormalSchemaFiles ||
		inspection.ExperimentalSchemaDigest != codexExperimentalSchemaDigest || inspection.ExperimentalSchemaFiles != codexExperimentalSchemaFiles ||
		!inspection.GmailIsolationReady {
		t.Fatalf("contract facts = %#v", inspection)
	}
	if starter.starts != 1 || !process.closed {
		t.Fatalf("starts = %d, closed = %v", starter.starts, process.closed)
	}
	if strings.Contains(fmt.Sprintf("%#v", inspection), "private@example.com") ||
		strings.Contains(fmt.Sprintf("%#v", inspection), "private-cursor") {
		t.Fatal("private app-server data escaped normalized inspection")
	}

	requests := decodeAppServerRequests(t, process.requests.Bytes())
	wantMethods := []string{"initialize", "initialized", "account/read", "config/read", "model/list", "model/list"}
	if len(requests) != len(wantMethods) {
		t.Fatalf("requests = %#v", requests)
	}
	for index, want := range wantMethods {
		if requests[index].Method != want {
			t.Fatalf("request %d method = %q, want %q", index, requests[index].Method, want)
		}
	}
	if requests[0].Params["capabilities"].(map[string]any)["experimentalApi"] != true {
		t.Fatalf("initialize params = %#v", requests[0].Params)
	}
	if requests[2].Params["refreshToken"] != false {
		t.Fatalf("account params = %#v", requests[2].Params)
	}
	if requests[3].Params["includeLayers"] != false {
		t.Fatalf("config params = %#v", requests[3].Params)
	}
	if requests[4].Params["includeHidden"] != true {
		t.Fatalf("model params = %#v", requests[4].Params)
	}
	if requests[5].Params["cursor"] != "private-cursor" || requests[5].Params["includeHidden"] != true {
		t.Fatalf("paginated model params = %#v", requests[5].Params)
	}
	for _, request := range requests {
		if strings.HasPrefix(request.Method, "thread/") || strings.HasPrefix(request.Method, "turn/") {
			t.Fatalf("inspector created a model turn: %#v", request)
		}
	}
}

func TestCodexAppServerInspectorUsesUniqueCatalogDefaultWhenConfigIsUnset(t *testing.T) {
	process := newFakeCodexAppServerProcess(strings.Join([]string{
		validCodexInitializeResponse,
		`{"id":2,"result":{"account":{"type":"apiKey"},"requiresOpenaiAuth":true}}`,
		`{"id":3,"result":{"config":{"model":null},"origins":{}}}`,
		`{"id":4,"result":{"data":[{"model":"gpt-5.6-sol","isDefault":false},{"model":"gpt-5.6-terra","isDefault":true}],"nextCursor":null}}`,
	}, "\n") + "\n")
	inspector := NewCodexAppServerInspector(&fakeCodexAppServerStarter{process: process}, CodexAppServerContract{ExecutableDigest: "fixture-executable"})

	inspection, err := inspector.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if inspection.DefaultModel != "gpt-5.6-terra" {
		t.Fatalf("catalog default = %q", inspection.DefaultModel)
	}
}

func TestCodexAppServerInspectorRejectsMalformedConfiguredModelSelector(t *testing.T) {
	process := newFakeCodexAppServerProcess(strings.Join([]string{
		validCodexInitializeResponse,
		`{"id":2,"result":{"account":{"type":"apiKey"},"requiresOpenaiAuth":true}}`,
		`{"id":3,"result":{"config":{"model":" gpt-5.6-sol "},"origins":{}}}`,
		`{"id":4,"result":{"data":[{"model":"gpt-5.6-sol","isDefault":true}],"nextCursor":null}}`,
	}, "\n") + "\n")
	inspector := NewCodexAppServerInspector(&fakeCodexAppServerStarter{process: process}, CodexAppServerContract{ExecutableDigest: "fixture-executable"})

	_, err := inspector.Inspect(context.Background())
	var probeError *ProbeError
	if !errors.As(err, &probeError) || probeError.Reason != corecap.ReasonCommandContractChanged {
		t.Fatalf("malformed configured selector error = %#v", err)
	}
}

func TestCodexAppServerInspectorRejectsNullTypedResolutionFields(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		modelList string
	}{
		{
			name:      "null config",
			config:    `{"config":null,"origins":{}}`,
			modelList: `{"data":[{"model":"gpt-5.6-sol","isDefault":true}],"nextCursor":null}`,
		},
		{
			name:      "null origins",
			config:    `{"config":{"model":null},"origins":null}`,
			modelList: `{"data":[{"model":"gpt-5.6-sol","isDefault":true}],"nextCursor":null}`,
		},
		{
			name:      "null model data",
			config:    `{"config":{"model":null},"origins":{}}`,
			modelList: `{"data":null,"nextCursor":null}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process := newFakeCodexAppServerProcess(strings.Join([]string{
				validCodexInitializeResponse,
				`{"id":2,"result":{"account":{"type":"apiKey"},"requiresOpenaiAuth":true}}`,
				`{"id":3,"result":` + test.config + `}`,
				`{"id":4,"result":` + test.modelList + `}`,
			}, "\n") + "\n")
			inspector := NewCodexAppServerInspector(&fakeCodexAppServerStarter{process: process}, CodexAppServerContract{ExecutableDigest: "fixture-executable"})

			_, err := inspector.Inspect(context.Background())
			var probeError *ProbeError
			if !errors.As(err, &probeError) || probeError.Reason != corecap.ReasonCommandContractChanged {
				t.Fatalf("null typed field error = %#v", err)
			}
		})
	}
}

func TestCodexAppServerInspectorFailsClosedForAuthenticationAndAmbiguousDefault(t *testing.T) {
	process := newFakeCodexAppServerProcess(strings.Join([]string{
		validCodexInitializeResponse,
		`{"id":2,"result":{"account":null,"requiresOpenaiAuth":true}}`,
		`{"id":3,"result":{"config":{"model":null},"origins":{}}}`,
		`{"id":4,"result":{"data":[{"model":"gpt-5.6-sol","isDefault":true},{"model":"gpt-5.5","isDefault":true}],"nextCursor":null}}`,
	}, "\n") + "\n")
	inspector := NewCodexAppServerInspector(&fakeCodexAppServerStarter{process: process}, CodexAppServerContract{ExecutableDigest: "fixture-executable"})

	inspection, err := inspector.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if inspection.AccountReady || inspection.AccountHandle != "" {
		t.Fatalf("account = %#v", inspection)
	}
	if inspection.DefaultModel != "" {
		t.Fatalf("ambiguous default = %q", inspection.DefaultModel)
	}
	if !inspection.Models["gpt-5.6-sol"] || !inspection.Models["gpt-5.5"] {
		t.Fatalf("models = %#v", inspection.Models)
	}
}

func TestCodexAppServerInspectorRejectsProtocolErrorsAndPaginationCycles(t *testing.T) {
	tests := []struct {
		name      string
		responses string
	}{
		{
			name: "invalid initialize contract",
			responses: strings.Join([]string{
				`{"id":1,"result":{}}`,
				`{"id":2,"result":{"account":{"type":"apiKey"},"requiresOpenaiAuth":true}}`,
				`{"id":3,"result":{"data":[],"nextCursor":null}}`,
			}, "\n") + "\n",
		},
		{
			name: "server error",
			responses: strings.Join([]string{
				validCodexInitializeResponse,
				`{"id":2,"error":{"code":-32600,"message":"PRIVATE SERVER ERROR"}}`,
			}, "\n") + "\n",
		},
		{
			name: "unknown account contract",
			responses: strings.Join([]string{
				validCodexInitializeResponse,
				`{"id":2,"result":{"account":{"type":"future-private-account"},"requiresOpenaiAuth":true}}`,
				`{"id":3,"result":{"data":[],"nextCursor":null}}`,
			}, "\n") + "\n",
		},
		{
			name: "pagination cycle",
			responses: strings.Join([]string{
				validCodexInitializeResponse,
				`{"id":2,"result":{"account":{"type":"apiKey"},"requiresOpenaiAuth":true}}`,
				`{"id":3,"result":{"config":{"model":null},"origins":{}}}`,
				`{"id":4,"result":{"data":[],"nextCursor":"same"}}`,
				`{"id":5,"result":{"data":[],"nextCursor":"same"}}`,
			}, "\n") + "\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process := newFakeCodexAppServerProcess(test.responses)
			inspector := NewCodexAppServerInspector(&fakeCodexAppServerStarter{process: process}, CodexAppServerContract{ExecutableDigest: "fixture-executable"})
			_, err := inspector.Inspect(context.Background())
			var probeError *ProbeError
			if err == nil || !strings.Contains(err.Error(), "capability probe failed") ||
				!errors.As(err, &probeError) || probeError.Reason != corecap.ReasonCommandContractChanged {
				t.Fatalf("error = %#v", err)
			}
			if strings.Contains(err.Error(), "PRIVATE") {
				t.Fatal("private protocol error leaked")
			}
			if !process.closed {
				t.Fatal("process was not closed")
			}
		})
	}
}

func TestCodexAppServerInspectorRejectsDifferentExecutableIdentityBeforeProtocolUse(t *testing.T) {
	process := newFakeCodexAppServerProcess(validCodexInitializeResponse + "\n")
	process.digest = "current-executable"
	inspector := NewCodexAppServerInspector(&fakeCodexAppServerStarter{process: process}, CodexAppServerContract{
		ExecutableDigest: "schema-executable",
	})
	_, err := inspector.Inspect(context.Background())
	var probeError *ProbeError
	if !errors.As(err, &probeError) || probeError.Reason != corecap.ReasonIncompatibleVersion {
		t.Fatalf("error = %#v", err)
	}
	if process.requests.Len() != 0 || !process.closed {
		t.Fatalf("requests = %q, closed = %v", process.requests.String(), process.closed)
	}
}

const validCodexInitializeResponse = `{"id":1,"result":{"codexHome":"/private/hidden","platformFamily":"unix","platformOs":"macos","userAgent":"codex_cli_rs/0.146.0-alpha.9.2"}}`

type decodedAppServerRequest struct {
	ID     *int           `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

func decodeAppServerRequests(t *testing.T, encoded []byte) []decodedAppServerRequest {
	t.Helper()
	scanner := bufio.NewScanner(bytes.NewReader(encoded))
	var requests []decodedAppServerRequest
	for scanner.Scan() {
		var request decodedAppServerRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, request)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return requests
}
