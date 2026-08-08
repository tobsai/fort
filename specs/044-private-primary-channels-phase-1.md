# Spec 044 — Phase 1: Private Primary Channels

**Status:** approved for implementation on 2026-08-08 — live provider use,
`primary` promotion, and the trial remain blocked until the deployment-specific
approval inputs below are complete
**Date:** 2026-08-08
**Decision owner:** Toby
**Depends on:** Spec 043 direction; Spec 041 durable conversation, immutable
seat, retry, cancellation, and restart contracts; Spec 042 Slice B exact-model
contract
**First shipping surface:** local web only

## Decision

Phase 1 is a design-gated, local-web implementation of multiple dependable
private Channels, each with exactly one immutable Primary Agent. A Channel is
the durable user-owned context boundary, not a public messaging feature, a
Project, or a provider session. It maps 1:1 to one existing canonical
conversation row: `channel_id == conversation.id`.

Messages, turns, targets, retries, and compiled model context are always scoped
to that Channel ID. Nothing from another Channel enters the prompt unless a
later explicit source-link contract permits it. Two Channels may use the same
Primary Agent while retaining byte-disjoint transcripts and context.
An existing durable schedule may have one explicit related-Channel link for
navigation/provenance; that link never changes its actual execution target.

Fort will continue to own the canonical conversation, exact agent identity,
target lifecycle, readiness, and cross-machine dispatch. A stateless text-only
provider adapter will answer without a provider agent loop, tools,
provider-authored cross-turn memory, session continuation, browser/MCP access,
files, or a callback capable of mutating user-owned or connected resources.

The default experience contains only:

1. **Channels** — pinned and recent private Channels ordered by newest durable
   activity;
2. **Transcript** — canonical human and attributed agent messages;
3. **Composer** — one input and Send action targeting the Channel's one persisted
   participant;
4. **Scheduled** — every durable schedule definition plus upcoming and recent
   occurrence state;
5. **Needs you** — unresolved latest failed Channel targets that have a real
   recovery action; and
6. **Settings** — Primary Agent identity, text-only eligibility, readiness, and
   explicit Recheck.

Phase 1 does not implement Memory V1, Act, new task records, schedule creation
or mutation, projects, playbooks, DAG authoring, or native Apple clients. It
does preserve and expose existing durable schedule execution truthfully.

## Authorization gate

This document is implementation-ready, but it is not self-authorizing.
Production work starts only after both conditions are satisfied:

- Toby selects or approves one or more of the recorded visual treatments for
  the shared Channel/Scheduled experience;
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
- private Primary Channels with exactly one immutable participant;
- Channel rename, pin, archive/reopen, and newest-first private navigation;
- a read-only Scheduled destination backed by all durable definitions and
  occurrences, not only today's projection;
- a quiet local-web shell at `/`;
- a narrow text-only runtime authority carried end to end;
- fail-closed readiness, model, machine, policy, and adapter validation;
- restart/reload, same-seat retry, cancellation, and SSE reconstruction;
- truthful Needs-you projection for failed primary targets;
- the new shell in full `fort serve`, preserving scheduler ownership;
- local and two-machine contract acceptance; and
- three coordinated design mockup directions for Web, macOS, and iOS before
  implementation.

### Explicitly deferred

- cross-conversation memory, user profile, summaries, context checkpoints,
  compaction, and transcript search;
- Act, approvals, receipts, files, connectors, email, browser, and other
  external mutations;
- new task records; schedule create/edit/pause/resume/delete/Run-now;
  Channel-bound scheduled prompts; Today/Week calendar boards; Projects;
  playbook/route/DAG/planner/solver authoring; assignments, metrics, and raw run
  activity;
- multi-agent chats, participant management, Everyone, and Ask another agent;
- cross-Channel full-text search, folders, and Channel deletion in the new
  shell;
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

### Normal Channel

Creating a Channel asks only for its name. Fort snapshots the configured Primary
Agent into exactly one participant. The header shows a compact identity such
as:

```text
Primary Agent · OpenAI GPT-5.6 Sol · MacBook Pro · Ready
```

An identity disclosure shows the full stored seat, text-only policy, adapter,
API-billing source, and retention disclosure. A compact **Text-only chat** label
explains that the model receives only this Channel context and cannot use
tools or change connected resources.

