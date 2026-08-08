# Spec 042 — Shared Conversation Completion Contracts

**Status:** Slice A deferred — not part of the current milestone; Slice B approved 2026-08-03 for implementation
**Live acceptance:** Spec 041's two-Mac acceptance requires separate explicit authorization.
**Depends on:** approved Spec 041

## Decision

This spec records two independently scoped completion contracts:

1. Slice A would let a failed answer **Choose another seat**; and
2. Slice B ensures every answer names the exact model that Fort dispatched.

Slice A is deferred and is not part of the current milestone. Its detailed
contract remains here for future consideration, but no alternative-target
implementation is authorized. Slice B is approved 2026-08-03 for
implementation.

The recorded contracts are:

- **Deferred Slice A:** **Choose another seat** appends a target for an
  already-active participant in the same conversation. Adding an agent remains
  a separate explicit membership action because it changes future **Everyone**
  semantics.
- **Approved Slice B:** A configured-default seat is ready only when a no-turn
  probe resolves a validated provider-native model selector and Fort pins that
  selector through dispatch. Fort never discovers the model after the person
  has sent the turn.
- **Deferred Slice A:** **Cancel** applies only to Queued or Working targets.
  Failed is already terminal and offers Retry or Recheck and retry, Choose
  another seat, and Details—not a false Cancel action.

If Slice A is approved later, its Cancel decision will clarify Spec 041 §7's
recovery actions; it does not change the existing monotonic target lifecycle.

Here, **exact model** means the exact provider-native selector Fort passes to
the provider, such as `gpt-5.6-sol`. It does not claim a hidden server-side
revision behind a provider alias such as `sonnet`; the current four adapters do
not expose that stronger identity uniformly.

## A. Alternative target recovery

**Status:** deferred — not part of the current milestone; no implementation authorized.

### Experience

Every current Failed target offers **Choose another seat** beside its matching
retry action:

- `seat_unready`: **Recheck and retry**, **Choose another seat**, **Details**;
- any other failure: **Retry**, **Choose another seat**, **Details**.

Queued and Working offer **Cancel** and **Details**. Canceled offers **Retry**
and **Details**. Answered offers **Details** only.

Choosing another seat reuses the existing Agents dialog in a recovery mode. It
shows complete immutable seat identities for active participants in that
conversation. A participant is eligible only when it:

- is not the failed target's participant;
- has no target of any attempt on the source turn; and
- still maps to the same currently ready exact seat.

Other active participants may remain visible but disabled with the closed
readiness reason. A successful peer is never eligible and can never be rerun by
this action.

When no alternative participant is eligible, the dialog explains that and
offers **Add agent…**. That opens the normal membership manager. Adding the
agent does not target it automatically; the person explicitly returns to
**Choose another seat**. This keeps participant membership and dispatch as two
visible decisions.

The original Failed target remains visible and independently retryable. The
alternative is a separate target row with its own complete seat label and
state. The action is single-flight, restores focus to its trigger on Close or
Escape, and ignores a late response after navigation to another conversation.
The source must be the latest attempt for its `(turn_id, participant_id)` slot
and must still be Failed. A historical failed attempt whose later retry is
Answered, Failed, or Canceled cannot create an alternative.

### HTTP and port contract

Add:

```text
POST /api/conversations/{conversation_id}/targets/{failed_target_id}/alternatives
```

Request:

```json
{
  "participant_id": "participant-3"
}
```

The server returns `202 Accepted` with the newly durable target only after the
store transaction commits. It returns `404` when the nested conversation and
target do not match. It returns `409` without dispatch when the source is not
Failed, the participant is removed/foreign/the source participant, the
participant already has any target on that turn, or the exact seat is not
currently ready.

The bounded `ui` port adds the equivalent operation:

```go
CreateAlternativeTarget(
    context.Context,
    failedTargetID string,
    participantID string,
) (conversation.Target, error)
```

The presentation layer supplies IDs only. It does not import the store,
capability inventory, or a runtime implementation.

### Store and dispatch semantics

The store performs one transaction that:

1. loads the source target, turn, and conversation;
2. requires the source target to be the latest attempt for its participant on
   that turn and to be Failed;
3. requires the selected participant to be active in that same conversation
   and different from the source participant;
4. rejects the participant when any target already exists for it on the source
   turn; and
5. inserts one new Queued target with the source `turn_id`, the selected
   `participant_id`, new target/run IDs, and `attempt = 1`.

The transaction changes no existing target, message, turn, participant, or
conversation activity timestamp. The existing unique
`(turn_id, participant_id, attempt)` constraint prevents duplicate targets; a
concurrent or repeated duplicate request starts zero additional runtimes.

After commit, the conversation service starts only the new target. Dispatch
reuses the source turn's persisted, non-empty `context_json` byte-for-byte and
its existing `through_message_id`; absence fails closed without a runtime call.
A participant added after that turn appears only in the addressed-participant
envelope. Fort never recomputes or rewrites the frozen snapshot. Normal
immediate pre-dispatch readiness validation remains authoritative.

