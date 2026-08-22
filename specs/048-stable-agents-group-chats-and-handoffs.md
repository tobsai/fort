# 048 — Stable Agents, group chats, and durable handoffs

**Status:** approved for implementation with Spec 047 by Toby on 2026-08-21
**Decision owner:** Toby
**Depends on:** Specs 041 and 042 for durable Conversations, exact target
snapshots, lifecycle, and no substitution; Spec 046 for the Agent-first shell;
Spec 047 for the Vercel/Supabase cloud control plane
**Reference research:**
[Grok Bot and Hermes Bot Mode — reference architecture for Fort](../docs/notes/bot-mode-reference-architecture.md)

## Goal

Make Fort a cross-framework chat service in which a durable named Agent is the
primary relationship, while every execution remains pinned to an exact,
immutable framework, profile, model, machine, adapter, authority, and policy
revision.

The first cloud release includes:

- one canonical home conversation for every Agent;
- secondary pinnable conversations with that Agent;
- multi-Agent group conversations;
- explicit, durable human-to-Agent and Agent-to-Agent handoffs;
- Agent-owned routines whose results return to an explicit conversation; and
- one cross-client history and evidence ledger independent of the framework or
  computer that executed a turn.

Fort generalizes the product pattern demonstrated by Grok Bot and Hermes Bot
Mode without becoming limited to Grok, Hermes, OpenClaw, Codex, Claude, or any
other single framework.

## Product model

```text
Fort
├── Agents
│   ├── Researcher
│   │   ├── Home                    (canonical Conversation)
│   │   ├── Market map             (secondary, pinned)
│   │   └── Weekly research        (Routine -> Home)
│   └── Builder
│       └── Home                    (canonical Conversation)
└── Groups
    └── Product launch
        ├── Researcher
        ├── Builder
        └── Reviewer
```

An Agent is a contact-like, account-scoped Fort identity. It may be backed by a
Hermes profile today and an explicitly approved OpenClaw instance later without
losing its name, canonical conversation, routines, group membership, or Fort
history. The execution change is a new immutable binding revision, never a
silent mutation or failover.

## Canonical language

| Term | Canonical meaning | It never means |
| --- | --- | --- |
| **Agent** | A durable named Fort identity and chat destination | A model alias, process, provider, machine, or Conversation |
| **Execution Source** | One concrete framework instance or gateway that inventories and runs source Agents | A display name or generic provider |
| **Source Agent** | An opaque framework-native profile/agent identity qualified by its Execution Source | A Fort Agent or unqualified profile name |
| **Agent Behavior Revision** | One immutable Fort-owned revision of role, standing instructions, enabled skills/tools, and other prompt material | Presentation metadata, source-managed memory, or a mutable prompt |
| **Agent Binding Revision** | One immutable mapping from a Fort Agent and Behavior Revision to an exact Source Agent, seat, adapter, model, machine, authority, policy, source-configuration digest, and accepted capabilities | The Agent's lifetime identity or a mutable preference |
| **Canonical Conversation** | The permanent home transcript created with one Agent | A provider-native session or the Agent itself |
| **Secondary Conversation** | An additional transcript/context boundary owned by one Agent | A replacement home chat or shared memory |
| **Group Conversation** | A durable Conversation whose versioned membership contains two or more Fort Agents | An Agent, Execution Source, or hidden delegation loop |
| **Handoff** | A durable, attributed command from one human or Agent stage to one exact recipient Agent | Copied prompt text, implicit context sharing, or provider rerouting |
| **Routine** | A versioned recurring or event-triggered command owned by one Agent and reporting to one Conversation | A framework cron row with ambiguous authority |

User-facing copy may say **Channel** for an Agent destination and **Group** for
a Group Conversation. Code and persistence retain `agent_channel` where needed
for additive compatibility, but a Channel ID identifies the stable Agent—not a
binding revision.

## Relationship to existing contracts

Upon approval, this spec supersedes Spec 046 only where it:

- lifetime-binds an Agent Channel to one immutable seat;
- permits an Agent Channel without a canonical Conversation;
- opens a last-used secondary Conversation instead of the Agent's canonical
  home by default;
