# Spec 041 — Shared Agent Conversations

**Status:** approved 2026-07-31 — implemented locally; live two-computer acceptance pending
**Product decision:** Fort's next milestone is one excellent shared conversation
across enrolled agents and computers, with lightweight Projects that organize
related conversations. Work on the broader command center, capability planner,
playbooks, DAG presentation, full calendar/capacity views, metrics, and native
client expansion pauses until this experience is accepted. A compact truthful
view of work in progress and persisted schedules for Today remains part of the
primary conversation experience.

## Why this replaces the current milestone

The live July 30 experience proves that Fort has assembled useful execution
parts without yet becoming an app a person can simply talk through:

- a “conversation” is currently inferred from one-shot runs rather than stored
  as a durable conversation with ordered messages;
- opening **New conversation** can preserve the old Deck scroll position and
  leave the conversation itself offscreen;
- the composer reports every Codex model as requiring setup while the agent and
  machine rail says that everything is ready;
- a simple completed reply is surrounded by assignment promotion, event-log,
  playbook, model, machine, performance, calendar-grid, and agent-status
  concepts;
- the existing remote runtime can execute on another Fort, but the product does
  not expose that primitive as a coherent multi-person-style conversation.

Spec 040 remains historical design and implementation evidence, but it is no
longer the next product milestone. Spec 039's full capability-planning work also
pauses. This spec reuses only the exact-profile readiness and remote-execution
pieces required to make shared chat truthful.

## Goal

Fort is a group chat whose non-human participants can live on different
computers.

A person creates a conversation, adds one or more **agent seats**, writes a
message, and explicitly addresses one seat, several seats, or everyone. Each
addressed agent receives the same durable conversation history and answers in
the same transcript. Every answer visibly names the agent, exact model profile,
and computer that produced it. Related conversations can be grouped into one
optional Project without merging their transcripts or turning the Project into
an orchestration workspace.

An **agent seat** is an immutable binding of:

```text
Fort profile + provider/model + enrolled machine
```

For example:

```text
Codex · Sol — Toby's MacBook
OpenClaw · Main — Talos Mac mini
```

The seat is the selection unit. The UI never asks a person to coordinate three
independent Agent, Model, and Machine controls.

## The experience

### 1. One primary surface

Fort opens to **Conversations**. The primary shell contains:

- **All conversations**, **Inbox**, and lightweight Project folders in the
  conversation sidebar;
- a newest-activity-first conversation list within the selected scope;
- the selected transcript;
- participant seats in the conversation header;
- one composer with a **To:** target and one **Send** action;
- a right-hand **Today** rail showing work actually in progress and persisted
  schedules for the current day; and
- a compact **Computers** view for enrollment and readiness.

The primary shell does not show Assign, Performance, standalone Week/Today
calendar grids, Playbooks, project rooms, gates, checkpoint progress, **Turn
this into work**, or the recorded run activity log. Existing APIs and persisted
records remain in place; they are not part of this milestone's default
experience.

### 2. Project folders

A conversation belongs to zero or one Project. Unassigned conversations appear
in **Inbox**; **All conversations** always spans every Project and Inbox.

Projects are folders inside Conversations, not a separate product area. A
person can:

- create and rename a Project;
- start a conversation in the currently selected Project;
- move an existing conversation into a different Project or back to Inbox; and
- collapse or expand Project folders without changing conversation state.

Projects are ordered by the latest durable message activity among their
conversations. Conversations within a Project remain newest-activity-first.
Moving a conversation does not change its activity timestamp, transcript,
participants, targets, run history, or unread/attention state.

A Project owns no agent, model, computer, prompt, memory, files, plan, status,
approval, or shared context. Agents receive only the selected conversation's
transcript. V1 has no nested Projects and no conversation can belong to several
Projects.

### 3. New conversation

**New conversation** opens an empty transcript at its top, with focus in the
composer and no inherited scroll position. When created from a Project folder,
the conversation starts in that Project; when created from All conversations or
Inbox, it starts in Inbox. The Project remains visible in the conversation
header and can be changed later.

