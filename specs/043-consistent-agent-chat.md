# Spec 043 — One Consistent Agent

**Status:** proposed product direction — feedback only; no implementation is
authorized by this document
**Date:** 2026-08-08
**Decision owner:** Toby
**Depends on:** the durable conversation, immutable-seat, readiness, retry, and
exact-model contracts in Specs 041 and 042 Slice B
**Would supersede if approved:** Specs 040 and 041 only for default product and
UI scope; their accepted persistence, identity, and execution invariants remain
in force

## Executive decision

Continue a tightly bounded Fort experiment, but do not yet recommit to Fort as
a product. Preserve the valuable foundation while proving that a much smaller
Fort earns a place beside provider-native systems.

Fort should be a quiet, first-party conversation and trust layer around mature
agent harnesses. It should not become another general agent runtime, model
loop, memory platform, messaging network, or visual workflow builder.

The next product should do one thing exceptionally well:

> Let one person maintain a dependable relationship with one primary agent,
> across time and devices, without losing the conversation record, identity,
> approved memory, pending work, authorization boundaries, or evidence of what
> actually happened.

Provider harnesses should continue to own model-specific reasoning, tools,
sandboxes, and transient session/compaction mechanics. Fort should own the
canonical context-pack manifest, selection policy, source IDs, and rebuild
behavior, plus the parts that must remain stable when the provider, model,
machine, or interface changes:

- the canonical conversation and event record;
- the exact agent, provider/model, and computer identity;
- explicit task and schedule state;
- source-linked memory and context provenance;
- user approvals, action receipts, and audit;
- provenance-labelled usage and cost;
- cross-device continuity; and
- a calm native chat experience.

This is not a retreat from the useful work already completed. It is a decision
to make that work serve a clear product rather than remain an expanding set of
orchestration primitives.

## Why this is the right reset

### What Fort has already earned the right to keep

The current shared-conversation implementation is substantive, not a mockup.
The repository now contains:

- durable conversations, participants, turns, targets, messages, attempts, and
  terminal attribution in `core/store` and `core/conversation`;
- immutable agent seats that bind profile, exact provider/model, and computer;
- pre-dispatch readiness revalidation that fails closed instead of silently
  moving work;
- independent target execution, cancellation, retry, watchdog, and restart
  reconciliation;
- a runtime seam across native provider CLIs and remote Fort nodes; and
- a working local shared-conversation web surface.

At the 2026-08-08 checkpoint, the current dirty worktree passed `go test ./...`,
focused race tests for conversation-related packages, and `git diff --check`.
This says the foundation is worth narrowing around. It does not say the product
is complete.

### What Fort does not yet provide

The current system is a durable group chat, not yet a consistent personal
agent:

1. **No personal continuity contract.** The prompt contains the frozen
   transcript and a generic participant instruction. There is no stable user
   profile, approved cross-conversation memory, or inspectable context pack.
2. **A hard context cliff.** Spec 041 and `core/conversation` reject a context
   above 65,536 bytes. There is no accepted continuity mechanism beyond that
   boundary.
3. **Execution success is not answer quality.** A provider's final normalized
   message can become the answer without a grounding, hallucination,
   continuity, or false-completion evaluation contract.
4. **Chat and action are conflated.** The shared-chat path can invoke ordinary
   tool-capable CLIs. Fort does not yet distinguish an advisory conversation
   from an authorized external mutation with a durable receipt.
5. **Budget is not measured spend.** The current budget mechanism charges fixed
   process-local dispatch units. It is not token, provider-credit, or dollar
   accounting.
6. **The product is split across surfaces.** Local web serves shared
   conversations, while gateway web and the native Apple clients still expose
   substantial parts of the older board, run, playbook, and Command Deck
   model. FortKit does not yet expose the shared-conversation contract.
7. **Chat startup is coupled to the old control plane.** Rules, flows, graph
   execution, playbooks, planning, and scheduling are still initialized around
   a product whose default surface is now chat.

The resulting risk is that Fort becomes more reliable underneath while
remaining inconsistent to use.

### Deterministic orchestration is necessary but not sufficient

Fort can deterministically choose a seat, persist a turn, retry the same target,
and enforce an approval. It cannot make a generative answer deterministic.