The composer contains one text input and Send. It has no provider, model,
computer, seat, participant, target, or Everyone control.

Changing the Primary Agent in Settings affects only Channels created afterward.
It never updates, retargets, relabels, or silently migrates an existing Channel.

### State and recovery

The transcript and Channel list use only durable/event-derived state:

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
unresolved Failed target for an open Primary Channel when that target has a
current recovery action.

- starting a retry removes the failed item while the latest attempt is Queued
  or Working;
- a later failure creates the new latest item;
- an Answered attempt resolves it;
- Canceled targets do not appear; and
- historical failed attempts with a newer attempt do not appear.

Each item deep-links to the exact conversation and target. The badge/drawer is
absent when the projection is empty.

### Scheduled

Scheduled is a top-level chronological destination, not a Today board or a
permanent dashboard rail. It lists every persisted definition—including
paused and non-today schedules—and its latest/upcoming occurrence evidence.
Each row shows:

- durable schedule ID and title;
- **Active** or **Paused** definition state;
- Once or recurring cadence in human language plus the stored IANA timezone;
- exact next and last fire instants;
- truthful target kind and identity, initially the existing `flow_id`;
- latest occurrence state: **Upcoming, Fired, Running, Completed, Failed,** or
  **Canceled**;
- occurrence error and linked run/observed execution identity when available;
- scheduler ownership state: **Active, Inactive,** or **Unknown**; and
- projection freshness time.

Unlinked flow schedules are labelled **System schedules**. An explicitly
linked schedule may show Related Channel and Open Channel, while its actual
flow target remains visible. Fort never guesses a Channel binding or implies
that a flow result appeared in a Channel. Upcoming items offer View schedule;
Fired/Running offer Open run; Completed offers View result; Failed offers
Review failure. There is no Retry because silently replaying an already claimed
occurrence would violate the once-only contract.

All Scheduled operations in Phase 1 are reads. Creating, editing, enabling,
pausing, resuming, deleting, or manually firing a schedule requires a later
approved contract.

### Navigation and responsive behavior

- `/` is the new private-Channels and Scheduled shell.
- `/shared` preserves the current Spec 041 multi-agent/shared-chat page in full
  `fort serve` mode.
- `/legacy` preserves the Command Deck/board rollback surface.

Desktop web uses a Channels rail with **New Channel**, pinned/recent Channels,
Scheduled, Needs you, and Settings. Selecting Scheduled replaces the transcript
with its chronological list; detail opens in a drawer. Needs you and Settings
remain temporary drawers, not permanent dashboard columns. Below approximately
860px, the Channels rail becomes a keyboard-accessible sheet.

Acceptance viewports are 1280×720 and 390×844, with a compact-phone check at
375×667 with the keyboard present. Interactive targets are at least 44 CSS
pixels. The page must have visible focus, logical tab order, no horizontal
overflow, reduced-motion behavior, and SSE reconnect/rebuild without duplicate
messages.

## Cross-platform design gate

The three mockup treatments must each show one coherent experience across:

1. Web desktop at approximately 1440×900;
2. native macOS at approximately 1240×800; and
3. native iPhone at approximately 393×852 without a device bezel.

All three surfaces use the same sample Channel, exact Primary Agent, message
order, schedule IDs/order, and durable states so the comparison is about
presentation. Each treatment shows both the active Channel state and the
Scheduled destination. The visual language may vary, but every option must
show:

- unmistakable private Channels navigation with pinned and recent Channels;
- one transcript and composer;
- a distinct Scheduled destination showing upcoming, recent, paused, and
  failed durable state;
- full identity available without dominating the conversation;
- the text-only boundary;
- a small Needs-you entry point;
- Settings with readiness and Recheck; and
- truthful Queued, Working, Answered, Failed, and recovery states.

The mockups must not show memory, Act, projects, Today/Week boards, schedule
authoring, playbooks, DAGs, participant chips, metrics, or a public user
directory. macOS and iOS
mockups are labelled future design. They do not claim implementation or parity
until FortKit consumes the canonical Primary Channel and schedule-read
contracts.

