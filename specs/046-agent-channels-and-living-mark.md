# Spec 046 — Agent Channels and the Living Fort Mark

**Status:** implemented and production-deployed behind the closed
`FORT_AGENT_CHANNELS` gate on 2026-08-21; provider enablement remains
unauthorized
**Date:** 2026-08-20
**Decision owner:** Toby
**Depends on:** Specs 041 and 042 for durable conversations, immutable exact
seats, target lifecycle, exact-model attribution, readiness, and retry; Spec 044
for the currently shipped private-chat authority boundary; Spec 045 for native
session continuity and the living-mark decision
**Shipping surfaces:** local Web, native macOS, and native iPhone

## Goal

Make Fort a chat service whose primary destinations are agents.

The top-level **Channels** in Fort represent distinct configured agents such as
OpenClaw, Codex, Claude, or Hermes. A Channel is not a transcript. Each agent
Channel owns one or more durable conversations, and a person may pin important
conversations beneath that agent for direct access.

Keep the Fort product mark visibly alive throughout every foreground app
surface. Its ambient motion is continuous brand presence, while stronger motion
may reflect truthful durable Working state. The product mark is not an agent
avatar and motion is never the only status signal.

## Executive decision

Fort's primary product model becomes:

```text
Fort
└── Channels (agents)
    ├── OpenClaw
    │   ├── current conversation
    │   ├── Product direction (pinned conversation)
    │   └── Sunday workflow (pinned conversation)
    ├── Codex
    │   └── current conversation
    └── Claude
        └── current conversation
```

Ordinary use is human-to-agent chat. The user chooses an agent Channel and
continues or starts a conversation with that exact agent. Fort owns the durable
conversation record, identity and authority snapshots, dispatch truth,
recovery, and cross-device presentation. The selected agent harness owns only
the reasoning and capabilities explicitly authorized by its accepted adapter.

The default product does not ask for one global Primary Agent and does not
present every conversation as a peer Channel. It also does not become an
autonomous agent-to-agent network, a public messaging service, or a workflow
dashboard.

## Canonical product language

| Term | Canonical meaning | It never means |
| --- | --- | --- |
| **Agent** | A named, provider-backed agent or harness, such as an enrolled OpenClaw agent | A model alias, transient process, or conversation |
| **Agent seat** | The exact dispatch identity: Fort profile, agent/harness, requested provider/model identity, enrolled computer, and accepted authority/policy revision | A display name or a silently replaceable capability |
| **Channel** or **Agent Channel** | A stable top-level Fort destination bound to exactly one immutable agent seat | A transcript, topic, Project, provider session, or public room |
| **Conversation** | One durable canonical transcript and context boundary owned by exactly one Agent Channel | A Channel, agent identity, or provider-native session |
| **Pinned conversation** | A navigation shortcut to one conversation beneath its owning Agent Channel | A copied transcript, new agent, shared context, or identity change |
| **Fort mark** | Fort's orbital-core product identity in app chrome and product states | The default avatar or status light for OpenClaw or another agent |

User-facing copy may shorten **Agent Channel** to **Channel** once the hierarchy
is clear. Code, storage, APIs, tests, and diagnostics use the unambiguous
`agent_channel` and `conversation` terms. New work must not use “conversation
channel.”

## Relationship to existing specs

Upon approval, this spec supersedes Specs 043 and 044 only where they:

- define `channel_id == conversation.id`;
- make one singleton Primary Agent the default product identity; or
- expose conversations directly as the top-level Channels list.

It supersedes Spec 041 only for the default multi-participant/Everyone
experience and Spec 040 only for default shell/navigation. It retains the
durable conversation/message record, atomic idempotent turns, immutable exact
seat snapshots, exact-model attribution, pre-dispatch readiness revalidation,
fail-closed no-reroute behavior, target lifecycle/retry/cancel semantics,
cross-machine execution, append-only evidence, and architecture seams from
Specs 040–044.