The correct reliability model is therefore:

```text
deterministic state and permissions
        +
bounded model judgment
        +
evidence and receipts
        +
representative evaluations
        =
an agent experience that can earn trust
```

The system should never translate “the runtime exited successfully” into “the
answer is correct” or “the action happened.”

## Product contract

### The user promise

A person should be able to say:

- “I can open Fort and continue the right conversation.”
- “I know which agent, requested model selector, provider-resolved identity to
  the precision exposed, harness version, and computer answered; unavailable
  identity is shown as unknown.”
- “Fort supplies personal durable memory only when it is approved or explicitly
  sourced.”
- “I can see what context Fort supplied or observed, what remains opaque, and
  correct Fort-managed context.”
- “Compaction or a restart never erases the canonical history.”
- “A provider or machine is never swapped silently.”
- “A task does not disappear into conversational prose.”
- “I can separate ongoing work into private channels and see every durable
  scheduled commitment without opening a board.”
- “No user-owned or connected resource, durable memory, or durable workspace
  state changes unless I explicitly authorize an action.”
- “Completed work has a receipt; otherwise Fort says it is unverified.”
- “Usage and cost are labelled by provenance; actual billing is never inferred
  from an estimate.”

Claims about the user, Fort state, or completion of an external action must cite
durable record IDs. Missing or contradictory evidence triggers abstention or a
question. Fort cannot control facts a pretrained model may know or invent; it
can control which personal records it supplies and what it accepts as proof.

### The product is not a public chat network

Fort should preserve the best part of the Telegram experience—simple,
channel-like separation of ongoing conversations—without public discovery,
public profiles, social recommendations, or a directory of strangers.

Search covers only the person's Fort conversations, agents, and explicitly
connected resources. There is no social layer and no reason to encounter an
unknown user's profile image.

### One primary agent, not a default swarm

Every normal Channel targets one **Primary Agent** seat automatically.
The person chooses that seat once in Settings. The binding remains the complete
immutable identity from Spec 041:

```text
Fort profile + exact provider/model + enrolled computer
```

“Primary Agent” is a user-facing role, not an alias that hides the seat. The
header always makes the exact identity available. If the seat is unavailable,
Fort fails closed and offers recheck or an explicit new choice. It never picks
a substitute.

The Primary Agent setting initializes new Channels only. An existing Channel
retains its persisted participant seat until the user explicitly changes that
Channel's membership. Changing Settings never retargets an existing Channel or
rewrites its identity.

### Identity precision

“Exact model” means the most exact identity Fort can observe, not an immutable
claim about hidden serving infrastructure. The target contract records:

- requested model selector;
- provider-resolved selector or revision when exposed, otherwise **unknown**;
- provider/endpoint identity;
- harness name and exact version/build; and
- response model metadata when exposed.

A pinned alias does not prove a frozen server-side revision. The UI must not
collapse requested, resolved, and unknown identity into one stronger claim.

Multi-agent work remains possible through an advanced **Ask another agent…**
action. It is explicit, secondary, and never recursively wakes other agents.
Participant management and **Everyone** are not part of the ordinary composer.

## Minimum primary experience

The default shell contains only:

1. **Channels** — private durable context boundaries, pinned and recent by
   newest activity;
2. **Transcript** — canonical human, agent, system, task, approval, and receipt
   entries;
3. **Composer** — one input and one Send action, targeting the Primary Agent;
4. **Scheduled** — a chronological read surface for durable definitions and
   occurrences across channels;
5. **Needs you** — a small badge or drawer that appears only when a durable
   question, approval, correction, or failed action is actually actionable;
6. **Settings** — Primary Agent, agents, exact models, computers, readiness,
   permissions, spend limits, and data boundaries.

The primary surface does not show:

- Projects or project rooms;
- Today or Week boards;
- assignments, capacity, performance, or metrics dashboards;
- playbooks, DAGs, routers, planners, or setup solvers;
- a raw event log;
- participant or machine pickers on every message;
- schedule authoring, mutation, or calendar/Today-board controls; or
- watch, CarPlay, and voice-specific navigation.

The underlying records can remain available to diagnostics and legacy/admin
surfaces. They should not be deleted during the reset.

### Channel organization