The Fort mark retains the existing orbital intelligence-core aesthetic: dense
concentric rings, fine technical arcs, luminous nodes, and a bright central
core. A treatment may recolor or rematerialize that orb to match its surface,
but it must not replace it with new geometry, flatten it into a different
symbol, or make logo color carry runtime state. Alternative presentations may
use the deep-navy/electric-blue, graphite/olive/lime, or daylight/cobalt
language while keeping the product structure identical.

### Latest mockup checkpoint — 2026-08-08

These are the current Phase 1 visual references. They intentionally preserve
one Channel/Scheduled UX and the original Fort orbital-core aesthetic. Only the
surface palette/material and matching orb colors vary. macOS and iOS remain
future-design evidence; these images do not claim implementation or parity.

#### Quiet Intelligence — original blue core

![Quiet Intelligence Phase 1 mockups for Web, macOS, and iOS](assets/044/quiet-intelligence-original-core.png)

#### Private Channels — original lime core

![Private Channels Phase 1 mockups for Web, macOS, and iOS](assets/044/private-channels-original-core.png)

#### Native Daylight — original daylight core

![Native Daylight Phase 1 mockups for Web, macOS, and iOS](assets/044/native-daylight-original-core.png)

One, two, or all three treatments may be approved for Web Phase 1. Approval
must name the included set; the shared product hierarchy, controls, state, and
API behavior may not fork by treatment. Saving these references does not by
itself approve implementation.

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

### `primary_channel`