- prohibits multi-Agent group chats and Agent-to-Agent work; or
- leaves Agent-owned routines outside the primary product contract.

It retains Spec 046's Agent-first navigation, secondary Conversations, pins,
living Fort mark, exact inspectability, closed provider eligibility, and legacy
rollback paths.

It restores the useful multi-participant kernel from Spec 041 but does not
restore ambient `Everyone` routing or autonomous loops. Every recipient set and
handoff is resolved explicitly and durably before dispatch. Specs 041 and 042
remain governing for atomic turns, frozen context, target lifecycle, exact
model attribution, retries, cancellation, and fail-closed no-reroute behavior.
For `/api/v2`, stable Agent IDs are the only membership and recipient-selection
identity. This explicitly supersedes Spec 041's use of
`conversation_participant` as current participant selection; those rows remain
immutable execution evidence only.

Spec 047 must implement this identity model in its cloud schema before the
SQLite-to-Postgres cutover. Porting the current lifetime-binding schema first
and changing identity later is not an accepted migration sequence.

## Stable Agent and binding revisions

An `agent_channel` is the durable Fort Agent. Its logical ID, profile history,
canonical Conversation, secondary Conversations, memberships, and routines
survive explicitly approved execution changes.

Presentation changes create an append-only Agent profile revision or an
equivalent audited presentation event. Name, title, avatar, hidden/pinned state,
and ordering never identify execution.

Behavior changes create an immutable Agent Behavior Revision containing role,
standing instructions, enabled skills/tools, and all other Fort-owned prompt
material. Accepting a Behavior Revision atomically creates a successor Agent
Binding Revision that references it and otherwise copies or explicitly changes
the prior execution fields. The behavior change applies only to future ordinary
turns. Queued targets, attempts, leases, retries, Routine runs, and Handoffs
retain the Behavior and Binding Revisions they already pinned.

Execution uses one current Agent Binding Revision. A revision records:

- exact Agent Behavior Revision ID;
- Execution Source ID and opaque Source Agent ID;
- exact Fort profile, provider, requested and resolved model;
- enrolled computer or disclosed cloud runtime;
- adapter ID and revision;
- source configuration digest;
- authority and policy IDs/revisions;
- declared provider session and memory behavior;
- capability evidence and the readiness contract accepted at creation; and
- activation, retirement, and superseding revision evidence.

A person may advance an Agent to a new binding only through an explicit command
that shows the old and proposed identities, revalidates readiness and authority,
and appends an audit event. The preview must disclose which source-managed
memory, skills, sessions, files, and tool state will not carry to the proposed
source. No system or framework may rebind because a machine is offline or
another model is available. If the source configuration digest differs from the
accepted revision at dispatch, execution fails closed before provider start.

Every target, attempt, lease, retry, Routine run, and Handoff pins the exact
Behavior and Binding Revisions it uses. Retry retains the original revisions. A
new ordinary turn uses the Agent's current accepted revision. An unavailable or
drifted revision fails closed with Recheck or an explicit Rebind action; it
never reroutes.

The existing `conversation_participant` seat snapshots remain immutable
execution evidence and never enumerate the current recipients of a v2
Conversation. Current one-Agent ownership and Group membership use stable Agent
IDs, with at most one current membership per `(conversation_id, agent_id)` and
at most one initial target per `(turn_id, agent_id)`. A
Conversation may accumulate historical participant snapshots for successive
Behavior or Binding Revisions of the same stable Agent. That does not make a
one-Agent Conversation multi-Agent. Each target references the stable Agent and
exact participant, Behavior Revision, and Binding Revision that were frozen at
target creation. Rebinding or changing membership affects only later targets.

## Execution-source identity and isolation

Source Agent identity is always `(execution_source_id, opaque_source_agent_id)`.
Display names never merge identities. Same-named Source Agents on two machines
remain independently addressable and visibly source-qualified, for example
`Researcher · MacBook` and `Researcher · Mac mini`.

Offline sources and their Agents remain in the roster with last-seen evidence.
Discovery cannot enroll, bind, rebind, or dispatch an Agent implicitly.

Every Execution Source declares whether these resources are `profile_scoped`,
`machine_shared`, `account_shared`, or `unknown`:

