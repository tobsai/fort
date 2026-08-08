# Spec 039 — Capability-aware planning and cross-machine execution

**Status:** approved by Toby on 2026-07-24 — implementation in progress; the
full capability-planning lifecycle is not implemented.

**Accepted milestone:** profile-readiness recovery approved by Toby on
2026-07-25 and accepted live on 2026-07-26. Current Codex defaults select the
exact ready `codex:gpt-5.5` profile, while stale or unavailable profiles fail
before a provider starts.

**Catalog v2 approval:** approved by Toby's 2026-07-27 instruction to expose
the latest models already installed with the Codex Mac app. Version 2 changes
only the closed Codex profile rows and verified Codex executable/schema tuple;
it does not add a logical capability, install software, or permit model
substitution.

**Governed by:** [021-fort-native](021-fort-native.md),
[022-multi-machine-orchestration](022-multi-machine-orchestration.md),
[024-mesh-enrollment](024-mesh-enrollment.md),
[036-playbooks](036-playbooks.md), and
[038-cross-machine-command-deck](038-cross-machine-command-deck.md).

## Incident

A Supabase diagnosis received by email exposed several distinct failures:

1. Gmail was functionally available through Himalaya on the Mac Mini, but Fort
   had no live representation of that capability and left the plan on the
   laptop.
2. Static agent claims placed an OpenClaw stage on the Mini even though the
   installed Fort used a stale provider contract.
3. A configured Codex model label reached a laptop CLI that could not run it.
4. Hermes printed a terminal timeout and exited zero, which Fort initially
   reported as success.
5. Interrupting a silent remote provider could leave work running on the peer.

The provider-status, error-propagation, cancellation, process-group, graph
attempt, and OpenClaw session bugs in items 2–5 are repaired separately. The
remaining product gap is generalized logical-capability discovery and
deterministic capability-aware plan placement.

Safe live evidence gathered for this spec on 2026-07-23:

- the Mini's read-only Himalaya inbox probe passed;
- neither machine had Supabase CLI installed;
- the laptop's Codex app-server completed a read-only Supabase
  `list_projects` tool call from an ephemeral, no-turn thread, proving
  connector access but not yet the stronger database-inspection execution
  binding defined below;
- the laptop's live Codex model catalog did not contain the configured
  `gpt-5.6-sol` identifier; and
- the deployed Fort runtime completed a laptop → Mini OpenClaw task and left no
  orphan.

No project names, references, mailbox data, credentials, executable paths, or
probe output belong in Fort's public capability data.

## Approved profile-readiness recovery milestone

The first production milestone is a deliberately narrow vertical slice. It
does not claim that generated capability plans, logical-capability brokers, or
cross-machine handoffs are complete.

Before enabling any broader capability-planning lifecycle, this milestone:

1. attaches one safe correlation ID to each control request and exposes relay
   connection transitions plus bounded failure reasons without logging request
   bodies, credentials, Noise material, or provider output;
2. inventories the exact execution profiles that each enrolled node can run
   under the daemon's execution identity, including provider contract,
   authentication, and exact model readiness;
3. serves authenticated, secret-free node inventory and a control-plane
   aggregate while preserving an explicit old-node result rather than the web
   UI fallback page;
4. checks the selected execution profile immediately before legacy playbook
   dispatch and blocks with a closed reason when it is unavailable;
5. corrects reusable default playbooks through an explicit new revision so a
   5.6 Codex assignment is never sent to Hermes, without overwriting a
   user-edited playbook; and
6. deploys the same inventory protocol to both nodes before the inventory is
   allowed to influence placement.

This milestone retains deterministic routing and single-host placement. It
does not generate a plan, infer a machine from model output, install or
authenticate tools, substitute a model, split work, or expose Gmail/Supabase
as ready. Logical Gmail/Supabase adapters, durable typed setup-versus-split
decisions, guarded remote execution, and bounded handoff receipts remain later
milestones and keep all acceptance criteria below.

The profile-readiness milestone is accepted only when:

- a request can be followed by one correlation ID from the client-facing HTTP
  boundary to durable run creation or one bounded failure response;
- both nodes return authenticated capability JSON, and an old node returns an
  explicit old-node response rather than dashboard HTML;
- unavailable authentication or an unavailable exact model produces zero
  provider starts;
- current default Quick Answer and planning stages select a ready exact
  profile without silent substitution; and
- focused race tests, the full Go suite, and live read-only two-node inventory
  checks pass before restart or rollout.

### Live acceptance findings — 2026-07-26

- The control plane now follows one request ID into durable run creation. A
  laptop-pinned `claude:configured-default` acceptance command completed with
  `FORT_OK` and a terminal `succeeded` run.
- Codex `model/list.isDefault` is the catalog default, not the effective
  `config.toml` override. The inspector now reads typed `config/read` first.
  The laptop's `codex:configured-default` therefore correctly reports
  `model_unavailable`, and preflight rejects it with zero provider starts;
  `codex:gpt-5.5` remains independently ready.
- OpenClaw 2026.7.1-2 deliberately respawns non-help commands with
  `detached: true`. Its config/model readiness commands can consequently leave
  re-parented `openclaw-config` processes in new process groups that Fort
  cannot safely own. The milestone quarantines `openclaw:main` as
  `command_contract_changed` and does not execute that readiness probe until a
  process-private contract exists. Ordinary probe descendants are still
  bounded and process-group terminated.
- The same inventory protocol is running on both enrolled Macs, and the laptop
  now receives protocol-v1 JSON from the Mini rather than `old_node` HTML.
- Fort 0.12.7 is running from the same verified arm64 artifact on both Macs.
  Executable hashing, cached-stage verification, and cache-miss staging now
  stream through a fixed 64 KiB buffer while retaining the 384 MiB hard cap,
  source snapshots, no-follow destination verification, and immutable 0500
  stage mode. After multiple settled refreshes, the laptop daemon's physical
  footprint was 17.3 MiB (21.5 MiB peak) and the Mini's was 15.9 MiB (16.5 MiB
  peak), replacing the 2.0–2.4 GiB whole-file-buffer baseline.
- A laptop-pinned 0.12.7 `hermes:configured-default` command reached a terminal
  `succeeded` run and persisted the exact `FORT-HERMES-OK` response.
- The Mini's Hermes state now resides in the internal
  `/Users/talos/.hermes` directory. Its launchd-owned
  `hermes:configured-default` profile reports ready, and a Mini-local Fort
  command reached a terminal `succeeded` run with the exact
  `FORT-MINI-HERMES-OK` response. The former external-volume tree remains an
  untouched rollback source.
- Default migration appended immutable GPT-5.5 revisions without changing old
  rows or user edits. The accepted live heads are Bug fix revision 2, Feature
  work revision 2, and Quick answer revision 3; Research remains revision 1.
  Repeated daemon restarts appended no further revisions.
- Quick answer revision 3 routed an exact `codex:gpt-5.5` request and persisted
  `FORT-CODEX-55-OK`. Explicitly replaying its unavailable configured-default
  predecessor failed with `model_unavailable` and zero provider starts,
  proving that the profile gate does not substitute silently.
- The profile-preserving gateway was promoted as deployment
  `dpl_5tDm5YkGbYuNHxFTNyLswDKc9hxw`. Authenticated production smoke checks
  returned the expected 405 for `GET /api/req` and 401 for an unauthenticated
  `POST`; the subsequent error and 5xx log queries were empty.
- The profile-preserving Apple client was archived as Fort 1.0.1 build
  `2607262`, passed nested code-sign verification, and was uploaded to App Store
  Connect without warnings or errors. Apple's receipt records the upload as
  successful and processing for TestFlight.

These results accept the bounded profile-readiness milestone. The subsequent
conversation-command-center acceptance found that the untouched Feature work
default still sent its post-approval Design stage to the quarantined
`openclaw:main` profile even when inventory had already reported it absent. The
bounded repair appends a new immutable built-in revision that assigns Design to
the same exact `codex:gpt-5.5` profile as Break down. Migration recognizes only
the shipped, untouched GPT-5.5 default; prior revisions and user-edited
playbooks remain unchanged. This does not accept the full spec: user-authored
unready stages are still vetoed at dispatch, and inventory still does not
perform capability-aware placement.

### Current implementation boundary

Production wiring currently covers correlation IDs, local and authenticated
peer inventory, normalized aggregate snapshots, exact-profile preflight, and
bounded streaming, content-addressed executable staging and binding. The pure
plan decoder, deterministic solver, setup solver, and placement-proof builders
exist under `core/capability`, but have no production coordinator or handler
callers.

The following full-spec layers remain absent: durable capability-plan,
decision, outbox, dispatch, and handoff records; capability-aware placement;
typed setup-versus-split and sign-off clients; guarded capability execution and
handoff receipts; Gmail/Supabase brokers; and the specified inner control
authority/JWS/CSRF protocol. Static `machines.yaml` claims still choose a host,
and profile readiness can only veto that already-selected host. Inventory must
not be described as full capability-aware planning until those layers are
wired and accepted.

## Required outcome

Before substantive work starts, Fort must:

1. discover what every reachable machine can functionally use under the same
   identity and environment as its execution runtime;
2. run a visible planner task that emits a bounded plan whose stages name only
   Fort-owned execution profiles and logical capabilities;
3. validate and persist that plan before placement;
4. deterministically prefer one machine that can run the whole plan;
5. when no single machine can run it, ask whether to add the missing
   agent/capability or run the exact disclosed plan across machines;
6. persist the accepted choice and mapping atomically before dispatch; and
7. revalidate assigned requirements on each target without silently moving
   work on retry, resume, drift, or downgrade.

Routing and placement remain model-free. Only the visible planner and
substantive task nodes invoke a `runtime.Runtime`.

## Invariants

- `POST /api/route` remains pure, synchronous, deterministic, and
  dispatch-free.
- Machine names are never accepted from model output.
- Installed does not mean ready.
- Static `machines.yaml` agent claims never satisfy a capability-aware plan.
- No silent model fallback, profile substitution, installation, authentication,
  or cross-machine split.
- No substantive task dispatches before mandatory durable placement
  resolution.
- A typed placement decision is not a binary plan approval.
- The accepted plan and mapping, not events or current live placement, are the
  source of truth for retry and resume.
- Capability-aware remote execution uses a versioned fail-closed protocol that
  an old node cannot accidentally accept.
- A gateway signing key alone cannot authorize inner native control; the daemon
  also binds authorization to the enrolled initiator static recovered from the
  actual Noise IK session.
- Public inventory is secret-free and contains closed reason codes only.
- Probe output is bounded, parsed in memory, discarded, and never copied into
  errors, events, logs, or API responses.

### Canonical control-plane hashes

Unless a named v1 hash below defines a more specific framing, it is
`SHA-256(ASCII(domain) || 0x00 || RFC8785(exact JSON value))`. JSON object
members are the exact members declared for that hash, arrays retain their
declared semantic order, strings are their exact UTF-8 values, and no displayed
`revision` or digest field is included unless the input object explicitly names
it. Digest-valued wire fields use `sha256:` plus 64 lowercase hexadecimal
characters; ID derivations that explicitly truncate or use base64url retain
their stated representation. This rule removes implementation-dependent
concatenation, map order, omitted-field, and self-hashing behavior.

## Non-goals

- No arbitrary package-manager or shell commands disguised as setup.
- No credentials, account names, project references, executable paths,
  fingerprints, mailbox contents, database rows, or raw probe output in public
  inventory.
- No binary/file artifact transfer in v1.
- No model-generated routing or placement.
- No replacement for mesh enrollment, the encrypted gateway, or existing
  topology configuration.
- No automated remote OAuth or credential entry in v1.
- No implicit approval of a generated plan merely because its placement fits
  one machine.

## Lifecycle and ingress

### Pure route preview

`POST /api/route` continues to resolve only the immutable playbook revision,
task type, configured execution-profile labels, delivery mode, and optional
ordinary plan-gate setting. It does not refresh capabilities, call a model, or
return a generated capability plan.

### Handoff starts capability planning

Every ingress decodes the original direction as one valid Unicode string and
caps its exact UTF-8 representation at 65,536 bytes before durable run
creation. Fort preserves those bytes without Unicode normalization; their
SHA-256 is the direction digest used by preflight, generated/static identity,
remote stages, and authenticated plan detail. An empty direction where the
existing ingress requires text or an oversized/invalid value returns `400` and
creates no run. Static flows and breakdown cannot bypass this bound.

An execution handoff creates a durable run immediately. Non-answer delivery
returns `ChatResult{kind:"flow", run_id, state:"planning"}`; `plan_id` is
omitted until a validated plan exists. Capability planning continues as visible
run work and is observable through the board, run detail, plan view, and SSE.
HTTP remains `200`; this is wire-decodable by old clients but not behaviorally
identical to the former immediate task response.

The lifecycle is:

1. refresh the capability snapshot;
2. deterministically preflight and assign the configured planner execution
   profile;
3. persist and dispatch one visible planner task;
4. validate its durable output and solve placement from the frozen safe
   projection in memory;
5. atomically persist the plan plus auto-placement or typed option set;
6. after placement is durable, present a typed capability-plan sign-off if the
   immutable playbook requires one; and
7. compile and execute the persisted plan.

The capability-placement choice and capability-plan sign-off are separate
typed decisions in the same transactional controller. Setup never counts as
either decision.

Auto-placement does not approve generated content. When immutable
`plan_gate=true`, a `capability_signoff` decision offers only **Approve plan**
and **Reject plan** and must resolve before execution. It is not a legacy graph
gate and cannot be reached by `graph.Executor.Approve/Reject`. When
`plan_gate=false`, that stored playbook policy is the explicit authorization to
execute a validated generated plan without an additional sign-off. V1
capability-plan sign-off is approve/reject only: a non-empty legacy `edit`
returns `409 plan_edit_unsupported`, because changing direction or stage
semantics requires a new plan and placement.

### Planner preflight

The planner is itself one capability-aware task, not a placement exception.
From the frozen inventory, Fort finds machines whose execution-profile offer
and matching no-capability execution-binding offer for the configured planner
profile are both `ready`, then selects the local machine or lowest registry
rank using the single-host rule. Before dispatch it persists
that exact planner profile, provider-native model, machine, catalog/profile
mapping versions, profile and execution-binding revisions, inventory revision,
and a `preflight_revision` hash bound to the run and immutable playbook
revision.

The preflight hash also covers the planner allowlist/pin, plan-relevant profile
projection, candidate/assignment or ordered setup/profile-switch options, and
their semantic presentation fields, plus the canonical planner prompt digest
and server-supplied operation/schema, JSON output bound, and 300-second
one-attempt policy. Full inventory revision remains provenance.

One compare-and-set transaction first verifies that the relevant
`inventory_generation` is still current, then persists that safe projection,
full inventory provenance, preflight revision, exact assignment or typed
option set, audit event, lifecycle state, and durable planner-dispatch work
item before any planner dispatch. If a newer relevant generation won, Fort
recomputes; after three CAS losses it returns a safe retryable conflict. A
restart reuses those records; it does not rebuild preflight from current
inventory.

An optional immutable `planner_machine` pin restricts that selection and never
falls back. The ordinary handoff `machine` pin applies to substantive plan
stages, not to this control-plane planner task. Immediately before planner
dispatch, the selected node performs the same live profile/binding guard. A
guard failure creates a new preflight decision with zero model calls. Planner
retry and crash resume reuse the persisted planner profile/machine; they never
relocate without a new explicit preflight choice.

If no machine can run the configured planner, Fort invokes zero runtimes and
creates a typed `capability_preflight` decision. Its opaque options may contain
only:

- an explicit one-run switch to another currently ready planner profile from
  the playbook's closed planner allowlist, including its exact display label and
  machine;
- catalog-approved setup instructions for the configured planner profile on one
  exact machine; and
- Cancel.

There is no split option for the single planner task. An accepted one-run
profile switch is persisted before dispatch and never edits reusable playbook
configuration. Setup enters the setup/Recheck state; successful Recheck starts
again from inventory refresh and creates a new `preflight_revision`. This
pre-plan decision has no `plan_id` or `plan_revision`.

### Covered ingress

The coordinator is the default for every user or unattended ingress that can
create substantive work:

- web, iOS, and macOS handoff;
- `/api/chat` and `/api/openclaw`;
- CLI `task add` and `flow run`;
- backlog execution;
- watched inbox work and every scheduler callback;
- `/api/breakdown` and CLI `task breakdown`; and
- playbook execution.

These surfaces call one control-layer coordinator. They do not instantiate
`graph.Executor` or a planner directly. Direct graph execution remains an
internal compiler/executor detail after durable capability resolution.

A trusted static flow may skip model inference only when every task node already
declares a schema-valid execution profile, closed requirements,
`output_format`, `max_output_bytes`, one named output, and either no input or
the immediately preceding task's named portable input. Its retry field must be
absent or exactly zero. It still uses inventory, solving, frozen placement, the
same handoff budget, server-supplied one-attempt/900-second stage policies, and
target-side revalidation.

V1 static-flow eligibility is deliberately narrow:

- zero-task flows may run only pure coordinator control nodes with no
  filesystem, command, network, provider, or payload-changing behavior;
- flows with tasks contain 1–16 task nodes whose reduced dependency graph is
  one direct task-to-task chain;
- only schema-declared no-op start/end metadata nodes may surround that chain;
  graph gate/check/transform/fanout/fanin nodes are unsupported in a v1
  capability-planned static flow, while the playbook's top-level `plan_gate`
  becomes the typed capability sign-off defined here; and
- command checks, file checks, arbitrary transforms, binary/file inputs,
  side-effect dependencies, implicit workspace state, or non-task nodes whose
  execution host matters are rejected in v1.

Fort traverses every schema-reachable edge during validation. A branch/joined
task graph, missing portable declaration, zero-task external action, graph
gate/check/transform/fanout/fanin node, existing shell check, or other
unsupported shape fails before dispatch
with `static_dag_unsupported` and a safe blocked decision. Existing flows with
command/file checks must be migrated to cataloged capabilities or pure checks;
Fort never guesses where those checks should run. This keeps one solver and
one disclosed handoff contract instead of inventing implicit data/host
semantics.

Before inventory solving, Fort normalizes an eligible static flow into the
same canonical plan schema, including exact prompts, named inputs/outputs,
output policy, and attempt policy. One transaction persists that immutable
normalized plan, source playbook ID/revision, static source digest, original
direction digest, catalog/profile-mapping versions, and run ID. The digest is
the canonical control-plane hash with domain `fort.static-source.v1` and exact
object `{playbook_id,playbook_revision,normalized_plan}`;
`normalized_plan` is the validated plan JSON value, not an encoded JSON string.
It therefore identifies all three without depending on mutable YAML. Placement
reads only this persisted record. A restart never reparses changed flow YAML;
an unsupported retry or shape fails before that identity transaction and
dispatches nothing.

`/api/breakdown` and CLI breakdown are planner-only generation operations, so
they use the same durable planner preflight/assignment/retry path but do not
recursively generate another capability plan. Their closed model output is
`{"items":[{"title":string,"body":string}]}` with 1–32 items, title ≤160 bytes,
body ≤8 KiB, canonical JSON ≤262,144 bytes, and no unknown fields. Agent,
model, machine, capability, placement, status, or IDs in model output fail
validation; each backlog item is capability-planned only when it is later
dispatched.

The existing human request fields have closed control-plane meaning. Empty
`agent` uses the immutable breakdown policy's planner profile; otherwise only
`claude`, `codex`, `hermes`, or `openclaw` are accepted and map respectively to
their version-1 `configured-default` profile (`openclaw` maps to
`openclaw:main`). That mapped profile is an immutable one-run planner override.
Empty `machine` leaves planner placement unpinned; a non-empty value must equal
one exact enrolled registry machine name and becomes only the immutable
`planner_machine` pin. Unknown values fail `400` before run creation. Neither
field is copied into generated backlog items, planner model output, later item
profiles, nor substantive machine placement. The normalized override/pin and
request digest are persisted in breakdown preflight identity and proof.

At startup Fort resolves exactly one Fort-owned
`breakdown_policy:{id:"fort.system.breakdown",revision,planner_profile,planner_profile_allowlist}`
from the daemon's configured planner profile/allowlist and catalog-mapping
version. Its revision is the canonical control-plane hash with domain
`fort.breakdown-policy.v1` and exact object
`{id:"fort.system.breakdown",planner_profile,planner_profile_allowlist,catalog_version,profile_mapping_version}`;
the displayed `revision` is excluded from its own input. No user playbook or
router result is implied. Run creation snapshots that exact policy. For
breakdown preflight, this policy ID and revision occupy the generic
`playbook_id` and immutable `playbook_revision` hash slots respectively; no
synthetic or absent playbook revision is permitted. Empty `agent` selects its
`planner_profile`; a non-empty closed human override must also appear in its
allowlist or fails `400`. A later config change creates a new policy revision
but never alters an existing breakdown run.

The planner output is persisted first. One transaction inserts all normalized
items with unique keys `(breakdown_run_id,item_index)`, marks ingestion
complete, appends the event, and completes the breakdown run. A retry/restart
returns the same rows; partial ingestion is impossible. The existing
asynchronous result keeps `run_id` and gains the closed state.

For Quick Answer, the handler joins the durable run for a configured 30-second
wait (bounded to 5–120 seconds):

- terminal success during the wait returns existing
  `kind:"answer"`, the answer, `run_id`, and `state:"succeeded"`;
- a non-signoff typed choice returns `kind:"flow"`, empty/omitted answer,
  `run_id`, and `state:"needs_user"`; a generated-plan sign-off returns
  `state:"awaiting_plan_approval"`;
- wait expiry returns `kind:"flow"`, empty/omitted answer, `run_id`, and
  the current nonterminal run state (`planning`, `rechecking`,
  `awaiting_plan_approval`, `running`, `canceling`, or `blocked`);
- terminal failure/cancel returns `kind:"flow"`, empty/omitted answer,
  `run_id`, and `state:"failed"` or `"canceled"`.

Pending answers complete through run detail plus SSE/polling. Client disconnect
after durable creation does not erase or silently cancel the run. Old Quick
Answer clients can decode the existing `flow` kind/run ID but may show their
legacy pending-flow notice, so updated web/Apple clients must deploy before the
feature flag becomes default. Unattended ingress always uses the asynchronous
path rather than guessing.