V1 names the durable user-owned boundary **Channel**. A channel is one private
ongoing outcome, topic, or relationship with one canonical transcript, one
immutable Primary Agent seat, and optional explicitly linked schedules. The
existing conversation record may implement that boundary 1:1, but the primary
UI and API must consistently say Channel rather than treating “channel-like”
behavior as an unstated side effect of a Chats list.

A provider session or bounded context segment is not a channel and never
appears as top-level navigation. Phase 2 may add checkpoints or fresh execution
segments without changing the channel ID or fragmenting its canonical history.

People can pin, rename, archive, and search their own channels. Project folders
may return only after observed use shows that those controls are insufficient.
A Project must not become a second context or memory boundary.

### Scheduled visibility

Scheduled is a dedicated chronological destination, not a permanent dashboard
rail. It shows each durable schedule's title, explicitly linked channel or
**System** when no truthful channel link exists, recurrence, timezone, next
fire, enabled/paused definition state, latest occurrence state, and observed
execution identity when available. Opening an item deep-links to its channel,
run, result, or failure evidence.

Phase 1 schedule visibility is read-only. It does not create, edit, pause,
resume, delete, or manually rerun a definition. In particular, it never labels
an action Retry unless a later accepted contract creates a new occurrence;
silently replaying the same occurrence would violate once-only scheduling.
Legacy flow schedules remain visible as **System schedules** and Fort never
guesses a channel binding.

### Needs you

The existing design insight—“what needs me?”—should survive the simplification,
but as a truthfully derived inbox rather than another work-management system.

An item appears only when a durable record is actionable, for example:

- approve or reject a proposed memory;
- answer a blocking question;
- authorize a bounded action;
- inspect a failed action;
- resolve a contradiction; or
- confirm a schedule or commitment.

Opening an item returns to its exact place in the conversation. Resolving it
removes it from Needs you without erasing its history.

## Chat and Act are different operations

### Chat

Chat is advisory and read-only by default. It can reason, retrieve approved
context, search exact history, and draft an action plan. It cannot represent an
external mutation as complete.

Read-only tool use is allowed only through an explicit Fort contract with
bounded scope and recorded results. Chat authority must be enforced through
OS/filesystem, tool, and network boundaries—not a prompt instruction alone. A
provider's own claims are not receipts.

Approved ephemeral sandbox files, provider telemetry, and session logs are
governed by the eligibility matrix and disclosed separately with their
retention. They do not count as Fort durable memory or permission to change a
user-owned resource.

The current runtime seam does not yet express authority mode, sandbox policy,
tool policy, normalized usage, or receipts. Current adapters therefore must not
be assumed read-only. A later Phase 1 implementation spec must either extend
that bounded runtime contract or require and verify a provider-native
read-only/disposable mode, including isolated provider memory/config paths,
before Chat can meet this contract.

### Act

External mutation begins with a separate **Act** operation. Before dispatch,
Fort shows:

- the intended outcome;
- the exact tool or runtime boundary;
- the target resource and scope;
- the data to be disclosed;
- whether the operation is reversible;
- the approval required; and
- the expected evidence of completion.

The user explicitly confirms the operation. Fort then stores the exact approved
request and the resulting receipt. A receipt is system-of-record evidence or an
independent read-after-write result, never model/provider prose. If the provider
reports success without that evidence, the action remains **Unverified**, not
**Completed**.

Act is retried automatically only when the operation has a stable idempotency
key or Fort can deterministically reconcile the system of record first. An
ambiguous timeout or result becomes **Unverified / Needs you**, not an automatic
same-seat retry.

This boundary should be enforced outside the prompt. Memory, model reasoning,
or a provider-native session cannot expand authority.

## Stop calling all state “memory”

Fort should use the following vocabulary in code, specs, evaluation, and UI:

| Layer | Meaning | Canonical owner |
| --- | --- | --- |
| Conversation state | Ordered messages and system events observed and persisted by Fort; provider-internal gaps are explicit | Fort |
| Working context | The bounded context Fort supplies or observes; opaque provider additions are labelled | Fort assembly plus provider context engine |
| Semantic memory | Current facts, preferences, relationships, and decisions | User-approved Fort records |
| Episodic evidence | Immutable prior events, transcripts, artifacts, and outcomes | Fort/source systems |
| Procedural memory | Versioned instructions, Skills, scripts, and operating rules | Reviewed files/packages |
| Task state | Queued, working, blocked, waiting, completed, failed, canceled | Fort state machine |
| Authorization and audit | Permission, approved request, actor, host, observed result, and receipt where the contract exposes them | Fort policy/event ledger |