- provider credentials;
- filesystem/workspace;
- browser sessions;
- framework sessions;
- source-managed memory; and
- tool/MCP configuration.

`unknown` never satisfies a requirement for isolation. A Fort Agent, Hermes
profile, Grok screen, or visual avatar is not itself a security boundary.

## Canonical and secondary Conversations

Creating an Agent atomically creates exactly one canonical Conversation. It is
the default destination when the Agent is selected on every client, is visibly
distinguished as **Home**, and cannot be archived, reassigned, or replaced while
the Agent remains open.

**New conversation** creates a secondary Conversation. A secondary Conversation
may be renamed, pinned, archived/reopened, and searched, but it never changes
the Agent's canonical home.

Compaction may change only the provider prompt input. It appends a
provenance-bearing checkpoint and never deletes, rewrites, or forks committed
Fort messages or events.

The four state boundaries remain distinct:

1. immutable Conversation history;
2. explicit Agent profile configuration;
3. optional future Fort Agent Memory; and
4. source-managed memory/session state.

The first cloud release does not claim or synthesize learned Fort memory.
Source-managed memory is inspectable and noncanonical. It cannot rewrite Fort
history or enter another Conversation unless a separately approved context or
memory policy explicitly selects it.

## Group Conversations

A Group Conversation has a stable internal ID independent of its display name
and contains between two and six open Agents in the first release. Membership
is versioned. Adding or removing a member while a Group Turn is active fails;
historical messages and targets retain the membership revision under which they
were created.

The composer resolves mentions through stable Agent IDs selected from the live
roster. Duplicate display names require a source-qualified choice. Unknown text
that begins with `@` is ordinary text and never selects an Agent by guessing.

A human Group Turn records before dispatch:

- client turn ID and idempotency key;
- membership revision;
- exact ordered recipient Agent IDs and current Behavior and Binding Revisions;
- immutable context/message snapshot;
- concurrency policy;
- maximum Agent messages, enforceable cost/token classification, and deadline;
  and
- cancellation and approval policy.

The first release creates exactly one frozen fan-out wave per human Group Turn,
with at most one initial target for each explicitly selected Agent. A human may
target one or more members or select **Everyone**; no mention does not silently
mean Everyone. Replies, silence, ordinary prose, and mentions in Agent output
create no additional target. Additional work is possible only through an
accepted durable Handoff. Agents may pass without generating filler.

Responses may execute concurrently only when the persisted Group Turn says so.
Durable event sequence, not completion timing, determines display order. All
initial targets and their accepted Handoff chains share a limit of ten Agent
messages and one hard deadline. The initial wave is depth zero; each accepted
Handoff advances the causal depth by one, through depth three. The turn settles
when its initial targets and accepted Handoff chains are terminal. Limit
exhaustion produces an actionable **Needs You** state.

A cost or token limit is hard only when the exact adapter proves before provider
start that the limit is enforceable. Otherwise its classification is `unknown`,
it is displayed as informational evidence, and the hard target-count and
deadline bounds remain authoritative.

Group context contains only the Group Conversation's selected messages,
Handoffs, and finalized immutable Fort context/output artifact IDs. It never
accepts arbitrary attachments, local paths, workspace files, framework-native
file IDs, or implicit imports from members' canonical or secondary
Conversations, Fort memory, or source-managed memory.

## Durable Handoffs

A Handoff is persisted and authorized before the recipient runs. It records:

- Handoff ID, idempotency key, lifecycle, and creation actor;
- source Group Turn, message, Agent, Behavior Revision, and Binding Revision;
- one recipient Agent, Behavior Revision, and Binding Revision;
- exact source Conversation and one output Conversation ID resolved before
  dispatch;
- bounded context manifest containing only explicit Fort message IDs and
  finalized immutable Fort context/output artifact IDs from Spec 047;
- requested result and reply linkage;
- initiating human/Group delegation grant, Handoff policy, approval requirement,
  budget classification, maximum depth, and deadline;
- attempt, lease, cancellation, receipt, and terminal evidence; and
- parent Handoff ID when the policy permits another stage.