Spec 045 remains governing for session renewal and living-mark accessibility.
If approved, this spec would supersede only Decision 4's use of Channel as a
conversation: authenticated startup opens the last-used available Agent Channel
and then its last-used open Conversation. It falls back to the Agent Channel
list when that selection is absent or unavailable. Decisions 1–3 remain
unchanged. Decision 5 is retained and broadened below across Web, macOS, and
iPhone.

While `FORT_AGENT_CHANNELS=off`, Spec 044 remains the shipping product contract.
Existing legacy/admin and rollback contracts remain intact until an accepted
migration removes them.

## Product contract

### Agent Channels are the primary destinations

Every open Agent Channel appears in the top-level Channels navigation even when
it has no conversation. Each binds one complete immutable agent seat and one
accepted authority/policy snapshot. Several Channels may use the same harness
only when each has an explicit identity, for example **OpenClaw — Personal** and
**OpenClaw — Church**.

An Agent Channel has its own stable logical ID, distinct from both its opaque
seat ID and every Conversation ID. Readiness may drift and presentation may
change without changing that logical identity or silently creating a new
Channel.

The Channel's display name, agent-supplied identity image, and archived state
are presentation fields. They may change without changing dispatch. Its agent
seat, provider/model selector, computer, authority, and policy revision do not
change in place in the first release. A different binding requires a new Agent
Channel and an explicit conversation migration contract; Fort never relabels a
replacement as the same agent.

Selecting an Agent Channel opens its last-used open conversation on that
device. If none exists, Fort shows a ready new-conversation state; the first
Send atomically creates the conversation and durable turn. **New conversation**
always starts a separate transcript/context boundary under the selected Agent
Channel and never asks the user to choose the agent again.

The Channel header makes the human-facing agent name primary and keeps the full
seat, requested/resolved model precision, computer, harness/adapter revision,
authority, and current readiness inspectable. Unknown runtime identity remains
**unknown**. Fort never upgrades an alias into a stronger identity claim.

### Conversations belong to one agent

Each new ordinary conversation has exactly one active participant copied from
its parent Agent Channel. The client does not send a participant, provider,
model, or machine on a turn. The server resolves the parent Channel and exact
conversation participant before it commits the target.

A conversation may be renamed, archived/reopened, and pinned/unpinned. Pinning
adds only a shortcut under the parent agent. It does not:

- change the Channel or participant;
- merge or duplicate messages;
- share context with another conversation;
- resume a provider-native thread unless the Channel's accepted adapter
  explicitly declares that behavior; or
- change ordering timestamps that represent real conversation activity.

Pinned conversations are ordered by `pinned_at DESC` with a stable ID
tie-breaker. Remaining conversations are newest-durable-activity first. On wide
layouts they appear nested under the owning agent. On compact layouts they may
appear in an agent-scoped sheet, but every row must display its owning agent and
must not create a flat ambiguous mixture of agents and transcripts.

Conversation search and recent history, when present, remain scoped or clearly
attributed to an Agent Channel. A search result always includes both agent and
conversation identity before it can be opened.

### Context and provider memory remain truthful

The canonical Fort context boundary is the conversation. Fort supplies no
messages from another conversation unless a later approved memory/source-link
contract explicitly authorizes them. Two conversations beneath the same Agent
Channel therefore retain disjoint Fort transcripts and compiled prompt inputs.

An agent harness may have provider-managed sessions or memory outside Fort.
Every eligible Channel adapter must declare that behavior as an inspectable
contract such as `ephemeral` or `agent_managed`; Fort must not claim transcript
isolation from state the harness can independently retain. Provider-managed
memory is not Fort canonical memory and cannot rewrite Fort history.

### Readiness and authority fail closed

Creating an Agent Channel accepts one opaque, currently eligible agent option.
The server resolves and stores the complete identity and authority; a client
cannot construct a seat from independent profile, model, machine, or policy
fields.

Readiness is live and may change without changing identity. An unavailable,
offline, drifted, or policy-ineligible Agent Channel remains visible with a
bounded explanation and **Recheck**. Sending fails before provider start. Fort
does not substitute another agent, model, provider, computer, adapter, or
authority. Retry retains the same conversation participant and target identity.

