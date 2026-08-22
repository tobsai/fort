package controlapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
)

func TestWorkerHandlerDispatchesAuthenticatedContextPage(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 23, 30, 0, 0, time.UTC)
	token, base := authenticatedWorkerRepository()
	repository := &fakeWorkerContextRepository{
		fakeWorkerRepository: base,
		page: controlapi.WorkerContextPage{
			ContextManifestID: "manifest:turn:1",
			ManifestDigest:    strings.Repeat("a", 64),
			Items: []controlapi.WorkerContextItem{
				{
					Kind:    controlapi.WorkerContextMessageKind,
					Ordinal: 0,
					Message: &controlapi.WorkerContextMessage{
						MessageID: 17, ConversationID: "conversation:1", TurnID: "turn:1",
						MessageKind: "human", AuthorKind: "human", AuthorID: "human:1",
						Body: "Use the pinned evidence only.", CreatedAt: now.Add(-time.Minute),
					},
				},
				{
					Kind:    controlapi.WorkerContextArtifactKind,
					Ordinal: 1,
					Artifact: &controlapi.WorkerContextArtifactReference{
						ArtifactID: "artifact:context:1", Kind: "context",
						ExecutionAttemptID: "attempt:source", ExpectedChunkCount: 2,
						ExpectedPlaintextLength: 4096, ExpectedEncodedLength: 4128,
						LogicalDigest: strings.Repeat("b", 64), CreatedAt: now.Add(-2 * time.Minute), FinalizedAt: now.Add(-time.Minute),
					},
				},
			},
			NextCursor: "",
		},
	}
	handler := controlapi.WorkerHandler(repository, func() time.Time { return now })
	body := `{
		"operation":"context_page","account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio",
		"idempotency_key":"context-page:1",
		"context_page":{"target_id":"target:1","execution_attempt_id":"attempt:1","lease_id":"lease:1","fence_token":41,"cursor":""}
	}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, workerRequest(body, token))
	if response.Code != http.StatusOK {
		t.Fatalf("context page status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if repository.calls != 1 {
		t.Fatalf("context page calls = %d, want 1", repository.calls)
	}
	command := repository.command
	if command.AccountID != workerTestAccountID || command.WorkerID != "worker:mac-studio" || command.MachineID != "machine:mac-studio" ||
		command.TargetID != "target:1" || command.ExecutionAttemptID != "attempt:1" || command.LeaseID != "lease:1" ||
		command.FenceToken != 41 || command.Cursor != "" || command.IdempotencyKey != "context-page:1" || command.ObservedAt != now {
		t.Fatalf("context page command = %#v", command)
	}
	var page controlapi.WorkerContextPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode context page: %v", err)
	}
	if len(page.Items) != 2 || page.Items[0].Message == nil || page.Items[0].Message.Body != "Use the pinned evidence only." ||
		page.Items[1].Artifact == nil || page.Items[1].Artifact.ArtifactID != "artifact:context:1" {
		t.Fatalf("context page = %#v", page)
	}
	encoded := response.Body.String()
	for _, forbidden := range []string{"ciphertext", "key_id", "nonce", "chunks", "encryption_key"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("context page exposed %q: %s", forbidden, encoded)
		}
	}
}

func TestWorkerHandlerRejectsInvalidContextIdentityAndMapsPageLimit(t *testing.T) {
	t.Parallel()

	token, base := authenticatedWorkerRepository()
	repository := &fakeWorkerContextRepository{fakeWorkerRepository: base}
	handler := controlapi.WorkerHandler(repository, time.Now)
	invalid := `{
		"operation":"context_page","account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio",
		"idempotency_key":"context-page:bad",
		"context_page":{"target_id":"target:1","execution_attempt_id":"attempt:1","lease_id":"lease:1","fence_token":0,"cursor":"not a cursor"}
	}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, workerRequest(invalid, token))
	if response.Code != http.StatusBadRequest || repository.calls != 0 {
		t.Fatalf("invalid context status/calls = %d/%d, want 400/0; body=%s", response.Code, repository.calls, response.Body.String())
	}

	repository.err = controlapi.ErrWorkerContextPageLimit
	valid := `{
		"operation":"context_page","account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio",
		"idempotency_key":"context-page:large",
		"context_page":{"target_id":"target:1","execution_attempt_id":"attempt:1","lease_id":"lease:1","fence_token":41,"cursor":""}
	}`
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, workerRequest(valid, token))
	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), `"code":"payload_limit"`) {
		t.Fatalf("large context status/body = %d/%s, want 413 payload_limit", response.Code, response.Body.String())
	}
}

type fakeWorkerContextRepository struct {
	*fakeWorkerRepository
	command controlapi.WorkerContextPageCommand
	page    controlapi.WorkerContextPage
	err     error
	calls   int
}

func (repository *fakeWorkerContextRepository) ReadWorkerContextPage(_ context.Context, command controlapi.WorkerContextPageCommand) (controlapi.WorkerContextPage, error) {
	repository.calls++
	repository.command = command
	if repository.err != nil {
		return controlapi.WorkerContextPage{}, repository.err
	}
	return repository.page, nil
}

var _ controlapi.WorkerContextRepository = (*fakeWorkerContextRepository)(nil)
