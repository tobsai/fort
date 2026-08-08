# Spec 044 — Phase 1: One Primary Chat

**Status:** proposed implementation contract — no production implementation is
authorized until Toby approves this spec and selects the Web/macOS/iOS design
direction
**Date:** 2026-08-08
**Decision owner:** Toby
**Depends on:** Spec 043 direction; Spec 041 durable conversation, immutable
seat, retry, cancellation, and restart contracts; Spec 042 Slice B exact-model
contract
**First shipping surface:** local web only

## Decision

Phase 1 is a design-gated, local-web implementation of multiple dependable
conversations, each with exactly one immutable Primary Agent. “One Primary
Chat” means one agent per chat, not one singleton conversation for all time.

Fort will continue to own the canonical conversation, exact agent identity,
target lifecycle, readiness, and cross-machine dispatch. A stateless text-only
provider adapter will answer without a provider agent loop, tools,
provider-authored cross-turn memory, session continuation, browser/MCP access,
files, or a callback capable of mutating user-owned or connected resources.

The default experience contains only:

1. **Chats** — open primary chats ordered by newest durable activity;
2. **Transcript** — canonical human and attributed agent messages;
3. **Composer** — one input and Send action targeting the chat's one persisted
   participant;
4. **Needs you** — unresolved latest failed targets that have a real recovery
   action; and
5. **Settings** — Primary Agent identity, text-only eligibility, readiness, and
   explicit Recheck.

Phase 1 does not implement Memory V1, Act, tasks, schedules, projects,
playbooks, DAGs, or native Apple clients. Those are separate decisions.

## Authorization gate

This document is implementation-ready, but it is not self-authorizing.
Production work starts only after both conditions are satisfied:

- Toby selects or approves one of the three cross-platform design directions;
  and
- Toby explicitly approves this implementation contract, including the
  OpenAI Responses text-only lane and its API-billing/retention disclosure.

If the selected mockup changes navigation, identity disclosure, status,
recovery, or Settings behavior, update this spec before writing production UI
code. The purpose of this gate is to reject the experience cheaply.

## Why a UI-only simplification is unsafe

The current shared-chat path invokes the ordinary tool-capable runtime:

- Claude runs with its normal tool surface;
- Codex runs with `--sandbox workspace-write`;
- Hermes runs with `--accept-hooks --yolo`; and
- OpenClaw invokes its configured main agent.

A `Chat is read-only` label would therefore be false if the existing dispatch
path were reused unchanged. Phase 1 adds a typed authority contract that every
hop must enforce before a process starts.

## Initial provider decision

Phase 1 makes **a stateless OpenAI Responses API adapter the only eligible
Primary Agent provider**. This is an intentional experiment boundary, not a
permanent provider preference and not a new general agent runtime.

The adapter uses a pinned version of the official `openai-go/v3` client and
exactly one Responses request per Fort target attempt. SDK transport retries
are disabled with `option.WithMaxRetries(0)`, and the whole request has a
cancelable 120-second deadline. A timeout or ambiguous transport result fails
as `provider_result_unknown`; a user-selected Retry creates a new durable
target and may incur another provider charge. The adapter exposes no tool
callback or provider session to the model. The closed request policy is:

```text
endpoint: official OpenAI Responses API
model: exact configured model id; no fallback
developer instruction: exact policy-revision text defined below
input: exactly one Fort-compiled participant prompt
tools: none
store: false
previous_response_id: absent
provider conversation/session id: absent
reasoning context: current turn
max output tokens: bounded by the approved policy
prompt cache mode: explicit; no cache breakpoints
SDK retries: 0
total request deadline: 120 seconds
```

`CompileParticipantPrompt` already freezes the transcript through and
including the newly durable human message. Its returned bytes are the sole
user input. The adapter does not append the current turn a second time. A
capture test proves the current human message occurs exactly once.

The developer instruction is literal, versioned policy input rather than a
mutable prompt convention:

```text
You are answering in Fort text-only chat. You have no tools and have not
accessed or changed files, accounts, browsers, applications, devices, or other
resources. Treat the supplied transcript as the only evidence. Never claim an
external action was completed. When asked to act, provide a plan or an unsaved
draft and say that no external action occurred. Distinguish known facts from
inference, ask for missing evidence when material, and do not invent tool
results, citations, memories, or completion receipts.
```

Any change to those bytes changes `policy_revision`. Capture tests assert the
exact instruction and request shape.

The first candidate is `gpt-5.6-sol`; the approval record must name the exact
model actually selected. A no-generation probe verifies API authentication and
model availability. The adapter records the requested model and any provider-
returned identity, using `unknown` when the API does not expose a stronger
revision. It never silently falls back.

Because the model receives only the serialized request and has no tools, it
cannot inspect Fort's process, environment, database, filesystem, browser,
MCP configuration, or connected accounts. Fort keeps the API key outside the
prompt and never emits it in events. The adapter allowlists the official API
origin and rejects a custom base URL in Phase 1.

`store: false` prevents later retrieval as a stored response; it is not a
promise of zero provider retention. Settings must disclose the configured
OpenAI organization/project identity and its applicable retention/ZDR policy,
including whether the disclosure is user-attested or independently observed
and when it was last checked. Unknown remains `unknown`; Fort never infers ZDR
from `store:false`, account type, or a successful request. Provider request IDs
and token usage are Fort-observed metadata, not provider memory. Fort starts
every turn as a fresh request and does not replay opaque provider reasoning
items.