Control-only mode may route and queue, but it reports capability state as
`unknown/no_execution_plane` and cannot plan, recheck, set up, or execute.

`FORT_CAPABILITY_PLANNING=0` is the rollback switch for the legacy path. It is
not the default after rollout.

## Closed catalog

### Logical capabilities and adapters

The Fort-owned catalog separates logical requirements from adapter
implementations. Initial logical IDs are:

- `email.gmail.read`
- `database.supabase.inspect`

Initial adapters are:

- `profile.claude.native`, `profile.codex.native`,
  `profile.hermes.native`, and `profile.openclaw.main`;
- `email.gmail.read.himalaya-broker`; and
- `database.supabase.inspect.codex-broker`.

`database.supabase.inspect.cli` is reserved but does not satisfy the v1
capability: the currently documented CLI project-list command proves management
visibility, not read-only database inspection.

Each adapter declares:

- whether it produces execution-profile offers or logical-capability offers;
- the profile IDs or logical IDs it satisfies;
- which execution profiles may use it;
- an exact, bounded, side-effect-free or read-only probe contract;
- timeout, concurrency, cache, and backoff policy;
- safe reason mapping with deterministic precedence;
- optional versioned setup instructions; and
- a parser that emits only normalized state.

### Stage execution bindings

Independent offers are not assumed composable. The catalog owns closed stage
binding templates that name one agent, one Fort runtime contract, and the exact
logical-adapter set that contract can expose together. Initial templates are:

| Binding ID | Agent | Runtime contract | Logical adapters |
| --- | --- | --- | --- |
| `claude-native` | Claude | native CLI | none |
| `codex-native` | Codex | native CLI | none |
| `hermes-native` | Hermes | native CLI | none |
| `openclaw-main` | OpenClaw | tested main agent | none |
| `codex-appserver+gmail` | Codex | isolated Codex app-server plus Fort dynamic-tool broker | Gmail/Himalaya |
| `codex-appserver+supabase` | Codex | isolated Codex app-server plus Fort dynamic-tool broker | Supabase connector |

A stage is eligible only when one ready binding covers its complete requirement
set and matches the profile's agent. V1 deliberately has no binding that
combines Gmail/Himalaya and Supabase in one model turn; the planner must emit
separate stages with a disclosed bounded handoff. Adding a combination requires
a tested catalog-version change.

The Gmail and Supabase rows do **not** give a model ordinary shell or raw
connected-app access. The substantive Codex thread exposes only the named
Fort-owned dynamic-tool namespace described below. Native Codex and OpenClaw
remain valid no-capability execution profiles, but v1 does not claim that
either can safely consume Gmail merely because it can launch `himalaya`.

### Execution profiles and model identity

A generated stage names a closed `profile` ID, not a free-form agent/model
pair. A profile contains:

```json
{
  "id": "codex:gpt-5.5",
  "agent": "codex",
  "selection": {"kind": "model", "model_id": "gpt-5.5"},
  "display_name": "Codex · GPT-5.5"
}
```

For `kind: "model"`, `model_id` is the exact provider-native identifier passed
to the CLI. `kind: "provider_model"` supplies separate tested provider and
model IDs. `kind: "configured_default"` explicitly selects and fingerprints
that provider's configured default. `kind: "configured_agent"` names a tested
provider-owned selector such as OpenClaw's `main`; it is not a model alias.
Display labels are never passed through as provider/model IDs.

Catalog/profile-mapping version 2 is closed. It preserves every v1 row and adds
the two provider-native GPT-5.6 profiles advertised by the approved Codex Mac
app executable:

| Profile ID | Provider selection | Accepted legacy label |
| --- | --- | --- |
| `claude:configured-default` | configured default | empty |
| `claude:sonnet` | model `sonnet` | `Sonnet` |
| `claude:opus` | model `opus` | `Opus` |
| `codex:configured-default` | configured default | empty |
| `codex:gpt-5.5` | model `gpt-5.5` | none |
| `codex:gpt-5.6-sol` | model `gpt-5.6-sol` | `5.6 Sol` |
| `codex:gpt-5.6-terra` | model `gpt-5.6-terra` | none |
| `codex:gpt-5.6-luna` | model `gpt-5.6-luna` | none |
| `hermes:configured-default` | configured default | empty |
| `hermes:openai-codex/gpt-5.6-sol` | provider/model `openai-codex` + `gpt-5.6-sol` | `Codex 5.6 Sol` |
| `openclaw:main` | configured agent `main` | empty or `Fable` |

The `Fable` mapping preserves the already deployed OpenClaw-main behavior; it
does not claim that Fable is a provider model. Adding or changing a row requires
a catalog-version change and contract tests.

### V2 compatibility matrix

Compatibility is catalog data, not an implementation-defined version range.
Catalog version 2 accepts only the following tuples. On this Mac, Fort resolves
Codex from the already-installed app resources before an older Homebrew CLI;
the normal immutable staging and executable-drift checks remain authoritative.

| Adapter or binding | Platform | Executable/protocol identity | V2 result |
| --- | --- | --- | --- |
| `profile.claude.native` | `darwin/arm64` | Claude Code `2.1.207`; the parsed `claude -p --help` and `claude auth status --json` contracts below | eligible for the cataloged Claude profiles |
| `profile.hermes.native` | `darwin/arm64` | Hermes Agent `0.15.1`; the parsed help/config/status contracts below | eligible for the cataloged Hermes profiles |
| `profile.openclaw.main` | `darwin/arm64` | OpenClaw `2026.7.1-2`; the parsed agent/config/model contracts below | eligible only for `openclaw:main` |
| `email.gmail.read.himalaya-broker` | `darwin/arm64` | Himalaya `1.2.0`; the parsed v1.2 account-scoped envelope/preview contract below | eligible for the Gmail logical offer |
| `profile.codex.native` | `darwin/arm64` | version output `codex-cli 0.146.0-alpha.9.2`; normal 275-file schema-bundle digest `617822e63708afdfcfd539255f34ffb31f07cd4172743bcfc62fc7e88bf976aa` | eligible for no-capability Codex profiles |
| `codex-appserver+gmail` | `darwin/arm64` | `codex-cli 0.146.0-alpha.9.2`; experimental 349-file schema-bundle digest `16bb47445caca91a3316a8b60ff9e0f9918918b3bb352cfa00f07c825a958130`; `dynamicTools`, empty `selectedCapabilityRoots`, and the exact Gmail namespace schema in this spec | eligible only after profile, Gmail, and isolation guards pass |
| `codex-appserver+supabase` | `darwin/arm64` | `codex-cli 0.146.0-alpha.9.2`; the same experimental bundle digest; exact root selection, `dynamicTools`, and the exact Supabase schemas below | eligible only after profile, Supabase, and isolation guards pass |

For the Supabase raw broker, `supabase.list_tables` must have exactly
`{project_id:string,schemas:string[],verbose:boolean}` and
`supabase.get_logs` must have exactly
`{project_id:string,service:"api"|"branch-action"|"postgres"|"edge-function"|"auth"|"storage"|"realtime"}`.
Requiredness, additional-property behavior, method names, and result/error
envelopes are part of the recorded schema fingerprint. Fort then exposes only
the narrower `public` and `api|postgres` policy defined below.

The Codex tuple is reproduced from an empty private output directory using
exactly `codex app-server generate-json-schema --out <dir>` for the normal
bundle and exactly
`codex app-server generate-json-schema --experimental --out <dir>` for the
capability bundle. No config/feature override is permitted. The normal and
experimental aggregate-file SHA-256 values are respectively
`3ec0a6d3f7bcf3c8a764c555140fcba07897094b8f0a4528dd23f86fbccea812`
and
`a54004ee2cd4bf96cc6c02ba43e703778f52cf7b11113dd42f9d1aa6a578fa8b`,
but the full canonical bundle digest in the table is authoritative.

To compute that digest, recursively collect every regular `.json` file,
normalize its relative path to UTF-8 `/` separators, sort paths by unsigned
UTF-8 bytes, parse each file, and encode it with RFC 8785 JSON Canonicalization
Scheme. Hash the byte domain `fort.codex-schema-bundle.v1` followed by one NUL,
then for every file an unsigned 64-bit big-endian path-byte length, path bytes,
unsigned 64-bit big-endian canonical-content length, and canonical content
bytes. The exact file count is part of the tuple. Two independently generated
bundles produced the recorded values; fixtures store the canonical manifest.

The implementation parses the other version and command contracts rather than
matching free-form text. An unlisted platform, executable version, bundle
digest/file count, required method, or raw-tool schema is
`unavailable/incompatible_version` (or
`unavailable/unsupported_platform` for the platform) and contributes no ready
binding. Supporting another tuple requires a catalog-version bump, fixtures,
negative isolation tests where applicable, and a rolling node upgrade before
the tuple can be selected.

Each immutable playbook resolution also carries:

```json
{
  "planner_profile": "claude:configured-default",
  "planner_profile_allowlist": [
    "claude:configured-default",
    "codex:gpt-5.5"
  ],
  "planner_machine": ""
}
```

The first profile is configured; later entries are explicit one-run
alternatives in order. For migration, legacy `FORT_PLANNER=<agent>` maps only
to that agent's `configured-default` profile (`openclaw` maps to
`openclaw:main`). An unknown agent/model label is `profile_unmapped`; no default
agent is guessed after resolution.

Legacy playbook labels are accepted only through that versioned mapping. An
unmapped label is `setup_required` with reason `profile_unmapped`. A mapped but
unavailable model is `unavailable` with reason `model_unavailable`. A planner
profile can be switched only through the explicit preflight choice. Changing a
persisted stage profile would be a semantic plan amendment; v1 does not support
that mutation. The user must cancel, change the reusable playbook/profile
allowlist, and start a new run. Fort never treats it as setup or selects a
replacement automatically.

The planner receives only the closed profile IDs that the selected playbook
permits. Its own configured profile must be ready before it is dispatched.

Execution-profile readiness is inventoried separately from logical-capability
readiness. A machine can therefore report that its Codex executable is usable
while `codex:gpt-5.5` is ready and another configured Codex model is
unavailable. Binary presence or a generic `agent.codex` flag never satisfies an
exact profile.

## States and safe reasons

A capability or execution-profile offer state is one of:

- `ready` — the complete bounded contract passed;
- `setup_required` — the adapter exists but configuration, authentication, an
  explicit profile mapping, or approved setup is needed;
- `unavailable` — absent or incompatible; or
- `unknown` — stale, unreachable, unsupported protocol, or not observed.

Closed public reason codes are:

- `absent`
- `auth_required`
- `cancellation_unconfirmed`
- `capability_drift`
- `command_contract_changed`
- `dispatch_state_unknown`
- `handoff_limit_exceeded`
- `handoff_state_unknown`
- `incompatible_version`
- `model_unavailable`
- `no_execution_plane`
- `old_node`
- `output_limit_exceeded`
- `planner_failed`
- `planner_invalid_output`
- `planner_timed_out`
- `plugin_unready`
- `probe_failed`
- `probe_timed_out`
- `profile_unmapped`
- `project_unavailable`
- `runtime_failed`
- `setup_not_automated`
- `solver_limit_exceeded`
- `stale`
- `static_dag_unsupported`
- `unreachable`
- `unsupported_platform`

If several checks fail, precedence is:

1. `unsupported_platform`
2. `no_execution_plane`
3. `old_node`
4. `dispatch_state_unknown`
5. `handoff_state_unknown`
6. `unreachable`
7. `stale`
8. `absent`
9. `incompatible_version`
10. `command_contract_changed`
11. `auth_required`
12. `cancellation_unconfirmed`
13. `profile_unmapped`
14. `model_unavailable`
15. `project_unavailable`
16. `plugin_unready`
17. `planner_timed_out`
18. `planner_failed`
19. `planner_invalid_output`
20. `runtime_failed`
21. `probe_timed_out`
22. `probe_failed`
23. `setup_not_automated`
24. `static_dag_unsupported`
25. `solver_limit_exceeded`
26. `output_limit_exceeded`
27. `handoff_limit_exceeded`
28. `capability_drift`

Normalization chooses the first reason in this total order when independent
checks fail simultaneously. Target-guard drift explicitly sets
`capability_drift`; it is not inferred from a lower-level probe tie.

### Closed readiness predicates

The top-level offer `state` and first-precedence `reason` are a presentation
summary, not sufficient setup-solver input. Every successfully observed
profile, logical-capability, and execution-binding offer also publishes its
complete cataloged non-secret predicate vector. One row is exactly:

```json
{
  "id": "predicate.codex.authenticated-subject.v1",
  "resolution": "probe",
  "state": "satisfied",
  "reason": "",
  "depends_on": ["predicate.codex.native-contract.v1"],
  "remedy_effect_ids": ["effect.codex.authenticated-subject.v1"]
}
```

`resolution` is exactly `probe` or `derived`. Predicate state is exactly
`satisfied`, `unsatisfied`, or `blocked`.
`satisfied` requires `reason:""`. `unsatisfied` means its bounded probe ran and
failed with one closed public reason. `blocked` means the probe could not
safely run because one or more declared dependencies were not satisfied; its
`reason` is the cataloged conditional reason that would apply if that
`probe` predicate still needed setup after its dependencies, not an assertion
that private state was observed. A `derived` predicate has no probe or remedy:
it is `blocked` with `reason:""` until all dependencies are satisfied, then is
deterministically `satisfied`. `depends_on` and `remedy_effect_ids` are
complete, deduplicated, and in catalog order. A predicate with no approved
instructions has
`remedy_effect_ids:[]`. There are at most four predicates, four dependencies
per leaf predicate, 32 dependencies per binding predicate, and two remedy
effects per predicate.

Every probe-predicate catalog entry has one deterministic `blocked_reason`.
It is the first reason in the global precedence table among all remedy rows
matching that exact target and predicate. Thus v1 uses `auth_required` for the
blocked Codex/Claude authentication, Hermes/OpenClaw configuration, Gmail, and
Supabase-readonly predicates; `model_unavailable` for an exact blocked Codex
model; and `incompatible_version` for a blocked app-server binding predicate.
Root predicates have no dependencies and therefore never publish `blocked`.
This rule, not map/table iteration order, supplies the conditional reason that
enters inventory, deficit identity, and hashes.

The node runs every independent bounded probe even after another predicate
fails. It never hides authentication, model, or configuration requirements
behind the top-level precedence reason. Catalog version 1 has these closed
graphs:

- every Codex profile: `predicate.codex.native-contract.v1`, then
  `predicate.codex.authenticated-subject.v1`, then
  `predicate.codex.model.<exact-profile-id>.v1`;
- every Claude profile: `predicate.claude.native-contract.v1`, then
  `predicate.claude.authenticated-subject.v1`; the tested aliases are command
  contract, not separately asserted entitlement;
- every Hermes profile: `predicate.hermes.native-contract.v1`, then
  `predicate.hermes.provider-model.<exact-profile-id>.v1`;
- `openclaw:main`: `predicate.openclaw.native-contract.v1`, then
  `predicate.openclaw.main-ready.v1`;
- Gmail: `predicate.himalaya.preview-contract.v1`, then
  `predicate.gmail.selected-imap-preview-read.v1`;
- Supabase: `predicate.codex.capability-runtime.v1`, then
  `predicate.supabase.selected-project-readonly.v1`; and
- each execution binding: one intrinsic
  `predicate.binding.<exact-binding-id>.v1` whose declared dependencies are
  every predicate of its exact profile and logical leaves.

The four native no-capability binding predicates are `derived`; the two Codex
app-server binding predicates are `probe` because their experimental schema,
broker, and isolation contracts are independently checked.

The angle-bracket component is replaced only by a closed catalog ID. A
composite does not republish its leaf predicate rows: its top-level reason may
reflect a leaf failure, but its own vector contains only the intrinsic binding
predicate. That predicate is `blocked` until all leaf dependencies are
symbolically or actually satisfied. This preserves causal ownership while
allowing one machine-wide effect to cover the same underlying requirement in
several target offers.

An offer is `ready` only when all of its own predicates and all declared leaf
dependencies are `satisfied`. Otherwise its public reason is the first
precedence reason among actually `unsatisfied` predicates and leaf offers;
`blocked` conditional reasons do not outrank the prerequisite that blocked
them. A missing, malformed, incomplete, duplicated, cyclic, out-of-order, or
catalog-mismatched vector makes the offer `unknown/command_contract_changed`
and supplies no setup candidate.

## Binding revision canonicalization

Every profile, logical-capability, and composite execution-binding revision is
an opaque node-local HMAC-SHA-256 under one persisted node key and a distinct
domain separator. Canonical inputs are length-delimited, versioned, sorted
where declared, and limited to stable dispatch semantics:

- catalog/profile-mapping version and adapter/binding/profile IDs;
- held executable content digest and parsed executable/protocol version;
- normalized non-secret configuration effects that change invocation;
- stable local credential, account, project-root, or authenticated-subject
  handles, represented only by opaque internal IDs or their HMACs;
- exact selected provider/model identity;
- dynamic-tool schema, service/folder/schema allowlist, broker isolation, and
  tool-policy versions; and
- for a composite, the exact profile revision plus sorted logical-capability
  revisions.

Every revision excludes executable paths, raw config, credential values,
access/refresh/session tokens, token expiries, secret bytes, raw probe results,
timestamps, durations, caches, and reachability. Rotating an ordinary token
without changing its stable subject/account/root handle does not create drift.
Changing that handle, executable identity, model, schema, or policy does.
Rotating or losing the persisted node HMAC key invalidates all ready proofs on
that node; the node must publish new revisions before dispatch.

The composite revision is derived only from its catalog/binding ID, exact
profile revision, sorted logical revisions, and non-secret schema/isolation/
policy versions. It never independently re-hashes every private broker input:
those inputs affect the composite only through their stable leaf revisions.

## Inventory model

### Logical capability offer

```json
{
  "id": "email.gmail.read",
  "adapter": "email.gmail.read.himalaya-broker",
  "state": "ready",
  "binding_revision": "opaque:...",
  "available_through": [
    "codex-appserver+gmail"
  ],
  "reason": "",
  "predicates": [
    {
      "id": "predicate.himalaya.preview-contract.v1",
      "resolution": "probe",
      "state": "satisfied",
      "reason": "",
      "depends_on": [],
      "remedy_effect_ids": ["effect.himalaya-1.2.0-preview.v1"]
    },
    {
      "id": "predicate.gmail.selected-imap-preview-read.v1",
      "resolution": "probe",
      "state": "satisfied",
      "reason": "",
      "depends_on": ["predicate.himalaya.preview-contract.v1"],
      "remedy_effect_ids": ["effect.gmail.selected-imap-read.v1"]
    }
  ]
}
```

`available_through` contains closed binding IDs in catalog order and is derived
from an agent/runtime-specific contract. Executable presence alone cannot
assert that an execution profile can use or combine the capability.
`binding_revision` follows the normative canonicalization above and covers the
logical adapter's stable invocation binding without exposing which private
input changed.
A ready logical offer requires a non-empty opaque revision; every non-ready
logical offer requires `binding_revision:""`.

### Execution profile offer

```json
{
  "id": "codex:gpt-5.5",
  "agent": "codex",
  "adapter": "profile.codex.native",
  "state": "ready",
  "binding_revision": "opaque:...",
  "reason": "",
  "predicates": [
    {
      "id": "predicate.codex.native-contract.v1",
      "resolution": "probe",
      "state": "satisfied",
      "reason": "",
      "depends_on": [],
      "remedy_effect_ids": ["effect.codex.capability-0.146.0-alpha.9.2-16bb4744.v2"]
    },
    {
      "id": "predicate.codex.authenticated-subject.v1",
      "resolution": "probe",
      "state": "satisfied",
      "reason": "",
      "depends_on": ["predicate.codex.native-contract.v1"],
      "remedy_effect_ids": ["effect.codex.authenticated-subject.v1"]
    },
    {
      "id": "predicate.codex.model.codex:gpt-5.5.v1",
      "resolution": "probe",
      "state": "satisfied",
      "reason": "",
      "depends_on": ["predicate.codex.authenticated-subject.v1"],
      "remedy_effect_ids": ["effect.codex.model-ready.codex:gpt-5.5.v1"]
    }
  ]
}
```

A profile offer is `ready` only when the exact native provider contract,
authentication check available to that provider, and provider-native
`model_id` check all pass. The closed profile definition supplies the adapter
and model identity; inventory never carries an executable path, credential, or
provider output. `binding_revision` follows the normative canonicalization
above. It changes when the dispatch identity changes but is neither an
executable/account fingerprint nor comparable across machines.
A ready profile requires a non-empty opaque revision; every non-ready profile
requires `binding_revision:""`. The wire never omits its complete predicate
vector.

### Execution-binding offer

The node also publishes the composite contracts the solver may actually use:

```json
{
  "id": "codex-appserver+gmail",
  "profile": "codex:gpt-5.5",
  "capabilities": ["email.gmail.read"],
  "state": "ready",
  "binding_revision": "opaque:...",
  "reason": "",
  "predicates": [
    {
      "id": "predicate.binding.codex-appserver+gmail.v1",
      "resolution": "probe",
      "state": "satisfied",
      "reason": "",
      "depends_on": [
        "predicate.codex.native-contract.v1",
        "predicate.codex.authenticated-subject.v1",
        "predicate.codex.model.codex:gpt-5.5.v1",
        "predicate.himalaya.preview-contract.v1",
        "predicate.gmail.selected-imap-preview-read.v1"
      ],
      "remedy_effect_ids": ["effect.codex.capability-0.146.0-alpha.9.2-16bb4744.v2"]
    }
  ]
}
```

This `binding_revision` follows the composite rule above. The node emits one
row per eligible profile/binding combination. The solver consumes these rows
rather than reconstructing composability from independent profile and logical
offers.
A ready execution binding requires a non-empty opaque revision; every non-ready
binding requires `binding_revision:""`. Its predicate vector contains only its
cataloged intrinsic predicate; leaf dependencies remain owned by the referenced
profile and logical offers.

### Machine inventory

```json
{
  "name": "taloss.mac.mini.lan",
  "local": false,
  "registry_rank": 1,
  "reachable": true,
  "protocol_version": 1,
  "catalog_version": 1,
  "profile_mapping_version": 1,
  "state": "ready",
  "reason": "",
  "observed_at": "2026-07-23T00:00:00Z",
  "profiles": [],
  "offers": [],
  "bindings": []
}
```