OpenClaw is a required example of the product model, not a declaration that the
current OpenClaw runtime is eligible. It may become a selectable Agent Channel
only after a separately approved adapter contract proves its exact command,
process lifecycle, identity, readiness, session/memory behavior, authority, and
terminal-event normalization. This spec does not bypass the current
`command_contract_changed` fail-closed result.

This spec changes organization, not execution authority. The first release may
surface only agents whose already approved authority contracts are eligible.
It does not make tool-capable chat, external actions, MCP, file access, browser
control, or autonomous delegation implicitly safe or authorized.

### Chat remains the product center

The authenticated root opens the chat service. The primary shell contains:

1. **Channels** — the person's enrolled Agent Channels;
2. **Conversation** — the selected agent's current durable transcript;
3. **Pinned and recent conversations** — always attributed to their agent;
4. **Composer** — one input and Send action for the selected agent;
5. **Needs you** — only current, durable, actionable chat failures or approvals;
   and
6. **Settings** — agent enrollment, identity, readiness, authority, and app
   preferences.

Existing schedules may remain available as a secondary read surface with exact
agent/conversation attribution. Projects, boards, playbooks, DAGs, routers,
planning controls, metrics, raw runs, and machine pickers do not return to the
ordinary chat shell.

### Status and recovery are durable

Queued, Working, Answered, Failed, and Canceled retain the event-derived
meanings in Specs 041, 042, and 044. A network request, optimistic Send,
loading placeholder, stale process, or provider claim does not by itself make
an agent Working or an answer complete.

A current Queued or Working target offers Cancel. A failed target offers only
the recovery actions permitted by its typed error. Retry is same-agent and
same-seat. An explicit request to use a different Agent Channel creates a new
target/conversation operation with visible provenance; it is never called a
retry.

## Living Fort mark

### Product identity, not agent identity

The canonical orbital core is Fort's product mark. It appears in persistent app
chrome, setup, empty, reconnecting, and product-level status surfaces. An Agent
Channel uses that agent's stable supplied identity asset or a deterministic
Fort-generated fallback bearing the agent name. Fort does not imply that an
OpenClaw, Codex, Claude, or Hermes message was authored by Fort merely because
the Fort mark is shown nearby.

OS-controlled app icons, favicons, notification icons, app-switcher snapshots,
and background processes are outside the animation contract.

### Ambient and Working motion

Every visible in-app Fort mark has restrained continuous ambient motion while
its scene, window, or browser tab is foreground-visible and Reduce Motion is
off. This includes setup, empty, idle, Queued, Answered, Failed, Canceled,
disconnected, and recovery states. The loop has no intentional static phase or
restart jump and does not move layout, hit targets, focus, or scroll position.

A mark scoped to one agent or conversation becomes distinctly more energetic
only while durable current activity says that exact agent/conversation is
Working. Working energy differs through rate, amplitude, or an additional
orbital layer, not color alone. A global Fort mark stays ambient unless adjacent
text truthfully discloses aggregated activity such as **2 agents working**.

Motion is brand presence, not a spinner and not the sole state indicator. Agent
name and state remain visible as semantic text.

### Accessibility, lifecycle, and power

The system Reduce Motion preference is honored immediately without relaunch.
Translation, rotation, scale, parallax, and orbiting arcs stop. A slow
non-spatial glow/luminance pulse of at least four seconds remains; Working may
use a stronger non-spatial glow. No state flashes or strobes.

Animation scheduling pauses while the mark is offscreen, the scene is inactive
or backgrounded, a macOS window is minimized or fully occluded, or a browser
tab is hidden. It resumes from current truthful state without catch-up or a
large phase jump. Low Power Mode or thermal pressure may reduce cadence, blur,
shadow, and layers, but a foreground-visible mark remains perceptibly alive.
Repeated decorative marks share one surface animation clock rather than each
starting an unbounded timer.