One Agent owns each Handoff stage. The initiating human Group Turn, direct
Handoff, or Routine occurrence persists the root delegation grant. Before
creating each target, Fort computes effective authority as the intersection of
that root grant, parent-stage authority, Handoff policy, recipient Binding
Revision policy, structured emitter request when present, and any explicit
approval receipt. Broader standing capabilities on the recipient are not
delegated automatically. Any requested excess fails before provider start or
becomes an actionable **Needs You** approval. A Handoff grants no new tool,
provider, filesystem, credential, or approval authority. The recipient sees the
sender, request, selected context, effective constraints, and exact output
Conversation.

Within a Group Conversation, the output Conversation is that same Group. For a
direct Handoff, the source is the exact originating Conversation rather than the
sender's canonical Conversation by default. The recipient's canonical
Conversation may be chosen as the output only when its exact ID is persisted
before dispatch. A successful Handoff creates exactly one authoritative
`conversation_message` body in its persisted output Conversation. Every other
affected Conversation receives a reference-only Handoff event/projection that
contains IDs, state, and linkage but never a copied result or context body. The
context manifest is the only shared input.

Before accepting a manifest, Fort verifies every referenced message or artifact
belongs to the same account, is authorized by the root delegation grant, and is
immutable. Artifacts must also be finalized and match their persisted digest
and size.

Agent-initiated Handoffs require a structured, Fort-owned emitter contract. Fort
never parses ordinary model prose or an `@name` string as authorization. An
eligible adapter must prove that a provider-native tool/action maps exactly to
the persisted Handoff command. Human-initiated Handoffs use the same command
without requiring that adapter capability.

Each Group Turn has a maximum Handoff depth of three and shares the turn's ten
Agent-message bound, cost/token classification, and hard deadline. Recursive
delegation beyond the persisted limit, cyclic Handoffs, self-Handoffs, recipient
changes, and hidden fan-out fail closed. A limit or judgment request becomes
**Needs You**.

At least two different framework families must complete an attributed Handoff
in production acceptance. Framework-native direct messaging may transport only
a Handoff already persisted and authorized by Fort; it is never the source of
truth.

## Agent-owned Routines

A Routine is owned by exactly one Agent. Its immutable revision records:

- trigger, schedule, timezone, and next occurrence;
- input source and freshness requirement;
- expected result and result Conversation;
- approval boundary;
- missing/stale-input behavior;
- retry, idempotency, catch-up, and lateness policy; and
- exact binding compatibility requirements.

The first cloud release executes only Routines whose authority is `fort_cloud`.
Source-native routines are stored as read-only `source_routine_projection`
records and cannot enqueue a Fort run. Import requires a source disable/fencing
receipt plus the exact last and next source occurrences; Fort atomically verifies
those facts, creates the Fort Routine revision, and assumes authority without a
double-fire window. A future source-authoritative execution mode requires a
separate signed occurrence/receipt protocol and a new approved spec.

A Binding or Behavior Revision change pauses affected Routines until
revalidated. Every run pins both revisions and records occurrence, attempt,
activity, failure, and next-action evidence as events/detail records. Only a
successful normalized result may create one message in the selected result
Conversation. **Test Routine** creates a real occurrence through the normal
approval, idempotency, lease, and result path; it is not an adapter dry run.

## Framework boundary

Fort owns narrow, source-neutral contracts:

- `AgentSourceInventory`: discover exact Source Agents, resource sharing,
  capabilities, and readiness; it cannot execute or enroll;
- existing `runtime.Runtime`: start/cancel one target using an exact binding
  revision;
- `HandoffEmitter`: validate a structured Agent-initiated Handoff command from
  one exact adapter; and
- `RoutineAuthority`: inspect and fence a source schedule before importing it;
  it cannot enqueue source-authoritative Fort work.

Framework implementations remain adapters outside `core`. Grok Bot product
documentation informs the Fort product model but exposes no accepted dispatch
API. Hermes Bot Mode or OpenClaw behavior does not bypass Fort's provider,
authority, tool, readiness, or exact-model gates.