The participant picker presents ready seats as complete choices, grouped by
computer. Each row shows agent, exact model, and computer. Seats that are known
but not usable appear under **Needs setup**, disabled with a closed reason such
as **Sign in required**, **Update required**, **Unavailable**, or **Computer
offline**, plus **Recheck**. A static roster claim never produces a ready seat.

The person adds at least one ready seat before sending. If exactly one seat is
ready, Fort may preselect it, but the selection remains visible before send.

### 4. Addressing agents

The composer has one explicit target control:

```text
To: [Codex · Sol — MacBook] [OpenClaw — Mac mini]
```

It supports:

- one selected seat;
- several selected seats; and
- **Everyone**, which means every current participant.

The most recently used target set remains selected for the next turn. Removing
a participant affects only future turns and never rewrites history.

There is no model-generated routing in this path. The selected seats are the
complete dispatch plan.

### 5. Shared-turn semantics

One send creates one durable human message and one target record per addressed
seat in a single transaction. All targets from that send receive the same
pre-turn transcript snapshot, including the new human message but excluding
responses still being generated for that same turn.

Targets run concurrently and independently, including when their seats are on
different computers. Their replies appear as they complete. On the next human
turn, every completed reply is part of the shared history seen by every newly
addressed seat.

V1 agents never dispatch, mention, or recursively wake other agents. To ask a
second agent to react to the first agent's answer, the person sends the next
turn to that second seat. This makes multi-agent chat useful without creating
unbounded agent-to-agent loops.

### 6. Truthful message states

Every addressed seat immediately gets one inline response slot with one of:

- **Queued** — the target is durably accepted but provider startup has not been
  observed;
- **Working** — Fort has observed a real provider start or activity event;
- **Answered** — a successful run produced one terminal normalized message;
- **Failed** — startup, transport, provider, or terminal-output validation
  failed; or
- **Canceled** — the person stopped that target.

Queued and Working are not inferred from elapsed time. The transcript shows a
simple state, not the raw event log. Exact errors and the request/run ID are
available from a per-message **Details** disclosure.

One target failing never hides or cancels another target's answer. **Retry** on
a failed target reuses that target's original seat and original transcript
boundary; it neither reruns successful peers nor incorporates later messages.

### 7. No silent movement

The server revalidates the exact seat immediately before every dispatch. If its
profile or computer is no longer ready, that target fails without invoking a
runtime. Fort never changes provider, model, profile, or computer automatically.

The inline recovery offers only explicit actions:

- **Recheck and retry** the same seat;
- **Choose another seat**, which creates a visibly different target; or
- **Cancel**.

Historical messages continue to show the seat that actually produced them.

### 8. Computers

The compact Computers view reports enrolled computers from functional,
secret-free readiness rather than static agent-name claims. It supports the
existing one-time `fort mesh invite` / `fort mesh join` path without inventing a
second enrollment protocol.

When the hub is being used locally, **Add computer** may create and display the
existing one-time join command. When the hub is accessed remotely and the
loopback-only invite API is unavailable, Fort says to run `fort mesh invite` on
the hub; it does not weaken the invite boundary.

Computer status is never presented as **Ready** when none of its seats are
actually dispatchable.

### 9. Today rail

The desktop right column is **Today**. It replaces the current always-visible
agent/machine roster and has two sections:

#### In progress

**In progress** shows all Fort work that is currently queued or truthfully
working, across every Project and computer:

- conversation targets in Queued or Working state; and
- compatible historical direct/flow runs that are durably queued or have live,
  reconciled execution evidence.

Rows are deduplicated by run ID. Each row shows the Project and conversation or
run title, agent/model, computer, truthful state, and last real activity time.
Selecting a conversation target opens its conversation; selecting a legacy run
opens bounded run details.

Working requires a persisted or streamed provider start/activity event from the
current daemon lifetime. A stale `run.status=running`, elapsed clock, process
listing, estimated duration, or static machine claim is insufficient. Queued
items appear as Queued, not Working. Blocked, failed, canceled, and completed
items do not remain in this section.