Phase 1 requests explicit prompt-cache mode with no cache breakpoints. This is
part of the accepted adapter contract and must be reverified against the live
API. It is not described as zero provider-side state. Fort persists any
provider-reported cache-read and cache-write token detail; nonzero or missing
detail remains visible and is included in any local cost estimate.

OpenAI API use is separately billed; it does not consume or inherit a Codex or
ChatGPT subscription allowance. That distinction must appear before the user
selects the provider. The user supplies a stable, nonsecret credential label
and billing-source disclosure; Fort never persists the API-key value or calls
an SDK estimate `billing_actual`.

Claude Code, Codex, Hermes, and OpenClaw remain visible in Settings as **Not
eligible for text-only chat**. Fort never substitutes one of them. A later
provider may enter Phase 1 only through a separately reviewed adapter change
that proves the same no-tools, no-session, prompt-only boundary. In particular,
CLI read-only or safe-mode flags are insufficient because the provider process
still has host environment, policy, filesystem, and network visibility.

## Scope

### Included

- one persisted exact Primary Agent selection;
- primary chats with exactly one immutable participant;
- a quiet local-web shell at `/`;
- a narrow text-only runtime authority carried end to end;
- fail-closed readiness, model, machine, policy, and adapter validation;
- restart/reload, same-seat retry, cancellation, and SSE reconstruction;
- truthful Needs-you projection for failed primary targets;
- a `fort chat` composition that does not require legacy orchestration config;
- local and two-machine contract acceptance; and
- three coordinated design mockup directions for Web, macOS, and iOS before
  implementation.

### Explicitly deferred

- cross-conversation memory, user profile, summaries, context checkpoints,
  compaction, and transcript search;
- Act, approvals, receipts, files, connectors, email, browser, and other
  external mutations;
- durable tasks, schedules, Today, Projects, playbooks, routes, DAGs, planner,
  solver, assignments, metrics, and raw run activity;
- multi-agent chats, participant management, Everyone, and Ask another agent;
- pinning, search, folders, and chat deletion in the new shell;
- provider-native session continuation;
- macOS, iOS, watch, CarPlay, gateway-web, TestFlight, or release work;
- deletion of legacy code or data; and
- further Spec 039 planner, solver, or setup expansion.

The existing 65,536-byte frozen-context limit remains unchanged and visible.
Crossing it fails before persistence or dispatch with
`conversation_context_limit`. Phase 2 must replace this cliff with the approved
continuity design; Phase 1 must not hide it.

## Experience contract

### First run

The root page shows a calm explanation and one **Choose Primary Agent** action.
It opens Settings. The composer is not shown as usable until an eligible exact
seat has been selected.

Settings groups options by computer and shows:

- profile and provider;
- exact requested/resolved model when available;
- computer;
- nonsecret credential label and organization/project identity;
- adapter and observed SDK version;
- text-only policy and adapter revision;
- retention and billing disclosure with source and observed-at time; unknown is
  shown explicitly;
- Ready, unavailable, setup-required, or ineligible text; and
- Recheck.

Recheck runs the existing bounded no-turn probes. It never installs software,
authenticates, changes models, dispatches a turn, or reroutes a seat.

### Normal chat

Creating a chat asks only for its title. Fort snapshots the configured Primary
Agent into exactly one participant. The header shows a compact identity such
as:

```text
Primary Agent · OpenAI GPT-5.6 Sol · MacBook Pro · Ready
```

An identity disclosure shows the full stored seat, text-only policy, adapter,
API-billing source, and retention disclosure. A compact **Text-only chat** label
explains that the model receives only this conversation context and cannot use
tools or change connected resources.

The composer contains one text input and Send. It has no provider, model,
computer, seat, participant, target, or Everyone control.

Changing the Primary Agent in Settings affects only chats created afterward.
It never updates, retargets, relabels, or silently migrates an existing chat.

### State and recovery

The transcript and chat list use only durable/event-derived state:

- **Queued** — the target is durable but the provider has not produced current
  activity;
- **Working** — current durable provider activity exists;
- **Answered** — the attributed terminal answer is durable;
- **Failed** — the durable target contains a bounded failure;
- **Canceled** — cancellation is durable.

The Fort orb may animate only while a target is truthfully Working. Status is
always shown in text and never by color or motion alone.

A current Queued or Working target offers Cancel. A current Failed target
offers Retry or Recheck and retry according to its error code. Retry targets
the same persisted participant, model, computer, text-only policy, and frozen
context. Fort never silently reroutes.

### Needs you

Needs you is a projection, not a new task system. It includes only the latest
unresolved Failed target for an open primary chat when that target has a
current recovery action.

- starting a retry removes the failed item while the latest attempt is Queued
  or Working;
- a later failure creates the new latest item;
- an Answered attempt resolves it;
- Canceled targets do not appear; and
- historical failed attempts with a newer attempt do not appear.

Each item deep-links to the exact conversation and target. The badge/drawer is
absent when the projection is empty.

### Navigation and responsive behavior