The first cloud release is not accepted until two independently approved real
`runtime.Runtime` adapter families can join one Group Conversation, and one
separately approved adapter implements the structured `HandoffEmitter`
contract. A framework family means a distinct approved runtime contract/adapter
family, not another model, profile, or machine behind the same adapter. Fake
adapters may prove schema, preview, and deterministic state transitions, but
never satisfy production acceptance. Human-directed Handoffs remain available
to all otherwise eligible text Agents. This spec authorizes no provider or
adapter: each real adapter retains its separate command, identity, readiness,
lifecycle, authority, and terminal-normalization approval gate.

## Persistence and migration

The Postgres schema in Spec 047 adds durable equivalents of:

```text
agent_channel                 stable Agent identity and current revision links
agent_profile_revision        name, title, avatar, presentation, audit evidence
agent_behavior_revision       role, instructions, skills/tools, prompt material
execution_source              exact framework instance/gateway identity
source_agent                  opaque identity qualified by execution source
agent_binding_revision        immutable behavior/source/seat/model/policy snapshot
agent_conversation            agent, conversation, canonical|secondary
conversation_member_revision  group membership version and stable Agent members
conversation_target_binding   target -> Agent + binding revision + participant
routine / routine_revision / routine_run
source_routine_projection      read-only framework-native schedule evidence
handoff / handoff_attempt / handoff_projection (reference-only)
```

Existing Conversation, participant, turn, target, message, run, event,
Agent-Channel, and pin rows are never renamed, rewritten, or deleted.

Migration is deterministic and previewed:

1. Each current Agent Channel becomes one stable Agent with one initial Behavior
   Revision and one initial Binding Revision copied byte-for-byte from its
   immutable behavior and execution binding.
2. If it owns one Conversation, that Conversation becomes canonical.
3. If it owns none, migration creates an empty canonical Conversation without
   dispatching a provider.
4. If it owns several, migration requires an explicit canonical selection and
   does not guess from activity, title, pin, or readiness.
5. Remaining owned Conversations become secondary and retain pins/state.
6. A legacy multi-participant Conversation becomes a Group only when every seat
   maps unambiguously to one stable Agent and the user explicitly accepts the
   preview. Otherwise it remains legacy/unassigned.
7. Existing schedules map to Agent-owned Routines only when owner, binding,
   authority, and result Conversation are complete and exact. Ambiguous rows
   remain legacy and do not run twice.

Migration replay creates no duplicate Agent, canonical Conversation, binding,
membership, Routine, or Handoff row. Concurrent cutover revalidates every
mapping inside the write transaction.

## Versioned HTTP contract

Spec 047's cloud protocol exposes these concepts under `/api/v2` rather than
changing the deployed `/api/agent-channels` contract silently:

```text
GET|POST       /api/v2/agents
GET|PATCH      /api/v2/agents/{agent_id}
POST           /api/v2/agents/{agent_id}/rebind
GET             /api/v2/agents/{agent_id}/conversations
GET             /api/v2/agents/{agent_id}/conversations/canonical
POST            /api/v2/agents/{agent_id}/conversations

GET|POST       /api/v2/groups
GET|PATCH      /api/v2/groups/{group_id}
POST            /api/v2/groups/{group_id}/members
POST            /api/v2/groups/{group_id}/turns

GET|POST       /api/v2/handoffs
GET             /api/v2/handoffs/{handoff_id}
POST            /api/v2/handoffs/{handoff_id}/cancel

GET|POST       /api/v2/agents/{agent_id}/routines
PATCH           /api/v2/agents/{agent_id}/routines/{routine_id}
POST            /api/v2/agents/{agent_id}/routines/{routine_id}/test
```

Every child route verifies the full parent chain. Foreign children are `404`;
valid resources in the wrong state are `409`. Commands reject unknown fields,
take client-generated idempotency keys, and cannot accept raw provider, model,
machine, or authority components where an opaque option/revision ID is
required. Lists return `[]`, never `null`, with stable ordering.

Group Send atomically creates the human message, Group Turn, frozen recipient
set/context, and its single initial fan-out wave. Handoff acceptance atomically
creates its record, exact output Conversation, reference-only projections,
effective-authority snapshot, exact target, and queued attempt.

## Delivery sequence with Spec 047

1. Approve Specs 047 and 048 together.
2. Prove two source inventories and exact Source Agent disambiguation in a
   non-production fixture.