Within the section, Working items sort by latest real activity and Queued items
follow by acceptance time. An empty section says **Nothing in progress**. The
rail never invents percentages, ETA, or “expected review” moments.

#### Scheduled today

**Scheduled today** shows one row for every enabled persisted schedule whose
next or most recent planned occurrence falls within the displayed local day.
Each row shows its real scheduled time, title, recurrence summary, and current
occurrence state: **Upcoming**, **Fired**, **Running**, **Completed**,
**Failed**, or **Canceled**. When a fired occurrence owns a run, the row links
to that run and the active run also appears in In progress.

The section is sourced only from durable schedule definitions and occurrence
records. The current UI's inferred backlog placement, mean-duration ETA, and
predicted sign-off logic are not schedule data and must not appear here.

Today uses Fort's configured IANA display timezone, defaulting to the host's
local timezone, and shows the timezone abbreviation beside the date. Day
boundaries and schedule matching use that timezone; persisted fire instants
remain UTC. Disabled schedules never appear. An empty section says **Nothing
scheduled today**.

The rail updates from persisted conversation/run/schedule events and refreshes
at the next local day boundary. At narrow widths it becomes a **Today** sheet
opened from the conversation header; it is never squeezed beside the
transcript.

## Durable model

Add these append-friendly records to SQLite:

### `project`

```text
id, name, created_at, updated_at
```

Project names are non-empty, capped at 120 UTF-8 bytes, and unique
case-insensitively. Project ordering is derived from conversation activity;
renaming a Project never reorders its conversations or invokes a runtime.

### `conversation`

```text
id, project_id(nullable), title, state(open|archived), created_at, updated_at
```

`updated_at` advances only for a new durable message or an explicit conversation
state change. Transient status changes and Project moves do not reorder the
conversation list. A null `project_id` means Inbox.

### `conversation_participant`

```text
id, conversation_id, profile, agent, model, machine,
state(active|removed), created_at, removed_at
```

Profile, agent, model, and machine are persisted together so old messages never
change identity when the catalog or roster changes.

### `conversation_turn`

```text
id, conversation_id, client_turn_id, human_message_id,
context_through_message_id, created_at
```

`(conversation_id, client_turn_id)` is unique. Repeating the same client ID
returns the original accepted turn and dispatches nothing twice.

### `conversation_target`

```text
id, turn_id, participant_id, run_id, attempt, state,
error_code, error, created_at, updated_at
```

`(turn_id, participant_id, attempt)` is unique. Retry appends a new attempt; it
does not mutate or erase the failed attempt.

### `conversation_message`

```text
id, conversation_id, turn_id, author(human|agent|system),
participant_id, target_id, body, created_at
```

Human messages are stored in the same transaction as the turn and targets.
Agent messages are appended only after successful terminal-output validation.
System messages are reserved for Fort-owned bounded state notices; provider
warnings and raw stderr are events/details, never transcript messages.

Existing `run` and `event` rows remain the execution and diagnostic record.
Each conversation target owns one run ID, so the conversation can project its
state without duplicating raw provider events.

### `schedule`

```text
id, title, mode(once|cron), expression, timezone, flow_id,
enabled, next_fire_at, last_fire_at, created_at, updated_at
```

The existing `fort schedule` entry point becomes a loopback client of the
serving daemon's bounded schedule-create contract. The daemon persists the
definition before registering its timer and owns enabled schedules immediately
and after restart; scheduling is no longer represented solely by the lifetime
of a foreground CLI process. If no local daemon is available, creation fails
with an instruction to start Fort and no schedule is recorded. `timezone` is an
IANA identifier and fire instants are stored in UTC.

### `schedule_occurrence`

```text
id, schedule_id, scheduled_for, state,
run_id, error, created_at, updated_at
```

`(schedule_id, scheduled_for)` is unique. Fort persists an occurrence and its
canonical run ID before dispatch. Retry/restart cannot fire the same occurrence
twice. Definitions plus occurrences are the only source for Scheduled today.

## Context contract

Fort constructs provider input deterministically from persisted data:

1. exact participant identities;
2. ordered human and successful agent messages through the turn's persisted
   `context_through_message_id`;