- `/` is the new primary-chat shell.
- `/shared` preserves the current Spec 041 multi-agent/shared-chat page in full
  `fort serve` mode.
- `/legacy` preserves the Command Deck/board rollback surface.
- `fort chat` does not mount legacy mutating APIs or present the old surfaces.

Desktop web uses a Chats rail and one transcript. Needs you and Settings are
temporary drawers, not permanent dashboard columns. Below approximately 860px,
the Chats rail becomes a keyboard-accessible sheet.

Acceptance viewports are 1280×720 and 390×844, with a compact-phone check at
375×667 with the keyboard present. Interactive targets are at least 44 CSS
pixels. The page must have visible focus, logical tab order, no horizontal
overflow, reduced-motion behavior, and SSE reconnect/rebuild without duplicate
messages.

## Cross-platform design gate

The three mockup directions must each show one coherent experience across:

1. Web desktop at approximately 1440×900;
2. native macOS at approximately 1240×800; and
3. native iPhone at approximately 393×852 without a device bezel.

All three surfaces use the same sample conversation, exact Primary Agent,
message order, and target state so the comparison is about design. The visual
language may vary, but every option must show:

- newest-first Chats;
- one transcript and composer;
- full identity available without dominating the conversation;
- the text-only boundary;
- a small Needs-you entry point;
- Settings with readiness and Recheck; and
- truthful Queued, Working, Answered, Failed, and recovery states.

The mockups must not show memory, Act, projects, Today, schedules, playbooks,
DAGs, participant chips, metrics, or a public user directory. macOS and iOS
mockups are labelled future design. They do not claim implementation or parity
until FortKit consumes the canonical primary-chat contract.

The preferred visual foundation is Fort's existing deep-navy/electric-blue
intelligence-core language and real orb asset, simplified from a command center
into a conversation. Alternative directions may test a flatter private-channel
language or an Apple-first daylight language; they must not blend incompatible
themes into one option.

## Persistence contract

Phase 1 uses additive tables and leaves every existing conversation row
untouched.

### `primary_agent_setting`