Compaction changes working context. It never changes canonical conversation
state. A schedule is a commitment, not a memory. A tool permission is policy,
not a preference. A skill is versioned procedure, not a remembered fact.

The conversation ledger is the canonical record of what Fort observed and
persisted. It is not authoritative evidence that the claims inside it are true,
and it does not claim completeness for a provider-internal trajectory. The
table above is a target taxonomy: current schemas do not yet persist every
exact tool argument/result, normalized usage record, or action receipt. Each
missing contract requires a separately approved schema and implementation
spec.

## Memory V1

### Principle

Memory must become more trustworthy before it becomes more intelligent.

V1 does not require embeddings, a vector database, a knowledge graph, or an
autonomous dreaming process. It uses small, inspectable records, exact search,
explicit proposals, and provenance.

### Four V1 artifacts

1. **Raw conversation ledger**
   - immutable and complete for Fort-observed entries;
   - names provider-internal trajectory and context gaps explicitly;
   - searchable by exact text and metadata;
   - never rewritten by compaction or summary.
2. **Pinned profile**
   - a small user-editable set of stable preferences, roles, constraints, and
     privacy boundaries;
   - always visible in Settings;
   - changes are explicit and versioned.
3. **Context checkpoints**
   - continuity artifacts for long conversations;
   - cover an exact range of message IDs;
   - store generator, model, prompt/version, time, and source links;
   - never replace the raw messages;
   - inspectable by the user;
   - corrected only by appending a superseding version, never by rewriting the
     old checkpoint.
4. **Proposed durable memories**
   - facts, decisions, preferences, and commitments the agent believes may be
     useful across conversations;
   - staged in Needs you;
   - approved, edited, or rejected by the user before cross-conversation use.

### Memory record contract

Every durable semantic memory should carry at least:

```text
id
type
statement or structured value
scope
source references
author: human | agent | tool
created_at
observed_at
review_at or expires_at
status: proposed | approved | superseded | archived | rejected
supersedes_id
sensitivity
confidence or verification state
```

The statement alone is never enough. Source, time, scope, and lifecycle are
part of the memory.

### Provider-memory isolation

For a Fort-managed chat, provider cross-session memory, auto-memory,
pre-compaction memory flush, and autonomous consolidation/dreaming must be
fully disabled inside an empty isolated provider home. If a harness requires a
memory mount, Fort may export and mount only a snapshot of already approved,
source-labelled records. A read-only mount of pre-existing provider memory is
not sufficient because it can still inject unapproved content. If a harness
cannot meet this boundary, it is ineligible for the approved-memory lane.

A provider-generated candidate may enter Fort only as a source-labelled
proposal. It receives no cross-conversation authority until the user approves
it. Provider-native memory can be tested in a separate experimental lane, but
must never be silently blended into Fort's approved memory.

### Retrieval ladder

Fort should add retrieval only in this order:

1. exact transcript and metadata search;
2. pinned profile injection;
3. approved memory by deterministic type, person, goal, or conversation scope;
4. source-linked checkpoint retrieval;
5. semantic/vector retrieval only after representative tests demonstrate a
   material miss that simpler retrieval cannot address.

Semantic retrieval remains an adapter behind a Fort contract. It does not
become the source of truth.

### Visible context

Every answer can disclose **Context supplied or observed by Fort**. The
disclosure lists:

- pinned profile version;
- approved memory IDs and source links;
- conversation checkpoint and covered range;
- raw message range;
- retrieved artifacts; and
- provider/model identity;
- provider session/trace IDs when exposed; and
- known observability gaps, including provider-injected or compacted context
  Fort cannot inspect.

The disclosure should be compact by default and complete on demand for what
Fort supplied or observed. It must not imply access to opaque provider context.
A user can correct a memory or exclude it from future use without editing a
hidden prompt.

### Long-conversation continuity

The 65,536-byte hard failure must not be the steady-state experience.

