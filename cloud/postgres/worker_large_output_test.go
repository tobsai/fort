package postgres

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/securebody"
	coreworker "github.com/tobsai/fort/core/worker"
)

func TestPrepareWorkerOutputMessageCreatesBoundedReferenceAboveInlineLimit(t *testing.T) {
	ring := collaborationTestKeyRing()
	store, err := newStoreWithKeyRing(&fakeDatabase{}, testAccountID, ring)
	if err != nil {
		t.Fatal(err)
	}
	command := controlapi.WorkerTerminalCommand{
		TargetID: "target:large", ExecutionAttemptID: "attempt:large",
		Status: coreworker.TerminalCompleted,
		Output: controlapi.WorkerOutputReference{
			ArtifactID: "artifact:large", Digest: strings.Repeat("a", 64),
		},
	}
	plaintextLength := int64(controlapi.MaximumArtifactChunkPlaintextBytes + 1)
	persisted, err := store.prepareWorkerOutputMessage(testAccountID, command, plaintextLength)
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		t.Fatal(err)
	}
	body, err := cipher.open(securebody.Scope{AccountID: testAccountID,
		RecordType: "conversation_message", RecordID: command.TargetID}, persisted)
	if err != nil {
		t.Fatal(err)
	}
	var reference workerOutputArtifactMessage
	if err := json.Unmarshal([]byte(body), &reference); err != nil {
		t.Fatalf("reference body = %q: %v", body, err)
	}
	if reference.Type != workerOutputArtifactMessageType || reference.ArtifactID != command.Output.ArtifactID ||
		reference.Digest != command.Output.Digest || reference.PlaintextLength != plaintextLength {
		t.Fatalf("reference = %+v", reference)
	}
	if persisted.PlaintextBytes > controlapi.MaximumArtifactChunkPlaintextBytes || persisted.Digest == command.Output.Digest {
		t.Fatalf("persisted reference bytes/digest = %d %s", persisted.PlaintextBytes, persisted.Digest)
	}
}

func TestPrepareWorkerOutputMessageRequiresInlineBodyAtOrBelowLimit(t *testing.T) {
	store, err := newStoreWithKeyRing(&fakeDatabase{}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatal(err)
	}
	command := controlapi.WorkerTerminalCommand{
		TargetID: "target:small", Status: coreworker.TerminalCompleted,
		Output: controlapi.WorkerOutputReference{ArtifactID: "artifact:small", Digest: strings.Repeat("b", 64)},
	}
	_, err = store.prepareWorkerOutputMessage(testAccountID, command,
		int64(controlapi.MaximumArtifactChunkPlaintextBytes))
	if !errors.Is(err, controlapi.ErrWorkerRequestInvalid) {
		t.Fatalf("small artifact without inline body error = %v", err)
	}
}