3. Add stable Agent, binding-revision, canonical/secondary Conversation, Group,
   Routine, and Handoff contracts to the shared SQLite/Postgres ledger suite.
4. Migrate existing Agent Channels into stable Agents without changing current
   dispatch behavior.
5. Ship `/api/v2` and v2-capable Web/macOS/iPhone clients while v1 remains the
   single write authority.
6. Prove human group targeting and human-directed Handoffs across two different
   framework families.
7. Enable one structured Agent-initiated Handoff adapter and run bounded group
   collaboration acceptance.
8. Complete Spec 047's preview data migration, paired Vercel deployments,
   worker leases, reconnect, recovery, and production cutover.
9. Retire v1 only after minimum-version and rollback acceptance.

Every phase remains feature-gated and preserves a runnable rollback path.

## Acceptance criteria

- Relaunching any client opens the same Agent and canonical Conversation by
  stable ID.
- Explicitly rebinding an Agent preserves its history and groups while every
  old/new target shows the exact revision it used.
- Taking a worker offline leaves its Agents visible and never moves them.
- Same-named Source Agents on different workers remain separately addressable.
- One Group contains Agents from at least two framework families and reconstructs
  the same ordered transcript on Web, macOS, and iPhone.
- Mention resolution, membership revision, recipient freezing, duplicate Send,
  retry, cancellation, and concurrent responses remain deterministic.
- The single initial fan-out wave, ten-message limit, depth-three Handoff limit,
  enforceability-classified cost/token evidence, and hard deadline stop further
  work durably and produce actionable state.
- A human-directed cross-framework Handoff and a structured Agent-initiated
  Handoff each execute once, survive process/reconnect interruption, and return
  an attributed linked result.
- Handoff context contains only manifest-selected records; no sibling transcript
  or memory leaks.
- A successful Handoff creates one authoritative result message in its exact
  output Conversation; every other projection is reference-only.
- Cycles, self-handoffs, unknown recipients, stale binding revisions, and
  authority escalation fail before provider start.
- A Routine has one Agent owner, one scheduling authority, one result
  Conversation, exact occurrence idempotency, and visible run history.
- Loss or rebuild of an execution machine cannot delete cloud-authoritative
  Agents, Conversations, Groups, Routines, Handoffs, approvals, or events.
- Resource-sharing labels remain truthful; no Agent/profile/avatar is presented
  as credential, browser, filesystem, memory, or OS isolation.
- Full ledger parity, concurrency/race, API, Web, FortKit, forced-SSE-reconnect,
  migration, backup/restore, and rollback suites pass.

## Non-goals

- public groups, external human participants, social discovery, or billing;
- Agents creating/deleting Agents, Groups, bindings, or authority policies;
- unbounded recursive delegation, hidden fan-out, or model-selected routing;
- implicit transcript, source-memory, credential, filesystem, or browser-state
  sharing;
- arbitrary attachment upload, local/workspace paths, or framework-native file
  identifiers in Group or Handoff context manifests;
- executable source-authoritative Routines in the first cloud release;
- transparent model, framework, source, or machine failover;
- claiming Grok Bot, Hermes Bot Mode, OpenClaw, or another framework is eligible
  without an approved exact adapter; and
- destructive rewrite or deletion of legacy/local evidence during migration.

## Approval gates

Approval confirms:

1. a stable Agent outlives explicit binding revisions;
2. every Agent has one permanent canonical home Conversation plus optional
   secondary Conversations;
3. first-release Groups contain two to six Agents and use explicit targets;
4. first-release Group work is one explicit fan-out wave plus accepted Handoffs,
   bounded to ten Agent messages, Handoff depth three, and one hard deadline per
   human Group Turn;
5. Agent-to-Agent Handoffs require a structured adapter emitter, preserve the
   initiating delegation grant, and write exactly one authoritative result;
6. two independently approved real Runtime adapter families must pass
   cross-framework production acceptance; and
7. Agent-owned `fort_cloud` Routines are part of the first cloud release,
   source-native routines are read-only until fenced/imported, and learned Fort
   Agent Memory remains deferred.

Approval does not authorize any currently ineligible provider, shared
credentials, remote computer control, external side effect, or destructive
cloud/local retirement.