A mark beside visible **Fort** or agent text is decorative and hidden from the
accessibility tree. A standalone product mark is named simply **Fort**. Agent
identity and status are semantic text; state transitions may announce once in
the existing polite status region, while animation frames never announce.

## Persistence and migration contract

Migration is additive and restart-safe. It does not rename, rewrite, or delete
existing `conversation`, `conversation_participant`, `conversation_turn`,
`conversation_target`, `conversation_message`, `primary_channel`,
`primary_channel_pin`, or `primary_agent_setting` rows.

The implementation adds durable equivalents of:

```text
agent_channel
  id, name, state,
  complete immutable seat fields,
  complete authority/policy fields,
  created_at

agent_channel_conversation
  agent_channel_id, conversation_id, created_at

agent_conversation_pin
  conversation_id, pinned_at
```

Exact columns must reuse the normalized identity and authority precision already
accepted for Primary Channels rather than storing only a display alias. Foreign
keys, uniqueness constraints, database triggers, and service validation enforce
that every new ordinary conversation belongs to one Agent Channel, has exactly
one matching active participant, and cannot be moved to a different agent by
editing a link.

Existing marked Primary Channels migrate as Conversations beneath Agent
Channels derived only from their complete persisted seat and authority/policy
identity. Rows may be grouped under one Agent Channel only when those canonical
identities are byte-equal. Display names, provider aliases, and current
readiness are never grouping keys. Existing `primary_channel_pin` rows project
as pinned Conversations beneath the mapped agent when migration first creates
the ownership link. That projection is not replayed after an Agent-side unpin;
the immutable legacy row remains evidence rather than overriding newer Agent
navigation state. No transcript or context is merged.

Multi-participant legacy conversations remain legacy/unassigned until an
explicit future user operation gives them a supported destination. Fort never
auto-splits them. Schedule-to-conversation links retain their exact conversation
and become attributable to its mapped Agent Channel without changing the
schedule execution target.

The migration produces a deterministic preview with counts and old-to-new IDs
before product cutover. Re-running it produces the same links and no duplicate
Agent Channels. Any incomplete or ambiguous persisted identity fails migration
for that row and leaves it available through the legacy/admin path. Concurrent
processes revalidate inside the write transaction: a second process accepts an
already-converged exact mapping but fails closed on an identity or ownership
conflict, even when its published preview became stale before apply.

## HTTP and port contract

The existing `/api/channels` contract means Primary-Channel conversations and
must not silently change response shape or semantics. Introduce a narrow new
contract for the agent-first product and migrate clients explicitly:

```text
GET  /api/agent-options
POST /api/agent-options/recheck
GET  /api/agent-needs-you

GET  /api/agent-channels?state=open|archived|all
POST /api/agent-channels
GET  /api/agent-channels/{channel_id}
PATCH /api/agent-channels/{channel_id}
POST /api/agent-channels/{channel_id}/turns

GET  /api/agent-channels/{channel_id}/conversations?state=open|archived|all
POST /api/agent-channels/{channel_id}/conversations
GET  /api/agent-channels/{channel_id}/conversations/{conversation_id}
PATCH /api/agent-channels/{channel_id}/conversations/{conversation_id}
POST /api/agent-channels/{channel_id}/conversations/{conversation_id}/turns
POST /api/agent-channels/{channel_id}/conversations/{conversation_id}/targets/{target_id}/retry
POST /api/agent-channels/{channel_id}/conversations/{conversation_id}/targets/{target_id}/cancel
GET  /api/agent-channels/{channel_id}/conversations/{conversation_id}/events
```

`POST /api/agent-channels` accepts an opaque `agent_option_id` plus a display
name. `POST .../conversations` accepts only conversation presentation fields.
`POST /api/agent-channels/{channel_id}/turns` accepts a conversation name,
client-turn UUID, and text and atomically creates the first Conversation plus
its first durable turn. Nested Conversation turn creation accepts a client-turn
UUID and text. Clients cannot submit seat, participant, provider, model,
machine, adapter, or policy fields on those paths.