```sql
CREATE TABLE IF NOT EXISTS primary_agent_setting (
  singleton INTEGER PRIMARY KEY CHECK(singleton=1),
  option_id TEXT NOT NULL,
  seat_id TEXT NOT NULL,
  profile TEXT NOT NULL,
  agent TEXT NOT NULL,
  model TEXT,
  machine TEXT NOT NULL,
  display_name TEXT NOT NULL,
  policy_id TEXT NOT NULL,
  policy_revision TEXT NOT NULL,
  adapter_id TEXT NOT NULL,
  adapter_revision TEXT NOT NULL,
  sdk_version TEXT,
  reasoning_effort TEXT NOT NULL,
  reasoning_context TEXT NOT NULL,
  max_output_tokens INTEGER NOT NULL,
  store_responses INTEGER NOT NULL CHECK(store_responses=0),
  prompt_cache_mode TEXT NOT NULL,
  sdk_retries INTEGER NOT NULL CHECK(sdk_retries=0),
  request_timeout_millis INTEGER NOT NULL,
  developer_instruction_revision TEXT NOT NULL,
  credential_ref TEXT NOT NULL,
  organization_id TEXT,
  project_id TEXT,
  retention_mode TEXT NOT NULL,
  retention_source TEXT NOT NULL,
  retention_observed_at TEXT,
  billing_source TEXT NOT NULL,
  billing_source_provenance TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

The row is an atomic upsert of a currently ready text-only option. Readiness
and failure reason are current projections and are not persisted as identity.
`option_id` is a `primary-option:v1` digest over the exact seat, credential
reference, organization/project, and policy identity. It does not change the
meaning of legacy `seat:v1`. The credential reference is a user-chosen label
that resolves only on the selected computer; no secret crosses the mesh or is
stored in SQLite.

### `primary_chat`

```sql
CREATE TABLE IF NOT EXISTS primary_chat (
  conversation_id TEXT PRIMARY KEY,
  participant_id TEXT NOT NULL UNIQUE,
  policy_id TEXT NOT NULL,
  policy_revision TEXT NOT NULL,
  reasoning_effort TEXT NOT NULL,
  reasoning_context TEXT NOT NULL,
  max_output_tokens INTEGER NOT NULL,
  store_responses INTEGER NOT NULL CHECK(store_responses=0),
  prompt_cache_mode TEXT NOT NULL,
  sdk_retries INTEGER NOT NULL CHECK(sdk_retries=0),
  request_timeout_millis INTEGER NOT NULL,
  developer_instruction_revision TEXT NOT NULL,
  credential_ref TEXT NOT NULL,
  organization_id TEXT,
  project_id TEXT,
  retention_mode TEXT NOT NULL,
  retention_source TEXT NOT NULL,
  retention_observed_at TEXT,
  billing_source TEXT NOT NULL,
  billing_source_provenance TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(conversation_id) REFERENCES conversation(id) ON DELETE CASCADE,
  FOREIGN KEY(participant_id) REFERENCES conversation_participant(id)
);
```

The existing participant remains the canonical profile, provider/model, and
computer snapshot. The marker adds the stable approved text-only policy and
nonsecret disclosure snapshot. It is immutable after chat creation: application
code and database triggers reject update or delete.

Creating a primary chat atomically inserts the conversation, exactly one
participant copied from the setting, and the marker. Existing conversations
have no marker and are legacy shared conversations; there is no backfill.

Participant add/remove operations reject a marked primary chat. Rename,
archive, and reopen remain allowed. The Phase 1 shell does not expose Delete.

### Per-attempt authority provenance

Add these columns to `conversation_target` through the existing idempotent
column migration pattern:

```text
authority, policy_id, policy_revision,
selected_adapter_id, selected_adapter_revision, selected_sdk_version,
requested_model, reasoning_effort, reasoning_context, max_output_tokens,
store_responses, prompt_cache_mode, sdk_retries, request_timeout_millis,
developer_instruction_revision,
credential_ref, organization_id, project_id,
observed_adapter_id, observed_adapter_revision, observed_sdk_version,
resolved_model, provider_request_id, provider_terminal_status,
usage_source, input_tokens, cached_input_tokens, cache_write_tokens,
output_tokens, reasoning_tokens
```

The policy, request-policy, credential reference, and selected-adapter fields
are resolved synchronously and populated before target commit and `202
Accepted`. Async dispatch must match that exact selection or fail drift before
a provider request. Observed adapter, provider identity, terminal status, and
usage fields are written only from a typed response-metadata receipt. They
commit atomically with the terminal answer or failure so an Answered target
cannot lack its claimed provenance. Unavailable values remain
empty/`unknown`. The UI derives the authority label of each answer from its own
target, never from the conversation marker alone.

The policy revision is stable conversation behavior. Selected and observed
adapter revisions and SDK versions describe one execution attempt. Retry may
select a newly approved compatible adapter only when it certifies the exact
same chat policy revision; it creates a new target and records the new
selection. A changed policy revision fails closed and requires a future
explicit migration or a new chat.

### Database enforcement against old code

Migrations add SQLite triggers that reject:

- insert of a primary marker unless its participant belongs to the same
  conversation and that conversation has exactly one active participant;
- update or delete of a primary marker;
- insert, update, or delete of a participant in a marked primary chat;
- deletion of a marked primary conversation; and
- insertion of a target for a marked primary chat unless its authority,
  policy id, policy revision, exact request-policy fields, credential
  reference, organization, and project exactly match the marker, and its
  selected-adapter identity is nonempty.

These triggers are part of the invariant, not merely UI validation. An older
binary omits the new target authority fields and therefore cannot append a
tool-capable turn to a marked chat. Existing legacy targets remain policy
`unknown`; they are never relabelled text-only.

Target authority, policy, request-policy, credential reference, and selected
adapter fields are immutable after insertion. Observed metadata may make one
validated transition from empty to the terminal receipt in the same
transaction as Answered/Failed/Canceled; it cannot be rewritten later.

Rollback does not deploy an older binary against a database containing primary
chats. Use the accepted new binary's full `fort serve` mode, or restore a
coordinated pre-Phase-1 database snapshot with the older binary.

## Text-only capability and policy

Reuse the bounded capability inventory instead of creating a second readiness
system.

Add:

- provider-agent key `openai-responses`;
- exact profile `openai-responses:gpt-5.6-sol` using
  `SelectionModel{ModelID: "gpt-5.6-sol"}`; provider identity remains the
  separate `openai-responses` agent key and is never prefixed onto
  `RunSpec.Model`;
- logical capability `model.chat.text-only`;
- adapter `model.chat.text-only.openai-responses`;
- policy `openai-responses-text-v1`; and
- a pinned official SDK/request-schema adapter revision.

This is a new API profile, not a Codex CLI seat. Its closed predicates require
an API credential resolvable from the selected computer's nonsecret credential
reference, an official OpenAI API origin, the configured organization/project
identity, the exact requested model returned by the no-generation probe, and
the accepted policy/adapter revision. `OPENAI_API_KEY` is one supported local
secret source for the initial adapter; the value is never returned, logged,
hashed into an identifier, persisted, or sent across the mesh. The initial
configuration also names `FORT_OPENAI_CREDENTIAL_REF`, with optional
`OPENAI_ORGANIZATION` and required approved project identity. A missing
nonsecret identity or unverifiable policy remains ineligible rather than being
guessed.

The option also carries nonsecret, operator-supplied disclosure config:

```text
FORT_OPENAI_RETENTION_MODE=unknown|default_abuse_monitoring|modified_abuse_monitoring|zero_data_retention
FORT_OPENAI_RETENTION_SOURCE=unknown|user_attested|admin_record|provider_admin_export
FORT_OPENAI_RETENTION_OBSERVED_AT=<RFC3339 or empty>
FORT_OPENAI_BILLING_SOURCE=<nonsecret API billing account label or unknown>
FORT_OPENAI_BILLING_SOURCE_PROVENANCE=unknown|user_attested|admin_record|billing_export
```

These values describe the selected organization/project; the no-generation
model probe does not discover them. `unknown` is a truthful selectable value
only when Toby explicitly accepts that disclosure at the design/approval gate.
Fort never upgrades a user-attested value to provider-observed provenance.

The option composes that ready exact OpenAI API profile on one computer with a
no-generation probe that verifies API authentication, exact model
availability, official API origin, SDK/request schema, and the approved policy.
A normally ready CLI profile is not automatically text-only eligible.

Primary Agent options combine current profile/model/computer readiness,
credential/project identity, and a ready policy-certified adapter. Selection
snapshots the disclosure and policy plus the currently observed adapter. Chat
creation snapshots the policy and disclosure. Turn creation synchronously
revalidates the same profile, model, computer, policy, and adapter and records
that exact selected adapter on the target before returning `202`. Dispatch
revalidates the selection once more. Missing or changed identity fails closed
before a provider request, with no reroute or fallback.

Remote computers publish the same information through a new closed,
secret-free capability wire type:

```go
type TextOnlyOptionOffer struct {
    ProtocolVersion          int
    MachineID                string
    SeatID                   string
    AgentKey                 string
    ProfileID                string
    RequestedModel           string
    ResolvedModel            string
    CredentialRef            string
    OrganizationID           string
    ProjectID                string
    PolicyID                 string
    PolicyRevision           string
    ReasoningEffort          string
    ReasoningContext         string
    MaxOutputTokens          int
    StoreResponses           bool
    PromptCacheMode          string
    SDKRetries               int
    RequestTimeoutMillis     int
    DeveloperInstructionRev  string
    AdapterID                string
    AdapterRevision          string
    SDKVersion               string
    RetentionMode            string
    RetentionSource          string
    RetentionObservedAt      string
    BillingSource            string
    BillingSourceProvenance  string
}
```

The node emits this offer only from normalized local configuration plus a
successful bounded readiness check. The hub validates closed enums, bounds,
RFC3339 fields, exact profile/agent/model relationships, machine/seat identity,
policy/adapter compatibility, and absence of secret material. It canonicalizes
the fields in the order above and computes `primary-option:v1` as the
versioned digest; a supplied or cached digest is never trusted. Duplicate or
conflicting offers make that machine's text-only options ineligible. The API
key value and its derivations are forbidden from the offer.

The capability protocol and inventory wire schema advance together. A peer
that omits `TextOnlyOptionOffer`, sends an unknown offer version, or cannot
validate its complete contract remains visible through ordinary profile
inventory but is **Not eligible for text-only chat**. The hub never fills
missing disclosure or adapter fields from its own environment and never hides
them inside predicate IDs.

Capability protocol versions must advance because an old peer cannot enforce
the new authority field. An old or unverified node is ineligible; Fort must not
send it a text-only turn and hope it ignores unknown fields safely.

The current `seat:v1` meaning remains unchanged. Text-only identity is the
persisted seat plus the separate policy snapshot; it does not silently alter
legacy seat semantics.

## Runtime contract

Extend `runtime.RunSpec` with a closed, wire-visible request policy rather than
free-form fields:

```go
type AuthorityMode string
type ReasoningEffort string
type PromptCacheMode string