```sql
CREATE TABLE IF NOT EXISTS primary_channel (
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
nonsecret disclosure snapshot. It is immutable after Channel creation: application
code and database triggers reject update or delete.

Creating a Primary Channel atomically inserts the conversation, exactly one
participant copied from the setting, and the marker. Existing conversations
have no marker and are legacy shared conversations; there is no backfill.

Participant add/remove operations reject a marked Primary Channel. Rename,
archive, and reopen remain allowed. The Phase 1 shell does not expose Delete.

Pin state is intentionally separate from immutable Channel identity:

```sql
CREATE TABLE IF NOT EXISTS primary_channel_pin (
  conversation_id TEXT PRIMARY KEY,
  pinned_at TEXT NOT NULL,
  FOREIGN KEY(conversation_id) REFERENCES primary_channel(conversation_id)
    ON DELETE CASCADE
);
```

Pinning is an idempotent upsert; unpinning deletes only this projection row.
Lists order pinned Channels by `pinned_at DESC`, then unpinned Channels by
durable conversation activity descending with stable ID tie-breaking.

An optional explicit display/provenance link associates an existing durable
schedule with a Channel without changing what the schedule executes:

```sql
CREATE TABLE IF NOT EXISTS schedule_channel_link (
  schedule_id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(schedule_id) REFERENCES schedule(id) ON DELETE CASCADE,
  FOREIGN KEY(conversation_id) REFERENCES primary_channel(conversation_id)
    ON DELETE CASCADE
);
```

Phase 1 does not expose link creation or mutation. When the row exists, the UI
may show **Related Channel** and Open Channel; it must still display the
truthful flow target separately. When absent, the schedule is **System**. Fort
never derives this link from matching names, prompts, flow IDs, or model prose.

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

The policy revision is stable Channel behavior. Selected and observed
adapter revisions and SDK versions describe one execution attempt. Retry may
select a newly approved compatible adapter only when it certifies the exact
same Channel policy revision; it creates a new target and records the new
selection. A changed policy revision fails closed and requires a future
explicit migration or a new Channel.

### Database enforcement against old code

Migrations add SQLite triggers that reject:

- insert of a primary marker unless its participant belongs to the same
  conversation and that conversation has exactly one active participant;
- update or delete of a primary marker;
- insert, update, or delete of a participant in a marked Primary Channel;
- deletion of a marked primary conversation; and
- insertion of a target for a marked Primary Channel unless its authority,
  policy id, policy revision, exact request-policy fields, credential
  reference, organization, and project exactly match the marker, and its
  selected-adapter identity is nonempty.

These triggers are part of the invariant, not merely UI validation. An older
binary omits the new target authority fields and therefore cannot append a
tool-capable turn to a marked Channel. Existing legacy targets remain policy
`unknown`; they are never relabelled text-only.

Target authority, policy, request-policy, credential reference, and selected
adapter fields are immutable after insertion. Observed metadata may make one
validated transition from empty to the terminal receipt in the same
transaction as Answered/Failed/Canceled; it cannot be rewritten later.

Rollback does not deploy an older binary against a database containing Primary
Channels. Use the accepted new binary's full `fort serve` mode, or restore a
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

A Primary Channel dispatch always supplies `chat_text_only_v1`,
`openai-responses-text-v1`, the complete `TextOnlyPolicy`, and the expected
policy revision. Hub-side preflight validates the revision and clears only the
private expected field. Authority, runtime contract, and text-only policy cross
the cluster/remote wire. The text-only node branch accepts only that exact
authority/contract/profile/adapter/policy combination; empty, legacy, unknown,
wrong-contract, and other-provider requests start zero work.

The full `fort serve` composition uses a closed local runtime multiplexer:

- exact text-only authority plus the `openai-responses` agent key can route
  only to `exec/openairesponses`;
- empty legacy authority can route only to the existing native-provider
  runtime; and
- every cross-combination or unknown value fails before work starts.

The same mux policy protects full-mode node execution. Tests prove that no
text-only request can reach a CLI and no legacy/native request can reach the
Responses adapter. The new Primary Channel HTTP/port surface has no generic
execution method and cannot construct an empty-authority request; existing
legacy execution remains separately scoped to its established admin/node
surface.

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

Add narrow `PrimaryChannelPort` and `ScheduleReadPort` contracts. Ordinary
clients never supply participant IDs, seat arrays, or target arrays.

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

Clears the default for future Channels. Existing Channel identity is unchanged.

Reuse `POST /api/conversation-seats/recheck` for an explicit fresh probe, or
add a narrow alias that calls the same bounded port. It never dispatches an
agent turn.

### Channels

```text
GET  /api/channels?state=open|archived|all
POST /api/channels                 { "name": "..." }
GET  /api/channels/{id}
PATCH /api/channels/{id}           { "name": "..." }
PATCH /api/channels/{id}           { "state": "open|archived" }
PATCH /api/channels/{id}           { "pinned": true|false }
POST /api/channels/{id}/turns      { "client_turn_id": "uuid", "text": "..." }
POST /api/channels/{id}/targets/{target_id}/retry
POST /api/channels/{id}/targets/{target_id}/cancel
GET  /api/channels/{id}/events
GET  /api/needs-you
```

`GET /api/channels` defaults to `state=open`; `archived` and `all` are the only
other accepted values. Every result emits `[]`, never `null`, with pinned
Channels first and the remaining Channels newest-first. This is the path for
reopening an archived Channel. `GET /api/channels/{id}` returns the canonical
conversation detail plus the complete `primary_identity` snapshot.

Turn creation resolves the marked Channel's sole participant server-side. A
client cannot select another target. `202 Accepted` means the prompt, frozen
context boundary, target, and run identity are durable; it does not mean the
provider succeeded.

Retry/cancel validate that the nested Channel and target match. Missing or foreign
IDs are `404`; valid IDs in the wrong current state are `409`.

`GET /api/needs-you` returns the projection defined above with non-null arrays.
It must not create, mutate, probe, or dispatch anything.

### Scheduled

```go
type ScheduleReadPort interface {
    List(context.Context, ScheduleFilter) (ScheduleList, error)
    Get(context.Context, string) (ScheduleDetail, error)
    Occurrences(context.Context, string, OccurrencePage) ([]scheduler.Occurrence, error)
}
```

```text
GET /api/schedules?state=active|paused|all
GET /api/schedules/{id}
GET /api/schedules/{id}/occurrences?limit=50&before=<RFC3339>&before_id=<id>
```

The list defaults to `state=all` and returns one read-transaction snapshot as
`{"snapshot_id":"schedule-snapshot:v1:...","observed_at":"...","items":[]}`
with non-null items. Phase 1 deliberately does not paginate this personal
catalog because `next_fire_at` and `updated_at` change while the scheduler is
running and cannot form a stable cross-page cursor. The snapshot has a hard
1,000-definition bound; exceeding it returns `schedule_catalog_limit` without
silently truncating. The snapshot ID is a versioned digest of the normalized
returned rows, not a mutable database counter.

The canonical ordering within that snapshot uses three closed buckets: active
definitions with a next fire first, ordered by `next_fire_at ASC, id ASC`;
active definitions without a next fire second, ordered by `updated_at DESC, id
ASC`; paused definitions third with the same updated-time order. Clients render
the server order and retain `snapshot_id` when comparing cross-surface state.

A list item contains definition ID/title, enabled state, kind, expression plus
human-readable recurrence, stored IANA timezone, exact next/last fire instants,
`target_kind:"flow"`, `target_id:<flow_id>`, optional explicitly linked
Channel ID/name, latest occurrence/error/run ID, scheduler ownership state,
and `observed_at`.

Detail includes the same definition plus exactly two non-null bounded
projections at the same observed time: up to 10 upcoming occurrences ordered
`scheduled_for ASC, id ASC`, and up to 10 recent occurrences ordered
`scheduled_for DESC, id DESC`. The full occurrence endpoint is newest-first,
uses a bounded `1..50` limit, and paginates by the exclusive tuple
`(scheduled_for,before_id)` so equal instants cannot duplicate or skip rows.

All three GETs are pure projections: zero writes, timer registration, probes,
model calls, or dispatches. Missing schedules are `404`; invalid filters or
cursors are `400`. The existing `POST /api/schedules` remains available only
on the legacy/admin surface and is not mounted or linked by the new shell.

Closed error codes added by Phase 1 are:

- `primary_agent_not_configured`;
- `primary_agent_unready`;
- `primary_agent_drift`;
- `chat_policy_unavailable`;
- `chat_authority_violation`;
- `primary_channel_invariant`;
- `provider_result_unknown`;
- `provider_incomplete`;
- `provider_refusal`;
- `provider_failed`;
- `schedule_catalog_limit`;
- `schedule_inventory_unaccepted`; and
- `schedule_inventory_drift`.

A confirmed canceled request uses the existing Canceled target state rather
than an error code.

Retain `seat_unready`, `conversation_context_limit`, and existing bounded
target errors.

## `fort serve` composition

Phase 1 does not add or promote a standalone `fort chat` service. A chat-only
process that deliberately omits scheduler/flow ownership could display
persisted schedules while leaving them inactive, which would be materially
misleading. Reintroducing those dependencies would merely recreate `serve`
under another command.

The accepted full `fort serve` composition therefore owns:

- the existing durable scheduler, flow execution, and at-most-once occurrence
  claim path unchanged;
- the new schedule-read projection and explicit scheduler ownership state;
- the OpenAI Responses/cluster/remote runtime with the text-only authority gate
  and closed local/node runtime mux;
- bounded capability/readiness and exact remote-seat transport;
- Primary Agent and Primary Channel services; and
- the new narrow Channels/Scheduled shell at `/`.

Legacy mutating schedule, flow, project, and board APIs remain reachable only
through their existing admin/legacy surfaces. `/shared` and `/legacy` remain
rollback surfaces. The new shell does not link or call them. Local web is the
only Phase 1 shipping surface; relay/gateway publication remains deferred.

`FORT_PRIMARY_CHANNELS` is a closed startup mode:

- `off` (default) — `/` retains the current shared surface;
  `/channels-preview`, all `/api/channels*` routes, and the new Primary Agent
  setting/Scheduled-read routes are not mounted and return `404`; no stale
  Phase 1 client can create or dispatch work;
- `preview` — `/` retains the current shared surface, the new shell is mounted
  at `/channels-preview`, and its narrow Phase 1 APIs are enabled; and
- `primary` — the new shell is mounted at `/`, `/channels-preview` redirects to
  `/`, and the same narrow APIs are enabled.

Unknown or empty nondefault values fail startup rather than choosing a mode.
Promotion changes only this same-binary mode; it never changes scheduler
ownership or service command.

Scheduler ownership is **Active** only when the same accepted process has
successfully started the durable scheduler. If ownership is inactive or
unknown, Scheduled says so and never presents future occurrences as assured.

Schedule visibility does not reclassify legacy flow authority. Existing flows
may use broader runtimes than Primary Channel text-only turns; each row must say
`target_kind: flow`, show the exact flow ID and observed run identity, and never
inherit the Channel's text-only badge. Before the Phase 1 trial, every enabled
legacy definition is inventoried. Fort computes a
`schedule-inventory:v1` digest over each enabled schedule's normalized ID,
kind, expression, timezone, flow ID, and the canonical loaded flow-definition
digest. The digest format is
`schedule-inventory:v1:<lowercase-hex-sha256>`, where SHA-256 covers the exact
UTF-8 bytes `schedule-inventory:v1\n` followed by the canonical JSON array and
one final newline. The canonical array is sorted by schedule ID and contains no
secrets or observation timestamps. The empty-inventory digest is therefore
`schedule-inventory:v1:7d5bf4173fd97e9d036d7acd974925bbc4b2ed0553c0c8e9e9ed210d9cea7b76`,
the digest of `schedule-inventory:v1\n[]\n`.

Preview exposes the current digest and inventory rows for review. Toby records
the exact digest and acceptance time in the approval record, then configures
the same nonsecret value as `FORT_ACCEPTED_SCHEDULE_INVENTORY`. `primary`
startup requires this input and compares it byte-for-byte with the freshly
computed digest before mounting the new shell or Phase 1 APIs. A missing input
fails startup with `schedule_inventory_unaccepted`; a mismatch, missing flow
digest, or later changed definition/flow fails startup with
`schedule_inventory_drift`. Preview may start with either condition but must
show current digest, accepted digest or **Not accepted**, and the closed warning;
it may not describe the trial as active. `off` does not require or evaluate the
accepted digest. Phase 1 provides no disable path, authority upgrade, or other
schedule mutation; any operational change requires separate authorization
outside this spec.

## TDD delivery sequence

Every step begins with a focused failing test, observes the failure, then adds
the minimum code to pass. Keep `go test ./...` green after each slice and run
`-race` for concurrency changes.

1. **Baseline checkpoint** — commit the verified Spec 041/042 work and Spec
   043 direction before any Phase 1 code. Exclude generated Apple artifacts.
2. **Domain and store** — setting round-trip/upsert; atomic Channel,
   participant, and marker creation; marker immutability; pin projection;
   pinned/newest-first filtering; two Channels with the same seat retain
   byte-disjoint context; setting changes leave prior Channel identity
   byte-for-byte unchanged; legacy rows are untouched.
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
5. **Control service** — configure the setting; create one-participant Channel;
   server-selected target; synchronously persist exact selected adapter and
   `RunSpec` policy before `202`; offline/drift causes zero starts; observed
   metadata commits atomically with the terminal state; same-seat retry retains
   identity, context, and authority; latest failure reducer is deterministic.
6. **Schedule read projection** — all active, paused, non-today, upcoming, and
   recent schedules/occurrences; UTC persistence with configured-zone display;
   exact flow target and ownership state; GETs cause zero writes, timer
   registration, probes, runtime calls, or model calls.
7. **HTTP** — exact Channel and Schedule methods, bodies, filters, pagination,
   statuses, codes, nested-ID validation, and non-null arrays; GET and Settings
   operations invoke no runtime.
8. **Web shell** — root contains only the six approved product concepts;
   Channel create/send single-flight and client-turn idempotency; Scheduled
   navigation/detail; reload/SSE rebuild; Needs-you deep link; keyboard, focus,
   reduced-motion, and responsive tests. Preserve `/shared` and `/legacy`.
9. **Composition** — full `fort serve` owns the proven scheduler and new read
   projection; only text-only authority reaches the Responses runtime;
   full-mode local/node mux has no cross-routing; direct mesh and capabilities
   remain functional; new UI invokes no legacy mutating APIs.
10. **Concurrency and canaries** — race tests, local/remote fake turns, daemon
   restart, remote offline without reroute, exact retry, old-node failure,
   request/metadata capture, typed terminal-shape validation, and live prompts
   that request a tool/file action but receive no such capability or false
   completion claim.
11. **Visual/live acceptance** — only after explicit authorization and design
    selection.

## Expected file boundary

New files are expected in these bounded areas:

```text
specs/044-private-primary-channels-phase-1.md
core/conversation/primary.go
core/conversation/primary_test.go
core/store/primary_channel.go
core/store/primary_channel_test.go
control/primary_channel.go
control/primary_channel_test.go
control/schedule_read.go
control/schedule_read_test.go
ui/primary_channel.go
ui/primary_channel_api_test.go
ui/primary_channel_page.go
ui/primary_channel_page_test.go
ui/schedule_read.go
ui/schedule_read_test.go
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
- `core/store/schedules.go`, `control/conversations.go`, and existing scheduler
  status wiring only for bounded read projection and Primary Channel
  enforcement; schedule claiming/execution semantics remain unchanged;