Fort keeps the full canonical ledger and owns the versioned context-pack
manifest, deterministic selection policy, source IDs, and rebuild behavior.
The model receives a context pack containing:

```text
pinned profile
+ relevant approved memories
+ latest context checkpoint(s)
+ recent raw turns
+ explicitly retrieved source evidence
```

A checkpoint is a lossy working-context artifact and must never be presented as
the canonical history. V1 should start a fresh provider execution for each
target and supply the Fort-built context pack. Provider-native continuation or
compaction may be evaluated later as a transient optimization. When used, it is
labelled opaque/non-reconstructable, cannot be required for continuity, cannot
silently affect a frozen retry, and cannot prevent Fort from rebuilding the
same manifest when switching providers.

## Task and schedule V1

Tasks should follow chat and memory, not precede them.

V1 adds a task only when a conversation produces a concrete outcome that must
survive the next turn. A task has explicit owner, state, next action, due or
review time when applicable, source conversation, and completion evidence.

The ordinary UI renders a compact task card in the conversation. It does not
open a board. Needs you shows only tasks that require a human decision.

Schedules are durable triggers attached to a task, review, or channel. They are
not inferred from a model's prose and they fire once per occurrence. Phase 1
exposes existing definitions and occurrences read-only; creating or materially
changing a commitment remains a separately approved later capability.

Visual DAGs, playbook selection, capacity planning, and autonomous task
decomposition remain frozen until a measured use case cannot be represented by
this simpler contract.

## Build, reuse, and freeze

| Decision | Capability |
| --- | --- |
| **Build and own in Fort** | canonical conversation/event ledger; exact immutable seats; deterministic dispatch and same-seat retry; explicit task/schedule state; memory proposal and provenance; action approval and receipts; cross-device sync; usage/cost provenance; quiet native UI |
| **Reuse behind bounded adapters** | Codex, Claude, Hermes, or OpenClaw agent loops; transient provider sessions and compaction; shell/container sandboxes; MCP tools; Agent Skills; provider tracing; optional memory engines; mature durable runtimes if a later enterprise workflow proves the need |
| **Freeze, do not delete** | router and rules UI; DAG/flow authoring; playbooks; capability planner and setup solver; board/backlog/assignment/capacity/performance; Projects; Today/Week; broad native client variants; watch and CarPlay |
| **Do not build now** | generic vector/graph memory platform; another model loop; another compactor; another shell sandbox; public messaging/channel matrix; autonomous skill or memory mutation; automatic multi-agent swarms |

### Harness strategy

The market has changed materially since February and March 2026. Current
OpenClaw and Hermes releases now document more explicit memory, session,
recovery, scheduling, and security contracts. OpenAI and Anthropic now expose
managed or reusable agent-loop, session, compaction, tool, approval, and tracing
primitives. Those platform capabilities are not automatically exposed by
Fort's installed CLI adapters; each adapter must be verified independently.

Feature documentation is not stability evidence. Fort should not choose a
harness from release notes or a demo. It should run a pinned bake-off:

1. Set a hard experiment-credit and dollar cap.
2. Screen pinned Codex, Claude Code, OpenClaw, and Hermes configurations twice
   with 5–8 high-signal cases, including historical failures.
3. Eliminate unsuitable candidates and advance the best two to 10–15 cases,
   repeated three times.
4. Expand only around observed failures.
5. Compare native systems as complete bundles. Attribute a difference to the
   harness only in a separate lane where model, provider, tool policy, and
   memory policy are actually held constant.
6. Run one lane with provider-native memory disabled, then score native memory
   separately.
7. Pin the exact build, configuration, commit or container digest, and resolved
   version behind any moving release-channel label.
8. Score outcome success, unsupported claims, false completion, restart and
   compaction continuity, memory precision, contradiction handling, approval
   compliance, duplicate side effects, tokens/cost, latency, and recovery work.
9. Start read-only and isolated. Do not expose personal or work secrets merely
   to make the test realistic.

Before any harness receives real personal data, approve an eligibility matrix
covering sensitivity, retention/training terms, export/deletion, credential
storage, least-privilege scopes, backup/restore, revocation, and what may enter
model context or traces.

Fort should integrate the winner through the existing runtime seam. A later
provider change should not change Fort's canonical user state.

