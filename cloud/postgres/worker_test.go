package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/securebody"
	coreworker "github.com/tobsai/fort/core/worker"
)

func TestMachineCredentialReadsOnlyOneExplicitAccountScopedIdentity(t *testing.T) {
	t.Parallel()

	tx := &fakeTransaction{queryRowHook: func(sql string, arguments []any) row {
		if !strings.Contains(sql, "from fort_private.worker") || !strings.Contains(sql, "account_id = $1") ||
			!strings.Contains(sql, "worker_id = $2") || !strings.Contains(sql, "machine_id = $3") {
			return fakeRow{err: errors.New("machine credential query is not explicitly scoped")}
		}
		if len(arguments) != 3 || arguments[0] != testAccountID || arguments[1] != "worker:mac-studio" || arguments[2] != "machine:mac-studio" {
			return fakeRow{err: errors.New("machine credential query arguments are wrong")}
		}
		return fakeRow{values: []any{
			testAccountID, "worker:mac-studio", "machine:mac-studio", strings.Repeat("a", 64), "enrolled",
		}}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}

	credential, err := store.MachineCredential(context.Background(), testAccountID, "worker:mac-studio", "machine:mac-studio")
	if err != nil {
		t.Fatalf("MachineCredential: %v", err)
	}
	if credential.AccountID != testAccountID || credential.WorkerID != "worker:mac-studio" || credential.MachineID != "machine:mac-studio" ||
		credential.TokenHash != strings.Repeat("a", 64) || credential.State != controlapi.MachineCredentialEnrolled {
		t.Fatalf("credential = %#v", credential)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("credential transaction = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestRecordWorkerReadinessPersistsImmutableEvidenceAndHeartbeatAtomically(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	evidence := json.RawMessage(`{"frameworks":["openclaw"],"ready":true}`)
	evidenceHash := sha256.Sum256(evidence)
	command := controlapi.WorkerReadinessCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		IdempotencyKey: "readiness:1", CapabilityRevisionID: "capability:7", Revision: 7,
		CapabilityEvidence: evidence, EvidenceDigest: hex.EncodeToString(evidenceHash[:]), ObservedAt: now,
	}
	tx := &fakeTransaction{execHook: func(sql string, arguments []any) (int64, error) {
		switch {
		case strings.Contains(sql, "set_config('fort.account_id'"):
			return 1, nil
		case strings.Contains(sql, "fort_private.idempotency_record"):
			return 1, nil
		case strings.Contains(sql, "insert into fort_private.worker_capability_revision"):
			if len(arguments) != 7 || arguments[0] != testAccountID || arguments[1] != command.CapabilityRevisionID ||
				arguments[2] != command.WorkerID || arguments[3] != command.Revision || arguments[4] != string(evidence) ||
				arguments[5] != command.EvidenceDigest || arguments[6] != now {
				return 0, errors.New("capability revision arguments are wrong")
			}
			return 1, nil
		case strings.Contains(sql, "update fort_private.worker"):
			if !strings.Contains(sql, "state <> 'revoked'") || len(arguments) != 4 {
				return 0, errors.New("worker heartbeat is not revocation guarded")
			}
			return 1, nil
		default:
			return 0, errors.New("unexpected readiness statement")
		}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.RecordWorkerReadiness(context.Background(), command)
	if err != nil {
		t.Fatalf("RecordWorkerReadiness: %v", err)
	}
	if result.Status != "ready" || result.CapabilityRevisionID != command.CapabilityRevisionID || result.ObservedAt != now {
		t.Fatalf("readiness result = %#v", result)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("readiness transaction = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestRecordWorkerReadinessRejectsRevokedMachineWithoutPartialCommit(t *testing.T) {
	t.Parallel()

	evidence := json.RawMessage(`{"ready":true}`)
	evidenceHash := sha256.Sum256(evidence)
	command := controlapi.WorkerReadinessCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		IdempotencyKey: "readiness:1", CapabilityRevisionID: "capability:7", Revision: 7,
		CapabilityEvidence: evidence, EvidenceDigest: hex.EncodeToString(evidenceHash[:]),
		ObservedAt: time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC),
	}
	tx := &fakeTransaction{execHook: func(sql string, _ []any) (int64, error) {
		if strings.Contains(sql, "set_config('fort.account_id'") {
			return 1, nil
		}
		if strings.Contains(sql, "idempotency_record") {
			return 1, nil
		}
		if strings.Contains(sql, "worker_capability_revision") {
			return 1, nil
		}
		if strings.Contains(sql, "update fort_private.worker") {
			return 0, nil
		}
		return 0, errors.New("unexpected statement")
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.RecordWorkerReadiness(context.Background(), command)
	if !errors.Is(err, controlapi.ErrWorkerRevoked) {
		t.Fatalf("revoked readiness error = %v, want ErrWorkerRevoked", err)
	}
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("revoked transaction = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestRecordWorkerReadinessReplayKeepsIdempotencyStableAcrossServerHeartbeatTimes(t *testing.T) {
	t.Parallel()

	evidence := json.RawMessage(`{"ready":true}`)
	evidenceHash := sha256.Sum256(evidence)
	command := controlapi.WorkerReadinessCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		IdempotencyKey: "readiness:replay", CapabilityRevisionID: "capability:7", Revision: 7,
		CapabilityEvidence: evidence, EvidenceDigest: hex.EncodeToString(evidenceHash[:]),
		ObservedAt: time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC),
	}
	var reservedDigest string
	first := &fakeTransaction{execHook: func(query string, arguments []any) (int64, error) {
		switch {
		case strings.Contains(query, "set_config"):
			return 1, nil
		case strings.Contains(query, "idempotency_record"):
			reservedDigest = arguments[3].(string)
			return 1, nil
		default:
			return 1, nil
		}
	}}
	second := &fakeTransaction{}
	second.execHook = func(query string, arguments []any) (int64, error) {
		switch {
		case strings.Contains(query, "set_config"):
			return 1, nil
		case strings.Contains(query, "idempotency_record"):
			if arguments[3] != reservedDigest {
				return 0, errors.New("replayed readiness digest changed with server time")
			}
			return 0, nil
		case strings.Contains(query, "worker_capability_revision"):
			return 0, errors.New("replayed readiness inserted duplicate capability evidence")
		default:
			return 1, nil
		}
	}
	second.queryRowHook = func(query string, _ []any) row {
		if !strings.Contains(query, "idempotency_record") {
			return fakeRow{err: errors.New("unexpected replay query")}
		}
		return fakeRow{values: []any{reservedDigest, "worker_capability_revision", command.CapabilityRevisionID}}
	}
	store, err := newStore(&fakeDatabase{transactions: []transaction{first, second}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordWorkerReadiness(context.Background(), command); err != nil {
		t.Fatalf("first readiness: %v", err)
	}
	command.ObservedAt = command.ObservedAt.Add(10 * time.Second)
	if _, err := store.RecordWorkerReadiness(context.Background(), command); err != nil {
		t.Fatalf("readiness replay: %v", err)
	}
}

func TestClaimWorkerTargetAtomicallyPinsExactIdentityAndFence(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 10, 0, 0, time.UTC)
	command := controlapi.WorkerClaimCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1",
		IdempotencyKey: "claim:1", CapabilityRevisionID: "capability:7",
		ClaimedAt: now, ExpiresAt: now.Add(2 * time.Minute),
	}
	authority := coreworker.AuthoritySnapshot{
		ID: "authority:effective:1", Revision: "revision:7", Permissions: []string{"message.append"},
		ContextRecordIDs: []string{"message:17"},
	}
	participantAuthorityJSON := []byte(`{"authority_id":"authority:binding:1","authority_revision":"revision:7","policy_id":"policy:1","policy_revision":"revision:2"}`)
	delegationJSON, err := json.Marshal(map[string]any{
		"id": authority.ID, "permissions": authority.Permissions, "context_record_ids": authority.ContextRecordIDs,
	})
	if err != nil {
		t.Fatal(err)
	}
	prompt := "prepared prompt"
	promptDigestBytes := sha256.Sum256([]byte(prompt))
	promptDigest := hex.EncodeToString(promptDigestBytes[:])
	ring := collaborationTestKeyRing()
	promptEnvelope, err := ring.Encrypt(securebody.Scope{
		AccountID: testAccountID, RecordType: "group_turn_prompt", RecordID: "turn:1",
	}, []byte(prompt))
	if err != nil {
		t.Fatal(err)
	}
	promptCiphertext, err := base64.RawURLEncoding.DecodeString(promptEnvelope.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	promptNonce, err := base64.RawURLEncoding.DecodeString(promptEnvelope.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	tx := &fakeTransaction{}
	tx.queryRowHook = func(query string, arguments []any) row {
		switch {
		case strings.Contains(query, "for update of target"):
			if !strings.Contains(query, "capability.capability_revision_id = $3") || !strings.Contains(query, "binding.worker_id = $2") {
				return fakeRow{err: errors.New("claim eligibility does not pin worker capability and binding")}
			}
			if strings.Contains(query, "delegation_grant as grant") || !strings.Contains(query, "delegation_grant as authority_grant") {
				return fakeRow{err: errors.New("claim eligibility uses a reserved delegation grant alias")}
			}
			return fakeRow{values: []any{
				"target:1", "queued", int64(0), now.Add(time.Hour), "initial", "turn:1", "conversation:agent:researcher", "turn:1", "human_group",
				"agent:researcher", "behavior:4", "binding:9",
				"participant:1", "seat:researcher", participantAuthorityJSON, []byte(delegationJSON), "delegation:1", "context:1",
				"source:studio", "source-agent:researcher", "researcher", "openclaw:researcher", "openclaw",
				"openclaw-main", "openclaw-main", "model.chat.openclaw", "adapter:1", strings.Repeat("a", 64),
				"authority:binding:1", "revision:7", "policy:1", "revision:2", "isolated", "source_managed",
				[]byte(`{"values":["openclaw-ready","workdir=/Users/fort/Workspaces/researcher"],"location_kind":"computer"}`), "ready:openclaw", "revision:4",
				"machine:mac-studio", promptCiphertext, promptEnvelope.KeyID, promptNonce, promptDigest, int64(len(prompt)),
				sql.NullTime{}, sql.NullString{}, sql.NullString{},
			}}
		case strings.Contains(query, "insert into fort_private.worker_lease"):
			return fakeRow{values: []any{int64(41)}}
		default:
			return fakeRow{err: errors.New("unexpected claim query")}
		}
	}
	tx.execHook = func(query string, arguments []any) (int64, error) {
		switch {
		case strings.Contains(query, "set_config('fort.account_id'"):
			return 1, nil
		case strings.Contains(query, "fort_private.idempotency_record"):
			return 1, nil
		case strings.Contains(query, "insert into fort_private.execution_attempt"):
			if !strings.Contains(query, "'leased'") || len(arguments) < 10 || arguments[1] != command.ExecutionAttemptID ||
				arguments[2] != command.TargetID || arguments[4] != "agent:researcher" || arguments[5] != "behavior:4" ||
				arguments[6] != "binding:9" || arguments[8] != command.WorkerID || arguments[9] != command.CapabilityRevisionID {
				return 0, errors.New("execution attempt is not exactly pinned")
			}
			return 1, nil
		case strings.Contains(query, "update fort_private.conversation_target"):
			if !strings.Contains(query, "state = 'claimed'") || !strings.Contains(query, "state = 'queued'") {
				return 0, errors.New("target claim update is not guarded")
			}
			return 1, nil
		case strings.Contains(query, "insert into fort_private.ledger_event"):
			if len(arguments) != 6 || arguments[4] != `{"lease_id":"lease:1"}` {
				return 0, errors.New("worker claim event metadata is not pgx-safe canonical JSON")
			}
			return 1, nil
		default:
			return 0, errors.New("unexpected claim statement")
		}
	}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, ring)
	if err != nil {
		t.Fatal(err)
	}

	assignment, err := store.ClaimWorkerTarget(context.Background(), command)
	if err != nil {
		t.Fatalf("ClaimWorkerTarget: %v", err)
	}
	wantPins := coreworker.ExecutionPins{
		AgentID: "agent:researcher", BehaviorRevisionID: "behavior:4", BindingRevisionID: "binding:9",
		SeatID: "seat:researcher", EffectiveAuthoritySnapshot: authority,
	}
	if assignment.TargetID != command.TargetID || assignment.ExecutionAttemptID != command.ExecutionAttemptID ||
		assignment.LeaseID != command.LeaseID || assignment.FenceToken != 41 || assignment.Pins.AgentID != wantPins.AgentID ||
		assignment.Pins.BehaviorRevisionID != wantPins.BehaviorRevisionID || assignment.Pins.BindingRevisionID != wantPins.BindingRevisionID ||
		assignment.Pins.SeatID != wantPins.SeatID || assignment.ContextManifestID != "context:1" ||
		assignment.Prompt != prompt || assignment.PromptEnvelope.KeyID != "" ||
		assignment.Execution.Provider != "openclaw" || assignment.Execution.ComputerID != command.MachineID ||
		assignment.Execution.Workdir != "/Users/fort/Workspaces/researcher" ||
		assignment.Execution.SourceConfigDigest != strings.Repeat("a", 64) {
		t.Fatalf("assignment = %#v", assignment)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("claim transaction = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestClaimNextWorkerTargetSelectsOldestCompatibleQueuedTargetUnderLock(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 11, 0, 0, time.UTC)
	command := controlapi.WorkerClaimNextCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		ExecutionAttemptID: "attempt:next:1", LeaseID: "lease:next:1", IdempotencyKey: "claim-next:1",
		CapabilityRevisionID: "capability:7", ClaimedAt: now, ExpiresAt: now.Add(2 * time.Minute),
	}
	prompt := "Run the exact selected target."
	digestBytes := sha256.Sum256([]byte(prompt))
	ring := collaborationTestKeyRing()
	envelope, err := ring.Encrypt(securebody.Scope{
		AccountID: testAccountID, RecordType: "conversation_message", RecordID: "turn:2",
	}, []byte(prompt))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, _ := base64.RawURLEncoding.DecodeString(envelope.Ciphertext)
	nonce, _ := base64.RawURLEncoding.DecodeString(envelope.Nonce)
	participantAuthority := []byte(`{"authority_id":"authority:binding:1","authority_revision":"revision:7","policy_id":"policy:1","policy_revision":"revision:2"}`)
	grant := []byte(`{"id":"authority:effective:1","permissions":["message.append"],"context_record_ids":[]}`)

	tx := &fakeTransaction{}
	tx.queryRowHook = func(query string, arguments []any) row {
		switch {
		case strings.Contains(query, "from fort_private.idempotency_record"):
			return fakeRow{err: pgx.ErrNoRows}
		case strings.HasPrefix(strings.TrimSpace(query), "select target.target_id"):
			if !strings.Contains(query, "target.state = 'queued'") ||
				!strings.Contains(query, "order by target.created_at, target.target_id") ||
				!strings.Contains(query, "limit 1 for update of target skip locked") ||
				!strings.Contains(query, "binding.worker_id = $2") ||
				!strings.Contains(query, "machine.machine_id = $4") ||
				len(arguments) != 5 || arguments[0] != testAccountID || arguments[1] != command.WorkerID ||
				arguments[2] != command.CapabilityRevisionID || arguments[3] != command.MachineID || arguments[4] != now {
				return fakeRow{err: errors.New("claim-next discovery is not stable, atomic, and machine compatible")}
			}
			return fakeRow{values: []any{"target:2"}}
		case strings.Contains(query, "for update of target"):
			if len(arguments) != 5 || arguments[3] != "target:2" {
				return fakeRow{err: errors.New("claim-next did not load its exact selected target")}
			}
			return fakeRow{values: []any{
				"target:2", "queued", int64(0), now.Add(time.Hour), "initial", "turn:2", "conversation:agent:researcher", "turn:2", "direct",
				"agent:researcher", "behavior:4", "binding:9", "participant:1", "seat:researcher",
				participantAuthority, grant, "delegation:1", "context:2",
				"source:studio", "source-agent:researcher", "researcher", "openclaw:researcher", "openclaw",
				"openclaw-main", "openclaw-main", "model.chat.openclaw", "adapter:1", strings.Repeat("a", 64),
				"authority:binding:1", "revision:7", "policy:1", "revision:2", "isolated", "source_managed",
				[]byte(`{"values":["openclaw-ready","workdir=/Users/fort/Workspaces/researcher"],"location_kind":"computer"}`), "ready:openclaw", "revision:4",
				command.MachineID, ciphertext, envelope.KeyID, nonce, hex.EncodeToString(digestBytes[:]), int64(len(prompt)),
				sql.NullTime{}, sql.NullString{}, sql.NullString{},
			}}
		case strings.Contains(query, "insert into fort_private.worker_lease"):
			return fakeRow{values: []any{int64(52)}}
		default:
			return fakeRow{err: errors.New("unexpected claim-next query")}
		}
	}
	tx.execHook = func(query string, arguments []any) (int64, error) {
		switch {
		case strings.Contains(query, "set_config('fort.account_id'"):
			return 1, nil
		case strings.Contains(query, "fort_private.idempotency_record"):
			return 1, nil
		case strings.Contains(query, "insert into fort_private.execution_attempt"):
			if arguments[2] != "target:2" {
				return 0, errors.New("attempt did not use discovered target")
			}
			return 1, nil
		case strings.Contains(query, "update fort_private.conversation_target"):
			if arguments[3] != "target:2" {
				return 0, errors.New("claim did not fence discovered target")
			}
			return 1, nil
		case strings.Contains(query, "insert into fort_private.ledger_event"):
			return 1, nil
		default:
			return 0, errors.New("unexpected claim-next statement")
		}
	}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, ring)
	if err != nil {
		t.Fatal(err)
	}

	assignment, err := store.ClaimNextWorkerTarget(context.Background(), command)
	if err != nil {
		t.Fatalf("ClaimNextWorkerTarget: %v", err)
	}
	if assignment.TargetID != "target:2" || assignment.FenceToken != 52 || assignment.Prompt != prompt ||
		assignment.Execution.Provider != "openclaw" || assignment.Execution.ComputerID != command.MachineID ||
		assignment.Execution.Workdir != "/Users/fort/Workspaces/researcher" ||
		assignment.OutputConversationID != "conversation:agent:researcher" || assignment.OutputMessageKind != "agent" ||
		assignment.OutputAuthorAgentID != "agent:researcher" || assignment.MaximumOutputPlaintextBytes != 128<<20 ||
		assignment.InlineOutputPlaintextBytes != 2<<20 || assignment.HardDeadline != now.Add(time.Hour) {
		t.Fatalf("claim-next assignment = %#v", assignment)
	}
}

func TestWorkerBindingWorkdirRequiresOneCanonicalAbsolutePinnedPath(t *testing.T) {
	t.Parallel()

	for name, evidence := range map[string][]byte{
		"missing":   []byte(`{"values":["openclaw-ready"],"location_kind":"computer"}`),
		"duplicate": []byte(`{"values":["workdir=/Users/fort/one","workdir=/Users/fort/two"],"location_kind":"computer"}`),
		"relative":  []byte(`{"values":["workdir=relative/path"],"location_kind":"computer"}`),
		"unclean":   []byte(`{"values":["workdir=/Users/fort/../other"],"location_kind":"computer"}`),
		"root":      []byte(`{"values":["workdir=/"],"location_kind":"computer"}`),
		"cloud":     []byte(`{"values":["workdir=/Users/fort/work"],"location_kind":"cloud"}`),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if workdir, err := workerBindingWorkdir(evidence); err == nil {
				t.Fatalf("workerBindingWorkdir = %q, want fail closed", workdir)
			}
		})
	}
	workdir, err := workerBindingWorkdir([]byte(`{"values":["openclaw-ready","workdir=/Users/fort/Workspaces/researcher"],"location_kind":"computer"}`))
	if err != nil || workdir != "/Users/fort/Workspaces/researcher" {
		t.Fatalf("workerBindingWorkdir = %q, %v", workdir, err)
	}
}

func TestClaimWorkerTargetNeverStartsRoutineEarlyOrMoreThanNinetySecondsLate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 10, 0, 0, time.UTC)
	command := controlapi.WorkerClaimCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:routine", ExecutionAttemptID: "attempt:routine", LeaseID: "lease:routine",
		IdempotencyKey: "claim:routine", CapabilityRevisionID: "capability:7",
		ClaimedAt: now, ExpiresAt: now.Add(2 * time.Minute),
	}

	for _, test := range []struct {
		name         string
		scheduledFor time.Time
		wantCommit   bool
		wantLateMark bool
	}{
		{name: "early", scheduledFor: now.Add(time.Second)},
		{name: "too late", scheduledFor: now.Add(-91 * time.Second), wantCommit: true, wantLateMark: true},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			routineRunMarked := false
			tx := &fakeTransaction{}
			tx.queryRowHook = func(query string, _ []any) row {
				if !strings.Contains(query, "for update of target") {
					return fakeRow{err: errors.New("unexpected query")}
				}
				return fakeRow{values: []any{
					"target:routine", "queued", int64(0), now.Add(time.Hour), "routine", "routine:1", "conversation:routine", "turn:routine", "routine",
					"agent:researcher", "behavior:4", "binding:9", "participant:1", "seat:researcher",
					[]byte(`{"authority_id":"authority:binding:1","authority_revision":"revision:1","policy_id":"policy:1","policy_revision":"revision:1"}`),
					[]byte(`{"id":"authority:1","permissions":[],"context_record_ids":[]}`), "delegation:1", "context:1",
					"source:1", "source-agent:1", "opaque:1", "profile:1", "openclaw", "model:1", "model:1",
					"adapter:1", "revision:1", strings.Repeat("a", 64), "authority:binding:1", "revision:1",
					"policy:1", "revision:1", "isolated", "source_managed",
					[]byte(`{"values":[],"location_kind":"computer"}`), "ready:1", "revision:1", "machine:mac-studio",
					[]byte("prompt"), "key:1", []byte("0123456789ab"), strings.Repeat("c", 64), int64(6),
					sql.NullTime{Time: test.scheduledFor, Valid: true}, sql.NullString{String: "queued", Valid: true},
					sql.NullString{String: "routine-occurrence:1", Valid: true},
				}}
			}
			tx.execHook = func(query string, _ []any) (int64, error) {
				switch {
				case strings.Contains(query, "set_config('fort.account_id'"):
					return 1, nil
				case strings.Contains(query, "missed_needs_attention"):
					if !test.wantLateMark {
						return 0, errors.New("unexpected late mark")
					}
					return 1, nil
				case strings.Contains(query, "update fort_private.routine_run"):
					routineRunMarked = true
					return 1, nil
				case strings.Contains(query, "state = 'needs_you'"):
					return 1, nil
				default:
					return 0, errors.New("claim progressed past routine time guard")
				}
			}
			store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
			if err != nil {
				t.Fatal(err)
			}

			_, err = store.ClaimWorkerTarget(context.Background(), command)
			if !errors.Is(err, controlapi.ErrWorkerNoCompatibleTarget) {
				t.Fatalf("routine claim error = %v, want ErrWorkerNoCompatibleTarget", err)
			}
			if test.wantCommit && tx.commits != 1 {
				t.Fatalf("late transition commits = %d, want 1", tx.commits)
			}
			if !test.wantCommit && tx.rollbacks != 1 {
				t.Fatalf("early transition rollbacks = %d, want 1", tx.rollbacks)
			}
			if routineRunMarked != test.wantLateMark {
				t.Fatalf("routine run marked needs-you = %t, want %t", routineRunMarked, test.wantLateMark)
			}
		})
	}
}