Machine state is `ready`, `partial`, or `unknown`. Non-null `profiles: []` and
`offers: []` mean a successfully observed machine with no profile or capability
offers; they never mean unreachable, unsupported, or stale.

Machine state is `unknown` when reachability, identity, or protocol state is
unknown. Otherwise it is `ready` only when every configured catalog probe
scheduled for that machine is `ready`; it is `partial` when any profile,
logical offer, or execution binding is non-ready or when all three collections
are empty. Machine reason is the highest-precedence reason among its non-ready
observations.

When no registry exists, the coordinator creates one synthetic local inventory
entry using the configured node name, registry rank `0`, and no public URL.

V1 accepts at most 16 execution machines, 64 profile offers per machine, 64
logical offers per machine, 128 execution-binding offers per machine, and a
512 KiB node response. Registry ranks are unique positive integers for peers
and are validated before publication; duplicate names, node IDs, or ranks fail
configuration. Catalog order, machine name byte order, and unique registry rank
provide every remaining tie-break.

### Snapshot and revision

```json
{
  "catalog_version": 1,
  "profile_mapping_version": 1,
  "revision": "sha256:...",
  "observed_at": "2026-07-23T00:00:00Z",
  "local_machine": "tobiass.macbook.pro.lan",
  "machines": []
}
```

The revision hashes a canonical projection containing every solver input:

- catalog version;
- profile-mapping version;
- local machine identity;
- machine name and registry rank;
- reachability plus protocol/catalog/profile-mapping state; and
- normalized, sorted, deduplicated profile and logical-capability offers.

Each profile projection includes ID, agent, adapter, state, reason,
`binding_revision`, and the exact ordered predicate vector. Each logical-offer
projection includes ID, adapter, state, reason, `binding_revision`, sorted
`available_through`, and the exact predicate vector. Each execution-binding
projection includes ID, profile, sorted capability IDs, state, reason,
`binding_revision`, and its exact intrinsic predicate vector.

The projection excludes `observed_at`, probe durations, raw diagnostics,
fingerprints, paths, URLs, and all credentials. Refreshing unchanged state
therefore produces the same revision.

## Probe execution

### Shared command identity

Native dispatch and capability probes share one injected command resolver and
environment policy. It:

1. resolves symlinks to an internal absolute regular-file path;
2. opens that file without following another link, hashes its bytes with
   SHA-256, captures device/inode/size metadata around the read, and rejects a
   replacement race;
3. parses the bounded version output into the adapter's tested version range;
4. runs every adapter check against that held executable identity and daemon
   environment; and
5. launches from the held executable identity, never by re-resolving its
   pathname.

The path, file metadata, and fingerprint never leave the process.
The platform launcher must use a proven held-handle execution primitive, or an
immutable Fort-owned staged copy created from that handle in a private `0700`
directory, fsynced, reopened without links, byte-reverified, and made
non-writable before spawn. The staged file is addressed by its content digest
and is never replaced in place. A platform without one of these contract-tested
launch paths is `unavailable/unsupported_platform`; reopening a matching path
and then spawning that path is forbidden because it preserves a check-to-exec
race.

`RunSpec.Env` cannot override executable search, authentication, provider
configuration, account selection, plugin roots, or other
capability-relevant environment keys. An adapter may add a closed tested
environment key only by including its normalized non-secret effect in
`binding_revision` and using the identical value for probe and dispatch.

### Cadence and bounds

- reachability is attempted once at startup; after each attempt settles, the
  background scheduler starts the next attempt exactly 60 seconds later unless
  failure backoff delays it;
- cheap executable/version checks use a 60-second TTL;
- background functional network probes use a five-minute cache;
- immediately before planning, all profile adapters relevant to the planner and
  permitted stage profiles, plus the closed logical-capability catalog, bypass
  that background cache when their coordinator receipt age exceeds 60 seconds;
  this planning refresh honors failure backoff;
- setup and explicit **Recheck** invalidate the selected adapter cache;
- probes are single-flight per adapter, limited to two concurrent probes per
  machine, and have an adapter timeout no greater than 15 seconds;
- failure backoff starts at 30 seconds, doubles to a 15-minute cap, and resets
  after success; explicit Recheck bypasses backoff;
- every local or remote target guard performs a live uncached functional check
  immediately before dispatch, sharing only an already-running identical
  single-flight check;
- one failure never blocks the rest of the snapshot; and
- a refresh publishes atomically only after every scheduled probe settles.

### Native agents

Native provider adapters produce execution-profile offers. Readiness includes
the exact command surface, authentication state when the CLI exposes a
non-generative check, and configured model compatibility. Provider help alone
is insufficient.

Claude's tested contract runs:

```sh
claude -p --help
claude auth status --json
```

The help parser requires Fort's print/stream/model/permission flags and the
closed `sonnet`/`opus` aliases. The auth result must report a supported logged-in
first-party account under the daemon environment; all identity fields are
discarded. Readiness means authenticated and version-recognized, not hidden
model entitlement, and Fort never configures `--fallback-model`.

Hermes's tested contract runs:

```sh
hermes --help
hermes config check
hermes status --deep
```

The version-gated parser requires Fort's one-shot/provider/model/hook flags,
valid configuration, a selected provider/model, and healthy matching
authentication. Output is bounded and discarded. A provider/model override
profile is ready only when the currently configured pair reported by
`hermes status --deep` exactly equals that profile's provider/model pair. V1
does not infer that a dispatch-only override is authenticated. Dispatch still
passes that identical explicit pair; any other pair is `model_unavailable`.
`hermes model --refresh` is interactive and never used as a probe.

OpenClaw uses the daemon-resolved executable and verifies, without a model turn:

```sh
openclaw agent --help
openclaw config validate
openclaw models status --agent main --check
```

The version-gated help parser requires every flag Fort dispatches:
`--local`, `--agent`, `--session-id`, `--message`, `--thinking`, `--timeout`,
and `--json`, plus the `models status` main-agent selector and `--check`.
Status must resolve the configured `main` profile and usable authentication.
Exit `0` is ready; exit `2` means expiring credentials and is conservatively
`setup_required/auth_required`; any other nonzero result maps through the
closed reason table. The public result exposes no model or auth detail.

### Himalaya

The initial Gmail adapter is a Fort-owned broker, not shell access. Fort
configuration binds it to one internal Himalaya account and the `INBOX`
mailbox, explicitly declares that binding as Gmail, and verifies the selected
account's version-gated configuration uses the IMAP backend with canonical
Google host `imap.gmail.com` and TLS. It never infers Gmail from the default
account.

Its functional readiness contract is:

```sh
himalaya envelope list --account <internal-gmail-account> --folder INBOX --page-size 1 --output json --quiet
himalaya message read --account <internal-gmail-account> --folder INBOX --preview <returned-id> --output json --quiet
```