`GET` operations and Recheck invoke no generation model. Recheck uses only the
accepted bounded readiness probes and never installs, authenticates, changes
models, starts a provider session, or dispatches a turn. Only a durable
conversation target may invoke `runtime.Runtime`.

Nested routes validate both parent Channel and Conversation. A foreign child is
`404`; a valid resource in the wrong state is `409`. Every list returns `[]`,
never `null`, with deterministic ordering and stable tie-breakers. SSE rebuilds
from persisted conversation state after interruption and is not the source of
truth.

## Architecture and implementation boundary

- `core/conversation` owns pure Agent Channel identity, ownership, pin, and
  validation types while retaining the existing conversation kernel.
- `core/store` owns additive schema, idempotent migration, immutable identity
  enforcement, deterministic list ordering, and atomic conversation/first-turn
  creation.
- `control` resolves eligible agent options, revalidates the exact parent
  Channel before dispatch, and adapts the existing Runtime without importing a
  concrete executor.
- `ui` depends only on bounded Agent Channel and Conversation ports. It does not
  import engine, graph, router, native execution, or provider packages.
- `cmd/fort` remains the composition root. Existing providers continue through
  bounded Fort-owned adapters.
- Web, iOS, and macOS consume the same canonical nested contract. A redirect,
  loopback Web wrapper, or legacy `/api/chat` call is not native parity.
- Routing, grouping, ordering, startup fallback, pinning, migration, and
  readiness selection make zero model calls.

Likely implementation areas are limited to:

```text
core/conversation/agent_channel*.go
core/store/agent_channel*.go
control/agent_channel*.go
ui/agent_channel*.go
ui/apple/FortKit/Sources/FortKit/PrimaryChannels*.swift
ui/apple/FortKit/Tests/FortKitTests/*
ui/apple/iOS/*
ui/apple/macOS/*
cmd/fort/* focused composition
```

Provider packages change only under a separately approved adapter contract.
Spec 045's gateway/session files are not part of this product-model migration.

## Delivery sequence

Every implementation slice begins with a focused failing test, observes the
failure, and adds the minimum code to pass. Keep the whole Go suite green after
each slice and run the race detector for concurrent work.

1. **Baseline and approval** — preserve the existing dirty worktree; approve
   this product model and record the accepted UI treatment before code changes.
2. **Domain and store** — add Agent Channel identity, ownership, pin, ordering,
   and immutability tests; then the additive schema and implementation.
3. **Migration** — preview and idempotently map existing exact Primary Channel
   identities and pins without rewriting legacy rows.
4. **Control and API** — add eligible-option resolution, nested ownership
   validation, atomic new-conversation/first-turn behavior, same-seat retry, and
   the new port/API contract.
5. **Web shell** — make agents the Channels rail, nest pinned/recent
   conversations, remove the singleton Primary Agent flow from ordinary use,
   and keep chat as the authenticated root.
6. **Native parity** — adopt the same contract in FortKit, macOS, and iPhone;
   update startup restoration from conversation-first to agent-then-conversation.
7. **Living mark** — separate product-mark and agent-avatar semantics; add
   ambient/Working/reduced-motion plus visibility and power lifecycle behavior
   through a shared semantic state model.
8. **Functional acceptance** — prove restart, migration, exact identity,
   unavailable-agent failure, pin navigation, animation/accessibility, and
   cross-surface parity before any promotion or release.

## Acceptance criteria

### Automated

- A Channel ID is distinct from its seat ID and every child Conversation ID.
- Two conversations may belong to one Agent Channel; each retains a disjoint
  canonical transcript and Fort-compiled context.
- A new conversation copies the parent's complete immutable seat and authority;
  a client cannot override any dispatch field.
- First Send in an Agent Channel atomically creates either both the new
  Conversation and its first durable turn or neither of them.
- Changing names, pin state, navigation selection, or current readiness leaves
  identity and provider input byte-for-byte unchanged.
- Pin/unpin is idempotent, preserves transcript/activity time, and appears only
  beneath the correct owning agent.
