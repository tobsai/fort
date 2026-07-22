# Spec 036 — Deterministic Playbooks and Route Preview

**Status:** approved-by-instruction (Toby: "Using the design document update the UI and application according to the designs and spec")
**Design source:** `design_handoff_fort_dashboard_redesign/SPEC-playbooks.md`, Turn 4 in `Fort Redesign.dc.html`, and Turn 2 in `Fort Mobile and Mac.dc.html`.

## Goal

Add reusable, versioned playbooks that choose an agent and model for each stage,
show the resolved route before a handoff, carry approved stage context forward,
and keep sign-offs in the existing gate inbox. Route selection remains a pure,
deterministic operation with zero model calls.

## Deterministic interpretation of the handoff

The design brief describes model-based trigger classification, but Fort's
governing invariant forbids model calls in the routing path. Fort therefore
classifies only from explicit, ordered signals: an optional `task_type`, an
explicit playbook override, and fixed text rules for question, bug, research,
then the default feature route. `POST /api/route` is pure: it persists nothing,
dispatches nothing, and returns no invented confidence score.

## Approach

1. Add pure playbook definitions, validation, deterministic classification,
   task-type branch resolution, and four seeded defaults (Feature work, Bug
   fix, Quick answer, Research).
2. Persist immutable playbook revisions in SQLite. Editing appends a revision;
   a route preview names the exact revision and handoff executes that revision,
   so edits never alter in-flight work.
3. Compile a resolved revision to a restricted DAG of task stages plus the
   optional plan gate. Only task nodes invoke `runtime.Runtime`. Model is an
   additive `RunSpec`/graph-node field and reaches supported native provider
   argv (`claude --model`, `codex --model`, `hermes --model`).
4. Playbook task prompts receive the original direction, current approved
   payload, and outputs explicitly marked for shared memory. Existing static
   flows keep their current prompt behavior.
5. Add bounded control-plane ports and HTTP contracts:
   `GET/PUT /api/playbooks`, `POST /api/playbooks/{id}/duplicate`,
   `POST /api/route`, and additive playbook fields on `POST /api/chat`.
6. Quick answer runs one task stage and returns `kind: answer` plus its text.
   Its event history is retained, while its playbook flow is excluded from the
   assignment board, schedules, summary counts, and performance scorecards.
7. Add the Playbooks editor and route-preview handoff to the web app, the
   route-preview/switcher to iPhone, and the Playbooks pane to macOS using the
   canonical design tokens and form-factor rules.

## Honest scope

- Method-version promotion/A-B actions remain deferred: no method artifact or
  promotion semantics exist yet. The UI does not fabricate method tags.
- Conditional stage skipping remains deferred until Fort has an approved,
  deterministic predicate grammar. The handoff's prose example (such as
  "no UI work in plan") is not evaluated by a model or guessed by the router.
- Runtime invocations carry the selected model, but the existing performance
  scorecard remains grouped by agent. Agent+model+method historical grouping
  requires a separate metrics-contract revision rather than rewriting old
  event rows with inferred attribution.
- Recurring calendar blocks remain deferred: the scheduler is not listable by
  the server. Up-next work is labeled as queued/unscheduled, never presented as
  a real appointment.
- Projects remain derived from runs and briefs as in spec 033; durable project
  grouping needs a separate approved spec.
- OpenClaw's CLI model flag remains unverified. Fort records the requested model
  but only adds provider argv for installed, locally verified CLIs.

## Affected files

- New `core/playbook/` definition, resolution, defaults, and tests.
- `core/store/` immutable revision persistence and tests.
- `core/runtime/runtime.go`, `core/graph/`, `exec/native/` for model/context
  propagation and tests.
- `control/` playbook catalog/compiler adapter.
- `ui/contract.go`, `ui/ports.go`, `ui/server.go`, `ui/page.go` and tests.
- `cmd/fort/` composition for full and control-only modes.
- FortKit and native iPhone/macOS screens.

## Test criteria

- Repeated previews are identical and dispatch zero runtime calls.
- Validation rejects missing stages, missing default branches, duplicate stage
  orders, and unsupported trigger/delivery values.
- Editing creates a new immutable revision; an earlier preview continues to
  resolve and execute its original revision.
- Shared-memory context and an edited plan gate reach later stages; ordinary
  graph flows retain their existing prompt contract.
- Requested model reaches local and remote `RunSpec`; verified native providers
  add the correct model flag.
- Quick answer returns text but creates no visible assignment, schedule block,
  summary count, or metric sample.
- Web Playbooks navigation, route preview, route switching, shortcut toggles,
  and handoff controls work at desktop and narrow widths.
- FortKit contract checks cover playbook catalog, route preview, and override
  encoding; iPhone and macOS expose the handoff designs.
- `go test ./...`, focused race tests, Swift contract checks, and browser
  design QA pass. Native visual QA is reported blocked if Xcode/Simulator is
  unavailable.

## Rollback

Revert spec-036 implementation. The SQLite additions and JSON fields are
additive; older binaries and clients ignore them.