const AuthorityChatTextOnlyV1 AuthorityMode = "chat_text_only_v1"

type TextOnlyPolicy struct {
    PolicyID                      string
    PolicyRevision                string
    Model                         string
    ReasoningEffort               ReasoningEffort
    ReasoningContext              string // exactly "current_turn"
    MaxOutputTokens               int
    Store                         bool   // exactly false
    PromptCacheMode               PromptCacheMode // exactly "explicit"
    SDKRetries                    int    // exactly 0
    RequestTimeoutMillis          int    // exactly 120000 in v1
    DeveloperInstructionRevision string
    SelectedAdapterID             string
    SelectedAdapterRevision       string
    SelectedSDKVersion            string
    CredentialRef                 string
    OrganizationID                string
    ProjectID                     string
}

type RunSpec struct {
    // existing fields
    Authority              AuthorityMode
    RuntimeContract        string
    TextOnlyPolicy         *TextOnlyPolicy
    ExpectedPolicyRevision string `json:"-"`
}

type ProviderUsage struct {
    InputTokens       int64
    CachedInputTokens int64
    CacheWriteTokens  int64
    OutputTokens      int64
    ReasoningTokens   int64
}

type ResponseMetadata struct {
    ProviderRequestID       string
    RequestedModel          string
    ResolvedModel           string
    SelectedAdapterID       string
    SelectedAdapterRevision string
    SelectedSDKVersion      string
    ObservedAdapterID       string
    ObservedAdapterRevision string
    ObservedSDKVersion      string
    TerminalStatus          string
    UsageSource             string
    Usage                   ProviderUsage
}