func TestHeartbeatWorkerLeaseRenewsExactFenceAndReturnsDurableCancel(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 12, 0, 0, time.UTC)
	command := controlapi.WorkerLeaseHeartbeatCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		IdempotencyKey: "heartbeat:1", ObservedAt: now, ExtendUntil: now.Add(2 * time.Minute),
	}
	tx := &fakeTransaction{}
	tx.queryRowHook = func(query string, arguments []any) row {
		if !strings.Contains(query, "from fort_private.worker_lease as lease") || !strings.Contains(query, "lease.fence_token = $6") ||
			!strings.Contains(query, "machine.machine_id = $7") || !strings.Contains(query, "for update of lease") {
			return fakeRow{err: errors.New("lease heartbeat lookup is not exactly fenced")}
		}
		return fakeRow{values: []any{
			"active", now.Add(time.Minute), "cancel_requested", "cancel_requested",
			"initial", "turn:1", "turn:1", 1, "agent:researcher", "behavior:1", "binding:1",
		}}
	}
	tx.execHook = func(query string, _ []any) (int64, error) {
		switch {
		case strings.Contains(query, "set_config('fort.account_id'"):
			return 1, nil
		case strings.Contains(query, "fort_private.idempotency_record"):
			return 1, nil
		case strings.Contains(query, "update fort_private.worker_lease"):
			if !strings.Contains(query, "fence_token = $7") || !strings.Contains(query, "state = 'active'") {
				return 0, errors.New("lease renewal is not fenced")
			}
			return 1, nil
		case strings.Contains(query, "update fort_private.worker"):
			return 1, nil
		default:
			return 0, errors.New("cancel-directed heartbeat mutated execution state")
		}
	}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.HeartbeatWorkerLease(context.Background(), command)
	if err != nil {
		t.Fatalf("HeartbeatWorkerLease: %v", err)
	}
	if result.TargetID != command.TargetID || result.ExecutionAttemptID != command.ExecutionAttemptID ||
		result.LeaseID != command.LeaseID || result.FenceToken != command.FenceToken ||
		result.Directive != coreworker.DirectiveCancel || result.ExpiresAt != command.ExtendUntil {
		t.Fatalf("heartbeat result = %#v", result)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("heartbeat transaction = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestHeartbeatWorkerLeaseRejectsStaleFenceWithoutWrites(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 12, 0, 0, time.UTC)
	tx := &fakeTransaction{queryRowHook: func(query string, _ []any) row {
		if strings.Contains(query, "worker_lease") {
			return fakeRow{err: pgx.ErrNoRows}
		}
		return fakeRow{err: errors.New("unexpected query")}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.HeartbeatWorkerLease(context.Background(), controlapi.WorkerLeaseHeartbeatCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:stale", LeaseID: "lease:old", FenceToken: 40,
		IdempotencyKey: "heartbeat:stale", ObservedAt: now, ExtendUntil: now.Add(2 * time.Minute),
	})
	if !errors.Is(err, controlapi.ErrWorkerStaleLease) {
		t.Fatalf("stale heartbeat error = %v, want ErrWorkerStaleLease", err)
	}
	if len(tx.execs) != 1 || !strings.Contains(tx.execs[0].sql, "set_config") || tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("stale heartbeat writes/lifecycle = execs %d commits %d rollbacks %d", len(tx.execs), tx.commits, tx.rollbacks)
	}
}

func TestHeartbeatWorkerLeaseRejectsAtExactTargetHardDeadline(t *testing.T) {
	t.Parallel()

	deadline := time.Date(2026, 8, 21, 20, 12, 30, 0, time.UTC)
	tx := &fakeTransaction{queryRowHook: func(query string, arguments []any) row {
		if !strings.Contains(query, "$8 < target.hard_deadline") || len(arguments) != 8 || arguments[7] != deadline {
			return fakeRow{err: errors.New("heartbeat did not pin the exact target hard deadline")}
		}
		return fakeRow{err: pgx.ErrNoRows}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.HeartbeatWorkerLease(context.Background(), controlapi.WorkerLeaseHeartbeatCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		IdempotencyKey: "heartbeat:deadline", ObservedAt: deadline, ExtendUntil: deadline.Add(time.Minute),
	})
	if !errors.Is(err, controlapi.ErrWorkerStaleLease) {
		t.Fatalf("hard-deadline heartbeat error = %v, want stale lease", err)
	}
	if tx.commits != 0 || tx.rollbacks != 1 || len(tx.execs) != 1 {
		t.Fatalf("hard-deadline heartbeat writes = %d commits=%d rollbacks=%d", len(tx.execs), tx.commits, tx.rollbacks)
	}
}

func TestAcknowledgeWorkerCancellationPersistsOneExactFencedReceipt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 13, 0, 0, time.UTC)
	command := controlapi.WorkerCancellationAckCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		AcknowledgementID: "cancel-ack:1", IdempotencyKey: "cancel-ack:1", AcknowledgedAt: now,
	}
	tx := &fakeTransaction{}
	tx.queryRowHook = func(query string, _ []any) row {
		if !strings.Contains(query, "from fort_private.worker_lease as lease") || !strings.Contains(query, "lease.fence_token = $6") ||
			!strings.Contains(query, "for update of lease") {
			return fakeRow{err: errors.New("cancellation acknowledgement is not exactly fenced")}
		}
		return fakeRow{values: []any{now.Add(time.Minute), "cancel_requested", "cancel_requested"}}
	}
	tx.execHook = func(query string, arguments []any) (int64, error) {
		switch {
		case strings.Contains(query, "set_config('fort.account_id'"):
			return 1, nil
		case strings.Contains(query, "fort_private.idempotency_record"):
			return 1, nil
		case strings.Contains(query, "insert into fort_private.worker_cancellation_ack"):
			if len(arguments) != 10 || arguments[1] != command.AcknowledgementID || arguments[2] != command.TargetID ||
				arguments[3] != command.ExecutionAttemptID || arguments[4] != command.LeaseID || arguments[5] != command.FenceToken ||
				arguments[6] != command.WorkerID || arguments[7] != command.MachineID || arguments[8] != command.IdempotencyKey ||
				arguments[9] != now {
				return 0, errors.New("cancellation acknowledgement evidence is incomplete")
			}
			return 1, nil
		default:
			return 0, errors.New("unexpected cancellation acknowledgement statement")
		}
	}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}

	ack, err := store.AcknowledgeWorkerCancellation(context.Background(), command)
	if err != nil {
		t.Fatalf("AcknowledgeWorkerCancellation: %v", err)
	}
	if ack.AcknowledgementID != command.AcknowledgementID || ack.TargetID != command.TargetID ||
		ack.ExecutionAttemptID != command.ExecutionAttemptID || ack.LeaseID != command.LeaseID ||
		ack.FenceToken != command.FenceToken || ack.AcknowledgedAt != now {
		t.Fatalf("cancellation acknowledgement = %#v", ack)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("cancellation acknowledgement transaction = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestCommitWorkerTerminalAtomicallyReleasesFenceAndPinsFinalizedOutput(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 14, 0, 0, time.UTC)
	command := controlapi.WorkerTerminalCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		TerminalReceiptID: "receipt:1", IdempotencyKey: "terminal:1", Status: coreworker.TerminalCanceled,
		ReceiptPlaintext: json.RawMessage(`{"status":"canceled"}`),
		Output:           controlapi.WorkerOutputReference{ArtifactID: "artifact:output:1", Digest: strings.Repeat("e", 64)},
		CommittedAt:      now,
	}
	tx := &fakeTransaction{}
	tx.queryRowHook = func(query string, _ []any) row {
		switch {
		case strings.Contains(query, "from fort_private.worker_lease as lease"):
			if !strings.Contains(query, "lease.fence_token = $6") || !strings.Contains(query, "for update of lease") ||
				!strings.Contains(query, "worker_cancellation_ack") {
				return fakeRow{err: errors.New("terminal lookup is not fenced with cancellation evidence")}
			}
			return fakeRow{values: []any{
				"active", now.Add(time.Minute), "cancel_requested", "cancel_requested", true, sql.NullTime{}, sql.NullString{},
				"initial", "conversation:1", "turn:1", "turn:1", "target:1", 1,
				"agent:researcher", "behavior:1", "binding:1",
			}}
		case strings.Contains(query, "from fort_private.artifact"):
			if !strings.Contains(query, "execution_attempt_id = $3") || !strings.Contains(query, "state = 'finalized'") {
				return fakeRow{err: errors.New("terminal output is not pinned to finalized attempt artifact")}
			}
			return fakeRow{values: []any{command.Output.Digest, int64(0)}}
		default:
			return fakeRow{err: errors.New("unexpected terminal query")}
		}
	}
	tx.execHook = func(query string, arguments []any) (int64, error) {
		switch {
		case strings.Contains(query, "set_config('fort.account_id'"):
			return 1, nil
		case strings.Contains(query, "fort_private.idempotency_record"):
			return 1, nil
		case strings.Contains(query, "update fort_private.execution_attempt"):
			if !strings.Contains(query, "terminal_receipt_ciphertext") || !strings.Contains(query, "state = 'canceled'") ||
				!strings.Contains(query, "terminal_receipt_id") || !strings.Contains(query, "worker_id = $10") || len(arguments) != 10 ||
				arguments[0] != command.TerminalReceiptID {
				return 0, errors.New("terminal attempt update is not exact")
			}
			return 1, nil
		case strings.Contains(query, "update fort_private.conversation_target"):
			if !strings.Contains(query, "state = 'canceled'") {
				return 0, errors.New("terminal target state is wrong")
			}
			return 1, nil
		case strings.Contains(query, "update fort_private.worker_lease"):
			if !strings.Contains(query, "state = 'released'") || !strings.Contains(query, "fence_token = $7") {
				return 0, errors.New("terminal lease release is not fenced")
			}
			return 1, nil
		case strings.Contains(query, "update fort_private.conversation_turn"):
			return 1, nil
		case strings.Contains(query, "insert into fort_private.ledger_event"):
			if len(arguments) != 6 || arguments[4] != `{"output_artifact_id":"artifact:output:1","terminal_receipt_id":"receipt:1"}` {
				return 0, errors.New("worker terminal event metadata is not pgx-safe canonical JSON")
			}
			return 1, nil
		default:
			return 0, errors.New("unexpected terminal statement")
		}
	}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.CommitWorkerTerminal(context.Background(), command)
	if err != nil {
		t.Fatalf("CommitWorkerTerminal: %v", err)
	}
	if !result.Created || result.TargetID != command.TargetID || result.ExecutionAttemptID != command.ExecutionAttemptID ||
		result.LeaseID != command.LeaseID || result.FenceToken != command.FenceToken || result.Status != command.Status ||
		result.TerminalReceiptID != command.TerminalReceiptID || result.Output != command.Output || result.CommittedAt != now {
		t.Fatalf("terminal result = %#v", result)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("terminal transaction = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestCommitWorkerTerminalRejectsAtExactTargetHardDeadline(t *testing.T) {
	t.Parallel()

	deadline := time.Date(2026, 8, 21, 20, 14, 30, 0, time.UTC)
	command := controlapi.WorkerTerminalCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		TerminalReceiptID: "receipt:deadline", IdempotencyKey: "terminal:deadline", Status: coreworker.TerminalCanceled,
		ReceiptPlaintext: json.RawMessage(`{"status":"canceled"}`),
		Output:           controlapi.WorkerOutputReference{ArtifactID: "artifact:output:1", Digest: strings.Repeat("e", 64)},
		CommittedAt:      deadline,
	}
	tx := &fakeTransaction{queryRowHook: func(query string, arguments []any) row {
		if !strings.Contains(query, "$8 < target.hard_deadline") || len(arguments) != 8 || arguments[7] != deadline {
			return fakeRow{err: errors.New("terminal did not pin the exact target hard deadline")}
		}
		return fakeRow{err: pgx.ErrNoRows}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CommitWorkerTerminal(context.Background(), command)
	if !errors.Is(err, controlapi.ErrWorkerStaleLease) {
		t.Fatalf("hard-deadline terminal error = %v, want stale lease", err)
	}
	if tx.commits != 0 || tx.rollbacks != 1 || len(tx.execs) != 1 {
		t.Fatalf("hard-deadline terminal writes = %d commits=%d rollbacks=%d", len(tx.execs), tx.commits, tx.rollbacks)
	}
}

func TestCommitWorkerTerminalCompletedInitialTargetAppendsOneBoundAgentMessage(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 14, 0, 0, time.UTC)
	body := "A durable answer from the exact Agent target."
	bodyDigest := sha256.Sum256([]byte(body))
	digest := hex.EncodeToString(bodyDigest[:])
	ring := collaborationTestKeyRing()
	command := controlapi.WorkerTerminalCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		TerminalReceiptID: "receipt:1", IdempotencyKey: "terminal:1", Status: coreworker.TerminalCompleted,
		ReceiptPlaintext:       json.RawMessage(`{"status":"completed","exit_code":0}`),
		Output:                 controlapi.WorkerOutputReference{ArtifactID: "artifact:output:1", Digest: digest},
		OutputMessagePlaintext: &body,
		CommittedAt:            now,
	}

	tx := &fakeTransaction{}
	turnSettlementChecked := false
	tx.queryRowHook = func(query string, arguments []any) row {
		switch {
		case strings.Contains(query, "from fort_private.worker_lease as lease"):
			return fakeRow{values: []any{
				"active", now.Add(time.Minute), "working", "working", false, sql.NullTime{}, sql.NullString{},
				"initial", "conversation:1", "turn:1", "turn:1", "target:1", 1,
				"agent:researcher", "behavior:1", "binding:1",
			}}
		case strings.Contains(query, "from fort_private.artifact"):
			return fakeRow{values: []any{digest, int64(len(body))}}
		case strings.Contains(query, "insert into fort_private.conversation_message"):
			if !strings.Contains(query, "'agent','agent'") || len(arguments) != 11 ||
				arguments[1] != "conversation:1" || arguments[2] != "turn:1" || arguments[3] != command.TargetID ||
				arguments[4] != "agent:researcher" {
				return fakeRow{err: errors.New("authoritative Agent message is not bound to the exact target")}
			}
			return fakeRow{values: []any{int64(73)}}
		default:
			return fakeRow{err: errors.New("unexpected completed terminal query")}
		}
	}
	tx.execHook = func(query string, arguments []any) (int64, error) {
		switch {
		case strings.Contains(query, "set_config('fort.account_id'"):
			return 1, nil
		case strings.Contains(query, "fort_private.idempotency_record"):
			return 1, nil
		case strings.Contains(query, "update fort_private.execution_attempt"):
			return 1, nil
		case strings.Contains(query, "update fort_private.conversation_target"):
			return 1, nil
		case strings.Contains(query, "update fort_private.worker_lease"):
			return 1, nil
		case strings.Contains(query, "update fort_private.conversation_turn"):
			if len(arguments) != 3 || arguments[1] != "turn:1" || arguments[2] != now ||
				!strings.Contains(query, "from fort_private.conversation_target") ||
				!strings.Contains(query, "from fort_private.handoff") ||
				!strings.Contains(query, "handoff.group_turn_id") {
				return 0, errors.New("source turn settlement did not fence all initial targets and Handoff chains")
			}
			turnSettlementChecked = true
			return 1, nil
		case strings.Contains(query, "update fort_private.conversation"):
			if len(arguments) != 3 || arguments[1] != "conversation:1" || arguments[2] != now {
				return 0, errors.New("Conversation activity was not advanced by the Agent message")
			}
			return 1, nil
		case strings.Contains(query, "insert into fort_private.ledger_event"):
			if len(arguments) != 6 || !strings.Contains(arguments[4].(string), `"conversation_message_id":73`) {
				return 0, errors.New("terminal event omitted authoritative message identity")
			}
			return 1, nil
		default:
			return 0, errors.New("unexpected completed terminal statement")
		}
	}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, ring)
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.CommitWorkerTerminal(context.Background(), command)
	if err != nil {
		t.Fatalf("CommitWorkerTerminal: %v", err)
	}
	if !result.Created || result.MessageID != 73 {
		t.Fatalf("completed terminal result = %#v", result)
	}
	if !turnSettlementChecked {
		t.Fatal("completed initial target did not evaluate source turn settlement")
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("completed terminal transaction = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestCommitWorkerTerminalRejectsOutputMessageDigestMismatchBeforeWrites(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 14, 0, 0, time.UTC)
	body := "This answer was encrypted for a stale fence."
	bodyDigest := sha256.Sum256([]byte(body))
	digest := hex.EncodeToString(bodyDigest[:])
	ring := collaborationTestKeyRing()
	changedBody := body + " changed"
	command := controlapi.WorkerTerminalCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		TerminalReceiptID: "receipt:1", IdempotencyKey: "terminal:1", Status: coreworker.TerminalCompleted,
		ReceiptPlaintext:       json.RawMessage(`{"status":"completed"}`),
		Output:                 controlapi.WorkerOutputReference{ArtifactID: "artifact:output:1", Digest: digest},
		OutputMessagePlaintext: &changedBody,
		CommittedAt:            now,
	}
	tx := &fakeTransaction{queryRowHook: func(query string, _ []any) row {
		switch {
		case strings.Contains(query, "from fort_private.worker_lease as lease"):
			return fakeRow{values: []any{
				"active", now.Add(time.Minute), "working", "working", false, sql.NullTime{}, sql.NullString{},
				"initial", "conversation:1", "turn:1", "turn:1", "target:1", 1,
				"agent:researcher", "behavior:1", "binding:1",
			}}
		case strings.Contains(query, "from fort_private.artifact"):
			return fakeRow{values: []any{digest, int64(len(body))}}
		default:
			return fakeRow{err: errors.New("invalid bound output reached another query")}
		}
	}}
	tx.execHook = func(query string, _ []any) (int64, error) {
		if strings.Contains(query, "set_config('fort.account_id'") {
			return 1, nil
		}
		return 0, errors.New("invalid bound output reached a durable write")
	}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, ring)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.CommitWorkerTerminal(context.Background(), command)
	if !errors.Is(err, controlapi.ErrWorkerRequestInvalid) {
		t.Fatalf("stale output message error = %v, want worker request invalid", err)
	}
	if len(tx.execs) != 0 || tx.commits != 0 || tx.rollbacks != 0 {
		t.Fatalf("stale output writes/lifecycle = execs %d commits %d rollbacks %d", len(tx.execs), tx.commits, tx.rollbacks)
	}
}