No schema migration or lineage field is added. Sharing the source `turn_id`
proves the frozen boundary, and the distinct persisted participant proves the
visible alternative seat.

## B. Provider-resolved seat identity

**Status:** approved 2026-08-03 for implementation.

### Why run output is too late

`runtime.RunSpec.Model` is currently empty for a configured-default profile and
means “let the provider choose.” `runtime.RunEvent` has no structured model
identity, and several current provider normalizers do not receive one. Learning
the model from run output would happen after the user selected the seat, could
not preserve an immutable seat, and would require new target/message
persistence.

Resolution therefore happens through the existing no-turn capability path,
before a seat becomes selectable. Seat listing and recheck continue to invoke
zero model turns.

### Capability contract

Add the public, secret-free field:

```go
type ProfileOffer struct {
    // existing fields...
    ResolvedModel string `json:"resolved_model,omitempty"`
}
```

`resolved_model` is an exact validated provider-native selector, never raw
probe output. Account handles, configuration origins, paths, cursors, tokens,
and executable identities remain private or opaque.

The private `ProbeObservation` carries a typed `ResolvedModel`. Only the model
predicate for a dynamic profile may populate it; unsuccessful normalization
clears it. The registry copies it into a Ready dynamic `ProfileOffer`. It never
parses generic `StableBinding` material to recover a model.

For `configured_default` and `configured_agent` selections:

- Ready requires a non-empty `resolved_model` produced by a typed no-turn
  inspector;
- missing or ambiguous resolution produces `model_unavailable`;
- malformed or changed inspector contracts fail closed with the existing
  bounded reason; and
- the resolved model contributes to the opaque profile binding revision and
  snapshot revision.

This additive peer field changes the public capability contract. Increment
`ProtocolVersion` from 1 to 2 and `ProfileMappingVersion` from 2 to 3; old peers
fail closed through the existing version checks. The catalog version changes
only if a catalog row itself changes.

### Seat projection and persistence

An explicit-model catalog profile continues to use its cataloged provider
selector. A dynamic profile uses its current `resolved_model`.

The conversation seat projection:

1. copies the exact selector into the existing `conversation.Seat.Model`;
2. shows an exact label such as `Codex · GPT-5.6 Sol (default)`; and
3. derives a deterministic opaque seat ID from profile, canonical machine, and
   resolved model.

Including the model in dynamic seat identity means a changed default is a new
seat rather than a silent mutation. Existing participant creation already
persists profile, provider, model, machine, display name, and seat ID; no
conversation schema migration is required.

Historical participants and answers always render their persisted model. An
old row whose model is empty says **Model not recorded**. It is never backfilled
from today's catalog or configuration.

The fake runtime advertises explicit deterministic fixture models. It does not
create an empty-model configured-default seat.

### Dispatch pinning

Immediately before each dispatch, the existing profile gate refreshes only the
selected machine/profile and compares the fresh resolved selector with the
participant's persisted model:

- same model: forward the persisted model unchanged in `RunSpec.Model`;
- missing model: fail `model_unavailable`, with zero downstream runtime calls;
- changed model: fail `capability_drift`, with zero downstream runtime calls;
- never overwrite the participant with the newly configured default.

Revalidation locates the current offer by persisted profile plus canonical
machine, not by the model-versioned ephemeral seat ID. A present dynamic offer
with a different resolved model is `capability_drift`; a present offer whose
model cannot be resolved is `model_unavailable`. Both are closed readiness
reasons persisted under target `error_code=seat_unready`, so the inline action
is **Recheck and retry**, not ordinary Retry.

The local native provider receives an explicit model argument. The existing
cluster/remote/node path carries the same `RunSpec.Model` to the selected
machine. The existing run row records that model before provider dispatch.
Retry keeps the original participant and therefore the original exact model.

No new runtime event, target column, message column, HTTP endpoint, routing
decision, or model call is added.

Exact pinning in this spec is scoped to conversation targets. A conversation
target for a dynamic profile must enter the shared profile gate with its
persisted non-empty model. Existing direct/flow configured-default callers may
continue to enter with an empty model and retain their legacy ambient-default
behavior; the gate does not fill their model after their run row already
exists, and those runs are not presented as exact-model conversation answers.
Expanding exact identity to every legacy caller is a separate milestone.

### Provider support at approval time

| Provider | No-turn resolution | Pinning | Conversation result |
| --- | --- | --- | --- |
| Codex | Existing typed app-server inspection resolves effective config or one unambiguous catalog default. | Existing `--model`. | Configured default can become ready. |
| Claude | No approved no-turn default resolver. | Existing `--model` once an exact selector is known. | Configured default remains unavailable; explicit Sonnet/Opus profiles remain. |
| Hermes | Current `status --deep` handling does not provide a typed validated default. | Existing `--model` once resolved. | Configured default remains unavailable; explicit provider/model profile remains. |
| OpenClaw | No typed model metadata contract; `main` is ambient configuration and readiness is already quarantined. | No verified Fort model override. | Remains unavailable. |
| Fake | Fort-owned fixture model. | Deterministic fixture dispatch. | Test/demo only. |