- `ui/ports.go` and `ui/server.go`;
- `cmd/fort/main.go`, `wire.go`, `capabilities.go`, `service.go`, and focused
  mesh/service tests; and
- README/governing-spec references only after acceptance.

`ui/apple/**` is excluded from Phase 1 production implementation. Mockups are
design evidence, not Swift code authorization. Scope expansion requires a spec
amendment before implementation.

## Acceptance

### Design gate

- Toby selects or approves the shared cross-platform experience and names the
  Web treatment set to implement.
- Every approved treatment's first-run, active-Channel, Scheduled,
  failure/Needs-you, Settings, and compact layouts match this contract.
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

- configure a ready Primary Agent, create a Channel, send a turn, and receive one
  attributed answer;
- create two Channels with the same Primary Agent and prove that neither frozen
  prompt contains the other Channel's messages or IDs;
- reload and restart with no lost, duplicated, or reordered messages;
- pin/unpin, rename, archive/reopen, and prove ordering and Channel identity
  remain deterministic;
- change Settings and prove the existing Channel identity is unchanged;
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
- full-mode runtime mux tests prove authority/provider cross-routing starts zero
  work;
- Scheduled contains every durable definition, including paused and non-today
  rows in one bounded `schedule-snapshot:v1`, with exact UTC instants rendered
  in the configured IANA timezone;