type RunEvent struct {
    // existing Type, Data, Code fields
    Response *ResponseMetadata
}
```

An empty authority retains the legacy execution contract in full `fort serve`
mode only. Any unknown nonempty authority or mismatched runtime contract fails
before a provider request. `TextOnlyPolicy` is required only for the exact
text-only authority and is forbidden for legacy authority. Hub and node
validate every enum, string identity, fixed value, and numeric bound; the
credential reference resolves to a secret only on the selected computer and no
secret enters this structure.

A primary-chat dispatch always supplies `chat_text_only_v1`,
`openai-responses-text-v1`, the complete `TextOnlyPolicy`, and the expected
policy revision. Hub-side preflight validates the revision and clears only the
private expected field. Authority, runtime contract, and text-only policy cross
the cluster/remote wire. Chat-mode nodes accept only that exact
authority/contract/profile/adapter/policy combination; empty, legacy, unknown,
wrong-contract, and other-provider requests start zero work.

The full `fort serve` composition uses a closed local runtime multiplexer:

- exact text-only authority plus the `openai-responses` agent key can route
  only to `exec/openairesponses`;
- empty legacy authority can route only to the existing native-provider
  runtime; and
- every cross-combination or unknown value fails before work starts.

The same mux policy protects full-mode node execution. The `fort chat` variant
contains only its text-only branch. Tests prove that no text-only request can
reach a CLI and no legacy/native request can reach the Responses adapter.

`fort chat` never mounts the unrestricted legacy node execution handler. It
mounts a restricted text-only transport backed only by the text adapter. A mesh
token holder may submit text-only model work, but cannot reach a shell, tool,
provider CLI, filesystem, browser, or connected resource through that endpoint.

The Responses adapter accepts only a terminal `completed` response containing
exactly one assistant text result. It may observe the closed inert output set
required by the approved model, including a `reasoning` item; opaque reasoning
is discarded and is never stored, shown, summarized, or replayed. A tool,
hosted-tool, computer, file-search, function-call, code/program, MCP,
multi-agent, or unknown output item fails the target as
`chat_authority_violation` and appends no agent answer.

`incomplete` (including max-output exhaustion), `failed`, `cancelled`, and
refusal terminal states are mapped to bounded non-Answered outcomes. Partial
text is never appended as the canonical answer. A completed response with
zero/multiple assistant text results, incoherent model/request/usage metadata,
or an unknown terminal status fails closed. The typed `ResponseMetadata`
receipt is the only transport for provider provenance; it is never encoded in
free-form `Data`. Hub persistence validates that receipt and commits it
atomically with the terminal answer or failure. A missing adapter, old
protocol, unknown contract, custom API origin, or request-schema mismatch
returns `chat_policy_unavailable` with zero provider requests.

## HTTP and port contract

Add a narrow `PrimaryChatPort`. Ordinary clients never supply participant IDs,
seat arrays, or target arrays.

### Primary Agent

```text
GET /api/settings/primary-agent
```

Returns the persisted selection or `null`, its current state/reason, and all
Primary Agent options. It is read-only and invokes no probe or runtime.

```text
PUT /api/settings/primary-agent
{ "option_id": "primary-option:v1:..." }
```

The server resolves a currently ready text-only option and persists the full
seat, credential/project/disclosure, and policy snapshot. It returns the stored
selection plus current state. Unknown, unready, drifted, or policy-ineligible
choices fail closed. A caller cannot construct a new combination by sending a
seat ID plus independent policy fields.

```text
DELETE /api/settings/primary-agent
```

Clears the default for future chats. Existing chat identity is unchanged.

Reuse `POST /api/conversation-seats/recheck` for an explicit fresh probe, or
add a narrow alias that calls the same bounded port. It never dispatches an
agent turn.

### Chats

```text
GET  /api/chats?state=open|archived|all
POST /api/chats                 { "title": "..." }
GET  /api/chats/{id}
PATCH /api/chats/{id}           { "title": "..." }
PATCH /api/chats/{id}           { "state": "open|archived" }
POST /api/chats/{id}/turns      { "client_turn_id": "uuid", "text": "..." }
POST /api/chats/{id}/targets/{target_id}/retry
POST /api/chats/{id}/targets/{target_id}/cancel
GET  /api/chats/{id}/events
GET  /api/needs-you
```

`GET /api/chats` defaults to `state=open`; `archived` and `all` are the only
other accepted values. Every result is newest-first and emits `[]`, never
`null`. This is the path for reopening an archived chat. `GET
/api/chats/{id}` returns the canonical conversation detail plus the complete
`primary_identity` snapshot.

Turn creation resolves the marked chat's sole participant server-side. A
client cannot select another target. `202 Accepted` means the prompt, frozen
context boundary, target, and run identity are durable; it does not mean the
provider succeeded.

Retry/cancel validate that the nested chat and target match. Missing or foreign
IDs are `404`; valid IDs in the wrong current state are `409`.

`GET /api/needs-you` returns the projection defined above with non-null arrays.
It must not create, mutate, probe, or dispatch anything.

Closed error codes added by Phase 1 are:

- `primary_agent_not_configured`;
- `primary_agent_unready`;
- `primary_agent_drift`;
- `chat_policy_unavailable`;
- `chat_authority_violation`;
- `primary_chat_invariant`;
- `provider_result_unknown`;
- `provider_incomplete`;
- `provider_refusal`;
- `provider_failed`.

A confirmed canceled request uses the existing Canceled target state rather
than an error code.

Retain `seat_unready`, `conversation_context_limit`, and existing bounded
target errors.

## `fort chat` composition

Add:

```text
fort chat
```

It wires only what primary chat requires:

- config, SQLite store, watchdog, and signal handling;
- OpenAI Responses/cluster/remote runtime with the text-only authority gate;
- the bounded capability/readiness subsystem;
- restricted chat-node and direct mesh transport for exact remote seats;
- the Primary Agent and primary-chat services; and
- the new web shell and its narrow APIs.

It does not load or initialize rules, router, engine, inbox, flows, graph
executor, scheduler, Today, planner, playbooks, setup solver UI, board, or
legacy chat APIs. It must start when rules and flow directories are absent and
when no schedule/playbook configuration exists.

Relay/gateway web is not mounted by `fort chat` in Phase 1. The existing relay
serves an identical mux and would otherwise publish UI and node APIs beyond the
local-web scope.

The accepted new binary's `fort serve` remains the full legacy rollback
composition. The launchd service continues to use `serve` until Phase 1
acceptance explicitly authorizes a switch. Chat-only startup must not weaken
the full-mode test suite.

## TDD delivery sequence

Every step begins with a focused failing test, observes the failure, then adds
the minimum code to pass. Keep `go test ./...` green after each slice and run
`-race` for concurrency changes.

1. **Baseline checkpoint** — commit the verified Spec 041/042 work and Spec
   043 direction before any Phase 1 code. Exclude generated Apple artifacts.
2. **Domain and store** — setting round-trip/upsert; atomic chat, participant,
   and marker creation; marker immutability; newest-first filtering; setting
   changes leave prior chat identity byte-for-byte unchanged; legacy rows are
   untouched.
3. **Capability policy** — new OpenAI Responses API profile and predicates;
   credential/project/disclosure binding; text-only catalog/policy validation;
   no-generation OpenAI auth/model probe; exact option projection; policy drift
   and old peers fail closed; zero generated tokens during readiness.
4. **Provider and transport authority** — closed enum validation; request
   capture proves official origin, exact model, one canonical input, stable
   developer instruction, no fallback/tools/session, `store:false`, explicit
   cache mode, zero SDK retries, and bounded timeout; other providers reject
   before a request; cluster/remote and the restricted chat node preserve the
   complete typed request policy and typed response metadata; the private
   expected policy revision never crosses the wire; completed/reasoning,
   incomplete, failed, canceled, refusal, tool, and unknown output shapes have
   focused tests.
5. **Control service** — configure the setting; create one-participant chat;
   server-selected target; synchronously persist exact selected adapter and
   `RunSpec` policy before `202`; offline/drift causes zero starts; observed
   metadata commits atomically with the terminal state; same-seat retry retains
   identity, context, and authority; latest failure reducer is deterministic.
6. **HTTP** — exact methods, bodies, statuses, codes, nested-ID validation, and
   non-null arrays; GET and Settings operations invoke no runtime.
7. **Web shell** — root contains only the five approved product concepts;
   create/send single-flight and client-turn idempotency; reload/SSE rebuild;
   Needs-you deep link; keyboard, focus, reduced-motion, and responsive tests.
   Preserve `/shared` and `/legacy` in full mode.
8. **Composition** — `fort chat` starts with missing legacy config; only the
   text-only authority can invoke its runtime; full-mode local/node mux has no
   cross-routing; restricted node, direct mesh, and capability remain
   functional; generic exec and relay are absent; `fort serve` remains green.
9. **Concurrency and canaries** — race tests, local/remote fake turns, daemon
   restart, remote offline without reroute, exact retry, old-node failure,
   request/metadata capture, typed terminal-shape validation, and live prompts
   that request a tool/file action but receive no such capability or false
   completion claim.
10. **Visual/live acceptance** — only after explicit authorization and design
    selection.

## Expected file boundary

New files are expected in these bounded areas:

```text
specs/044-one-primary-chat-phase-1.md
core/conversation/primary.go
core/conversation/primary_test.go
core/store/primary_chat.go
core/store/primary_chat_test.go
control/primary_chat.go
control/primary_chat_test.go
ui/primary_chat.go
ui/primary_chat_api_test.go
ui/primary_chat_page.go
ui/primary_chat_page_test.go
cmd/fort/chat.go
cmd/fort/chat_test.go
exec/openairesponses/runtime.go
exec/openairesponses/runtime_test.go
exec/runtime_mux.go
exec/runtime_mux_test.go
```

Focused modifications are permitted in:

- `core/store/store.go`;
- `core/runtime/runtime.go`;
- bounded capability catalog, predicate, version, probe, registry, and gate
  files plus their tests;
- `go.mod` and `go.sum` for one pinned official OpenAI SDK;
- `exec/cluster`, `exec/remote`, and a restricted `exec/node` chat path plus
  their tests; the existing native-provider commands remain unchanged;
- `control/conversations.go` only where target execution must load/enforce the
  primary marker;
- `ui/ports.go` and `ui/server.go`;
- `cmd/fort/main.go`, `wire.go`, `capabilities.go`, `service.go`, and focused
  mesh/service tests; and
- README/governing-spec references only after acceptance.

`ui/apple/**` is excluded from Phase 1 production implementation. Mockups are
design evidence, not Swift code authorization. Scope expansion requires a spec
amendment before implementation.

## Acceptance

### Design gate

- Toby selects or approves one cross-platform direction.
- The selected first-run, active-chat, failure/Needs-you, Settings, and compact
  layouts match this contract.
- Any product change is reflected here before code starts.

### Automated

```text
go test ./...
go test -race ./cmd/fort ./control ./core/capability ./core/conversation \
  ./core/store ./exec/capability ./exec/openairesponses ./exec/cluster \
  ./exec/remote ./exec/node ./ui
go vet ./...
git diff --check
```

Architecture tests must continue to prove that `core` imports no `ui` or
concrete executor and `ui` imports no engine, graph, router, or native package.

### Local functional

- configure a ready Primary Agent, create a chat, send a turn, and receive one
  attributed answer;
- reload and restart with no lost, duplicated, or reordered messages;
- change Settings and prove the existing chat identity is unchanged;
- disclose exact requested/resolved model, computer, policy, adapter/SDK
  version, credential/project identity, API billing source/provenance, and
  retention mode/source/observed-at; unavailable identity remains `unknown`,
  never inferred;
- an unavailable or drifted seat fails before provider start and never
  reroutes;
- same-seat retry starts only the failed target;
- captured requests contain no tool, MCP, browser, file, provider-session, or
  previous-response field, use `store:false`, explicit cache mode, zero SDK
  retries, and the bounded timeout; exactly one compiled input includes the
  human turn once and no local material beyond the Fort prompt;
- one inert reasoning item plus one completed assistant text result is accepted
  while reasoning is discarded; tool/unknown items and incomplete, failed,
  canceled, refusal, ambiguous-timeout, partial, or incoherent responses append
  no canonical answer and retain typed provenance;
- full-mode and chat-only runtime mux tests prove authority/provider
  cross-routing starts zero work;
- missing rules, flows, schedules, planner, and playbooks do not prevent
  `fort chat` startup; and
- the 65,536-byte context overflow fails before new turn/target persistence and
  provider dispatch.

### Two-machine, separately authorized

- both machines run the same accepted text-only protocol/build;
- one remote Primary Agent answers with exact attribution;
- taking that computer offline produces `seat_unready` and no reroute;
- Recheck and retry starts the same failed seat after readiness returns;
- restart/reload preserves the canonical transcript and target history; and
- an old or unverified policy/adapter is not selectable and sends zero provider
  requests.

### Trial gate

Dogfood for 7–14 days and adjudicate at least 40 ordinary turns against
prewritten rubrics. Continue only if:

- at least 38 of 40 ordinary turns pass;
- there are zero lost/duplicate turns, silent identity changes, authority
  violations, unapproved mutations, or false external-completion claims;
- provider token usage, including reported cache-read/cache-write detail, is
  recorded as `provider_usage`; any dollar conversion is labelled
  `local_estimate`, and billing actual is reconciled manually against the
  OpenAI billing source named in the approval record;
- manually reconciled spend remains inside the separately chosen trial cap;
  Fort does not claim to enforce a dollar cap in Phase 1; and
- time saved exceeds maintenance and recovery time.

## Rollback

No destructive migration is permitted. Before service promotion, rollback is
simply stopping `fort chat` and running the same accepted binary's `fort
serve`. After promotion, restore the launchd command to `serve` on every
participating machine.

Do not run a pre-Phase-1 binary against a database containing primary chats.
If binary rollback is necessary, stop all writers and restore the coordinated
pre-Phase-1 database snapshot together with the prior binary. The database
triggers deliberately make unsupported old writes fail, but snapshot restore
is the only rollback that removes new policy semantics completely.

`/shared` and `/legacy` are not deleted. Historical runs, projects, schedules,
and playbooks are not changed. The additive Primary Agent and primary-chat
rows may remain unused without a data rewrite.

## Current baseline warning

The Spec 041/042 checkpoint captured in commit `b4bd10c` is internally
test-green, but it is not a live-release claim. As of 2026-08-08, Fort's
changed capability catalog pins the bundled Codex command contract to
`codex-cli 0.146.0-alpha.9.2`, while the current ChatGPT-bundled executable
reports `0.147.0-alpha.6.5` and therefore fails closed as
`incompatible_version`. The PATH Codex is `0.143.0`.

That drift does not authorize loosening the check and does not affect the
Responses API Phase 1 lane, but it must be resolved through the accepted
catalog update process before anyone calls the broader build live-ready. Spec
041's remaining live-status wording must likewise be reconciled before release.

## Current provider references

These are time-sensitive implementation inputs, not reliability guarantees.
The accepted adapter revision must retest the live API/SDK contract:

- OpenAI [current model guidance](https://developers.openai.com/api/docs/guides/latest-model)
  for exact model IDs, `store:false`, reasoning context, prompt-cache policy,
  and manual context replay behavior;
- OpenAI [model catalog](https://developers.openai.com/api/docs/models) for the
  currently available GPT-5.6 variants;
- the official [OpenAI Go SDK](https://github.com/openai/openai-go) for the
  bounded Responses client, explicit retry disabling, and request timeout;
- OpenAI [API data controls](https://platform.openai.com/docs/models/default-usage-policies-by-endpoint)
  for application-state, abuse-monitoring, and ZDR distinctions; and
- OpenAI [billing separation guidance](https://help.openai.com/en/articles/8156019-how-can-i-move-my-chatgpt-subscription-to-the-api)
  for the independent API billing account.

## Approval record

Record the following before implementation begins:

```text
Selected design direction: ____________________
Spec 044 approved by Toby: ____________________
Initial exact OpenAI profile/model: ___________
Accepted policy / adapter / SDK: ______________
Reasoning effort / max output tokens: __________
Credential ref / organization / project: ______
Manual trial spend cap / billing source: _______
Billing provenance: ____________________________
Provider retention mode / source / date: _______
Implementation start commit: __________________
```