func TestCommitWorkerTerminalRejectsStaleFenceBeforeArtifactOrReceiptWrites(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 14, 0, 0, time.UTC)
	tx := &fakeTransaction{queryRowHook: func(query string, _ []any) row {
		if strings.Contains(query, "worker_lease") {
			return fakeRow{err: pgx.ErrNoRows}
		}
		return fakeRow{err: errors.New("stale terminal reached another query")}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.CommitWorkerTerminal(context.Background(), controlapi.WorkerTerminalCommand{
		AccountID: testAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TargetID: "target:1", ExecutionAttemptID: "attempt:stale", LeaseID: "lease:old", FenceToken: 40,
		TerminalReceiptID: "receipt:stale", IdempotencyKey: "terminal:stale", Status: coreworker.TerminalCompleted,
		ReceiptPlaintext: json.RawMessage(`{"status":"completed"}`),
		Output: controlapi.WorkerOutputReference{ArtifactID: "artifact:output:1", Digest: func() string {
			digest := sha256.Sum256([]byte("output"))
			return hex.EncodeToString(digest[:])
		}()},
		OutputMessagePlaintext: func() *string { value := "output"; return &value }(),
		CommittedAt:            now,
	})
	if !errors.Is(err, controlapi.ErrWorkerStaleLease) {
		t.Fatalf("stale terminal error = %v, want ErrWorkerStaleLease", err)
	}
	if len(tx.execs) != 1 || tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("stale terminal writes/lifecycle = execs %d commits %d rollbacks %d", len(tx.execs), tx.commits, tx.rollbacks)
	}
}