- A foreign Channel/Conversation pair cannot read, mutate, stream, retry, or
  cancel the conversation.
- An unavailable or drifted agent starts zero provider processes and never
  reroutes; Retry retains the same exact seat and policy.
- Migration groups only byte-equal complete identities, maps existing pins,
  preserves every old row, skips ambiguous legacy conversations, and is
  idempotent under restart/concurrency.
- `/api/channels` retains its old contract while the new API is introduced; no
  client receives a response whose meaning changed under the same path.
- Agent listing, pinning, ordering, migration, startup restoration, Recheck,
  and all GETs invoke zero generation models.
- Fake eligible Codex, Claude, Hermes, and OpenClaw options can each create a
  correctly attributed Agent Channel; real options remain ineligible unless
  their accepted adapter proves readiness.
- Idle frames of a foreground-visible Fort mark differ over time; Working
  energy is measurably stronger than ambient.
- Queued, loading, optimistic Send, failed, gated, and offline state select
  ambient rather than Working motion.
- Under Reduce Motion every spatial transform is identity while a slow glow
  changes; hidden/background/offscreen state produces no animation ticks.
- Resume has no catch-up jump; Web, iOS, and macOS use the same semantic motion
  matrix through an injectable clock rather than sleep-based tests.
- Accessibility exposes agent/product identity and textual status without
  duplicate mark announcements or frame-driven announcements.
- Architecture tests retain the existing package seams and deterministic
  routing invariant.

Required verification for an implementation includes:

```text
go test ./...
go test -race ./core/conversation ./core/store ./control ./ui ./ui/apple
go vet ./...
git diff --check
(cd ui/apple/FortKit && swift run FortKitContractChecks)
```

### Product and live acceptance

- The Channels rail shows agents, not conversation titles.
- Selecting **OpenClaw** or another eligible agent opens that exact agent and
  never an identically named substitute.
- A person can start two conversations with one agent, pin one, reload/restart,
  and return to the same agent, conversation, message order, and pin state.
- Desktop and compact layouts make the agent-to-conversation hierarchy obvious
  without adding a per-message agent/model/machine picker.
- The exact agent, model precision, computer, adapter, authority, readiness, and
  provider-managed memory/session behavior are inspectable.
- Taking an agent's computer offline produces a bounded unavailable state and no
  reroute. Recheck plus Retry uses the same identity after readiness returns.
- Web, macOS, and iPhone show the same canonical Agent Channels,
  Conversations, messages, target states, and pins.
- The visible Fort product mark remains calmly alive in every foreground
  non-Working state, becomes more energetic only for truthful Working, honors
  Reduce Motion immediately, and stops consuming animation work when hidden.
- Agent messages use agent identity rather than the Fort product mark.
- Ordinary authenticated startup goes directly to the last-used available
  Agent Channel and its last-used open Conversation, or to the Agent Channel
  list when no valid selection exists.

## Non-goals

- enabling or repairing OpenClaw, Hermes, Claude, Codex, or any other provider;
- changing the accepted text-only authority or authorizing tool-capable actions;
- autonomous agent-to-agent conversations, recursive delegation, Everyone, or
  multi-agent group chat;
- shared context or memory across conversations;
- provider-native thread/session migration;
- public channels, discovery, social profiles, or external participants;
- Projects, folders, boards, playbook/DAG authoring, planner/solver UI, or raw
  orchestration dashboards;
- schedule creation/mutation or a new scheduling execution contract;
- watchOS, complications, CarPlay, voice, or release/distribution work; and
- deleting legacy/admin data or APIs during the first migration.

## Rollback

The first implementation is feature-gated and uses the same accepted binary.
Disabling Agent Channels removes the new shell and `/api/agent-channels*`
routes, restores the existing Primary Channels shell and `/api/channels`
contract, and does not change scheduler ownership or provider configuration.