3. the addressed seat's identity; and
4. a Fort-owned instruction to answer as that participant.

The payload is canonical JSON embedded in a fixed Fort prompt. User and agent
text remains JSON string data; it is never interpolated as routing or machine
instructions.

V1 does not reuse provider-native sessions. The durable Fort transcript is the
source of truth on every computer. This keeps local and remote turns behaviorally
identical and makes restart/retry inspectable.

The complete compiled prompt is capped at 65,536 UTF-8 bytes. Fort rejects an
oversized turn with `409 conversation_context_limit` before persisting or
dispatching it. It never silently truncates, summarizes, or drops messages.

## HTTP contract

Add a conversation-focused API behind `ui` ports:

```text
GET    /api/conversation-seats
GET    /api/today
POST   /api/schedules
GET    /api/projects
POST   /api/projects
PATCH  /api/projects/{id}
GET    /api/conversations
POST   /api/conversations
GET    /api/conversations/{id}
PATCH  /api/conversations/{id}
POST   /api/conversations/{id}/participants
DELETE /api/conversations/{id}/participants/{participant_id}
POST   /api/conversations/{id}/turns
POST   /api/conversations/{id}/targets/{target_id}/retry
POST   /api/conversations/{id}/targets/{target_id}/cancel
GET    /api/conversations/{id}/events
```

`GET /api/today` returns the configured display date/timezone, deduplicated
in-progress items, and scheduled items for that day. It is a read-only
projection and invokes zero runtimes. Its response uses non-nil empty arrays and
includes a freshness timestamp.

`POST /api/schedules` is loopback-admin-only and is used by `fort schedule`.
It validates the once/cron expression and IANA timezone, persists the definition
first, registers it exactly once in the running daemon, and returns the durable
schedule ID plus next fire time. It is not called by the primary web UI.

`GET /api/conversations` accepts an optional `project_id` scope. An omitted
scope returns All conversations; `project_id=inbox` returns unassigned
conversations. `PATCH /api/conversations/{id}` changes only title, Project, or
archived state and never dispatches work. An unknown Project fails without
moving the conversation.

Seat IDs are opaque. The server resolves a seat to one current exact
profile/machine binding, persists that binding when it joins a conversation,
and revalidates it at dispatch.

`POST .../turns` accepts:

```json
{
  "client_turn_id": "client-generated-uuid",
  "text": "Compare the two approaches.",
  "participant_ids": ["participant-1", "participant-2"]
}
```

It returns `202 Accepted` only after the human message and all target records
are durable. Provider startup happens afterward. The conversation event stream
reports bounded state/message updates; reconnecting clients rebuild completely
from `GET /api/conversations/{id}` and therefore do not depend on an unbroken
SSE connection.

The existing `/api/chat`, playbook, run, gate, backlog, and metrics contracts
stay compatible for now, but the new default surface does not call them.

## Architecture

- `core/conversation/` owns pure Project/conversation types, Project grouping,
  target validation, context construction, ordering, and state reduction. It
  imports only core seams.
- `core/store/` owns the additive tables and atomic create-turn operation.
- `core/scheduler/` owns durable once/cron definitions, occurrence
  idempotency, next-fire calculation, and restart registration.
- `control/conversations.go` adapts the store, exact-profile catalog/readiness,
  deterministic machine bindings, and `runtime.Runtime` into a conversation
  service.
- `control/today.go` reduces persisted conversation targets, reconciled runs,
  schedule definitions, and occurrences into the bounded Today read model.
- `ui` depends only on a bounded conversation port and wire types. It does not
  import engine, router, graph, native, remote, or capability implementations.
- `cmd/fort` wires the service to the existing cluster runtime and live
  readiness coordinator.
- `exec/cluster`, `exec/remote`, and `exec/node` remain the local/remote
  transport. A conversation target is still one ordinary `runtime.RunSpec`.

Only conversation targets and due schedule occurrences invoke a Runtime. Seat
listing, participant changes, target selection, context building, placement
disclosure, retries, Today projection, and schedule calculation make zero model
calls.