The account and message ID are separate argv elements and are never
shell-expanded or exposed. The second command may use only an ID returned by
the first command in the same probe. Both results are capped and discarded.
Himalaya v1.2's upstream command contract states that ordinary read marks a
message seen and `--preview` uses the non-mutating peek path; Fort therefore
requires `--preview` and version-gates that behavior against the
[v1.2 read implementation](https://github.com/pimalaya/himalaya/blob/v1.2.0/src/email/message/command/read.rs).
An installed version whose help/source contract does not prove preview/peek is
`unknown/incompatible_version`. Binary presence or envelope listing alone
never satisfies body-read readiness.

The substantive binding exposes one app-server dynamic-tool namespace:

```json
{
  "type": "namespace",
  "name": "fort_gmail",
  "description": "Bounded read-only access to the preselected Gmail inbox.",
  "tools": [
    {
      "type": "function",
      "name": "list_recent",
      "description": "List a bounded number of recent INBOX envelopes.",
      "inputSchema": {
        "type": "object",
        "properties": {"limit": {"type": "integer", "minimum": 1, "maximum": 20}},
        "required": ["limit"],
        "additionalProperties": false
      }
    },
    {
      "type": "function",
      "name": "read_message",
      "description": "Preview one message returned by list_recent without changing flags.",
      "inputSchema": {
        "type": "object",
        "properties": {"message_id": {"type": "string", "minLength": 1, "maxLength": 128}},
        "required": ["message_id"],
        "additionalProperties": false
      }
    }
  ]
}
```

`list_recent` always invokes the prebound account, `INBOX`, JSON output, quiet
mode, and the requested bounded page size. It returns a bounded normalized
envelope projection and records the returned IDs in the stage-session
allowlist. `read_message` accepts only one of those IDs and always invokes the
same account/folder with `--preview`; it returns at most 256 KiB of normalized
message text. No arbitrary query, folder, account, raw argv, send, reply,
forward, flag, move, delete, attachment, or configuration operation exists.

On app-server `item/tool/call`, Fort validates the exact
thread/turn/call/namespace/tool tuple, strict arguments, stage-session ID
allowlist, output bound, cancellation, and still-current Gmail binding revision
before invoking Himalaya. It answers with the protocol's typed
`DynamicToolCallResponse`. Unknown or replayed call IDs, cross-turn IDs, extra
arguments, and binding drift fail the tool call without invoking Himalaya.

This adapter is offered only through `codex-appserver+gmail`. Adding native
Codex, OpenClaw, another account/folder, or another mail operation requires a
catalog-version change and contract tests.

### Codex model compatibility

The adapter starts the exact daemon-resolved Codex executable as:

```sh
codex app-server --stdio
```

This is the documented
[Codex app-server JSONL transport](https://learn.chatgpt.com/docs/developer-commands?surface=cli#cli-codex-app-server).
It initializes JSONL with `experimentalApi: true` and calls:

```text
account/read {"refreshToken":false}
model/list {"includeHidden":true}
```

`account/read` must return a non-null account when the server requires OpenAI
authentication; identity fields are discarded. The configured provider-native
model ID must appear in the complete paginated `model/list` result. The
configured-default profile requires exactly one advertised row with
`isDefault:true` and binds its model ID internally. No turn or model request is
created.

For Codex, `ready` means **authenticated and advertised by the account's
current catalog**, not a guarantee of entitlement or service availability.
Fort intentionally does not make a hidden billable generation request.
Catalog absence, including the incident's absent `gpt-5.6-sol`, blocks before
dispatch; a later account-entitlement or service rejection is preserved as a
real provider failure.

Codex app-server is currently experimental. The adapter supports only explicitly
tested CLI versions and protocol shapes. A missing method, schema drift, or
malformed response returns `incompatible_version`; Fort never falls back to a
real generation request as a probe.

### Fort-owned Codex capability runtime

Both logical-capability bindings use a two-process trust boundary:

1. a **substantive worker** runs the model turn and receives only Fort's
   `dynamicTools`; and
2. a **capability broker** holds the selected Gmail or Supabase private binding
   and executes only the closed broker operations below.

The substantive worker starts from the held Codex executable identity with an
empty private working directory, a Fort-created isolated configuration home,
an environment allowlist, `environments: []`, no configured MCP servers or
connected-app roots, and no Himalaya/Supabase credentials. That home contains
only the exact model-provider authentication/config binding already represented
by the profile revision. Its tested
`thread/start` request uses `ephemeral:true`, `sandbox:"read-only"`,
`approvalPolicy:"never"`, the exact model, and one closed `dynamicTools`
namespace. When the protocol supports `selectedCapabilityRoots`, Fort supplies
an explicit empty array. The process-level execution policy denies model-issued
child-process execution and reads outside the empty stage directory, and its
network policy permits only the model provider endpoints required by the
app-server. Broker callbacks run outside that sandbox.

These are enforcement controls, not prompt instructions. A tested Codex/platform
pair is compatible only after negative contract tests prove that the
substantive turn cannot invoke shell, raw MCP/connected-app tools, arbitrary
network access, environment access, or credential/config files, while its
declared Fort dynamic tools still work. If any default tool remains usable or
the installed protocol cannot express the exact namespace, the execution
binding is `unknown/incompatible_version`.

For each server request with method `item/tool/call`, the runtime validates
`threadId`, `turnId`, `callId`, namespace, tool, and strict arguments against
the active stage session. It returns exactly
`{success,contentItems:[{type:"inputText",text:...}]}` with bounded UTF-8/JSON
content. A call ID is single-use. Cancellation prevents new broker calls,
cancels the in-flight bounded operation, closes the thread, and terminates both
processes.

### Supabase through the Codex broker

The v1 broker is bound internally to exactly one project-scoped, read-only
Supabase capability root and schema `["public"]`. Its only log services are
`["api","postgres"]` in that order. An account-scoped connection, ambiguous
root, or empty project discovery is `setup_required/project_unavailable`;
`list_projects` alone can never satisfy database inspection.

The raw broker app-server creates an ephemeral no-turn thread with only the
internally selected capability root. Fort never accepts a root or project ID
from a planner, stage prompt, dynamic-tool argument, public request, or public
inventory. A tested version must support exact capability-root selection for
this broker; absence is `unknown/incompatible_version`.

Readiness is the following bounded sequence:

1. start the raw broker app-server and its root-bound ephemeral thread;
2. wait for the matching
   `mcpServer/startupStatus/updated` notification whose `name:"codex_apps"` and
   `threadId` equal the broker thread to reach `status:"ready"` within the
   adapter deadline; `failed`/`cancelled` normalize safely and unrelated
   notifications are ignored;
3. paginate `mcpServerStatus/list` with
   `detail:"toolsAndAuthOnly"` and validate authenticated `codex_apps` entries
   plus the exact schemas of raw `supabase.list_tables` and
   `supabase.get_logs`;
4. directly call raw `supabase.list_tables` with
   `{"project_id":<internal-project>,"schemas":["public"],"verbose":false}`;
5. directly call raw `supabase.get_logs` once for each allowed service, with
   the same hidden project ID and `service:"api"` then
   `service:"postgres"`; and
6. require non-error bounded responses, discard all content, close the broker,
   and retain only normalized state/reason.

`mcpServerStatus/list` validates auth and tool schemas; it is never treated as a
startup-readiness signal. The probe makes no model turn and persists no
transcript. If either promised operation or either promised service is
unauthorized/unavailable, the logical capability is not ready.

The substantive worker receives only:

```json
{
  "type": "namespace",
  "name": "fort_supabase",
  "description": "Bounded read-only inspection of the preselected Supabase project.",
  "tools": [
    {
      "type": "function",
      "name": "list_tables",
      "description": "List tables from the prebound public schema.",
      "inputSchema": {
        "type": "object",
        "properties": {},
        "additionalProperties": false
      }
    },
    {
      "type": "function",
      "name": "get_logs",
      "description": "Read bounded logs for one approved service.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "service": {"type": "string", "enum": ["api", "postgres"]}
        },
        "required": ["service"],
        "additionalProperties": false
      }
    }
  ]
}
```

At dispatch, Fort repeats the live broker startup/auth/schema/functional guard,
keeps that broker alive, then starts the isolated substantive worker. For
`fort_supabase.list_tables`, Fort inserts the hidden project ID, fixed schema,
and `verbose:false`; for `fort_supabase.get_logs`, it inserts the same hidden
project ID and accepts only the closed service enum. Each raw result is capped
at 256 KiB before it is returned to the substantive worker. SQL execution,
migrations, branches, storage operations, auth administration, arbitrary
project selection, and every other raw Supabase/app tool do not exist in the
substantive thread.

The tested turn request contains `threadId`,
`input:[{"type":"text","text":<bounded-prompt>}]`, the exact model ID, and
`approvalPolicy:"never"`. Terminal turn events and final assistant text map to
Fort's existing runtime event/result contract. Public inventory never exposes
project/root identifiers or returned data. The adapter is version-gated because
app-server methods are experimental. [Supabase's current MCP
guidance](https://supabase.com/docs/guides/ai-tools/mcp) supports OAuth, project
scoping, and read-only mode; setup must use both.

### Supabase CLI reservation

The current documented command:

```sh
supabase projects list --output json
```

is the
[Supabase projects-list contract](https://supabase.com/docs/reference/cli/supabase-projects-list);
it proves CLI authentication and management-plane project visibility, not
read-only database inspection. It therefore never emits
`database.supabase.inspect=ready`.

No Supabase CLI was installed on either incident machine, so setup begins as
instructions plus **Recheck** for the Codex broker adapter. This spec approves
neither an installer command nor a CLI inspection adapter. A future CLI adapter
needs a separately approved, version-gated, bounded read-only database
inspection contract for the selected project.

## Initial setup remedy catalog

Setup options are generated only from this closed, versioned reason-to-template
catalog. A row matches an exact target adapter/profile or the exact composite
binding, exact predicate, and one listed safe reason. An unlisted tuple has no
setup remedy and cannot contribute a setup option.

| Exact target | Exact predicate | Matching reasons | Instruction template ID | Intended post-Recheck effect |
| --- | --- | --- | --- | --- |
| `profile.codex.native` | `predicate.codex.native-contract.v1` | `absent`, `incompatible_version`, `command_contract_changed` | `setup.codex.capability-runtime-update.v1` | install the exact catalog-supported normal and experimental Codex tuple |
| `profile.codex.native` | `predicate.codex.authenticated-subject.v1` | `auth_required` | `setup.codex.login.v1` | authenticate the selected local Codex account |
| exact `codex:*` profile | `predicate.codex.model.<exact-profile-id>.v1` | `model_unavailable` | `setup.codex.model-availability.v1` | update Codex/account access and Recheck that exact model; it does not substitute a model |
| `profile.claude.native` | `predicate.claude.native-contract.v1` | `absent`, `incompatible_version`, `command_contract_changed` | `setup.claude.install-or-update.v1` | install catalog-supported Claude Code |
| `profile.claude.native` | `predicate.claude.authenticated-subject.v1` | `auth_required` | `setup.claude.login.v1` | authenticate Claude Code |
| `profile.hermes.native` | `predicate.hermes.native-contract.v1` | `absent`, `incompatible_version`, `command_contract_changed` | `setup.hermes.install-or-update.v1` | install catalog-supported Hermes Agent |
| `profile.hermes.native` | `predicate.hermes.provider-model.<exact-profile-id>.v1` | `auth_required`, `model_unavailable` | `setup.hermes.configure-provider.v1` | configure/authenticate the exact cataloged provider/model |
| `profile.openclaw.main` | `predicate.openclaw.native-contract.v1` | `absent`, `incompatible_version`, `command_contract_changed` | `setup.openclaw.install-or-update.v1` | install the catalog-supported OpenClaw main-agent contract |
| `profile.openclaw.main` | `predicate.openclaw.main-ready.v1` | `auth_required`, `model_unavailable` | `setup.openclaw.configure-main.v1` | configure/authenticate the exact `main` agent |
| `email.gmail.read.himalaya-broker` | `predicate.himalaya.preview-contract.v1` | `absent`, `incompatible_version`, `command_contract_changed` | `setup.himalaya.install-or-update.v1` | install Himalaya `1.2.0` with the preview contract |
| `email.gmail.read.himalaya-broker` | `predicate.gmail.selected-imap-preview-read.v1` | `auth_required` | `setup.gmail.configure-readonly.v1` | configure the Fort-selected Gmail IMAP account |
| `database.supabase.inspect.codex-broker` | `predicate.supabase.selected-project-readonly.v1` | `auth_required`, `plugin_unready`, `project_unavailable` | `setup.supabase.connect-readonly-project.v1` | connect exactly one project-scoped read-only Supabase root |
| `database.supabase.inspect.codex-broker` | `predicate.codex.capability-runtime.v1` | `absent`, `incompatible_version`, `command_contract_changed` | `setup.codex.capability-runtime-update.v1` | install the exact experimental Codex capability tuple |
| `codex-appserver+gmail` or `codex-appserver+supabase` after every leaf is hypothetically ready | corresponding `predicate.binding.<exact-binding-id>.v1` | `incompatible_version`, `command_contract_changed` | `setup.codex.capability-runtime-update.v1` | install a Codex tuple whose broker/isolation contract passes |

`profile_unmapped`, `unsupported_platform`, `no_execution_plane`,
`unreachable`, `stale`, `probe_failed`, `probe_timed_out`,
`old_node`, `cancellation_unconfirmed`, `dispatch_state_unknown`,
`handoff_state_unknown`, and solver/planner/output/handoff failures have no
setup template. An old-node
snapshot cannot prove what
profiles/capabilities would exist after upgrade, so Fort may explain the
required supported tuple but cannot present a hypothetical executable mapping.
These reasons use a permitted profile choice, Recheck, retry, Cancel, or remain
blocked as defined by the decision matrix. A reusable-playbook profile mapping
change requires cancel/edit/new run and is never represented as setup.

The immutable template payloads are:

- `setup.codex.model-availability.v1` and
  `setup.codex.capability-runtime-update.v1`: explain the exact supported
  version/schema requirement and link to the
  [official Codex installation guide](https://help.openai.com/en/articles/11096431)
  and, for capability runtime compatibility, the
  [Codex app-server protocol](https://learn.chatgpt.com/docs/developer-commands?surface=cli#cli-codex-app-server);
- `setup.codex.login.v1`: explain local interactive authentication and display
  the literal argv `["codex","login"]`;
- `setup.claude.install-or-update.v1` and `setup.claude.login.v1`: link to the
  [official Claude Code setup guide](https://docs.anthropic.com/en/docs/claude-code/getting-started)
  and require its local interactive login;
- `setup.hermes.install-or-update.v1` and
  `setup.hermes.configure-provider.v1`: link to the
  [official Hermes Agent documentation](https://hermes-agent.nousresearch.com/docs/)
  and require local provider configuration/authentication;
- `setup.openclaw.install-or-update.v1` and
  `setup.openclaw.configure-main.v1`: state the exact catalog-supported
  `2026.7.1-2`/`main` contract and direct the user to the enrolled machine's
  approved OpenClaw package source; they contain no guessed installer command;
- `setup.himalaya.install-or-update.v1` and
  `setup.gmail.configure-readonly.v1`: link to the
  [Himalaya v1.2 source and setup documentation](https://github.com/pimalaya/himalaya/tree/v1.2.0),
  require a selected Gmail IMAP account using `imap.gmail.com` with TLS, and
  explain that Fort will verify non-mutating `--preview` access;
- `setup.supabase.connect-readonly-project.v1`: link to
  [Supabase's MCP setup guide](https://supabase.com/docs/guides/ai-tools/mcp)
  and require OAuth, one explicit project scope, and read-only mode.

Every matched row also resolves to one closed `remedy_effect_id`, one
`postcondition_id`, and an exact cataloged set of predicates that hypothetical
success establishes on that machine:

- the one Codex update template uses
  `effect.codex.capability-0.146.0-alpha.9.2-16bb4744.v2`, meaning the exact
  `codex-cli 0.146.0-alpha.9.2` normal and experimental schema tuples in the
  compatibility matrix;
- Codex login uses `effect.codex.authenticated-subject.v1`, and model
  availability uses
  `effect.codex.model-ready.<exact-profile-id>.v1`;
- Claude install/login use `effect.claude-2.1.207.v1` and
  `effect.claude.authenticated-subject.v1`;
- Hermes install/provider use `effect.hermes-0.15.1.v1` and
  `effect.hermes.provider-model.<exact-profile-id>.v1`;
- OpenClaw install/main use `effect.openclaw-2026.7.1-2.v1` and
  `effect.openclaw.main-ready.v1`;
- Himalaya install and Gmail configuration use
  `effect.himalaya-1.2.0-preview.v1` and
  `effect.gmail.selected-imap-read.v1`; and
- Supabase connection uses
  `effect.supabase.selected-project-readonly.v1`.

Each effect's `postcondition_id` is the corresponding effect ID with
`effect.` replaced by `postcondition.`. Its predicate set is exact:

- the Codex capability tuple establishes
  `predicate.codex.native-contract.v1`,
  `predicate.codex.capability-runtime.v1`,
  `predicate.binding.codex-appserver+gmail.v1`, and
  `predicate.binding.codex-appserver+supabase.v1` intrinsic contract;
- each authentication, exact-model, provider-model, OpenClaw-main, Himalaya,
  Gmail-preview, and Supabase-readonly postcondition establishes exactly the
  predicate named on its matched remedy-table row (with the same closed
  profile parameter where present); and
- the Claude, Hermes, and OpenClaw install postconditions establish only their
  respective native-contract predicate.

No effect establishes a predicate on another machine. An effect may establish
the same catalog predicate for several target offers on one machine; that
shared causal coverage is deliberate.

The angle-bracket profile component is replaced only by one closed catalog
profile ID and is therefore not free-form input. An install/update remedy is
eligible only when its exact postcondition is not already true; if the required
tuple is already installed but its contract probe still fails, no reinstall
option is offered. Authentication/configuration effects are likewise
considered satisfied only by their corresponding functional probe.

The server injects only the public target machine label and required supported
version/tuple/effect into these templates. Templates are instructions-only;
clients can copy literal argv but never run it, and Fort never receives
credentials. Every link, literal command, effect ID, and postcondition above is
part of template version 1. Changing one requires a template-version and
catalog-version change.

Setup closure is causal and conservative. For one hypothetical
stage/machine/binding candidate, the solver starts from the complete predicate
graphs of the exact profile, logical leaves, and intrinsic binding. It performs
this deterministic fixed point:

1. copy observed predicate states; never invent a ready HMAC revision;
2. in dependency depth then catalog order, promote each `derived` predicate
   whose dependencies are symbolically satisfied; it creates no deficit;
3. select the first non-satisfied `probe` predicate whose dependencies are now
   symbolically satisfied; an observed `unsatisfied` row or newly eligible
   `blocked` row requires one matching versioned remedy and creates its target
   predicate deficit;
4. deduplicate that work by
   `(machine,remedy_effect_id,postcondition_id)`; before applying the operation,
   enumerate every required target-predicate instance in its exact
   postcondition set whose immutable observed state was not `satisfied`,
   resolve its observed reason or deterministic `blocked_reason` to the same
   operation, and create/attach that target deficit to the operation's
   `covers`; if any such instance lacks that exact row, the candidate is
   ineligible; then apply the effect immediately and once and restart at step
   2; and
5. succeed only when every required predicate is symbolically satisfied and
   every effect operation has an acyclic instruction dependency order.

A `blocked` downstream authentication/model/configuration predicate is
therefore included conservatively even though its private state was not
observed. Its instruction is conditional/idempotent and Recheck may prove it
was already satisfied. If a non-satisfied or newly unblocked predicate has no
cataloged remedy, or the fixed point makes no progress, that candidate is
ineligible. A leaf failure and its derived composite summary are never counted
twice; only the composite's separately owned intrinsic predicate can add a
binding deficit. One shared operation may cover several distinct target
deficits. Only Recheck can replace the hypothetical predicates with real ready
revisions, and no setup option itself can dispatch.

## Remote discovery and downgrade safety

The node server adds:

```text
GET /api/node/capabilities
Authorization: Bearer <mesh token>
```

It returns only the node-owned safe projection:

```json
{
  "protocol_version": 1,
  "catalog_version": 1,
  "profile_mapping_version": 1,
  "node_id": "enrolled-node-id",
  "observed_at": "2026-07-23T00:00:00Z",
  "state": "ready",
  "reason": "",
  "profiles": [],
  "offers": [],
  "bindings": []
}
```

The response cannot choose its public machine name, coordinator-local status,
registry rank, or URL. The coordinator binds the authenticated response to the
contacted enrollment record, rejects a `node_id` mismatch, assigns the
registry-owned name/rank, stamps its own receipt time for freshness, and always
sets a peer to `local:false`. Peer wall-clock `observed_at` is informational and
never a TTL, ranking, or hash input. Only the synthetic coordinator entry can
be local/rank `0`.

V1 accepts exactly version tuple
`(protocol_version:1,catalog_version:1,profile_mapping_version:1)` before any
offer enters normalization. A different value in any position produces machine
state `unknown/old_node`; the coordinator never mixes offers from divergent
catalogs. The retained tuple participates in the normalized snapshot and its
revision.

The control plane refreshes peers concurrently. A `404`, unsupported version
tuple, identity mismatch, invalid payload, timeout, or failed authentication
produces machine state `unknown` with a safe reason. Static agent claims remain
display-only.

Freshness is not overloaded onto the cached `GET`; the node also adds:

```text
POST /api/node/capabilities/recheck
Authorization: Bearer <mesh token>
```

The strict request is:

```json
{
  "protocol_version": 1,
  "request_id": "uuid",
  "mode": "planning",
  "max_age_seconds": 60,
  "adapters": ["profile.codex.native", "email.gmail.read.himalaya-broker"]
}
```

`mode` is `planning` or `user_recheck`; adapter IDs are closed catalog IDs,
deduplicated, and limited to 32. `planning` requires
`max_age_seconds:60`, bypasses any node observation older than 60 seconds, and
honors failure backoff. When backoff prevents that probe, the named stale
offers/bindings publish `unknown/stale`; an older ready result is never reused
for planning. `user_recheck` requires `max_age_seconds:0`, invalidates the named
cache, and bypasses both cache and backoff. The node performs a bounded
single-flight refresh and returns the same node-owned projection as `GET`; the
coordinator still binds peer identity/name/rank/local fields.
Unknown fields, adapters, modes, or bounds fail before a probe starts.

Before planning, the coordinator uses its authenticated receipt time. A peer
projection older than 60 seconds must be refreshed through this endpoint; a
plain `GET` can never satisfy that requirement. A gate-level Recheck uses
`user_recheck`. Target guards remain live and uncached regardless of either
endpoint.

### Durable handoff receipts

Portable stage data crosses a machine boundary before the consuming stage is
dispatched. The target node exposes:

```text
PUT /api/node/handoffs/{receipt_id}
GET /api/node/handoffs/{receipt_id}/status
Authorization: Bearer <mesh token>
```

The strict `PUT` body is:

```json
{
  "protocol_version": 1,
  "catalog_version": 1,
  "profile_mapping_version": 1,
  "run_id": "run-...",
  "plan_id": "plan-...",
  "plan_revision": "sha256:...",
  "choice_revision": "sha256:...",
  "input_contract_revision": "sha256:...",
  "source_stage_id": "read-email",
  "consumer_stage_id": "diagnose",
  "source_machine": "taloss.mac.mini.lan",
  "target_machine": "tobiass.macbook.pro.lan",
  "output": "incident_evidence",
  "format": "text",
  "max_bytes": 65536,
  "sha256": "sha256:...",
  "bytes": 1234,
  "content_b64u": "<unpadded base64url of the exact payload bytes>"
}
```

`input_contract_revision` is the canonical control-plane hash with domain
`fort.input-contract.v1` and this exact object:

```json
{
  "run_id": "run-...",
  "plan_id": "plan-...",
  "plan_revision": "sha256:...",
  "choice_revision": "sha256:...",
  "source_stage_id": "read-email",
  "consumer_stage_id": "diagnose",
  "source_machine": "taloss.mac.mini.lan",
  "target_machine": "tobiass.macbook.pro.lan",
  "output": "incident_evidence",
  "format": "text",
  "max_bytes": 65536
}
```

There are no omitted, null, or additional fields in that hash input. The
authenticated coordinator sends exactly the persisted accepted handoff row;
the target recomputes the revision, requires `target_machine` to be its
enrollment-owned name, and later requires the consuming stage request to repeat
that revision. The coordinator is the authority for the accepted plan; a node
never accepts a planner/model-created contract.

Fort persists one such input contract for every chain edge when placement is
accepted. Cross-machine contracts correspond exactly to disclosed `handoffs`;
same-machine contracts have equal source/target machines and create only the
target-local receipt. Thus receipt validation never depends on a node holding
the whole private plan manifest.

`content_b64u` always encodes payload bytes rather than an embedded JSON value.
Text payload bytes must be valid UTF-8 exactly as persisted. JSON payloads are
RFC 8785 canonical UTF-8 bytes before byte count, digest, and base64url
encoding. The target validates enrollment, version tuple,
run/plan/choice/contract/stage/output binding, decoded UTF-8 or canonical JSON
format, byte length, digest, and `bytes <= max_bytes` before atomically storing
the immutable receipt and returning
`{receipt_id,run_id,plan_id,input_contract_revision,sha256,bytes,state:"stored"}`.
Repeating the same
receipt ID with a byte-equivalent semantic body returns the same stored
acknowledgment without storing a second body; different fields return the
node-only `409 handoff_conflict` shape. The status endpoint returns that
acknowledgment or
`404 {receipt_id,state:"absent"}` and never returns content.

Before each physical cross-machine `PUT`, the coordinator transactionally
reserves the declared payload bytes in the run's append-only physical-handoff
ledger and records a fresh `receipt_id`; reservation happens before the first
body byte can be sent. A successful acknowledgment is recorded against that
reservation. After an ambiguous disconnect, recovery queries status first. A
stored receipt is acknowledged without resending. An absent receipt may be
sent only under a newly reserved ledger entry and new receipt ID, so even a
possibly lost first transmission remains counted. If status cannot be
determined, the coordinator persists `reconciliation_started_at` and queries
the same receipt at offsets `0, 1, 2, 4, 8, 15, 30, 45, 60` seconds. Restart
derives the next offset from that timestamp and never resets the schedule. A
stored/absent result follows the rules above; no determination at 60 seconds
moves the run to `blocked/handoff_state_unknown` with the receipt and ledger
intact for later cancellation/operator recovery. It never blindly resends. A
reservation that would exceed 1 MiB writes no outbox send and fails with
`handoff_limit_exceeded`.

Every completed stage also atomically stores a target-local output receipt with
its terminal result. A later stage on the same node references that receipt
without a network transfer or physical-ledger charge. Cross-machine transfer
creates a target receipt bound to the one source/consumer/output tuple. Receipt
content is available only to the exact authenticated consuming stage and is
retained until 24 hours after the run reaches a terminal state, longer than
every execution/reconciliation horizon, then deleted with the run's protected
payloads.

Fort allocates and freezes the consuming stage's `dispatch_id` and exact
execution body only after every required target receipt is durably
acknowledged. Therefore an absent-receipt recovery may choose a new receipt ID
without mutating any already-sent dispatch, while every reconnect of a sent
dispatch remains byte-equivalent.

Capability-aware execution does not add optional fields to legacy
`POST /api/exec`. It uses:

```text
POST /api/node/exec-capability
Authorization: Bearer <mesh token>
```

The request is a strict discriminated union. A remote planner dispatch has no
plan yet:

```json
{
  "kind": "planner",
  "protocol_version": 1,
  "catalog_version": 1,
  "profile_mapping_version": 1,
  "run_id": "run-...",
  "dispatch_id": "dispatch-...",
  "planner_operation": "capability_plan",
  "preflight_revision": "sha256:...",
  "machine": "taloss.mac.mini.lan",
  "profile": "codex:gpt-5.5",
  "binding": "codex-native",
  "expected_profile_binding_revision": "opaque:...",
  "expected_execution_binding_revision": "opaque:...",
  "output": "capability_plan",
  "output_format": "json",
  "max_output_bytes": 131072,
  "attempt_policy": {
    "provider_max_attempts": 1,
    "deadline_seconds": 300
  },
  "runtime_input": {
    "prompt": "<bounded persisted planner prompt>"
  }
}
```

A substantive stage dispatch is:

```json
{
  "kind": "stage",
  "protocol_version": 1,
  "catalog_version": 1,
  "profile_mapping_version": 1,
  "run_id": "run-...",
  "dispatch_id": "dispatch-...",
  "plan_id": "plan-...",
  "plan_revision": "sha256:...",
  "choice_revision": "sha256:...",
  "stage_id": "read-email",
  "machine": "taloss.mac.mini.lan",
  "profile": "codex:gpt-5.5",
  "binding": "codex-appserver+gmail",
  "requires": ["email.gmail.read"],
  "output": "incident_evidence",
  "output_format": "text",
  "max_output_bytes": 65536,
  "attempt_policy": {
    "provider_max_attempts": 1,
    "deadline_seconds": 900
  },
  "expected_profile_binding_revision": "opaque:...",
  "expected_capability_binding_revisions": {
    "email.gmail.read": "opaque:..."
  },
  "expected_execution_binding_revision": "opaque:...",
  "runtime_input": {
    "direction": "<bounded original direction>",
    "stage_prompt": "<persisted stage prompt>",
    "inputs": []
  }
}
```

Every field shown is required for its variant; plan/choice fields are forbidden
for `planner`, and preflight fields are forbidden for `stage`. The authenticated
enrollment determines the target; `machine` must equal the coordinator-owned
registered name and is never a routing input at the node.

`planner_operation` is exactly `capability_plan` or `breakdown_items`.
`capability_plan` requires output name `capability_plan` and maximum 131,072
bytes; `breakdown_items` requires output name `breakdown_items` and maximum
262,144 bytes. Both require format `json`, one provider attempt, and the
300-second deadline. Planner `runtime_input` contains only that operation's
persisted bounded prompt.
Stage `inputs` contain exactly the persisted named `input_from` values as
`{name,format,sha256,bytes,receipt_id,input_contract_revision}` rows in plan
order; no content appears in an execution request. The target resolves each
immutable local receipt, requires its exact input-contract/run/plan/source/
consumer/output/digest binding, and consumes
only its stored UTF-8 text or canonical JSON. Unknown input names, duplicate
names, missing receipts/digests, or extra runtime fields fail before dispatch.
Runtime prompts and handoff content are transported only over authenticated
node channels (and sealed relay where applicable), are never logged, and are
not part of public inventory or conflict bodies.

### Capability execution response and reconnect

This endpoint does not inherit legacy `/api/exec` disconnect semantics. A
successful initial or byte-equivalent duplicate `POST` returns
`Content-Type: application/x-ndjson; charset=utf-8` and a replay-plus-live tail
of one durable per-dispatch journal. The node also exposes:

```text
GET /api/node/exec-capability/{dispatch_id}?run_id={run_id}
GET /api/node/exec-capability/{dispatch_id}/events?run_id={run_id}&after={sequence}
Authorization: Bearer <mesh token>
```

For an existing dispatch, status returns `200 application/json` with exactly
`{protocol_version,run_id,dispatch_id,semantic_request_sha256,state,last_sequence,terminal,cancel_reconciliation}`.
`semantic_request_sha256` is SHA-256 over domain
`fort.exec-capability-request.v1`, one NUL, and RFC 8785 canonical request-body
JSON; transport/auth headers are excluded.
`state` is `accepted`, `running`, `cancel_requested`, `succeeded`, `failed`,
`timed_out`, or `canceled`; `terminal` is null for the first three and the
stored terminal frame otherwise. `cancel_reconciliation` is null before any
cancel request and otherwise exactly
`{generation,state:"reconciling"|"confirmed"|"unconfirmed"}` for the highest
persisted generation. It is separate from the immutable execution terminal.

After authenticating the enrollment and validating the required `run_id` and
`dispatch_id`, a supported node with no durable dispatch record returns exactly
`404 application/json`
`{protocol_version:1,run_id,dispatch_id,state:"not_started"}`. The invariant
that `accepted` is stored before launch makes this affirmative proof that the
provider did not start. A generic/HTML/malformed 404, absent protocol field,
wrong IDs, unsupported inventory tuple, transport failure, or old-node route
404 is never `not_started`; it yields `dispatch_state_unknown` or the exact
version-drift path. The event GET validates the same enrollment/run ownership,
requires `after` in `-1..last_sequence`, replays every frame with sequence
greater than `after`, then follows new frames until terminal or transport
detach. `after:-1` means from the beginning.

Every NDJSON line is one strict frame:

```json
{
  "protocol_version": 1,
  "run_id": "run-...",
  "dispatch_id": "dispatch-...",
  "attempt_id": "attempt-...",
  "sequence": 0,
  "event_id": "evt_...",
  "kind": "accepted",
  "state": "accepted"
}
```

`sequence` is a JSON integer, starts at zero, is contiguous, and cannot exceed
9,007,199,254,740,991. `event_id` uses the canonical control-plane hash with
domain `fort.dispatch-event.v1` and exact object
`{dispatch_id,sequence,frame}`, where `frame` is the complete strict frame JSON
object with only `event_id` omitted. It is exactly
`evt_[base64url(all-32-hash-bytes)]`, so the suffix is 43 unpadded base64url
characters. Frame kinds are:

- `accepted`: sequence 0, state `accepted`, no other fields;
- `progress`: state exactly `running` or `cancel_requested` and
  `phase:"starting"|"provider_running"|"broker_call"|"canceling"`, with no
  provider text;
- `terminal` success: state `succeeded` plus
  `result:{output_format,sha256,bytes,content_b64u}` where decoded content is
  the already-validated exact output; or
- `terminal` non-success: state `failed`, `timed_out`, or `canceled`, plus
  `safe_code` and optional
  `private_diagnostic_b64u` containing valid UTF-8 and capped at 64 KiB
  decoded.

Nonterminal frames are at most 4 KiB canonical JSON. A terminal success is
bounded by declared output bytes plus base64/4 KiB framing; a non-success frame
is at most 96 KiB. Private diagnostics travel only over mesh/encrypted relay
and enter the protected attempt record; they never enter public errors, SSE, or
logs.

The node transactionally stores the semantic request hash and `accepted` frame
before provider launch. Every later frame is stored before any socket flush,
and terminal state/result are one atomic record. A transport disconnect only
stops flushing. The coordinator accepts frames only in contiguous sequence,
deduplicates by `(dispatch_id,sequence,event_id)`, persists its high-water mark
before acknowledging progress, and reconnects after that sequence on a gap or
detach. A repeated sequence with different `event_id` or payload is
`dispatch_conflict`; no provider is restarted.

After authentication, size-bounded JSON parsing, and extraction of
run/dispatch IDs plus the canonical semantic hash, the target first looks up
`dispatch_id`. An existing byte-equivalent stored request replays its journal
under its recorded protocol even if current catalog state drifted, without
re-running a capability guard or provider. A different request is
`dispatch_conflict`. Only a new dispatch proceeds through strict current
version/body validation and, before writing stream headers or invoking
anything, recomputes the
profile, logical-capability, and composite binding revisions from its current
private binding and validates every catalog/profile/proof, receipt,
attempt-policy, output-format, and output-bound field. It validates terminal
output against the declared format and byte maximum before persisting or
streaming a successful result; progress metadata may stream, but output bytes
are buffered within the declared maximum until validation. It then invokes the selected binding runtime,
which may be native CLI or the isolated Codex app-server broker runtime. Any
binding mismatch invokes zero provider/model calls and returns the node-only
shape:

```json
{
  "error": {
    "code": "node_capability_drift",
    "message": "The target capability binding changed.",
    "run_id": "run-...",
    "dispatch_id": "dispatch-...",
    "mismatches": [
      {
        "kind": "capability",
        "id": "email.gmail.read",
        "expected_binding_revision": "opaque:...",
        "current_binding_revision": "opaque:...",
        "current_state": "ready",
        "reason": ""
      }
    ]
  }
}
```

`mismatches` has 1–10 rows sorted as profile, capability ID, then composite
binding. `kind` is exactly `profile`, `capability`, or `binding`; `id` is a
closed catalog ID. Expected revision is the submitted proof. Current revision
is the current opaque HMAC when ready and `""` otherwise; `current_state` and
`reason` use the closed public enums. No run view, global inventory/relevant
revision, or private diagnostic can be computed or returned by a node.

The coordinator validates the response against the persisted dispatch, performs
the bounded relevant refresh, and in one transaction fences the work item,
persists the refreshed projection, moves the run to `needs_user`, creates a
fresh `capability_drift` placement decision with its accepted mapping, and
appends the audit event. Client-facing `capability_drift` is generated only from that
coordinator record and therefore carries the authoritative
`CapabilityRunView` and relevant revisions.

If the target's version tuple changes after placement, it instead returns:

```json
{
  "error": {
    "code": "node_version_mismatch",
    "message": "The target Fort capability protocol changed.",
    "run_id": "run-...",
    "dispatch_id": "dispatch-...",
    "expected": {
      "protocol_version": 1,
      "catalog_version": 1,
      "profile_mapping_version": 1
    },
    "current": {
      "protocol_version": 1,
      "catalog_version": 2,
      "profile_mapping_version": 1
    }
  }
}
```

The node returns only its tuple and invokes zero provider calls. The
coordinator refreshes that enrolled node and deterministically re-solves. It
creates a new placement choice if another ready mapping exists; otherwise it
creates a fresh placement `blocked` decision with safe reason `old_node` and
Recheck/Cancel only, because an old-node snapshot cannot prove a post-upgrade
mapping. The node response itself never manufactures a client decision or
global preview.

`dispatch_id` is a server-generated stable ID for one durable work item.
Duplicate authenticated requests with the same ID and byte-equivalent semantic
body replay the exact sequenced journal and tail the one live execution; they
never spawn a second provider attempt. Reuse with different fields or a
conflicting journal frame is `409 dispatch_conflict`. An old node has no
matching endpoint and cannot silently ignore the proof.

Remote cancellation is explicit and idempotent:

```text
POST /api/node/cancel-capability
Authorization: Bearer <mesh token>

{
  "protocol_version": 1,
  "run_id": "run-...",
  "dispatch_id": "dispatch-...",
  "reconciliation_generation": 2,
  "prior_unconfirmed_through": 1
}
```

`reconciliation_generation` is an integer from 1 through 3 and
`prior_unconfirmed_through` is exactly `reconciliation_generation-1`. It is an
authenticated coordinator assertion that every earlier global run generation
ended at run level as `cancellation_unconfirmed` and therefore authorizes this
later global generation; it does not overwrite a confirmed per-dispatch target
result. The exact response
while work remains is:

```json
{
  "protocol_version": 1,
  "run_id": "run-...",
  "dispatch_id": "dispatch-...",
  "reconciliation_generation": 2,
  "reconciliation_state": "reconciling",
  "result": null
}
```

Every response has those same six fields. `reconciliation_state` is
`reconciling` if and only if `result` is null. Its finalized `result` is exactly
`{outcome:"confirmed",dispatch_state,last_sequence,terminal}` or
`{outcome:"unconfirmed",dispatch_state:"cancel_requested",last_sequence,terminal:null}`.
For a non-null result, `reconciliation_state` is exactly equal to
`result.outcome`.
A confirmed `dispatch_state` is `not_started`, `succeeded`, `failed`,
`timed_out`, or `canceled`; `terminal` is the exact stored terminal journal
frame except for `not_started`, where it is null and `last_sequence` is `-1`.

The target authenticates the same enrollment/run ownership and stores one
record keyed by `(dispatch_id,reconciliation_generation)`. Generation 1 is the
first cancellation. Before accepting generation `g>1`, it compares its target
records with the authenticated `prior_unconfirmed_through:g-1`:

- an earlier immutable `unconfirmed` record remains unchanged;
- a missing earlier record gets an immutable internal unconfirmed gap marker
  with source `coordinator_unreachable`;
- an earlier `reconciling` record finalizes `unconfirmed` with source
  `coordinator_generation_elapsed`; and
- an earlier immutable `confirmed` record remains unchanged and causes the new
  generation to confirm immediately from the already-terminal/not-started
  execution state.

The two source labels are protected target audit metadata and do not enter
public responses. A request with the wrong prior count, a lower generation than
the highest target record except exact replay, or a different body for an
existing generation fails `409 dispatch_conflict`; it never starts work. This
allows a machine that missed one or both earlier generations to rejoin a later
authenticated global generation without rewriting history.

First acceptance atomically creates a `reconciling` record, moves a nonterminal
execution to `cancel_requested`, and signals the same held app-server/native
process group. Repeated requests for that pending generation perform its
bounded liveness checks; once `result` is non-null the record is immutable and
exact repeats replay it. An already terminal dispatch, or an authenticated
`not_started` lookup, confirms immediately.

A later generation creates a new reconciliation record and may signal/recheck
the same stored process identity, but it never changes an older result. No
generation, target/coordinator restart, or cancellation acceptance may launch a
provider, allocate a new attempt/dispatch ID, or move `cancel_requested` back
to `starting`/`running`. If the stored process identity is proven gone, the
target appends the one terminal `canceled` frame when no terminal frame already
exists. If liveness remains unknowable at the bound, that generation finalizes
`unconfirmed` while the execution remains nonterminal `cancel_requested`.
`cancellation_unconfirmed` is therefore a coordinator/run safe reason and a
generation outcome, never an execution state or terminal frame.

An execution-stream disconnect only detaches transport; the coordinator
reconnects to the same `dispatch_id`. Only an explicit authenticated
run/dispatch cancellation or the persisted deadline triggers this path.

## Capability plan

### Schema

Fort assigns `plan_id`; the model never does. The planner emits exactly one
JSON object:

```json
{
  "stages": [
    {
      "id": "read-email",
      "order": 1,
      "title": "Read the Supabase failure email",
      "prompt": "Read the relevant Gmail message and extract bounded incident evidence.",
      "profile": "codex:gpt-5.5",
      "requires": ["email.gmail.read"],
      "input_from": [],
      "output": "incident_evidence",
      "output_format": "text",
      "max_output_bytes": 65536
    },
    {
      "id": "diagnose",
      "order": 2,
      "title": "Diagnose the Supabase failure",
      "prompt": "Use the approved incident evidence to diagnose the Supabase failure.",
      "profile": "codex:gpt-5.5",
      "requires": ["database.supabase.inspect"],
      "input_from": ["incident_evidence"],
      "output": "diagnosis",
      "output_format": "text",
      "max_output_bytes": 65536
    }
  ]
}
```

The generated plan is a strict sequential chain in v1. Stage 1 has no input;
each later stage has exactly the immediately preceding stage's output in
`input_from`. There is no fanout, skip edge, multi-input join, or model-defined
machine.

Bounds:

- 1–16 stages;
- IDs match `[a-z0-9][a-z0-9_-]{0,63}`;
- title ≤ 160 UTF-8 bytes;
- prompt ≤ 8 KiB;
- at most 8 requirements per stage and at most 1 chain input;
- canonical plan JSON ≤ 128 KiB;
- `output_format` is exactly `text` or `json`;
- `max_output_bytes` is 1–262,144; and
- the sum of reserved `max_output_bytes` for every cross-machine edge in an
  accepted mapping is at most 1 MiB.

Unknown profiles/capabilities, requirement combinations with no cataloged stage
binding, duplicate IDs/outputs, non-chain references, missing handoffs, machine
fields, invalid UTF-8, or exceeded bounds fail closed as a failed planner
attempt. Fort persists a safe validation code and creates a typed
`capability_preflight` decision in `planner_failed` state with **Cancel** and,
while the three-dispatch budget remains, **Retry planner**. Retry reuses the
persisted planner assignment and creates a new durable attempt; it never edits
or guesses at malformed model output.

The solver treats a mapping whose reserved cross-machine maxima exceed 1 MiB as
ineligible. At runtime each stage output is validated as UTF-8 text or JSON and
must fit its declared maximum before persistence. Cross-machine transfer uses a
content digest and a durable per-run byte ledger. Every payload byte reserved
for sending, including an ambiguously lost send and any new-receipt send after
status proves the earlier receipt absent, counts toward the 1 MiB run budget;
stored-receipt acknowledgment replay does not send the body again. Before a
transfer that would exceed the remaining budget, Fort sends zero bytes and
terminates the plan with safe
`handoff_limit_exceeded`. An oversized or malformed stage result fails with
`output_limit_exceeded`; Fort never truncates it into a valid handoff.

### Planner prompt

The planner receives:

- original direction and immutable playbook intent;
- permitted closed execution profiles with display descriptions;
- the complete closed logical capability catalog, including capabilities not
  currently ready;
- closed per-agent co-use rules derived from stage binding templates, without
  machine availability;
- strict schema and bounds; and
- no machine names, URLs, account identifiers, project identifiers, probe
  details, or credentials.

Capability inference is generation and occurs only inside this visible planner
task. The canonical planner prompt is at most 128 KiB; exceeding it fails
preflight without dispatch. Validation and placement invoke no model or
runtime.

### Durable identity and crash boundary

The planner node's bounded raw result is first persisted as its successful
attempt output. A crash after planner completion therefore revalidates that
exact output and never repeats a model call.

Fort then validates and solves in memory. One compare-and-set transaction first
verifies the solver-relevant inventory generation is still current; a newer
generation forces deterministic recomputation before any automatic placement
or option persistence. That transaction persists:

- `plan_id` and original run ID;
- immutable playbook ID/revision and direction digest;
- immutable ingress constraints: permitted profiles, explicit machine pin,
  delivery mode, and typed capability-signoff policy;
- canonical validated plan JSON;
- catalog/profile-mapping version;
- the complete safe canonical frozen solver projection, not only its inventory
  hash;
- the solver result and either the waiting option set or exact auto-placement;
- `plan_revision`, the immutable semantic plan hash defined below; and
- the audit event plus next lifecycle state.

A persistence failure leaves no executable plan. A crash before this
transaction reprocesses the stored planner output; a crash after it reconstructs
the exact persisted choice or placement.

`plan_revision` is SHA-256 over canonical length-delimited fields with domain
separator `fort.capability-plan.v1`:

1. `plan_id` and `run_id`;
2. the exact source union: for `generated`,
   `{kind,playbook_id,playbook_revision}`; for `static`,
   `{kind,playbook_id,playbook_revision,source_digest}` where that digest also
   binds the preceding identifiers and normalized plan;
3. original direction digest;
4. immutable ingress constraints: closed permitted profiles,
   planner-profile override, planner-machine pin, substantive machine pin,
   delivery mode, and sign-off policy;
5. canonical validated plan JSON, including exact stage prompts, outputs,
   formats, maxima, and server-supplied attempt policies; and
6. catalog and profile-mapping versions.

It explicitly excludes inventory/full/relevant revisions, machine observations,
candidate sets, solver projection, placement/setup options, accepted mapping,
binding revisions/proofs, choice revision, instruction bundles, timestamps,
events, attempts, and outputs. Those mutable selection facts are bound later by
`relevant_revision` and `choice_revision`. A semantically identical plan has
one hash regardless of inventory refresh; changed prompts or execution policy
cannot retain it. Inventory revision alone never identifies generated planner
output.

### Durable continuation ownership

No HTTP handler, gate callback, restart path, or graph callback invokes a
planner, Recheck, compiler, stage, or resume path directly. Every state
transition that requires follow-on work inserts one unique durable outbox row
in the same store transaction:

```json
{
  "work_id": "work-...",
  "run_id": "run-...",
  "kind": "planner_dispatch",
  "state": "pending",
  "continuation_version": 1,
  "dispatch_id": "dispatch-...",
  "claim_generation": 0,
  "attempt_id": null
}
```

Closed work kinds are `planner_dispatch`, `capability_recheck`,
`plan_compile`, `stage_dispatch`, `handoff_transfer`, `stage_continue`, and
`dispatch_cancel`. A unique constraint on
`(run_id,kind,continuation_version)` prevents two callers from enqueueing the
same continuation. Runtime-producing and cancellation work carry the stable
`dispatch_id`; handoff work carries its reserved `receipt_id`; Recheck,
compile, and stage-continuation work omit both. For `dispatch_cancel`,
`continuation_version` is exactly the run's reconciliation generation 1, 2, or
3, so a later approved generation gets new work without duplicating the prior
generation.

A worker claims `pending → claimed` by CAS, increments a fencing
`claim_generation`, assigns a 30-second lease, and, for runtime-producing work,
creates the corresponding runtime `attempt_id` in the same transaction before
any external call. The
current claimant renews every 10 seconds by CAS against
`(work_id,claim_generation,state:"claimed")`. Only that generation may append
attempt events, commit a result, enqueue the next continuation, or change run
state. A claimant that loses renewal detaches from the stream and stops
committing; it does not cancel the stable dispatch merely because ownership of
the transport changed. Request handlers merely commit and wake workers.

The local and remote runtime adapters key one live/terminal execution by
`dispatch_id`. A duplicate dispatch reconnects to that attempt or replays its
stored terminal result; it cannot spawn again. An expired claim is not blindly
reissued: startup/recovery first reconciles the stable dispatch ID with the
local process/attempt table or remote node. Active work is reattached under a
new fenced claim; a terminal result is committed; a
durably-proven-`not_started` item is reclaimed using the same dispatch ID. If
liveness cannot be proved, the run becomes `blocked` with
`dispatch_state_unknown` instead of starting a second provider attempt.

Every runtime work record carries a server-supplied immutable
`attempt_policy`; clients and planners cannot choose it. V1 policies are:

| Work | Provider attempts per dispatch | Persisted wall deadline | New-dispatch retry |
| --- | --- | --- | --- |
| capability planner | exactly 1 | 300 seconds from target-accepted start | only an explicit **Retry planner**, at most 3 total planner dispatches for the run |
| substantive generated/static stage | exactly 1 | 900 seconds from target-accepted start | none |

The target stores the accepted start time before provider launch and enforces
the deadline even if every client/worker transport detaches. Transport
reconnect, claim transfer, terminal replay, and status reconciliation are the
same attempt. A new `attempt_id` and `dispatch_id` are created only for an
accepted explicit planner retry; the original attempt remains immutable.
Generated and static stage provider errors/timeouts are terminal run failures,
never implicit retries. Static source `retry` must be absent or exactly zero.
After three failed/timed-out/invalid planner dispatches, the planner decision
contains no retry option and can only be canceled.

Attempt states are exactly `created`, `starting`, `running`,
`cancel_requested`, `succeeded`, `failed`, `timed_out`, and `canceled`. The
first four are nonterminal; the remainder are terminal. State moves forward by
CAS. A deadline atomically records
`cancel_requested`, starts the same durable cancel reconciliation, and becomes
`timed_out` only after process termination is confirmed; inability to confirm
leaves the attempt `cancel_requested` and blocks the run with
`cancellation_unconfirmed`. Provider
failure maps to `failed` with a safe code and preserved private diagnostics in
the existing protected attempt record, not public inventory.

Each run cancellation generation and each target dispatch record persists its
own `accepted_at`, next-offset cursor, and result. Explicit cancellation retries
status reconciliation at offsets `0, 1, 2, 4, 8, 15, 30, 45, 60` seconds from
that generation's `accepted_at`. Restart derives the next offset from the
current generation and never resets it. Confirmed terminal/not-started state
ends that dispatch schedule. At 60 seconds an unresolved dispatch result
becomes immutable `unconfirmed`, and the coordinator blocks the run with
`cancellation_unconfirmed`. A user retry creates generation `g+1` with a new
epoch and the same stable dispatch/attempt/process identities; only its
reconciliation schedule resets.

`capability_recheck` is still a durable work item even though its probes are
read-only. Recovery can repeat its single-flight probe under a new claim and
must publish the next actionable decision; `rechecking` cannot remain stranded
after a crash. Plan sign-off acceptance, initial auto-placement, placement
selection, successful Recheck proof refresh, stage completion, retry, and
restart all enqueue their one next continuation through this contract.

The existing graph executor receives only already-claimed compiled task work
from this adapter. It cannot independently upsert/dispatch capability-plan
tasks, and legacy graph gate methods cannot resume a capability run.

## Deterministic solver

For each stage, candidates are machines that:

1. are reachable with a current supported protocol;
2. offer the stage's exact execution profile as `ready`;
3. publish one ready execution-binding offer for that exact profile and
   complete requirement set;
4. publish every underlying logical offer as ready with the binding revisions
   incorporated into that execution-binding offer; and
5. satisfy an explicit pin when one exists.

The selected binding ID, profile binding revision, logical binding revisions,
and composite execution-binding revision are part of the mapping/proof. Fort
never composes independently ready offers ad hoc.

### Result states

- `ready` / `single` — at least one machine can run every stage.
- `choice_required` — a ready split and/or one or more setup alternatives
  exists, but no ready single host exists.
- `blocked` — no executable placement and no approved setup alternative exists.

`stale` is not a solver result from one snapshot. It is a decision-time
comparison failure.

### Single-host ranking

Choose:

1. the local machine when eligible; otherwise
2. the lowest registry rank.

Every stage is stamped with that machine.

### Split ranking

Because the generated plan is a strict chain and v1 has at most 16 stages and
16 machines, the exact solver enumerates machine subsets by increasing
cardinality and registry-rank bit order. For each subset whose members cover
every stage, a two-layer chain DP keyed by `(stage_index,last_machine)` chooses
the best assignment inside that subset. Once at least one assignment exists at
cardinality `k`, larger subsets cannot improve the first objective and are not
visited. The solver evaluates every subset of cardinality `k` before selecting
the lexicographically smallest tuple:

1. number of distinct machines;
2. number of cross-machine `input_from` edges;
3. number of stages not on the local machine; and
4. vector of registry ranks in stage order.

This minimizes machines and disclosed handoffs before applying local and
registry preferences.

Candidate bitsets and transition costs are precomputed. V1 has a deterministic
ceiling of 300,000,000 transition checks and a 64 MiB solver arena; exceeding
either yields `blocked/solver_limit_exceeded` with zero placement or runtime
calls. Allocation outside that arena is forbidden after normalization. The
semantic result never depends on elapsed time. Release acceptance benchmarks
the maximum-density 16-stage/16-machine fixture at less than 10 seconds on the
oldest supported deployment class; a build that misses that bound cannot ship.

### Setup ranking

A setup alternative is an exact, bounded instruction bundle plus the
hypothetical single or split mapping that would become ready if every
instruction established its declared postcondition. For each
stage/machine/binding tuple, the solver runs the cataloged predicate/effect
fixed point above. It may replace a non-satisfied predicate only when that exact
target/predicate/reason has a versioned instructions-only remedy. A derived
composite summary is suppressed; only its separately owned intrinsic predicate
can add a binding deficit.

Each semantic deficit has a stable ID
`def_[base64url(first-16-bytes(hash))]`, where `hash` uses the canonical
control-plane algorithm with domain `fort.setup-deficit.v1` and exact object
`{kind,target_id,predicate_id,machine,reason,template_id,template_version,remedy_effect_id,postcondition_id}`.
Thus the same predicate target on two machines remains two disclosed deficits.

A remedy operation has stable ID
`remop_[base64url(first-16-bytes(hash))]`, where `hash` uses domain
`fort.remedy-operation.v1` and exact object
`{machine,remedy_effect_id,postcondition_id}`. All target deficits with that
same tuple are covered by one operation and one instruction.

Instruction dependencies come only from this closed per-effect relation, never
from the readiness dependencies of every downstream predicate an effect
covers:

| Effect operation | Earlier operation when both exist on the same machine |
| --- | --- |
| Codex capability update | none |
| Codex login | Codex capability update |
| exact Codex model availability | Codex login |
| Claude install/update | none |
| Claude login | Claude install/update |
| Hermes install/update | none |
| Hermes provider/model configuration | Hermes install/update |
| OpenClaw install/update | none |
| OpenClaw main configuration | OpenClaw install/update |
| Himalaya install/update | none |
| Gmail selected read-only configuration | Himalaya install/update |
| Supabase selected read-only project | Codex capability update |

An edge exists only when both named operations are present for the same
machine; otherwise the predecessor predicate was already actually ready.
Codex model availability depends transitively on update through login. The
shared Codex update operation has no dependency merely because it also covers a
downstream binding predicate. Any unlisted effect relation or cycle is invalid.
Instructions use the resulting stable topological order, then machine registry
rank, effect catalog rank, and operation ID.

The option-relevant remedy-operation universe is sorted by machine registry
rank, effect catalog order, postcondition, and operation ID, then assigned an
immutable bit position. If that universe exceeds 64, setup solving fails
`solver_limit_exceeded`. A candidate needing more than eight distinct
operations or more than 64 target predicate deficits is ineligible. For each
enumerated machine subset, the exact chain DP state is
`(stage_index,last_machine,remedy_operation_bitset)`. Feasibility of later
transitions depends only on those globally applied machine effects, so the DP
keeps the lexicographically best accumulated assignment for an identical
state using the complete ranking tuple below, including the covered
target-deficit vector at its final tie. Different operation bitsets are never
collapsed or approximated. Every
transition unions the candidate operation bits and discards results above
eight. The target deficits and their stage coverage are retained on the winning
assignment for disclosure, not used as duplicate cost bits. The solver
evaluates every reachable bounded state before semantic deduplication, sorting,
and truncation to 16 options.

The solver retains the lexicographically smallest 16 distinct semantic setup
alternatives by:

1. number of distinct remedy operations/instructions;
2. number of target machines requiring setup;
3. number of machines in the resulting mapping;
4. number of cross-machine handoffs;
5. number of non-local stages;
6. registry-rank vector in stage order; and
7. ordered adapter/profile catalog IDs and covered predicate-deficit IDs.

This includes setup-to-single and setup-to-split recovery. For example, Gmail
already ready on the Mini plus one approved Supabase setup on the laptop
produces a valid setup-to-split option instead of `blocked`. Missing execution
profiles, models, and external capabilities are all eligible predicate
deficits. A deficit with no approved instructions, or an alternative requiring
more than eight deduplicated remedy operations, remains ineligible. Every
operation uses the one template fixed by its remedy-catalog effect row and
lists all covered target deficits. Opaque presentation IDs never participate
in ranking.

Setup search shares the same 300,000,000-transition/64 MiB ceiling with ready
placement. A limit failure is `blocked/solver_limit_exceeded`, never a partial
option list. Ready placement and setup placement use separate exact DP passes
but one combined operation/arena budget, so an approximation cannot change
whether Fort offers collocation or a split.

An explicit handoff machine pin restricts all substantive stage candidates and
setup alternatives to that machine. It never splits or falls back. Planner
placement follows its separate `planner_machine` rule above.

### Choice revision

The solver creates one timestamp-free `relevant_projection` with this exact
shape:

```json
{
  "catalog_version": 1,
  "profile_mapping_version": 1,
  "local_machine": "tobiass.macbook.pro.lan",
  "scope": {
    "profiles": [],
    "capabilities": [],
    "bindings": [
      {"profile": "codex:gpt-5.5", "capabilities": ["email.gmail.read"]}
    ]
  },
  "machines": []
}
```

`scope.profiles` and `scope.capabilities` are the deduplicated plan requirements
in catalog order. `scope.bindings` contains each distinct exact
profile/complete-capability-set pair in first-stage order. `machines` is sorted
by registry rank then name. Each machine row is exactly
`{name,local,registry_rank,reachable,protocol_version,catalog_version,profile_mapping_version,state,reason,profiles,offers,bindings}`.
The three offer arrays contain only rows relevant to `scope`, with the exact
snapshot projections—including predicates and binding revisions—defined above,
in catalog order. No observation time, URL, path, private handle, or unrelated
offer appears.

`relevant_revision` is the canonical control-plane hash with domain
`fort.relevant-inventory.v1` over that exact `relevant_projection`. The full
inventory revision remains provenance.

The exact input object for `choice_revision` is:

```json
{
  "plan_revision": "sha256:...",
  "relevant_revision": "sha256:...",
  "candidate_sets": [],
  "ready_alternatives": [],
  "setup_alternatives": [],
  "instruction_bundles": [],
  "semantic_options": []
}
```

`candidate_sets` is in plan stage order. Each row is exactly
`{stage_id,ready,setup}`. `ready` rows are sorted by machine registry rank,
binding catalog rank, profile catalog rank, then machine-name UTF-8 bytes;
`setup` rows use those keys followed by the lexicographic
`remedy_operation_ids` vector and `target_deficit_ids` vector. Duplicate sort
keys are invalid. `ready` contains rows
`{machine,profile,binding,profile_binding_revision,capability_binding_revisions,execution_binding_revision}`;
`setup` contains ordered rows
`{machine,profile,binding,remedy_operation_ids,target_deficit_ids}`. Revision
map keys exactly equal that candidate's sorted requirements; operation and
deficit arrays use their solver orders. `ready_alternatives` contains each solver-ranked
`{placement,mapping,handoffs}` value. `setup_alternatives` contains each
solver-ranked
`{mode:"instructions",deficits,instruction_bundle_ref,hypothetical_mapping,effect_summary}`
value. `instruction_bundles` contains the corresponding complete
content-derived bundles in setup-alternative order. `semantic_options` contains
every persisted/presented option object with only its opaque `id` omitted. Its
closed role order is `use_planner_profile`, `run_mapping`, `setup`,
`retry_planner`, `recheck`, `approve_plan`, `reject_plan`, `cancel`. Within
`use_planner_profile`, order is configured allowlist position then machine
registry rank; within `run_mapping` and `setup`, order is the exact ready/setup
solver ranking above; the remaining permitted roles are singleton by the
decision matrix. Role subsets retain this relative order. Role, label, target,
profiles, deficits, proofs, handoffs, instruction references, and effect
summaries remain in the hashed objects.

`choice_revision` is the canonical control-plane hash with domain
`fort.choice-revision.v1` over that exact object. Missing, null, extra,
duplicated, or differently ordered fields fail validation.

Opaque option IDs are deterministically derived with the canonical
control-plane hash. Placement uses domain `fort.placement-option.v1` and exact
object `{plan_revision,semantic_option}`; preflight uses domain
`fort.preflight-option.v1` and exact object
`{preflight_revision,semantic_option}`. Each ID is
`option_[base64url(first-16-bytes(hash))]`. `semantic_option` is the exact
presented option with only `id` omitted. Option IDs are excluded as independent
revision inputs, so an unchanged semantic choice retains stable IDs without a
hash cycle.

At selection time Fort refreshes relevant offers and recomputes this projection.
Unrelated timestamp or capability changes do not invalidate a choice; any
change that affects candidates, ranking, mapping, or setup options returns a
typed stale conflict with the refreshed preview.

## Typed human decision

`GateItem.decision` is a strict discriminated union with kinds
`capability_preflight`, `capability_placement`, and `capability_signoff`. These
records live outside the graph gate table and cannot be answered through the
legacy binary approval shape.

Every typed decision has an opaque `decision_id` matching
`capdec_[A-Za-z0-9_-]{22,86}` from a namespace disjoint from graph node IDs.
One ID identifies one unresolved actionable choice. It remains stable only
while that choice is waiting and can receive `decision_version` replacements
for refreshed options. Accepting any option terminally resolves that ID
forever. Any subsequent actionable choice receives a fresh decision ID at
version 1 and records the resolved predecessor ID. For compatibility, the
containing `GateItem.node_id` equals the current `decision_id`; it is not a
graph node ID.

Closed decision states are:

- `choice_required`
- `signoff_required`
- `planner_failed`
- `setup_instructions`
- `setup_failed`
- `capability_drift`
- `blocked`

Closed option roles are:

- `use_planner_profile`
- `run_mapping`
- `setup`
- `retry_planner`
- `recheck`
- `approve_plan`
- `reject_plan`
- `cancel`

The exact kind/state/reason matrix below is authoritative; a role is not
permitted merely because another state of the same kind uses it. Setup mode is
exactly `instructions` in v1. Unknown kind/state/reason/role combinations fail
schema validation and render read-only.

### Decision wire contract

Every decision object requires:

```json
{
  "decision_id": "capdec_...",
  "kind": "capability_placement",
  "state": "choice_required",
  "decision_version": 1,
  "predecessor_decision_id": null,
  "inventory_revision": "sha256:...",
  "relevant_revision": "sha256:...",
  "options": []
}
```

A preflight decision additionally requires `preflight_revision` and forbids
all plan/choice fields. Placement and sign-off decisions require `plan_id`,
`plan_revision`, and `choice_revision` and forbid `preflight_revision`.
`predecessor_decision_id` is required and is either null for the first decision
or the exact immediately preceding resolved capability decision. Missing,
extra, or cross-kind fields are rejected.

State-specific fields are closed:

| Kind and state | Allowed `safe_reason` | Exact option-role cardinality | Required state fields |
| --- | --- | --- | --- |
| preflight `choice_required` | absent | one or more total `use_planner_profile`/`setup`, plus exactly one `cancel` | none |
| placement `choice_required` | absent | one or more total `run_mapping`/`setup`, plus exactly one `cancel` | none |
| sign-off `signoff_required` | absent | exactly one `approve_plan` and one `reject_plan` | none |
| preflight `planner_failed` | exactly `planner_failed`, `planner_timed_out`, or `planner_invalid_output` | exactly one `cancel`; also exactly one `retry_planner` iff fewer than 3 planner dispatches have been used | `planner_attempts_used` in `1..3` |
| preflight or placement `setup_instructions` | absent | exactly one `recheck` and one `cancel` | `instruction_bundle` |
| preflight or placement `setup_failed` | one of `absent`, `auth_required`, `command_contract_changed`, `incompatible_version`, `model_unavailable`, `old_node`, `plugin_unready`, `probe_failed`, `probe_timed_out`, `project_unavailable`, `stale`, or `unreachable` | exactly one `recheck` and one `cancel` | prior `instruction_bundle` |
| placement `capability_drift` | exactly `capability_drift` | exactly one `recheck`, zero to 16 `setup`, and exactly one `cancel` | `accepted_mapping` |
| preflight or placement `blocked` with a recheckable reason | one of `absent`, `auth_required`, `command_contract_changed`, `incompatible_version`, `model_unavailable`, `old_node`, `plugin_unready`, `probe_failed`, `probe_timed_out`, `project_unavailable`, `stale`, or `unreachable` | exactly one `recheck` and one `cancel`; no `setup` | none |
| preflight or placement `blocked` with a terminal configuration/solver reason | one of `no_execution_plane`, `profile_unmapped`, `setup_not_automated`, `solver_limit_exceeded`, `static_dag_unsupported`, or `unsupported_platform` | exactly one `cancel` | none |

Every row forbids `instruction_bundle`, `accepted_mapping`, and
`planner_attempts_used` unless it requires that field. Decisions never contain
`operation_id`; during durable Recheck the run is `rechecking` with
`decision:null`. `cancellation_unconfirmed`, `output_limit_exceeded`, and
`handoff_limit_exceeded` are run safe errors, not ordinary capability-Recheck
decisions.

Each option has common required fields `id`, `role`, and `label`, then exactly:

- `use_planner_profile`: `profile`, `machine`,
  `profile_binding_revision`, `execution_binding_revision`;
- `run_mapping`: `placement`, `mapping`, and `handoffs`;
- `setup`: `mode:"instructions"`, ordered `deficits`,
  `instruction_bundle_ref`, `hypothetical_mapping`, and `effect_summary`;
- `retry_planner`, `recheck`, `approve_plan`, `reject_plan`, or `cancel`: no
  additional fields.

`deficits` contains 1–64 rows of exactly
`{deficit_id,kind:"profile"|"capability"|"binding",id,predicate_id,machine,predicate_state:"unsatisfied"|"blocked",reason,remedy_effect_id,postcondition_id}`
in machine/catalog/predicate order. `reason`, predicate, effect, and
postcondition IDs are closed; private identifiers are absent. A `blocked` row
is visibly conditional, not presented as an observed authentication/model
failure.
`instruction_bundle_ref` is exactly `{id,version}`.
Each `hypothetical_mapping` row is exactly
`{stage_id,machine,profile,binding,deficits:[<deficit-id>]}`. It intentionally
has no binding revisions because setup has not produced them; it can rank and
disclose the intended result but can never dispatch. Recheck replaces it only
with live node-issued proof. Preflight setup uses one synthetic
`stage_id:"planner"` row; placement setup uses validated plan stage IDs.
Every listed deficit ID must resolve exactly once to that option's deficit
array. Every instruction's non-empty `covers` array references one or more of
those IDs, each deficit is covered by exactly one instruction, and the union of
hypothetical-mapping references must equal the option's deficit set. Duplicate,
missing, multiply covered, or cross-option references fail validation.

Option IDs are stable opaque semantic IDs. Sign-off IDs are
`option_[base64url(first-128-bits(SHA-256("fort.signoff-option.v1" ||
plan_revision || choice_revision || role)))]`; a changed plan or accepted
mapping therefore cannot retain Approve/Reject IDs. Retry/Recheck/Cancel IDs
use domain `fort.control-option.v1` over decision ID, decision version, role,
and the referenced operation/bundle/safe reason. Clients never submit commands,
profiles, adapters, mappings, machine names, capabilities, or instructions as
authority.

Each mapping entry is:

```json
{
  "stage_id": "read-email",
  "machine": "taloss.mac.mini.lan",
  "profile": "codex:gpt-5.5",
  "binding": "codex-appserver+gmail",
  "profile_binding_revision": "opaque:...",
  "capability_binding_revisions": {
    "email.gmail.read": "opaque:..."
  },
  "execution_binding_revision": "opaque:..."
}
```

The map keys must exactly equal the stage requirements. Handoff entries require
`output`, `from`, `to`, `format`, and `max_bytes`; they are sorted by stage
order.

An incident placement payload therefore contains one `run_mapping` option whose
first mapping entry is the Mini's `codex-appserver+gmail` binding, whose second
is the laptop's `codex-appserver+supabase` binding, and whose one disclosed
handoff is `incident_evidence`. A setup-to-split option carries the complete
hypothetical mapping and ordered bundle needed to make it ready.

### Closed transitions

| Current state | Accepted role/system result | Next state |
| --- | --- | --- |
| preflight `choice_required` | `use_planner_profile` | durable planner-dispatch work |
| placement `choice_required` | `run_mapping` | sign-off decision or durable plan-compile work |
| `signoff_required` | `approve_plan` | durable plan-compile work |
| `signoff_required` | `reject_plan` | terminal `canceled` with rejected audit outcome |
| actionable preflight/placement decision | `setup` | resolve it; create a fresh `setup_instructions` decision |
| `planner_failed` | `retry_planner` | resolve it; run `planning` with no decision and a new durable planner attempt |
| setup/drift/recheckable-blocked decision | `recheck` | resolve it; run `rechecking` with no decision plus durable Recheck work |
| run `rechecking` | exact authorized proof ready | assignment/placement/resume, or a fresh mapping/sign-off decision when required |
| run `rechecking` | still non-ready | fresh `setup_failed`, `blocked`, or `choice_required` decision |
| any state with Cancel | `cancel` | `canceling` plus durable cancel work |
| running | target guard/version fails | fresh placement drift/choice/blocked decision |

Only a refresh/stale replacement of an unresolved choice increments
`decision_version` on the same ID. An accepted option resolves that record,
removes it from `/api/gates`, and retains it for audit/idempotent replay. Setup
instructions, planner retry failure, Recheck outcome, placement sign-off, and
later drift always allocate fresh IDs linked through
`predecessor_decision_id`; they never reopen a resolved decision.

### Selection request

`POST /api/gate` accepts exactly one legacy binary shape or this typed shape:

```json
{
  "run_id": "run-...",
  "node_id": "capdec_...",
  "selection": {
    "decision_id": "capdec_...",
    "kind": "capability_placement",
    "option_id": "option-...",
    "decision_version": 1,
    "plan_id": "plan-...",
    "plan_revision": "sha256:...",
    "inventory_revision": "sha256:...",
    "relevant_revision": "sha256:...",
    "choice_revision": "sha256:...",
    "idempotency_key": "018f3f1c-7d3a-7c1d-a176-9c52c606c6e4"
  }
}
```

For typed requests, top-level `node_id`, `selection.decision_id`, and the
persisted `decision_id` must all match. Preflight selection replaces plan/choice
fields with `preflight_revision`; sign-off uses the placement proof shown
above. The strict JSON union rejects extra fields. For a legacy-shaped request,
a non-empty `edit` against `capability_signoff` returns
`409 plan_edit_unsupported`; legacy `approve`/`reject`, and `edit` against any
other typed kind, return `409 option_required`. This kind/body precedence is
evaluated before legacy action semantics; nothing resumes or dispatches.

### Atomicity and idempotency

Before accepting a planner/profile, mapping, or setup option, the controller
performs the bounded relevant refresh, publishes a monotonically increasing
`inventory_generation`, and recomputes the semantic projection. Cancel,
Reject plan, and an exact idempotent replay do not need a capability refresh.
Full inventory revision is provenance, not a whole-snapshot equality
precondition.

If semantic input changed, one CAS transaction replaces the still-waiting
decision's frozen relevant projection and option set/proof, increments
`decision_version`, and returns
`409 stale_inventory` with that exact persisted view. If a newer relevant
generation wins the refresh-to-transaction race, Fort recomputes before
replacement or acceptance. This same generation CAS applies to initial
preflight assignment and initial auto-placement. Three internal CAS losses
return retryable `decision_busy`; no stale automatic assignment is committed.

Acceptance is one store transaction that:

1. verifies the stable decision ID, kind, waiting state, and submitted
   `decision_version`;
2. resolves the opaque option from the persisted option set;
3. verifies its preflight or plan/relevant/choice proof against the current
   stored generation;
4. persists the accepted option and exact original response status/body;
5. persists any one-run planner override, setup bundle, or exact mapping and
   binding proofs;
6. appends the audit event;
7. terminally resolves the current decision and changes the durable lifecycle
   state;
8. creates any immediately required successor decision with a fresh ID,
   version 1, and `predecessor_decision_id`; and
9. inserts the unique next outbox work item when follow-on work is required.

The transaction commits before workers are woken. Events are audit history,
never source of truth. Initial single-host auto-placement uses this same
generation-CAS/mapping/outbox transaction; automatic placement resolves
placement but never approves generated content.

Idempotency keys are either canonical UUIDs or 22–128 character unpadded
base64url strings and are unique within `(run_id,decision_id)`. Exact replay of
the byte-equivalent normalized selection returns the original body **and
original HTTP status**, even after the run advances. Reusing a key for different
normalized fields returns `409 idempotency_conflict`. After any option is
accepted, a different key—including one naming the same option—returns
`409 already_decided` for that resolved decision. A successor decision has its
own key scope and cannot be mistaken for the predecessor.

Selecting setup returns its original `200`; Recheck returns its original `202`;
Cancel returns its original `202`; sign-off and accepted mapping return their
original `200`.

### Setup instructions and Recheck

Selecting setup never approves the plan or resumes substantive work. It stores
the exact server-owned bundle referenced by the option:

```json
{
  "id": "instruction-bundle_...",
  "version": 1,
  "effect_summary": "Configure read-only Gmail access for Fort.",
  "instructions": [
    {
      "id": "instruction_...",
      "version": 1,
      "operation_id": "remop_...",
      "template_id": "setup.gmail.configure-readonly.v1",
      "template_version": 1,
      "target": "tobiass.macbook.pro.lan",
      "covers": ["def_..."],
      "remedy_effect_id": "effect.gmail.selected-imap-read.v1",
      "postcondition_id": "postcondition.gmail.selected-imap-read.v1",
      "effect_summary": "Configure the Fort Gmail broker.",
      "steps": [
        {"kind": "text", "text": "On this Mac, configure the Fort-selected Gmail IMAP account with TLS, then return to Fort and choose Recheck."},
        {"kind": "link", "label": "Himalaya v1.2 setup", "url": "https://github.com/pimalaya/himalaya/tree/v1.2.0"}
      ]
    }
  ]
}
```

Instruction IDs are deterministic content IDs. For one instruction, hash the
exact object
`{version,operation_id,template_id,template_version,target,covers,remedy_effect_id,postcondition_id,effect_summary,steps}`
with the canonical control-plane algorithm and domain
`fort.setup-instruction.v1`; its ID is
`instruction_[base64url(first-16-bytes(hash))]`. The displayed `id` is excluded
from its own input.

The bundle ID uses domain `fort.setup-bundle.v1` over the exact object
`{version,effect_summary,instructions}`, where `instructions` is the ordered
array of complete derived instruction objects including their IDs. Its ID is
`instruction-bundle_[base64url(first-16-bytes(hash))]`; the bundle's displayed
`id` is excluded. Re-solving byte-identical semantics reuses these IDs. If one
truncated ID ever resolves to different canonical semantics, option generation
fails closed with `solver_limit_exceeded`; Fort never salts or randomly
reissues an ID because those IDs participate in `choice_revision`.

A bundle has 1–8 instructions, each has 1–12 steps, and total canonical bundle
JSON is at most 32 KiB. Each instruction's `operation_id`, target, non-empty
ordered `covers`, `template_id`, `template_version`, `remedy_effect_id`, and
`postcondition_id` must resolve to exactly one deduplicated remedy operation
and to every covered option deficit. `covers` has at most 64 IDs in option
deficit order. Instruction ordering is the stable dependency/topological order
defined by setup ranking. The bundle ID/version, every instruction/step,
effect summary, covered deficit predicate/reason/effect/postcondition, target,
and hypothetical mapping participate in `choice_revision`.

Text is plain UTF-8 without HTML and at most 2 KiB per step. Links are `https`
only, with label ≤160 bytes and URL ≤2 KiB. `display_command` has only
`kind` and `argv`; argv has 1–16 non-empty UTF-8 elements, each ≤512 bytes and
≤2 KiB total, with no NUL, newline, Unicode control, or line/paragraph separator
characters. Clients render arguments as literal tokens. A Copy action copies
the exact JSON argv array or one selected literal argument; clients never join
it into a shell command and never offer Run. Fort executes no instruction.
Credentials and OAuth remain interactive and never transit the gateway.

Gate-level Recheck is an opaque selection through `POST /api/gate`; the global
inventory endpoint alone cannot advance a run. Its transaction enters
`rechecking`, terminally resolves the selected decision, records an
`operation_id` on the run, sets `decision:null`, and inserts one durable
`capability_recheck` work item. The worker uses the exact `user_recheck` remote
refresh contract, then publishes the next result transactionally. Any next
actionable choice is a fresh decision at version 1 linked to the resolved
Recheck decision; repeated Recheck never reopens an accepted ID.

A setup Recheck may adopt changed binding revisions only for the same
hypothetical mapping and deficits explicitly authorized by that selected setup
bundle. It atomically persists the refreshed frozen projection, logical/profile/
execution binding proofs, relevant/choice revisions, audit event, and next
dispatch work item; it regenerates every immutable input contract from that new
choice revision before any receipt or stage dispatch. A drift Recheck resumes automatically only when the exact
accepted mapping **and all accepted binding revisions** are ready again. If any
machine changes, or a ready binding revision changed outside an authorized
setup, Fort emits a new disclosed `run_mapping` option—even when machine names
are unchanged—and requires selection. It never silently switches account,
project/root, executable, tool policy, profile, or machine.

### Cancel

Cancel atomically records the accepted option, moves the run to `canceling`,
removes the decision from `/api/gates`, and inserts one `dispatch_cancel` work
item. It returns `202` without performing external work in the request handler.
The worker cancels/reconciles every active stable dispatch and marks terminal
`canceled` only after local process groups and remote targets confirm a
terminal state. A run with no active dispatch confirms immediately through the
same worker path.

If cancellation cannot be confirmed after the bounded retry/reconciliation
policy, the run becomes `blocked` with `cancellation_unconfirmed`; Fort never
starts replacement work. Cancel is not graph rejection or a generic provider
failure. The eventual `RunSummary.status`, run detail, capability view, and SSE
expose `canceled`; the original idempotent ActionResult remains its accepted
`202 canceling` response.

A user can cancel while no gate is open through:

```text
POST /api/runs/{run_id}/cancel
Content-Type: application/json

{"idempotency_key":"018f3f1c-7d3a-7c1d-a176-9c52c606c6e4"}
```

This coordinator endpoint requires control authentication and the browser-mode
Origin/CSRF rules below. Its strict body has only `idempotency_key`. For
`planning`, `needs_user`, `rechecking`, `awaiting_plan_approval`, `running`, or
`blocked` with `dispatch_state_unknown`/`handoff_state_unknown`, one transaction
resolves any open decision with a system-cancel audit outcome, moves the run to
`canceling`, inserts the unique `dispatch_cancel` outbox row, and stores the
original `202 CapabilityRunView`. Cancellation addresses every persisted
dispatch ID; an unknown receipt can remain retained because no downstream
dispatch may start after cancel acceptance.

`blocked/cancellation_unconfirmed` permits a new reconciliation generation
under a new key, using the same persisted dispatch, attempt, and process
identities but a new per-generation epoch/offset cursor. It never rewrites an
older generation's `unconfirmed` result. V1 permits at most three total
cancellation-reconciliation generations per run, including the original;
exhaustion returns `409 operator_required` and leaves work blocked for explicit
machine/operator intervention. The closed transitions are
`nonterminal → canceling(g) → canceled|blocked/cancellation_unconfirmed` and
`blocked/cancellation_unconfirmed + new key + g<3 → canceling(g+1)`.

The idempotency/terminal precedence is exact:

1. an existing byte-equivalent record for the submitted key returns its
   original status/body in every later state;
2. a currently `canceled` run with any new key returns its current view `200`;
3. a currently `succeeded` or `failed` run returns `409 run_terminal`;
4. a currently `canceling` run with a new key returns
   `409 already_canceling`;
5. eligible blocked states follow the generation rules above; and
6. any other accepted nonterminal state creates generation 1 and returns
   `202`.

Reusing a recorded key with different normalized bytes is still
`idempotency_conflict`. No handler contacts a runtime directly. Gate-level
Cancel and this endpoint converge on the same durable cancellation state
machine.

## Capability views

All run-bearing responses use one exact additive envelope:

```json
{
  "run_id": "run-...",
  "state": "needs_user",
  "decision": {},
  "plan": null,
  "safe_error": null,
  "operation_id": null
}
```

This is `CapabilityRunView`. Closed run states are `planning`, `needs_user`,
`rechecking`, `awaiting_plan_approval`, `running`, `canceling`, `blocked`,
`succeeded`, `failed`, and `canceled`. The exact field matrix is:

| Run state | `decision` | `plan` | `safe_error` | `operation_id` |
| --- | --- | --- | --- | --- |
| `planning` | null | null | null | null |
| `needs_user` | one non-signoff preflight/placement decision | null for preflight; summary required for placement | null | null |
| `rechecking` | null | null for preflight; summary required for placement | null | required non-empty opaque ID |
| `awaiting_plan_approval` | one `capability_signoff/signoff_required` decision | required summary | null | null |
| `running` | null | required summary | null | null |
| `canceling` | null | null before planning or the persisted summary afterward | null | null |
| `blocked` | null | null before planning or the persisted summary afterward | required with code `cancellation_unconfirmed`, `dispatch_state_unknown`, or `handoff_state_unknown` | null |
| `succeeded` | null | required for capability/static execution; null for planner-only breakdown | null | null |
| `failed` | null | null before plan persistence or the persisted summary afterward | required terminal safe error | null |
| `canceled` | null | null before planning or the persisted summary afterward | null | null |

A decision whose own state is `blocked`, `planner_failed`, `setup_failed`, or
`capability_drift` still makes the run `needs_user` because it has a closed
action set. A malformed combination fails encoding rather than being loosened
with nullability.

`safe_error` is exactly `{code,message}`. `message` is a fixed server-owned
catalog string and `code` is a closed safe reason. In `failed`, it is exactly
one of `planner_failed`, `planner_timed_out`, `planner_invalid_output`,
`runtime_failed`, `output_limit_exceeded`, or `handoff_limit_exceeded`.
Private provider/probe output, stage content, machine URL, and identifiers are
forbidden.

When present, `plan` is the prompt-free `CapabilityPlanSummary`:

```json
{
  "plan_id": "plan-...",
  "plan_revision": "sha256:...",
  "inventory_revision": "sha256:...",
  "relevant_revision": "sha256:...",
  "choice_revision": "sha256:...",
  "stages": [
    {
      "id": "read-email",
      "order": 1,
      "title": "Read the Supabase failure email",
      "profile": "codex:gpt-5.5",
      "requires": ["email.gmail.read"],
      "input_from": [],
      "output": "incident_evidence",
      "output_format": "text",
      "max_output_bytes": 65536
    }
  ],
  "mapping": [],
  "handoffs": []
}
```

Mapping/handoffs are persisted accepted values, never live recomputation.
Prompts, direction text, outputs, probe details, private identifiers, and
instructions outside the current setup decision are omitted.

The authenticated plan-detail representation is `CapabilityPlanDetail`. It
contains every summary field plus:

```json
{
  "source": {
    "kind": "generated",
    "playbook_id": "playbook-id",
    "playbook_revision": 7
  },
  "direction": "<exact persisted original direction>",
  "direction_digest": "sha256:...",
  "stages": [
    {
      "id": "read-email",
      "order": 1,
      "title": "Read the Supabase failure email",
      "prompt": "Read the relevant Gmail message and extract bounded incident evidence.",
      "profile": "codex:gpt-5.5",
      "requires": ["email.gmail.read"],
      "input_from": [],
      "output": "incident_evidence",
      "output_format": "text",
      "max_output_bytes": 65536,
      "attempt_policy": {
        "provider_max_attempts": 1,
        "deadline_seconds": 900
      }
    }
  ]
}
```

The source object is a strict union. `generated` requires exactly
`kind,playbook_id,playbook_revision` and forbids `source_digest`; `static`
requires exactly those fields plus `source_digest`. Its source fields,
direction, exact prompts, output policies, and attempt policies are immutable
fields already bound by `plan_revision`. It contains no outputs or private
capability identifiers. Every web/iOS/macOS sign-off surface must fetch and
render this detail before enabling **Approve plan**; the submitted sign-off
`plan_revision`/`choice_revision` must equal both the detail and current
decision. Summary-only watchOS/CarPlay remains read-only.

`RunDetail` retains `{run,nodes,events}` and adds
`capability:CapabilityRunView|null`. Typed `ActionResult` retains its existing
legacy `state` semantics and adds `capability:CapabilityRunView`; it does not
overload the legacy field. Structured-error `preview` is the same
`CapabilityRunView`. `GET /api/capability-plans/{plan_id}` requires control
authentication and returns `CapabilityPlanDetail`; run-bearing responses expose
only `CapabilityPlanSummary`.

## Structured conflicts

Typed `409` responses use:

```json
{
  "error": {
    "code": "stale_inventory",
    "message": "Capabilities changed since this choice was prepared.",
    "expected_relevant_revision": "sha256:...",
    "current_relevant_revision": "sha256:...",
    "preview": {
      "run_id": "run-...",
      "state": "needs_user",
      "decision": {},
      "plan": null,
      "safe_error": null,
      "operation_id": null
    }
  }
}
```

Codes include:

- `already_decided`
- `already_canceling`
- `capability_drift`
- `decision_busy`
- `idempotency_conflict`
- `no_execution_plane`
- `operator_required`
- `option_required`
- `plan_edit_unsupported`
- `run_terminal`
- `stale_decision`
- `stale_inventory`
- `stale_plan`
- `stale_preflight`
- `static_dag_unsupported`
- `unknown_option`

`stale_decision` is a decision-version mismatch while the same decision remains
open. `stale_preflight` is a submitted preflight-proof mismatch;
`stale_plan` is a submitted plan/choice-proof mismatch; `stale_inventory` means
a fresh solver-relevant observation changed the persisted semantic options.
All carry the current persisted run view. `already_decided` carries the
terminal resolution and current view, including pre-plan/system-cancel
outcomes.
`idempotency_conflict` identifies only the scoped decision, never either body.
`capability_drift` carries safe revisions but no raw diagnostics.
`unknown_option` and `option_required` return the current decision;
`decision_busy` is retryable and carries the latest view.

Every control-plane typed conflict requires exactly `code`, one server-owned
safe `message`, and `preview`, plus:

- `stale_decision`: `expected_decision_version`,
  `current_decision_version`;
- `stale_preflight`: `expected_preflight_revision`,
  `current_preflight_revision`;
- `stale_plan`: expected/current plan and choice revisions;
- `stale_inventory` or `capability_drift`: expected/current relevant
  revisions;
- `already_decided`: `resolution`, exactly
  `{kind:"option",option_id}` or `{kind:"system_cancel"}`;
- `idempotency_conflict`: `decision_id`; and
- `unknown_option`, `option_required`, `decision_busy`,
  `plan_edit_unsupported`, `no_execution_plane`, or
  `static_dag_unsupported`, `already_canceling`, `operator_required`, or
  `run_terminal`: no
  additional fields.

Cross-code fields are forbidden. Node-only conflicts are a separate mesh API
union and never contain `preview`:

- `dispatch_conflict` is
  `{error:{code,message,run_id,dispatch_id}}`;
- `handoff_conflict` is
  `{error:{code,message,run_id,receipt_id}}`;
- `node_capability_drift` is the exact bounded mismatch envelope defined by
  guarded execution; and
- `node_version_mismatch` is the exact expected/current tuple envelope defined
  there.

The coordinator validates and translates node-only errors into durable run
state and a new client-facing decision before returning any control-plane
response. A node cannot return `capability_drift` directly.

Clients retain and decode response bodies rather than reducing conflicts to
plain text.

### Authorization and CSRF

Opaque decision/option IDs are not authorization. Every control mutation,
including typed `POST /api/gate` and local capability Recheck, requires an
authenticated control principal. V1 has four non-overlapping access modes:

| Client path | Authentication checked by | Browser Origin/CSRF | Inner Fort authorization |
| --- | --- | --- | --- |
| loopback browser on `127.0.0.1`/`::1` | inner Fort | inner Fort requires exact configured Origin and per-session CSRF on mutations | Fort-issued `HttpOnly`, `Secure` where applicable, `SameSite=Strict` session |
| encrypted-gateway browser | outer gateway broker plus inner daemon | outer broker requires its exact Origin and gateway-session CSRF both when minting an assertion and when posting the sealed relay frame | active browser client authority bound to the actual Noise initiator plus one request-bound signed assertion inside ciphertext |
| encrypted-gateway iOS/macOS native client | outer gateway broker plus inner daemon | not applicable; no browser cookie or CSRF | active native client authority bound to the actual Noise initiator plus one request-bound signed assertion inside ciphertext |
| same-user local control CLI | inner Fort on loopback | not applicable | daemon-start random bearer read from the owner-only runtime token file |
| direct non-loopback browser/native client | unsupported | unsupported | use the encrypted gateway |

Loopback session issuance is available only on `127.0.0.1`/`::1`; the CSRF
token is returned in bootstrapped UI JSON/document state rather than a cookie.
A browser never receives or stores the native bearer.

#### Gateway client authority

A gateway-signed assertion is necessary but not sufficient inner
authorization. The daemon also requires an active daemon-enrolled client
authority whose stored X25519 public key equals the initiator static public key
recovered by the Noise IK responder from the current handshake transcript. The
responder supplies that recovered key through trusted connection context; no
HTTP header, body, broker routing field, or JWS claim can supply or override it.

Native FortKit keeps one persistent Noise IK initiator static keypair per
gateway/node authority in the app Keychain. Gateway browser code keeps one
persistent origin/node keypair in origin storage. Noise ephemeral keys remain
fresh per session. A fresh per-session initiator static key is not valid for
control traffic. An unknown or revoked initiator may access only the sealed
authority-enrollment route below; every other inner route fails `401` before
the control mux.

The local operator creates an invite with exactly:

```text
fort relay client invite --mode gateway_native
fort relay client invite --mode gateway_browser
```

The daemon generates 32 random bytes, prints their 43-character unpadded
base64url form, and stores only its SHA-256 plus `node_id`, the exact gateway
issuer, mode, creation time, ten-minute expiry, and eventual consumed binding.
It permits one successful binding. The client sends the code only inside its
Noise-sealed request:

```text
POST /api/control-authorities/enroll
Authorization: Fort-Enroll <code>

{
  "protocol_version": 1,
  "node_id": "enrolled-node-id",
  "gateway_issuer": "https://gateway.example",
  "amr": "gateway_native"
}
```

The daemon requires the body fields to equal the invite, verifies expiry,
computes the authority ID below from the actual responder-recovered initiator
key, and atomically consumes the invite into:

```json
{
  "authority_id": "fca_<43-char-unpadded-base64url>",
  "node_id": "enrolled-node-id",
  "gateway_issuer": "https://gateway.example",
  "amr": "gateway_native",
  "noise_ik_initiator_public_key_b64u": "<43-char-unpadded-base64url>",
  "scopes": ["control.read", "control.write"],
  "state": "active"
}
```

The key is exactly the 32-byte initiator static recovered from Noise, never a
body field. A same-code replay from that same recovered key returns the same
authority while the consumed invite record is retained; a different key fails.
Revocation is daemon-owned, immediately rejects new sessions, and requires
loopback/local-CLI authority or an already active `control.write` authority.

After enrollment, the client binds `{node_id,authority_id}` to its authenticated
outer browser session or native-bearer record. That outer binding is routing
state, not the authorization root. Assertion minting returns `403` unless the
current session/bearer has exactly one matching node authority and `amr`.
Therefore a compromised gateway that holds its own signing key and knows the
daemon public key still cannot originate native control traffic: it lacks the
enrolled Noise initiator private key. Gateway-served browser JavaScript retains
spec 028's explicitly documented web-origin residual trust because it can use
that origin's browser initiator key.

For a gateway request, the authenticated browser first calls the outer broker:

```text
POST /v1/control-assertions

{
  "node_id": "enrolled-node-id",
  "authority_id": "fca_...",
  "method": "POST",
  "request_target_sha256": "sha256:...",
  "body_sha256": "sha256:..."
}
```

For browser authentication the broker validates its user/machine session,
exact Origin, and CSRF token. For native authentication it instead validates
the existing outer gateway native bearer already used by FortKit to select and
open that machine's encrypted tunnel; it accepts no browser cookie and requires
no CSRF/Origin. Both modes validate target enrollment, closed HTTP method,
lowercase SHA-256 of the exact inner
request-target bytes (path plus canonical query), and lowercase SHA-256 of the
exact inner body bytes. It returns one signed token with claims exactly
`{iss,kid,sub,amr,aud,node_id,scope,method,request_target_sha256,body_sha256,iat,exp,jti}`.
`aud` and `node_id` are the enrolled inner daemon, scope is server-derived from
the method as `control.read` or `control.write`, expiry is at most 60 seconds
after issuance (`exp=iat+60` in v1), `jti` is 128 bits of randomness, and `amr` is exactly
`gateway_browser` or `gateway_native` as derived from outer authentication.

`iss` is the exact canonical HTTPS gateway origin stored at relay/client
enrollment: lowercase scheme and DNS host, IDNA A-label host, default port 443
omitted, non-default port retained, and no trailing slash, userinfo, path,
query, or fragment. Let `B64U` mean unpadded base64url and all hash inputs below
be exact bytes. The signing-key ID is:

```text
kid = "gwk_" + B64U(SHA-256(
  ASCII("fort.gateway-signing-key.v1") || 0x00 ||
  raw_ed25519_public_key_32
))
```

The subject is the active client-authority ID computed and stored by the inner
daemon at enrollment:

```text
sub = authority_id = "fca_" + B64U(SHA-256(
  ASCII("fort.control-authority.v1") || 0x00 ||
  U64BE(len(UTF8(iss)))     || UTF8(iss) ||
  U64BE(len(UTF8(node_id))) || UTF8(node_id) ||
  U64BE(len(UTF8(amr)))     || UTF8(amr) ||
  raw_noise_ik_initiator_public_key_32
))
```

Lengths are unsigned 64-bit big-endian byte lengths. `iss`, `kid`, and `sub`
are strings. They never contain an email, OIDC subject, machine display name,
or secret.
The outer broker sees only digests, not an inner route, plan ID, query, option,
or body; the inner daemon enforces its own closed route/scope table after
decryption.
The successful response is exactly
`{assertion:"<compact-jws>",token_type:"Fort-Control",expires_in:60}`.

The assertion is a compact JWS using Ed25519/`EdDSA` (RFC 7515). The protected
header is exactly
`{"alg":"EdDSA","kid":"gwk_<43-char-unpadded-base64url>","typ":"fort-control+jwt"}`;
the payload is exactly the claims above. The protected-header `kid` and payload
`kid` are both required, byte-for-byte equal, and equal the identifier derived
from the 32-byte Ed25519 public key that verifies the signature. Header and
payload JSON use RFC 8785
canonicalization, each segment and the 64-byte signature use unpadded base64url,
and the signing input is the ASCII
`base64url(header) + "." + base64url(payload)`. `iat` and `exp` are integer
UTC NumericDate seconds, `aud` is one string, `scope` and `amr` are one string
each, and `jti` is unpadded base64url of exactly 16 random bytes. Every digest
claim/request field is `sha256:` followed by 64 lowercase hexadecimal
characters. The request-target digest covers the exact ASCII origin-form target
stored in the sealed inner request; the body digest covers its exact body
bytes, including zero bytes for an empty body. Unknown protected headers,
claims, JSON types, padding, or algorithms fail closed.
The inner daemon requires `0 < exp-iat <= 60`, permits at most five seconds of
clock skew at either boundary, and still consumes `jti` until `exp+5`.

The browser or FortKit native relay inserts that token as
`Authorization: Fort-Control <token>` in the plaintext inner HTTP headers
**before** sealing the request, then sends the opaque ciphertext through the
existing relay `POST /api/req`. On that relay POST the outer broker revalidates
the same browser session/Origin/CSRF or native bearer mode used to mint the
assertion, then forwards ciphertext byte-for-byte. It cannot inject, strip,
read, or choose an inner header, path, option, or body.
The inner daemon verifies signature and then requires all of these equalities:

- payload `kid` equals protected-header `kid` and the signing public key's
  derived ID;
- payload `iss` equals both that key's trust-record issuer and the active client
  authority's `gateway_issuer`;
- payload `sub` equals the active authority's stored `authority_id` and the
  value recomputed from the responder-recovered Noise initiator static key;
- `aud` and `node_id` both equal the exact target enrollment node ID;
- `amr` equals both the outer-authentication mode and the authority record;
- `scope` appears in the active authority record and matches the inner closed
  method/route scope table; and
- method, request-target digest, body digest, expiry, and unused `jti` match the
  actual sealed request.

No asserted client/Noise key can override the handshake context. Any inequality
fails `401` before processing. The daemon atomically consumes `jti`; replay is
`401`. An SSE assertion is consumed at the authenticated GET handshake and does
not expire the established stream.

Gateway signing keys are Ed25519 keys distributed through the existing
machine-enrollment trust record. A broker advertises the next public key before
activation; nodes accept active plus previous key for at most five minutes,
longer than the 60-second token TTL, then delete the previous key. An unknown
`kid`, activation before enrollment propagation, or stale key fails closed.
Private signing material never reaches a client or inner node.
Signing-key rotation changes `kid` but not `iss` or `sub`. Changing gateway
issuer, node, client mode, or initiator static key requires a new daemon client
authority enrollment.

Browser CSRF is also one exact wire. The cookie name is `fort_session` for
loopback Fort and `fort_gateway_session` for the outer gateway; both are
`HttpOnly`, `SameSite=Strict`, `Path=/`, and `Secure` whenever their configured
origin is HTTPS. Their authenticated bootstrap JSON adds
`{"control_auth":{"mode":"loopback_browser"|"gateway_browser","csrf_token":"<43-char-unpadded-base64url>","expires_at":"RFC3339"}}`.
The token is exactly 32 random bytes, stored server-side with that session, and
sent on every browser mutation as `X-Fort-CSRF`. The authenticating hop compares
it in constant time and requires the `Origin` header to equal its configured
scheme/host/port byte-for-byte after lowercase host normalization; there is no
Referer fallback. Gateway-native bearer requests send neither cookie nor CSRF
header.

For the same-user local CLI, the daemon creates 32 random bytes at each start,
writes their unpadded base64url form through an owner-only create-new temporary
file in its configured runtime directory, fsyncs it, and atomically renames it
to `fort-control.token`. Before replacement it opens the directory without
links and rejects an existing target unless it is a regular non-linked file
owned by the daemon UID with mode `0600`; it then fsyncs the directory. This
safely replaces a crash-stale token but never a symlink/foreign file. The daemon
removes the file during clean shutdown. It
accepts that exact value as `Authorization: Bearer <token>` only from loopback
and grants the local node's `control.read`/`control.write`; restart invalidates
it. Failure to create or permission-check the token file disables local CLI
control rather than falling back to unauthenticated HTTP. A sandboxed native
app never reads this file; iOS/macOS FortKit uses the gateway-native assertion
path.

Node inventory/recheck/exec/handoff endpoints require the separate peer mesh
token and never accept user control assertions/bearers. A browser mutation with
missing/mismatched Origin or CSRF fails at the browser-authenticating hop.
Native bearer requests require no Origin/CSRF. Authentication failures are
ordinary `401/403`, not typed `409`. The capability feature cannot be enabled
until each used mode and key-rotation/replay path is contract-tested.
Inner control reads of machines, capabilities, runs, gates, and plan detail
require `control.read`; every gate, Recheck, run-cancel, backlog, breakdown, or
other substantive mutation requires `control.write`.

## Persistence and reconstruction

Add run-scoped durable records for:

- planner preflight safe projection, option set/revision, assignment, and any
  explicit one-run profile override;
- validated capability plan with source/static digest, direction, exact
  prompts, output policies, and attempt policies;
- safe frozen solver projection plus
  plan/catalog/inventory/relevant/choice revisions;
- typed decision ID/payload, predecessor, option set, decision version,
  terminal resolution, and successor link;
- accepted option, normalized idempotency request, and original response
  status/body;
- run-cancel idempotency request, original response, reconciliation generation,
  per-generation/per-dispatch immutable results, and cancellation schedule;
- exact stage placement, profile/logical/composite binding proofs, and
  handoffs;
- setup instruction bundle and Recheck operation state;
- durable continuation/outbox claims, fencing generations, dispatch/attempt
  IDs, semantic request hashes, sequenced target journals, coordinator
  high-water marks, and terminal runtime results; and
- immutable input contracts and local/remote handoff receipts, physical-send
  reservations/acknowledgments, fixed status-reconciliation cursor, plus the
  per-run handoff byte ledger.

The inner daemon also durably records client-authority invites/consumed
bindings/revocations and consumed gateway assertion `jti` values until their
expiry, so restart cannot forget a revoked initiator or replay a still-valid
write assertion. The outer gateway durably binds each browser session/native
bearer to its exact `{node_id,authority_id,amr}` routing tuple; it never stores
the Noise private key.

Typed capability decisions live in a dedicated capability-decision table and
are projected into `GateItem` by the control layer. They are never encoded as a
legacy graph `gate` node. `graph.Executor.Approve/Reject`, direct store gate
mutation, and the legacy CLI approval path cannot find or resume them; every
surface must call the typed decision controller.

Generated capability data is not written into reusable playbook revisions. Old
clients therefore cannot decode and re-save a playbook while dropping it.

Store methods use compare-and-set transactions. Failure to persist a plan,
choice, mapping, proof refresh, sign-off, work item, or runtime attempt prevents
dispatch.

Flow reconstruction first looks for a persisted capability plan by run ID. It
compiles that exact plan and accepted placement; it never reconstructs dynamic
stages from the current playbook catalog or calls the live placer.

## Compilation, execution, and handoff

The persisted sequential plan compiles to task nodes stamped with:

- execution profile;
- provider-native model;
- closed execution binding;
- expected profile, logical-capability, and composite binding revisions;
- exact machine;
- logical requirements;
- named inputs and output;
- plan and choice proof; and
- bounded output policy and immutable attempt policy.

Only task nodes invoke the runtime.

Each stage receives:

- the original direction;
- its persisted prompt; and
- only receipt references for the explicitly approved prior outputs in
  `input_from`.

It does not implicitly receive every memory output. Cross-machine transfer is
limited to validated UTF-8 text or JSON within the approved bounds.

Immediately before dispatch, the target revalidates the exact executable,
profile, model, selected execution binding, private capability bindings, and
requirements. Drift:

1. invokes zero provider/model calls;
2. returns only the node drift/version envelope to the coordinator;
3. fences the work and persists the refreshed run state;
4. creates a fresh typed **Needs you** choice under the closed matrix; and
5. never changes machine automatically.

Transport reconnect, claim transfer, and crash resume reuse the persisted
machine, requirements, receipts, output/attempt policy, binding proofs, and
stable dispatch ownership contract. Substantive provider retry is unsupported
in v1.

## API

Additive inner-daemon contracts:

- `GET /api/machines` adds non-null `profiles: []`, non-null `offers: []`,
  non-null `bindings: []`, and machine capability state.
- `GET /api/capabilities` returns the normalized snapshot.
- `POST /api/capabilities/recheck` performs a bounded refresh and returns the
  resulting snapshot; it never advances a run decision.
- `GET /api/capability-plans/{plan_id}` is control-authenticated and returns
  `CapabilityPlanDetail`; run envelopes carry only `CapabilityPlanSummary`.
- `POST /api/gate` accepts the strict typed selection union and returns
  `ActionResult` with additive `capability:CapabilityRunView` while preserving
  legacy `state`.
- `POST /api/runs/{run_id}/cancel` is the control-authenticated idempotent
  run-level cancel path.
- `GET /api/node/capabilities` is mesh-authenticated.
- `POST /api/node/capabilities/recheck` is the mesh-authenticated bounded remote
  refresh path.
- `PUT /api/node/handoffs/{receipt_id}` and
  `GET /api/node/handoffs/{receipt_id}/status` store/reconcile immutable
  mesh-authenticated portable-input receipts.
- `POST /api/node/exec-capability` is the versioned guarded execution path.
- `GET /api/node/exec-capability/{dispatch_id}` and its `/events?after=`
  subresource are the mesh-authenticated durable status/journal reconnect path.
- `POST /api/node/cancel-capability` idempotently cancels one authenticated
  stable dispatch ID.

`ChatResult` gains optional `state` and `plan_id`; state is one of the closed
run states above. `BreakdownResult` keeps its existing `run_id` and gains
optional `state`. New fields are defaultable in FortKit and omitted on legacy
responses.

`POST /api/route` stays unchanged and pure.

The outer gateway broker's `/api/machines` remains only its authenticated relay
machine picker. Capability offers belong exclusively to the inner Fort daemon
response reached through the sealed relay.
The outer broker additionally owns `POST /v1/control-assertions` for the exact
browser/native pre-seal exchange above; it is not an inner-daemon route.

## Clients and gateway

Both local-web gate renderers, iOS Board and Gates, macOS `FortWindow` and
`MenuContent`, and the gateway Command Deck use the same typed option order,
server labels, mapping, handoff disclosure, setup instructions, Recheck,
Cancel, typed plan sign-off, and stale refresh behavior.

While a decision is open, **Cancel** submits its opaque gate option. During
`planning`, `rechecking`, `awaiting_plan_approval`, or `running` with no
cancel-option surface, and during a recoverable blocked state, **Cancel run** uses
`POST /api/runs/{run_id}/cancel`. Clients never synthesize a gate decision to
cancel running work.

FortKit adds optional/defaulted models so:

- new clients decode old servers;
- old clients ignore additive fields; and
- old clients cannot act on typed gates because the server rejects binary
  decisions.

watchOS and CarPlay show typed gates as read-only **Open Fort on iPhone or Mac**
items. They never send one-tap legacy approval for a typed choice.

Every surface renders an unknown/malformed decision kind, state, role, or
instruction step read-only with **Update Fort**. It never falls back to legacy
approve/reject.

The encrypted relay protocol needs no route-specific extension; it already
transports arbitrary sealed inner HTTP requests and SSE. Tests must prove that
capability, machine, plan, and option sentinels never appear in outer broker
frames or logs.

Safe SSE events are:

- `capability_inventory_changed`
- `capability_decision_changed`
- `capability_run_state_changed`
- `capability_plan_ready`
- `placement_choice_required`
- `placement_choice_accepted`
- `capability_signoff_required`
- `capability_signoff_resolved`
- `capability_drift`
- `capability_rechecking`
- `setup_instructions_ready`
- `setup_failed`
- `capability_run_canceling`
- `capability_run_canceled`

Every transaction that creates, refresh-replaces, or resolves a typed decision
appends one `capability_decision_changed` event **per affected decision**. Its
payload is:

```json
{
  "event_sequence": 42,
  "run_id": "run-...",
  "decision_id": "capdec_...",
  "predecessor_decision_id": null,
  "kind": "capability_preflight",
  "decision_state": "choice_required",
  "decision_version": 1,
  "change": "created",
  "run_state": "needs_user",
  "plan_id": null,
  "plan_revision": null,
  "relevant_revision": "sha256:...",
  "choice_revision": null
}
```

`event_sequence` is the run's monotonically increasing durable event sequence.
`change` is exactly `created`, `replaced`, or `resolved`. On resolution, the
payload describes the just-resolved immutable decision/version while
`run_state` describes the committed successor lifecycle. Pre-plan plan fields
are null; placement/sign-off plan and choice fields are non-null. This generic
event is required even when a more specific event is also appended, covering
preflight choices, planner failure, setup, drift, blocked decisions, sign-off,
stale version replacement, and system cancellation.

When one transaction resolves a predecessor and creates a successor, it appends
two events in ascending sequence: resolved predecessor first, created successor
second. A refresh replacement appends one event. Every transaction that changes
run lifecycle also appends `capability_run_state_changed` after its decision
events, with exact payload
`{event_sequence,run_id,previous_state,state,plan_id,safe_error_code}`.
`plan_id` and `safe_error_code` are nullable. This covers transitions with no
decision, including `canceling → blocked/cancellation_unconfirmed`,
handoff/dispatch-state blocking, and terminal completion.

Inventory events emit only when normalized revision changes. Clients register
named SSE listeners; polling remains a fallback. Event data is a bounded JSON
object containing only the exact safe fields above or the named event's
run/plan/decision IDs, state, and revisions. SSE never contains prompts,
outputs, instructions, probe data, machine URLs, or
capability-root/project/account identifiers. Specific-event `plan_id` is
omitted—not empty—on pre-plan events; the generic fixed schema uses null.

## Architecture seams

- `core/capability` — closed types, validation, canonical hashing, candidate
  calculation, pure solver, choice projection; no `ui` or concrete `exec`
  imports.
- `exec/capability` — shared command resolver, bounded adapters, cache/registry,
  safe local inventory, Gmail/Supabase brokers, target guard, and authenticated
  node handlers.
- `exec/relay` / `exec/relay/secure` — expose the responder-recovered Noise IK
  initiator static only as trusted inner connection context; client-authority
  enrollment and control authorization never trust a broker/header copy.
- `exec/codexapp` — version-gated app-server `runtime.Runtime` implementation
  for isolated dynamic-tool Codex turns; it exposes no app-server types to core.
- `control` — refresh coordinator, planner lifecycle, typed decision
  controller, durable work dispatcher, compiler/reconstruction adapters, and UI
  ports.
- `core/store` — transactional plan, choice, mapping, setup, outbox, dispatch,
  and handoff-ledger records.
- `core/playbook` / `core/graph` — persisted profile, requirements, machine,
  named handoff, and proof fields.
- `core/machines` — legacy static topology and no-requirement placement remain
  intact.
- `cmd/fort` — composition only.
- `ui` — contracts and presentation only; no engine, graph, router, or native
  imports.

## TDD implementation order

1. Pure `core/capability` tests for closed catalogs, normalization,
   exact platform/version/schema compatibility, profile/logical/composite
   binding revisions and token-rotation exclusions, timestamp-free hashing,
   independently regenerated normal/experimental Codex bundles and canonical
   manifest digests,
   plan bounds, candidate proof construction, pins, single/split/
   setup-to-split ranking, closed predicate/effect fixed point, shared-effect
   target coverage, deterministic blocked reasons, closed effect-dependency
   ordering, exact remedy-operation-bitset DP, content-derived
   instruction/bundle IDs, multi-instruction bundles, 1 MiB reserved
   feasibility, stable semantic IDs, 300,000,000-operation/64 MiB ceilings,
   maximum-density benchmark, and 1,000-repeat determinism with zero
   runtime/model calls.
2. Store migration/transaction tests for old DB compatibility, frozen
   reconstruction, unresolved-ID version replacement plus fresh successor
   decision IDs separate from graph gates, capability sign-off/detail proof,
   decision and run-cancel original status/body replay, relevant generation CAS
   for preflight/auto-placement, proof-refresh atomicity, and
   outbox/mapping/event/state rollback.
3. Shared command resolver tests proving probes and dispatch launch the same
   held identity or immutable staged bytes, including symlink/path replacement
   races, environment binding, unsupported-platform failure, and no
   path/fingerprint/output escape.
4. Adapter/cache tests for absent, incompatible, auth-required, timeout/output
   bounds, reason precedence, exact 60-second scheduler/TTL semantics,
   planning refresh honoring backoff, user Recheck bypass, remote refresh
   request validation, and uncached target guards.
5. Native profile tests for exact Claude auth, configured-pair Hermes,
   OpenClaw main, Codex account plus paginated model catalog, and unavailable
   model preflight.
6. Gmail broker tests for selected account/host/TLS, envelope plus
   `--preview` body-read readiness, no Seen mutation, strict stage-session ID
   provenance, dynamic-call replay/cancellation, and negative send/delete/
   arbitrary-account/shell/raw-tool access.
7. Supabase broker tests for root selection,
   `startupStatus/updated → status/list`, exact raw schemas, hidden project
   insertion, list tables plus both allowed log services, bounded results,
   isolated dynamic tools, and negative SQL/mutation/raw-root access. A CLI
   test proves `projects list` never satisfies inspection.
8. Inventory/client/coordinator tests for synthetic local state, coordinator
   peer identity/rank/local ownership, bindings collection, catalog/profile
   versions, complete predicate vectors, ready-to-ready
   account/project/policy revision changes, node-ID mismatch, concurrent peer
   refresh, partial failure, old-node `404`, bounds, and receipt-time freshness.
9. Planner lifecycle/remote proof tests for pure `/api/route`, preflight before
   model dispatch, strict planner/stage output and attempt-policy wire variants,
   host fallback/pin/target guard, generation races, retry/resume no relocation,
   malformed/timeout recovery, one durable bounded plan, exact plan-revision
   domain and generated/static source union, ingress direction bound, and
   rejection of machine fields.
10. Decision API tests for every required/forbidden state field, stable
    unresolved decision ID and fresh post-acceptance IDs, exact
    kind/state/reason/role matrices, deficit-reference resolution,
    single/split/setup options, instructions/Recheck/gate Cancel/running Cancel/
    sign-off transitions, sign-off semantic IDs and authenticated detail,
    stale decision/version, stale relevant replacement, legacy edit precedence,
    concurrent selections, original `202` replay, different-key
    already-decided/already-canceling, terminal precedence, blocked
    cancellation generations/operator-required, control auth, Origin, and
    CSRF.
11. Durable continuation tests with competing request/restart/worker callers,
    30-second leases/10-second fenced renewal, transport detach versus explicit
    cancel, expired-lease reconciliation, local and remote dispatch-ID
    deduplication, authenticated exact `not_started` versus generic old-node
    `404`, generation-1 unconfirmed then generation-2 confirmed against the
    same dispatch/attempt/process, a target unreachable for generation 1 then
    accepting generation 2 through the authenticated gap marker, immutable
    generation-1 replay, exactly one terminal frame, closed attempt
    states/deadlines/retry caps,
    stranded-Recheck recovery, typed sign-off approve/reject races, and zero
    duplicate provider starts.
12. Compiler/executor/handoff tests for placement persistence before work,
    only named portable inputs, reserved and physical 1 MiB accounting,
    input-contract hashing, canonical payload bytes,
    receipt storage/status/fixed ambiguous-ack recovery with no blind resend,
    digest-only execution inputs, sequenced persist-before-flush dispatch
    journal/status/reconnect deduplication, oversized output failure, no re-placement on
    reconnect/resume, setup-authorized proof refresh, unapproved binding change
    requiring choice, and zero provider calls on drift.
13. Static/breakdown tests for persisted normalized static identity, retry-zero
    enforcement, strict source union, immutable Fort-owned breakdown policy,
    closed human agent/profile allowlist and machine-pin mapping,
    title/body-only model output, non-propagation of request pins to items,
    model machine rejection, atomic item ingestion/restart, valid static
    chains, zero-task pure flows, and fail-closed
    graph-gate/check/transform/fanout/fanin, shell/file, and branched shapes.
14. Versioned node/cluster tests for planner/stage union proof round-trip,
    selected runtime dispatch, stable-dispatch and handoff replay/conflict,
    node-only binding/version drift plus coordinator translation, cancellation,
    old-node failure, and remote refresh modes.
15. Crash tests at run creation, outbox insert/claim, planner attempt/output,
    plan/solve CAS, placement/sign-off acceptance, ledger reservation,
    transfer/store/ack/status recovery, assertion-jti consumption, and every
    Recheck boundary.
16. Full ingress and Quick Answer tests for web/chat/OpenClaw/CLI task/flow/
    breakdown, backlog/inbox/every scheduler callback, unattended choices,
    inline success, pending choice, wait timeout, failure/cancel, restart
    completion, and old-client decoding.
17. Web, gateway web, FortKit, iOS, macOS, watchOS, and CarPlay parity,
    malformed-decision fail-closed rendering, authenticated plan-detail
    sign-off, loopback browser/gateway browser/gateway native/local CLI auth
    matrix, one-time client-authority enrollment/revocation, persistent Noise
    initiator binding, compact-JWS outer assertion mint, exact
    `iss`/header-and-payload-`kid`/`sub` equality, wrong/unenrolled initiator
    rejection, broker-key impersonation rejection for native, exact CSRF wire,
    key rotation/replay, owner-only local token, control-auth storage, and
    sealed sentinel round trips.
18. Live two-machine acceptance: Mini Gmail broker extraction → explicit
    disclosed handoff → laptop Supabase broker diagnosis, setup-to-single,
    setup-to-split, transient drift, binding-change choice, cancellation, and
    duplicate-dispatch probes.
19. `go test ./...`, focused `go test -race`, `go vet`, maximum solver
    benchmark, FortKit contract tests, Apple builds, gateway tests, and
    `git diff --check`.

## Acceptance criteria

- The Mini's explicitly selected Gmail/Himalaya offer is visible only after
  envelope and non-mutating body-preview probes, and a substantive Codex turn
  can use only the Fort Gmail dynamic tools.
- The laptop's functional Supabase adapter is visible only after the exact
  project-scoped read-only broker startup, table, and allowed-log probes; a
  substantive turn sees only the Fort Supabase dynamic tools and never the
  project ID or raw connector.
- An unavailable configured model blocks before planner/provider dispatch and
  offers only explicit change/upgrade actions.
- An unlisted platform, executable version, app-server schema fingerprint, or
  Supabase raw-tool schema fails closed and cannot advertise a ready binding.
- Codex compatibility is reproduced from the exact normal/experimental schema
  generation commands and canonical full-bundle manifests; a CLI version label
  alone never satisfies it.
- A plan satisfiable on one machine is automatically stamped entirely onto that
  machine.
- A plan requiring Mini Gmail plus laptop Supabase yields the exact disclosed
  two-machine mapping, collocation options, and any minimum-remedy
  setup-to-split option.
- Setup never starts the original plan before successful Recheck.
- Setup choices use only the closed remedy catalog, never double-count a leaf
  failure and its derived composite, and every instruction/deficit/mapping
  reference resolves exactly once.
- Setup never masks a downstream auth/model predicate behind an install
  reason, and one shared Codex effect on one machine produces one instruction
  that covers every affected profile/capability deficit.
- No plan, choice, stage, private binding, or provider attempt moves or
  duplicates on reconnect, concurrent selection, or restart; substantive
  stages have no implicit provider retry.
- Cancel reaches terminal `canceled` only after every stable dispatch is
  confirmed stopped; gate Cancel and running-run Cancel share one durable
  operation, blocked unknown-work states remain cancelable, generation 1 may
  remain immutably unconfirmed while generation 2 confirms the same process,
  a target that missed generation 1 can accept generation 2 only through the
  authenticated coordinator gap marker, and unconfirmed cancellation blocks
  replacement work under the three-generation cap.
- Drift Recheck resumes only the accepted mapping and binding proofs; any
  remap or unapproved ready-to-ready binding change is disclosed and selected.
- A setup-authorized binding change updates frozen proof and dispatch ownership
  in one transaction.
- Old nodes and old clients fail closed.
- Unrelated refresh timestamps do not stale a choice; solver-relevant changes
  do, including account/project/root/tool-policy revisions.
- Initial preflight/auto-placement cannot commit across a newer relevant
  inventory generation.
- Static shell/file flows and model-generated breakdown machine fields fail
  closed; breakdown item ingestion is atomic.
- Cross-machine reservation and physical transfer remain within 1 MiB with no
  truncation, digest-only stage dispatch, or blind retransmission after an
  ambiguous acknowledgment; fixed reconciliation ends in the typed
  `handoff_state_unknown` blocked state.
- Remote execution persists contiguous idempotent frames before flush, and
  reconnect/status replay cannot restart a provider or duplicate a coordinator
  event.
- Generated-plan sign-off displays the authenticated exact persisted direction
  and prompts whose policies are bound by `plan_revision`.
- Typed mutations require the exact loopback/gateway/native control-auth mode;
  browser CSRF/Origin is enforced at the hop that authenticates that browser,
  gateway-native FortKit obtains the same compact-JWS assertion without browser
  CSRF, the inner daemon requires the actual enrolled Noise initiator and exact
  JWS issuer/key/subject bindings, and the gateway forwards sealed inner
  headers unchanged. A broker with its signing key but without the native
  initiator private key cannot forge a native control request.
- Every ingress enforces the 65,536-byte original-direction bound before run
  creation; static and breakdown identity use their exact persisted source
  unions/policies.
- No secret, project identifier, mailbox content, executable path, raw probe
  output, or gateway plaintext leaks from its allowed boundary.
- Every covered ingress uses the same coordinator semantics.

## Decisions approved by accepting this spec

1. Capability planning starts after pure route preview and is visible,
   persisted run work.
2. Every substantive ingress uses capability planning by default.
3. Generated plans are bounded sequential plans using closed execution
   profiles and capabilities.
4. Display model labels are never provider model IDs without an explicit
   mapping.
5. Snapshot hashes exclude timestamps; choice revisions bind the exact plan and
   solver-relevant alternatives.
6. Split ranking minimizes machine count and cross-machine handoffs before
   local/registry preference.
7. Placement and optional generated-plan sign-off are authenticated typed
   transactional decisions, never ordinary graph-gate approval.
8. Gmail and Supabase substantive turns receive only Fort-owned dynamic tools;
   private accounts/projects and broader CLI/connector operations remain in
   bounded brokers.
9. Supabase readiness uses exact no-turn startup/status/table/allowed-log calls;
   CLI project listing does not satisfy database inspection.
10. All v1 setup, including Supabase installation and remote OAuth, is
   instructions plus **Recheck**, never automated shell execution.
11. Capability-aware remote refresh and execution use new versioned
    fail-closed endpoints with private binding proofs.
12. Cross-machine payloads are only the explicitly approved bounded text/JSON
    outputs.
13. watchOS and CarPlay cannot approve typed choices.
14. Quick Answer uses a bounded synchronous join over a durable asynchronous
    run.
15. Generated plans are chains; trusted-static flows additionally reject
    implicit filesystem/command/payload semantics.
16. Setup solving may propose a minimum-remedy single-host or split result,
    conservatively closes blocked readiness predicates, and carries one complete
    deduplicated ordered instruction bundle.
17. Every follow-on action is a fenced durable work item with stable
    dispatch-ID deduplication; request handlers never dispatch directly.
18. Ready-to-ready private binding changes alter deterministic proofs and
    cannot be adopted silently.
19. V1 compatibility and setup remedies are closed catalog data; unlisted
    versions/reasons fail closed and setup never executes commands.
20. Each accepted human action resolves one decision forever; any next action
    receives a fresh linked decision ID, while stale refresh alone versions an
    unresolved ID.
21. Stage dispatch carries digest-only receipt references, immutable
    output/attempt policy, and no substantive provider retry; ambiguous
    cross-machine sends are reconciled by durable receipt status.
22. Loopback browser, gateway browser, gateway-native FortKit, and local CLI
    authentication are distinct contracts; each gateway client is enrolled
    once to its actual persistent Noise initiator and then obtains and seals a
    compact-JWS request-bound assertion before the opaque relay.
23. Codex compatibility uses reproducible canonical full-bundle schema
    manifests, and every portable input uses an immutable plan-bound contract
    plus durable receipt/journal reconciliation.
24. Unconfirmed cancellation is an immutable per-generation reconciliation
    result, not a terminal provider state; a later bounded generation may
    confirm the same execution but can never restart it.

## Rollout and rollback

Roll out control authentication, broker isolation, inventory/remote-Recheck,
guarded execution/cancellation, and stable-dispatch reconciliation to every
node before enabling planning. Existing gateway browser/native clients must
receive the persistent-initiator build and complete one client-authority
enrollment before typed control mutations are enabled; there is no
gateway-signed-only compatibility bypass. Old nodes remain visible but
`unknown`.

Generated plans and choices stay in additive run-scoped tables; reusable
playbook revisions and `machines.yaml` are unchanged. New API fields are
optional/defaulted.

Disable `FORT_CAPABILITY_PLANNING` to restore legacy planning and static
placement. Existing capability records and audit events remain safely ignored.
Existing typed decisions remain non-graph records and therefore cannot become
legacy approvable gates after binary downgrade. A reverted binary leaves those
runs inert until a capability-aware binary resumes or cancels them; downgrade
does not delete records or silently dispatch work.