## Reliability and evaluation contract

### Golden set

Before dogfooding, create a small versioned evaluation set containing:

- ordinary personal questions with known answers;
- a preference that should be recalled;
- an obsolete preference that should not be recalled;
- two contradictory source records;
- a missing fact where the correct behavior is to abstain or ask;
- a sensitive fact outside the current scope;
- a provider-native auto-memory write that must remain disabled or become only
  a source-labelled proposal;
- a conversation with at least 100 turns whose serialized uncompressed context
  demonstrably exceeds 65,536 bytes;
- a provider restart during a turn;
- a provider continuation with opaque context that must be labelled as such;
- an offline Primary Agent seat;
- an attempted unapproved external mutation;
- a provider claim of success without a receipt; and
- a repeated retry that must not duplicate an effect.

### Required measurements

Fort records, where available:

- task/outcome success;
- evidence coverage for factual and completion claims;
- unsupported-claim and false-completion rate;
- memory retrieval precision, missed relevant memory, and contradictions;
- user corrections and whether the same error recurs;
- lost, duplicated, or reordered turns;
- silent identity changes;
- authorization boundary violations;
- provider/model, provider-reported usage, local cost estimate, billing actual
  when independently available, subscription/allocation unknown, latency, and
  retries;
- time saved; and
- human recovery or maintenance time.

Cost data uses four explicit provenance classes:

- **provider_usage** — provider-reported token or resource counters;
- **local_estimate** — Fort's calculation against a versioned price schedule;
- **billing_actual** — an independently imported authoritative bill/receipt;
- **subscription_allocation_unknown** — usage occurred under a subscription or
  credit pool whose per-turn allocation is unavailable.

A provider or SDK cost field is not billing actual unless the billing system
confirms it. When usage or cost is unavailable, Fort stores **unknown**. It does
not convert dispatch units into money or imply precision it does not have.

### Gate A — Channel foundation

The local-web Channel foundation is ready for a 7–14 day advisory trial only
when:

- a new Channel receives the configured Primary Agent while existing Channel
  seats never change through Settings;
- two Channels using the same agent retain byte-disjoint transcript/context
  boundaries;
- the exact seat, model, and computer never change silently;
- the runtime has verified OS/filesystem, tool, network, and provider-memory
  enforcement for its read-only or disposable-sandbox authority mode;
- reload and restart produce no lost, duplicate, or reordered turns;
- same-seat retry cannot rerun a successful peer;
- an offline Primary Agent fails closed;
- the accepted full service retains active durable-scheduler ownership;
- every durable schedule, including paused and non-today definitions, is
  truthfully visible without model calls or schedule mutation; and
- the local web surface uses the canonical Channel/conversation and
  schedule-read contracts.

### Gate B — Continuity and Memory V1

Memory V1 is ready only when:

- a test transcript with at least 100 turns and serialized uncompressed context
  above 65,536 bytes continues without losing canonical history;
- seeded current decisions and preferences are recalled with durable record
  IDs;
- stale, contradictory, missing, and out-of-scope cases cause the specified
  correction, abstention, or question;
- every checkpoint and memory correction is append-only and superseding;
- context-pack manifests can be rebuilt independently of provider compaction;
  and
- no durable personal memory becomes available across conversations without
  the required user approval.

### Gate C — Act

Act is ready only when:

- Chat cannot perform an external mutation;
- Act requires explicit approval of exact scope;
- the runtime contract carries the required authority and receipt fields;
- a receipt is system-of-record evidence or independent read-after-write;
- a provider claim without evidence remains Unverified;
- retry requires an idempotency key or deterministic reconciliation, and an
  ambiguous result becomes Unverified / Needs you; and
- action-truth and authorization evaluations are green.

### Gate D — Tasks and schedules

Tasks and schedules are ready only when:

- their state is durable and never inferred from chat prose;
- each task names its source conversation, owner, next action, and completion
  evidence;
- schedule creation or material change requires the specified approval;
- a persisted occurrence fires no more than once; and
- Needs you contains only currently actionable durable records.

### Trial decision threshold