- schedule GETs produce zero writes, registrations, provider/runtime calls, or
  model calls, and never invent a Channel binding;
- active scheduler ownership and occurrence-to-run evidence survive reload and
  restart without duplicate occurrence claims; and
- `primary` promotion accepts only the reviewed `schedule-inventory:v1` digest;
  a new/changed enabled definition or flow digest produces visible inventory
  drift and invalidates the trial without mutating the schedule;
- `off` mounts no preview or Phase 1 routes, `preview` exposes them only at the
  preview shell, and `primary` promotes the same shell without changing
  scheduler ownership;
- the 65,536-byte context overflow fails before new turn/target persistence and
  provider dispatch.

### Two-machine, separately authorized

- both machines run the same accepted text-only protocol/build;
- one remote Primary Agent answers in one exact Channel with exact attribution;
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

No destructive migration is permitted. The launchd command remains `fort
serve`; Phase 1 never switches scheduler ownership to another composition.
Before promotion the new shell is exercised in `preview` mode. Same-binary
rollback sets `FORT_PRIMARY_CHANNELS=off` and restarts `serve`, which removes
the preview and every Phase 1 route while preserving scheduler ownership;
`/shared` and `/legacy` remain available throughout.

Do not run a pre-Phase-1 binary against a database containing Primary Channels.
If binary rollback is necessary, stop all writers and restore the coordinated
pre-Phase-1 database snapshot together with the prior binary. The database
triggers deliberately make unsupported old writes fail, but snapshot restore
is the only rollback that removes new policy semantics completely.

`/shared` and `/legacy` are not deleted. Historical runs, projects, schedules,
and playbooks are not changed. The additive Primary Agent, Primary Channel,
and pin rows may remain unused without a data rewrite.

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

Record the implementation decision here. Deployment-specific blanks must be
completed before a live provider request, `primary` promotion, or trial start;
they do not authorize Fort to infer credentials, billing, retention, or spend
policy.

```text
Approved Web visual treatment set: Quiet Intelligence, Private Channels, Native Daylight / 2026-08-08
Spec 044 approved by Toby: 2026-08-08
Initial exact OpenAI profile/model: openai-responses:gpt-5.6-sol / gpt-5.6-sol
Accepted policy / adapter / SDK: openai-responses-text-v1 / implementation must pin exact adapter and SDK
Reasoning effort / max output tokens: medium / 4096
Credential ref / organization / project: deployment configuration required
Manual trial spend cap / billing source: pending / unknown
Billing provenance: unknown
Provider retention mode / source / date: unknown / user accepted unknown for implementation only / 2026-08-08
Accepted schedule-inventory:v1 digest / date: pending preview inventory
Implementation start commit: pending visual checkpoint commit
```