The closed implementation flag is `FORT_AGENT_CHANNELS=off|primary`, defaulting
to `off`. `primary` is valid only while `FORT_PRIMARY_CHANNELS` remains
`preview` or `primary`; `preview` is the non-promoting compatibility companion.
This prerequisite keeps the existing Primary shell and API live as the exact
one-flag rollback target. A launchd install or restart preserves both explicit
values and never invents either one when absent.

Rollback does not delete Agent Channel, ownership, or pin rows and never rewrites
conversation history. Newly created conversations remain durable even when the
old shell cannot present them; the legacy/admin API may expose them only under
its existing truthful semantics. Re-enabling the feature reconstructs the same
hierarchy from persisted links.

A prior binary may be restored only after proving it will not misinterpret or
mutate conversations created under the new contract. Otherwise use the new
binary's feature-off mode. Destructive database rollback requires a coordinated
pre-migration snapshot and separate authorization.

## Release-candidate implementation record

The approved first release is implemented behind
`FORT_AGENT_CHANNELS=primary`. No provider was enabled. The implementation
includes the additive Agent Channel domain and migration, nested HTTP/control
contracts, agent-first Web and native Apple shells, restart-stable pins and
idempotency keys, exact-binding dispatch checks, and lifecycle-aware
living-mark motion.

The required automated verification passed on 2026-08-20:

```text
go test ./... -count=1 -timeout=240s
go test -race ./core/conversation ./core/store ./control ./ui ./ui/apple -count=1 -timeout=240s
go vet ./...
git diff --check
(cd ui/apple/FortKit && swift run FortKitContractChecks)
```

Production publication and deployment were authorized on 2026-08-21. Exact
deployment evidence after independent verification:

- Source commit `b7e32e063c979fc6b701fd5e6bd1237e34c605e0` was fast-forwarded
  to both `main` and `codex/phase1-consistent-agent`.
- Vercel deployment `dpl_EpDbB8HTdmbJovd5VbFHvaiVFExw` was promoted to
  `https://fort-gateway.vercel.app`. The authenticated renewal route exists
  and fails closed with `401` without a bearer; the root still redirects to
  sign-in.
- The live launchd service runs the signed bundled daemon
  `fort 0.14.0+b7e32e0` with `FORT_PRIMARY_CHANNELS=preview` and
  `FORT_AGENT_CHANNELS=primary`. The migration produced one Agent Channel and
  one ownership link, retained the legacy Channel projection, and left zero
  queued or Working targets. The pre-cutover database backup is
  `fort-pre-agent-channels-b7e32e0-20260821.db` with SHA-256
  `42073df5a4e400e6ec2a5cb1052468a8089fadcaf322ed2d19a7a19b0b6100a2`.
- macOS `1.0.3 (2608211)` was signed, notarized under submission
  `6463d7be-4c7c-4729-9bc6-dbdd6a91ca70`, stapled, Gatekeeper-accepted, and
  installed at `/Applications/FortMac.app`. The release DMG SHA-256 is
  `15d15aa369053424f86d66695247edc475e043104abf9b204931453f9c28daa9`;
  the previous bundle remains recoverable at
  `/Applications/FortMac.backup-20260821T1545Z.app`.
- iOS `1.0.3 (2608211)` was uploaded once under delivery
  `7c06f888-d6c3-45ba-a259-a04c76d2528b`. App Store Connect independently
  reports `VALID`, internal `IN_BETA_TESTING`, and external
  `READY_FOR_BETA_SUBMISSION`.
- Live health, Agent options, Agent Channels, the legacy Channels contract,
  and the agent-first root all returned `200` after restart. No provider smoke
  turn was issued: the installed `codex-cli 0.143.0` does not match the
  accepted `codex-cli 0.147.0-alpha.6.5` adapter contract, so readiness remains
  `setup_required / incompatible_version`. No provider setting was changed and
  no fallback provider was substituted.

## Approval record

```text
Product model approved by Toby: 2026-08-20
Agent-first navigation treatment approved by Toby: 2026-08-20
Implementation authorized by Toby: yes, 2026-08-20
Provider enablement authorized: none
Release authorized by Toby: yes, 2026-08-21
```