These are initial hypotheses and should be revisited after a baseline. A
**successful ordinary turn** is one that satisfies a prewritten outcome rubric
and contains no critical evidence, identity, authorization, or
completion-truth failure. Adversarial safety cases and infrastructure-only
probes are scored separately. The proposed minimum sample is 40 adjudicated
ordinary turns; 95% means at least 38 pass.

The relevant trial passes only with:

- zero lost or duplicate messages;
- zero silent identity changes;
- zero unapproved memory or external-action changes for the phases under test;
- zero false claims that an external action completed;
- at least 38 of 40 ordinary turns passing their prewritten rubric;
- predictable spend within the user-set cap; and
- evidence that time saved exceeds maintenance and recovery effort.

## Cross-surface acceptance

Any surface claiming that the experience is available or at parity must use the
same canonical conversation contract and show the same:

- conversation IDs and order;
- messages and target states;
- Primary Agent identity;
- context/memory provenance;
- Needs you items;
- action approvals and receipts; and
- task state; and
- schedule definitions, occurrence order/state, timezone rendering, and
  scheduler ownership disclosure.

FortKit must expose the real `/api/channels` contract—the canonical Primary
Channel façade over existing conversation storage—before a native client can
claim feature parity. `/api/conversations` remains the legacy shared/admin
contract; a web redirect or legacy `/api/chat` path is not native parity.

Phase 1 can accept local web alone. Gateway web, macOS, and iOS may remain
clearly labeled legacy or unavailable until they use the same canonical
contract. They must not claim parity in the interim.

## Delivery sequence

### Phase 0 — Reconcile and freeze

- Reconcile Specs 041 and 042 status with current code and live acceptance
  evidence.
- Make `/` versus `/legacy` and shared-chat versus board positioning explicit in
  the README and architecture docs.
- Freeze Spec 039 planning work, Spec 042 Slice A, Projects, Today/Week and
  schedule-authoring UI, and new client expansion.
- Preserve the current dirty checkpoint without folding unrelated work into
  this spec.
- Run the two-stage harness screen and operate the proposed loop manually in
  one provider-native harness with markdown context cards and explicit memory
  proposals. Record which missing capability causes a measured failure before
  authorizing Fort work.

### Phase 1 — Private Primary Channels

- Add a persisted Primary Agent setting without weakening immutable-seat
  identity.
- Make Channel the explicit 1:1 private conversation/context boundary.
- Reduce the primary shell to Channels, Transcript, Composer, Scheduled, Needs
  you, and Settings.
- Add a read-only all-schedules projection with truthful target, recurrence,
  timezone, next/last fire, occurrence, and scheduler ownership state.
- Preserve the proven full-service durable scheduler; do not promote a
  chat-only service that would leave visible schedules inactive.
- Establish the read-only Chat boundary.
- Verify Channel isolation, restart, retry, offline-seat, schedule visibility,
  at-most-once occurrence truth, and cross-machine continuity.

Phase 1 requires its own surgical implementation spec and explicit approval.
It targets local web first.

### Phase 2 — Continuity and Memory V1

- Add exact transcript search.
- Add the user-editable pinned profile.
- Replace the hard context cliff with source-linked context checkpoints and
  inspectable context packs.
- Add proposed-memory approve/edit/reject/archive.
- Add context-supplied/observed disclosure and correction.
- Run the golden set and memory evaluations.

Phase 2 requires a separate approved schema and implementation spec.

### Phase 3 — Act

- Add Act approval, exact scope, result verification, and durable receipts.
- Pilot one read-only workflow or generate draft text inside Fort before any
  external mutation. Creating a draft in Gmail or another mailbox is an Act and
  requires approval plus a system-of-record or read-after-write receipt.

Phase 3 requires a separate approved authority, receipt, schema, and
implementation spec.

### Phase 4 — Explicit tasks and schedule mutation

- Add the minimal durable task card and Needs you projection.
- Add Channel-bound scheduled prompts and schedule
  create/edit/pause/resume/delete only where a real personal loop needs them.

Phase 4 requires a separate approved task/schedule implementation spec.

### Phase 5 — One real life loop

- Operate one 90-day personal goal through chat, approved memory, daily next
  action, and weekly evidence review.
- Add one external workflow only after the chat/memory loop is dependable.
- Use the results to decide whether any frozen orchestration capability should
  return.

## Stop conditions

