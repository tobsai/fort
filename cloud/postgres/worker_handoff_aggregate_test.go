package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	coreworker "github.com/tobsai/fort/core/worker"
)

func TestStartWorkerHandoffAggregatePinsAttemptBeforeProviderStart(t *testing.T) {
	now := time.Date(2026, 8, 21, 23, 20, 0, 0, time.UTC)
	evidence := workerAggregateEvidence{
		targetID: "target:handoff:1", originID: "handoff:1", attemptID: "attempt:1",
		attemptNumber: 1, authorAgentID: "agent:builder", behaviorID: "behavior:builder:2",
		bindingID: "binding:builder:3",
	}
	tx := &fakeTransaction{queryRowHook: func(query string, _ []any) row {
		if strings.Contains(query, "from fort_private.handoff") {
			return fakeRow{values: []any{"queued", 1, ""}}
		}
		return fakeRow{err: errors.New("unexpected Handoff start query")}
	}}
	insertedMapping, markedWorking := false, false
	tx.execHook = func(query string, arguments []any) (int64, error) {
		switch {
		case strings.Contains(query, "insert into fort_private.handoff_attempt"):
			insertedMapping = len(arguments) == 8 && arguments[1] == evidence.originID &&
				arguments[2] == evidence.attemptID && arguments[4] == evidence.authorAgentID
			return 1, nil
		case strings.Contains(query, "update fort_private.handoff"):
			markedWorking = strings.Contains(query, "state='working'")
			return 1, nil
		default:
			return 0, errors.New("unexpected Handoff start statement")
		}
	}
	stopped, err := startWorkerHandoffAggregate(context.Background(), tx, testAccountID, evidence, now)
	if err != nil || stopped || !insertedMapping || !markedWorking {
		t.Fatalf("start Handoff aggregate = stopped %t, mapping %t, working %t, err %v",
			stopped, insertedMapping, markedWorking, err)
	}
}

func TestCommitWorkerHandoffAggregateCreatesOneAuthoritativeLinkedResult(t *testing.T) {
	now := time.Date(2026, 8, 21, 23, 21, 0, 0, time.UTC)
	evidence := workerAggregateEvidence{
		targetID: "target:handoff:1", originID: "handoff:1", attemptID: "attempt:1",
		authorAgentID: "agent:builder", behaviorID: "behavior:builder:2", bindingID: "binding:builder:3",
		conversationID: "conversation:group:1", turnID: "turn:handoff:1",
	}
	body := collaborationEncryptedBody{Ciphertext: []byte("ciphertext"), KeyID: "key:1",
		Nonce: []byte("0123456789ab"), Digest: strings.Repeat("d", 64), PlaintextBytes: 19, Version: 1}
	tx := &fakeTransaction{queryRowHook: func(query string, arguments []any) row {
		switch {
		case strings.Contains(query, "from fort_private.handoff"):
			return fakeRow{values: []any{"working", "turn:group:1"}}
		case strings.Contains(query, "insert into fort_private.conversation_message"):
			if !strings.Contains(query, "'handoff_result'") || arguments[4] != evidence.originID ||
				arguments[5] != evidence.authorAgentID {
				return fakeRow{err: errors.New("Handoff result message lost attribution")}
			}
			return fakeRow{values: []any{int64(91)}}
		default:
			return fakeRow{err: errors.New("unexpected Handoff terminal query")}
		}
	}}
	updatedHandoff, settledChild, settledSource, appendedEvent := false, false, false, false
	tx.execHook = func(query string, arguments []any) (int64, error) {
		switch {
		case strings.Contains(query, "update fort_private.handoff"):
			updatedHandoff = strings.Contains(query, "state='succeeded'")
			return 1, nil
		case strings.Contains(query, "update fort_private.conversation_turn") && !strings.Contains(query, "from fort_private"):
			settledChild = arguments[1] == evidence.turnID
			return 1, nil
		case strings.Contains(query, "update fort_private.conversation_turn"):
			settledSource = arguments[1] == "turn:group:1"
			return 1, nil
		case strings.Contains(query, "insert into fort_private.ledger_event"):
			var metadata map[string]any
			_ = json.Unmarshal([]byte(arguments[len(arguments)-2].(string)), &metadata)
			appendedEvent = metadata["conversation_message_id"] == float64(91)
			return 1, nil
		default:
			return 0, errors.New("unexpected Handoff terminal statement")
		}
	}
	messageID, err := commitWorkerHandoffAggregate(context.Background(), tx, testAccountID,
		evidence, controlapi.WorkerTerminalCommand{Status: coreworker.TerminalCompleted, CommittedAt: now}, body)
	if err != nil || messageID != 91 || !updatedHandoff || !settledChild || !settledSource || !appendedEvent {
		t.Fatalf("commit Handoff aggregate = message %d handoff %t child %t source %t event %t err %v",
			messageID, updatedHandoff, settledChild, settledSource, appendedEvent, err)
	}
}