## Implementation slices

Implementation begins only after this spec is approved and follows TDD.

### Slice A — durable conversation kernel

- Write failing store/domain/service tests.
- Add Project/conversation tables, atomic turns, idempotency, context
  compilation, and a fake-runtime conversation port.
- Add durable schedule definitions/occurrences, restart registration, and the
  pure Today reducer.
- Prove one local seat can hold a multi-turn durable conversation across a
  service restart.

### Slice B — shared web chat

- Write failing HTTP and source/interaction tests.
- Replace the default Command Deck presentation with Conversations + Computers,
  including All conversations, Inbox, and Project folders.
- Replace the agent/machine rail with In progress + Scheduled today.
- Add seat picker, explicit targets, per-target states, stop/retry, reconnect,
  and truthful empty/error states.
- Verify the new-conversation scroll position, keyboard focus, narrow reflow,
  and accessible state announcements.

### Slice C — two-computer acceptance

- Extend the existing mesh end-to-end fixture so one turn targets a local fake
  seat and a remote fake seat concurrently.
- Verify remote events and terminal messages land in the one hub transcript.
- Run the live acceptance scenario on the enrolled MacBook and Mac mini.

Native iOS, watchOS, CarPlay, and the custom SwiftUI macOS presentation do not
change in these slices. After the shared web chat passes live acceptance, the
same bounded conversation API can be adopted by native clients as a separate
approved milestone.

## Test criteria

### Domain and persistence

- Conversation ordering is based on latest durable message activity only.
- A conversation belongs to zero or one Project; null means Inbox.
- Project names are validated and unique case-insensitively.
- Moving a conversation changes only its Project binding and does not alter its
  activity timestamp, transcript, participants, targets, or run history.
- All conversations, Inbox, and a Project scope return complete,
  newest-activity-first results without dispatching a runtime.
- Creating a turn persists its human message and complete target set atomically.
- Repeating a `client_turn_id` returns the original turn and starts zero new
  runtimes.
- All targets receive byte-identical pre-turn transcript content and never see
  same-turn peer responses.
- A successful target appends exactly one terminal agent message.
- Missing terminal output, transport loss, non-zero exit, and provider failure
  each produce an inline failed target and no agent-authored message.
- Retry uses the same participant identity and transcript boundary, creates one
  new attempt, and does not rerun successful targets.
- The compiled-context limit fails before persistence and dispatch, with no
  silent truncation.

### Today and schedules

- In progress deduplicates conversation targets and run rows by run ID.
- Queued work is labeled Queued; Working requires current persisted provider
  start/activity evidence.
- Stale running rows, blocked work, and terminal work do not appear as in
  progress.
- In-progress ordering uses real activity/acceptance timestamps and never an
  estimated completion time.
- Schedule definitions persist before registration and reload exactly once
  after daemon restart.
- `(schedule_id, scheduled_for)` prevents duplicate occurrence dispatch across
  retry and restart.
- Once and cron definitions match Today in their stored IANA timezone,
  including DST and UTC/local-day boundaries.
- Disabled schedules and inferred backlog/ETA rows never appear in Scheduled
  today.
- Empty Today arrays are non-nil and the complete Today projection invokes zero
  model calls.

### Routing and readiness

- Seat options contain only exact profile/machine pairs from functional
  readiness; static registry agent names alone cannot create a ready seat.
- Explicit seat selection survives request → participant → target → persisted
  run → `runtime.RunSpec` unchanged.
- Readiness drift invokes zero runtimes and never moves a target.
- Unknown, removed, mismatched, offline, or unready seats fail closed with a
  bounded reason code.
- All conversation selection and placement tests assert zero model calls.

### Concurrency and remote execution

- Two seats addressed in one turn dispatch concurrently and can complete in
  either order without corrupting message order or target state.
- One local and one remote fake seat answer in the same durable conversation.
- Canceling one target leaves its peers running.
- Transport disconnect yields one terminal failed state and no goroutine leak.
- Focused concurrency packages pass with `go test -race`.

### Experience

- Fort opens directly to Conversations and exposes only Conversations and
  Computers as primary product areas.