Pause implementation and reconsider the product if any of the following remains
true after the trial:

- Fort is used less than three days per week;
- conversations are routinely abandoned for a provider's native UI;
- memory corrections recur without improving later behavior;
- maintenance and recovery consume more time than the system saves;
- provenance-labelled billing actual or local estimate is unpredictable or
  materially higher than native harness use;
- provider switching breaks canonical state;
- the simple UI still requires knowledge of seats, runs, DAGs, or playbooks for
  ordinary use; or
- a critical authorization or completion-truth failure occurs.

Passing these gates justifies continuing Fort as a personal trust layer. It
does not automatically justify rebuilding the broader orchestration product.

## Relationship to existing specs

- **Spec 021:** retain the Go architecture and hard seams; treat the older broad
  control-plane product description as governing until a coordinated approved
  change updates `CLAUDE.md`, `AGENTS.md`, the README, governing-spec
  references, and the `/` versus `/legacy` route story. This spec cannot
  silently make Spec 021 historical.
- **Spec 039:** freeze planner, solver, and setup-expansion work. Keep only the
  readiness facts needed for an exact Primary Agent seat.
- **Spec 040:** historical design evidence; not the default product.
- **Spec 041:** retain durable conversation, target, frozen-boundary, immutable
  seat, no-silent-reroute, retry, cancellation, and attribution contracts.
  Replace Projects, Today, per-turn participant selection, and the broader
  primary shell if this spec is approved.
- **Spec 042:** retain approved exact-model Slice B. Slice A remains deferred;
  **Ask another agent…** requires a separate accepted contract if it changes
  persisted target semantics.

## Decisions required before implementation

1. Approve or reject this narrower product direction. Approval authorizes only
   the direction and drafting the next scoped spec, not implementation.
2. Choose whether to authorize a separate Phase 1 implementation spec.
3. Choose whether Chat must be strictly read-only or may write only inside a
   disposable/provider sandbox before Act.
4. Approve the initial Primary Agent harnesses and experiment cap for the
   bake-off.
5. Choose the first shipping surface; the recommendation is local web for
   contract acceptance, followed by native macOS/iOS through FortKit.
6. Defer Memory V1, Act, new task records, schedule mutation/Channel-bound
   execution, and the first external workflow to their own later decisions
   after the Channel foundation evidence exists; basic schedule visibility is
   part of Phase 1.

No code implementation is authorized by accepting this direction document.

## Current-source notes

These sources describe current capabilities, not guaranteed real-world
reliability. Fort should validate them with the bake-off above.

- OpenClaw: [memory](https://docs.openclaw.ai/concepts/memory),
  [tasks](https://docs.openclaw.ai/automation/tasks), and
  [release channels](https://docs.openclaw.ai/install/development-channels),
  and [releases](https://github.com/openclaw/openclaw/releases)
- Hermes: [memory](https://hermes-agent.nousresearch.com/docs/user-guide/features/memory/),
  [sessions](https://hermes-agent.nousresearch.com/docs/user-guide/sessions/),
  and [releases](https://github.com/NousResearch/hermes-agent/releases)
- OpenAI: [current model guidance](https://developers.openai.com/api/docs/guides/latest-model),
  [conversation state](https://developers.openai.com/api/docs/guides/conversation-state),
  [compaction](https://developers.openai.com/api/docs/guides/compaction), and
  [Agents SDK](https://openai.github.io/openai-agents-python/), including
  [tracing](https://openai.github.io/openai-agents-python/tracing/)
- Anthropic: [Claude Code memory](https://code.claude.com/docs/en/memory),
  [context window](https://code.claude.com/docs/en/context-window),
  [Agent SDK sessions](https://code.claude.com/docs/en/agent-sdk/sessions), and
  [Managed Agents memory](https://platform.claude.com/docs/en/managed-agents/memory),
  plus [model IDs and versions](https://platform.claude.com/docs/en/about-claude/models/model-ids-and-versions)
- LangGraph/LangMem: [persistence](https://docs.langchain.com/oss/python/langgraph/persistence)
  and [memory concepts](https://langchain-ai.github.io/langmem/concepts/conceptual_guide/)
- Agent evaluation: Anthropic,
  [Demystifying evals for AI agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents/)
