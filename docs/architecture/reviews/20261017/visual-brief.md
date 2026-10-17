# Visual handoff — proposed Fort foundation

Use REVIEW.md as the substantive source. Source pin: e02eb571e66620e407a360584817616074730b55. Pinned source copies and hashes are included. The executive visual should make four decisions legible without implying implementation or Apple approval.

## Executive canvas

Title: **Own the workflow. Keep the boundaries small.**
Subtitle: Proposed architecture • source reviewed 17 Oct 2026 • TestFlight feasibility unverified.

Four decision cards:

1. **One Fort authority** — identity, history, runs, approvals and routines; one durable ledger.
2. **Removable capabilities** — memory, voice, service connectors and VM placement; text messaging survives their removal.
3. **Isolated execution** — official runtimes behind provider-neutral contracts; credentials stay out of prompts and the ledger.
4. **Prove the delivery path** — TestFlight client/helper feasibility before backend commitment; Swift platform layer, Go candidate, TypeScript reference.

Footer decision: **Approve the boundaries and feasibility test before a rewrite.** This is a proposed decision for the owner, not a request to authorize implementation in this review.

## Components and arrows

| ID | Component label | Boundary | Incoming | Outgoing |
| --- | --- | --- | --- | --- |
| C | Native clients | Owner devices | State, receipts, approval requests | Versioned authenticated commands and replay cursor |
| A | Fort application | Fort authority | Client commands, executor evidence | Domain decisions, scoped dispatch, event/state updates |
| L | Durable ledger | Fort authority | Atomic state/events/outbox | Ordered state, compact receipts, recovery facts |
| M | Optional memory | Trusted context module | Retrieval request and authorized source scope | Bounded revisioned excerpts and provenance |
| V | Optional voice | Client/input edge | Audio or approved text | Editable draft or playback |
| E | Execution host | Proven containment | Service-owned attempt/generation, scope, deadline | Bounded correlated evidence and action requests |
| R | Official runtime adapter | Executor | Dispatch | Official vendor protocol; no ledger writes |
| K | Service connector | Isolated capability boundary | Exact scoped action authorization | External operation, normalized untrusted result |
| S | Credential adapters | Private stores on selected host | Opaque account/grant references | Scoped use; never prompt or event content |
| P | AI/service vendors | External trust boundary | Selected data/authorized operations | Model/tool content treated according to provenance |

Arrows are data/command flows, not source-code import arrows. Core owns interfaces; platform/provider/connector adapters depend on those interfaces. Composition chooses implementations. Optional boxes should use dashed borders. Trust boundaries require labels, not merely colors.

## Local/cloud comparison

Option A: All-TestFlight local bundle → sandboxed per-user host/helper → local ledger + embedded official runtime. Mark account flow, containment and update/removal **unverified**.

Option B: TestFlight client → secure reachable transport → owner-controlled service/ledger/executor. Mark **owner decision required**, not recommended escape hatch. Cloud does not mean vendor-managed Fort, and local/self-hosted does not mean offline inference.

Do not place a VM in the mandatory path. VM is one optional execution placement. Do not draw two authoritative ledgers or imply automatic local/cloud synchronization.

## Evidence badges

Verified source: provider literals in public contracts; separate attempt IDs; repeated history copies; per-second schedule checkpoint; real install/remove adapter absent.

Recorded checks: 120 service/runtime tests and 56 native tests; engineering builds. These are prior worker records, not independently rerun here.

Unverified: performance budgets; TestFlight install; provider compatibility of the proposed host; removable module builds; new principles committed locally at 1a86499 and separately reviewed; no remote CI enforcement or product compliance is claimed.

## Language card copy

**Swift:** native clients and Apple lifecycle.
**Go:** leading portable-service candidate, pending packaging and workload proof.
**TypeScript:** working behavior/reference; still viable if it meets measured constraints.
**Rust:** targeted systems option when justified by measured needs.

Owner routines remain native forms and named skills, regardless of backend language.

## Source anchors

- Core/provider coupling: source/successor/contracts/v1.ts:2–7.
- Attempt correlation: source/successor/service/openai.ts:387–391; source/successor/runner/chat-runtime.ts:3–10; source/successor/runner/codex-chat.ts:142.
- Repeated snapshots: source/successor/service/openai.ts:250–260; history copies:384–391.
- Idle schedule writes: source/successor/service/openai.ts:320–352,755.
- Lifecycle interface: source/successor/apple/FortKit/Sources/FortKit/OwnerLifecycle.swift:31–39.
- Delivery limits: source/successor/TESTFLIGHT-DELIVERY-20261017.md:36–72.

A polished visual is being produced by the parent task. These Mermaid files and the component/arrow table are the design sources, not evidence that a renderer or live system was tested.