- The desktop right rail shows only truthful In progress work and persisted
  Scheduled today items; it contains no static agent roster, inferred ETA,
  percentages, or predicted review moments.
- Selecting an active conversation target opens that conversation; selecting a
  legacy or scheduled run opens its bounded details.
- The Today date and timezone are visible, update at the local day boundary, and
  the rail becomes a reachable sheet at narrow widths.
- The conversation sidebar exposes All conversations, Inbox, and collapsible
  Project folders without introducing a separate project-room surface.
- A person can create and rename a Project, start a conversation inside it, and
  move a conversation between Projects or back to Inbox.
- Project grouping never merges conversation history or implies shared agent
  context.
- New conversation always starts at the visible top with composer focus.
- A person can select complete agent seats without coordinating separate agent,
  model, and machine pickers.
- Every agent answer visibly names its agent, exact model, and computer.
- **Everyone** is explicit; Fort never adds a target implicitly.
- Queued, Working, Answered, Failed, and Canceled are announced in text and to
  assistive technology; color and animation are not the sole signals.
- The transcript and composer fit the viewport at 1280×720 and at 390×844 with
  no document-width overflow; targets and primary actions retain 44pt targets.
- Reloading during active or completed work restores the same conversation from
  persisted state.
- The previous Deck controls, run-activity panel, assignment promotion, and
  broader orchestration navigation are absent from the default surface.

### Live acceptance

On the real enrolled MacBook and Mac mini:

1. Fort lists at least one functionally ready seat on each computer.
2. One human turn explicitly targets both computers.
3. Both answers land in the same transcript and each shows the correct exact
   profile and computer.
4. Taking the mini offline makes its next target fail inline without moving to
   the MacBook; the MacBook target still completes.
5. Re-enrolling/restarting the same mini seat and choosing **Recheck and retry**
   completes only that failed target.
6. Browser reload preserves the whole transcript and target history.
7. A persisted schedule for the displayed day appears at its exact local time,
   survives daemon restart, fires once, and links to its resulting run.

No milestone is accepted from `/health`, a process listing, a static roster, or
unit tests alone.

## Non-goals

- No autonomous agent-to-agent loops or agent-authored dispatch.
- No DAG, fanout/fanin editor, planner, capability-plan solver, setup solver,
  playbook selection, gate, backlog, metrics, or promotion flow in the
  shared-chat path.
- No Week grid, capacity forecast, estimated review time, drag scheduling, or
  schedule editor in the primary UI. Schedule creation continues through the
  existing CLI contract; this milestone makes it durable and visible Today.
- No silent provider/model/machine fallback.
- No provider-native session continuation in v1.
- No file attachments, shared filesystem, artifact transfer, email/database
  broker, or external-tool setup workflow.
- No project rooms, nested Projects, multi-Project conversations, Project-level
  agents, Project-wide transcript/context, Project files, or Project workflow.
- No deletion or migration of historical run, flow, playbook, or gate data.
- No native-client redesign or TestFlight release in this milestone.

## Affected files

- `core/conversation/` (new)
- `core/scheduler/scheduler.go`, `core/scheduler/scheduler_test.go`
- `core/store/store.go`, `core/store/store_test.go`
- `control/conversations.go`, `control/conversations_test.go` (new)
- `control/today.go`, `control/today_test.go` (new)
- `ui/contract.go`, `ui/ports.go`, `ui/server.go`, `ui/page.go`
- `ui/*conversation*_test.go`, `ui/page_source_test.go`
- `cmd/fort/wire.go`, `cmd/fort/flow.go`, `cmd/fort/mesh_e2e_test.go`
- focused `exec/cluster`, `exec/remote`, or `exec/node` tests only if the new
  concurrent target acceptance exposes a transport defect
- `README.md` after live acceptance, limited to the shared-chat happy path

## Rollback

Revert the conversation/schedule services and default surface. The additive
Project, conversation, schedule, and occurrence tables can remain unused in
existing databases. Historical run/event/playbook data and the prior HTTP
contracts are unchanged, so rollback requires no data rewrite.