Adding support for another dynamic provider requires its own typed no-turn
inspector and tests. Fort never derives a dynamic `resolved_model` from an
incidental human-readable status line.

## Determinism and architecture

- Alternative selection and model resolution make zero model calls.
- A selected participant/model/machine is never silently replaced.
- Only the newly committed alternative target invokes `runtime.Runtime`.
- `core` keeps importing only core seams; `ui` keeps importing only bounded
  ports and wire types.
- Public capability payloads stay secret-free and machine identity remains the
  enrolled canonical name.

## TDD acceptance

### Alternative targets

- Store tests prove a distinct active participant is appended at attempt 1 on
  the original turn and frozen context boundary.
- Same, removed, foreign, already-targeted, and cross-conversation participants
  fail atomically with no new target or run.
- A historical Failed attempt with a newer attempt is rejected without a new
  target or runtime call.
- Concurrent duplicate requests create one target and start one runtime.
- Service tests prove Failed A plus Answered B can explicitly append C without
  rerunning A or B, and that C preserves its exact profile/model/machine.
- A participant added after the turn is absent from the preserved
  `context_json`, present only in the addressed-participant envelope, and a
  missing frozen context starts zero runtimes.
- Unready C creates no target and invokes zero runtimes.
- HTTP tests cover the exact nested route/body, `202`, `404`, and `409` cases.
- UI tests cover the state/action matrix, eligibility, disabled readiness,
  Add agent handoff, single-flight/navigation guards, and two separately
  labeled target slots.
- Terminal Failed targets never render or accept Cancel.

### Resolved model identity

- Codex configured override wins over catalog default; one unique catalog
  default is the fallback.
- Missing, ambiguous, malformed, or unavailable dynamic defaults fail closed.
- Public payloads expose only the validated model selector.
- Snapshot normalization preserves `resolved_model`; revisions change when it
  changes; old protocol/mapping peers fail closed.
- A dynamic configured-default seat cannot be Ready with an empty model.
- A selected dynamic seat persists its exact model and model-versioned opaque
  seat identity.
- Dispatch with the same resolution preserves profile/model/machine byte for
  byte through local and remote transport.
- Missing resolution or default drift invokes zero downstream runtimes.
- Dynamic conversation drift persists `seat_unready` plus the closed
  `capability_drift` or `model_unavailable` reason and offers Recheck and retry.
- Retry, reload, and later inventory refresh keep showing the participant's
  original persisted model.
- Historical empty-model rows say **Model not recorded**.
- Explicit-model profiles retain their current behavior.
- Legacy direct/flow configured-default runs retain empty-model ambient
  behavior and are never relabeled as exact conversation answers.
- Seat listing, recheck, resolution, and routing tests assert zero model calls.

Focused concurrent packages pass with `go test -race`; `go test ./...`,
`go test -race ./...`, `go vet ./...`, and `git diff --check` remain green.

## Implementation slices

### Slice A — alternative recovery

Deferred. Do not implement this slice in the current milestone. The contract
remains future design input only.

If approved later, write the store tests first, then the service, bounded HTTP
port/handler, and recovery-mode dialog. Browser-verify desktop and 390×844
behavior, keyboard focus, explicit labels, and reload history.

### Slice B — exact configured-default identity

Approved 2026-08-03 for implementation.

Write capability-contract and Codex inspector tests first, then propagate the
validated selector through snapshot, seat, gate, local/remote dispatch, and
historical UI rendering. Re-run the existing two-node fake fixture before any
live-machine action.

Live MacBook/Mac mini acceptance remains the final Spec 041 gate and requires
separate explicit authorization; Slice B implementation approval does not
authorize it.

## Affected files

If approved later, alternative recovery would be limited to:

- `core/store/conversations.go`, `core/store/conversations_test.go`
- `control/conversations.go`, `control/conversations_test.go`
- `ui/ports.go`, `ui/server.go`, `ui/conversations.go`
- `ui/conversations_api_test.go`
- `ui/conversation_page.go`, `ui/conversation_page_test.go`

Approved resolved identity implementation is limited to:

- `core/capability/types.go`, `core/capability/catalog.go`,
  `core/capability/inventory.go`, and focused catalog/version/inventory tests
- `exec/capability/codex_inspector.go`, `local_prober.go`, `registry.go`, and
  focused tests
- `exec/capability/gate.go` and tests
- `control/capabilities.go`, `control/conversations.go`, and focused tests
- `exec/cluster`, `exec/remote`, and `exec/node` tests where needed to prove
  unchanged model transport
- `ui` conversation identity projection/tests
- `cmd/fort` wiring/version fixture tests

## Rollback

For Slice B, revert capability-field propagation. If Slice A is approved later,
its bounded alternative operation/UI can be reverted separately. No table or
column is removed. Persisted participants and run rows keep their recorded
model. A node built for the newer capability protocol fails closed against an
older peer rather than silently accepting an empty configured default.
