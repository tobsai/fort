# Fort architecture constitution

Fort's design principles, set by Toby on October 17, 2026: vendor independent,
modular, fast, and easy to install and remove. The build is a Node/TypeScript core
with Swift Mac and iPhone clients. [Spec 059](specs/059-modular-node-v1.md) has the
long-form history.

## 1. Vendor independent

Core domain rules, public contracts and native UI must not require a particular
AI/runtime vendor. Concrete SDKs, wire protocols, account/session mechanics and
billing behavior belong behind optional capability-based adapters. Select the
exact requested provider/model/host; unsupported capabilities must be visible.
Never substitute an identity, model, provider or paid API path silently. Preserve
the owner's preference for existing subscriptions while verifying each official
runtime's actual capabilities and terms separately.

Review independence by dimension: **AI/runtime, hosting, storage, optional
modules, and client platform/distribution**. State the dependency, replaceable
boundary, migration/removal cost and user impact for each. A storage engine or
OS API is still a dependency even when self-hosted or open source. Do not build
several backends merely to claim portability; prove the boundary needed now.

The owner requires native iPhone/Mac clients delivered through **TestFlight**.
That is an explicit Apple platform/distribution dependency, not AI-vendor
independence. Keep service contracts language/platform-neutral so an alternate
client does not require rewriting domain rules. Apple delivery feasibility and
background-service constraints still need evidence.

**Proof:** run the same public provider behavior suite against an independently
shaped non-OpenAI fixture with different session semantics and unsupported
capabilities, not a renamed Codex fake. Remove the OpenAI adapter/dependencies
and build/test core plus clients. Fixtures prove abstraction only; real official
subscription integration needs separate authorized provider acceptance.

## 2. Modular

Memory, voice, virtual machines/execution environments, and service connectors
must be optional modules with stable Fort-owned contracts. Define dependency
direction, versioning, configuration, lifecycle, data ownership and failure
behavior. Core messaging/persistence cannot import optional implementations;
composition chooses adapters. A connector's credentials and external authority
stay scoped to that connector. Agent-host access and computer-tool sandbox
access are separate boundaries and must be described and tested separately.

**Proof:** remove each optional module from dependencies and produced artifacts,
then build and run the core send/persist/replay journey. Test missing/disabled,
failed, removed and re-added modules; retained history must stay readable. A
feature flag or an interface declaration alone does not prove removability.
No speculative plugin marketplace, distributed services or generic framework is
required. Choose the smallest boundaries that make removal practical.

## 3. Fast

Choose languages and components using measured development and runtime costs.
Set numeric target budgets and a reproducible comparison
for clean build, incremental build, cold/warm startup and Fort request/event
processing. A background host also needs idle CPU, memory, wakeup/energy and
active-load budgets. Measure Fort/framework overhead separately from network
and provider latency; a slow provider is not evidence that Fort is efficient.

Record hardware/OS, toolchain versions, workload/data size, process topology,
cache state, repeated samples and reported percentiles. Distinguish proposed
budgets from measured results. Compare credible candidates at the same public
seam before an undecided language/framework choice. The owner's explicit Node
selection supersedes a language-selection comparison; measured Node budgets still
apply. No build duration in an incidental log is a benchmark. Unassigned budgets are an open design decision, not a pass.

## 4. Easy to install and remove

List dependencies, downloads, permissions, background processes, data locations
and external accounts before installation. Keep owner setup understandable and
provide a local/self-hosted path without a mandatory vendor control plane.
Design a performant, removable Mac background service with explicit ownership,
start/stop/update behavior and visible status; investigate Apple/TestFlight
constraints before promising how it ships. Do not activate it during design.

Preserve the owner's decisions: no pairing expiry by default, explicit optional
expiry, easy disconnect, and history kept unless deletion is separately chosen.
Distinguish confirmed revocation from local forgetting. Handle lost replies,
expired credentials, interrupted shutdown and uninstall retries without claiming
remote effects that were not verified. Removal must cover owned processes,
startup registrations, credentials and app files; list intentionally retained
history and external grants that still need revocation.

**Proof:** fresh install, update/rollback, disconnect/revoke, interrupted recovery
and uninstall/absence checks against exact installed builds. iPhone and Mac
acceptance must use the actual TestFlight delivery and include accessibility,
screenshots and authorized real-provider inspection. Source tests, ad-hoc apps,
archives, upload receipts, Apple processing and tester availability are separate
evidence stages. CI/CD must reproduce tests/packages without requiring private
accounts for ordinary verification; release credentials belong to an explicitly
approved delivery stage.

## Applying the principles

Use these principles as judgment in normal PRs, not as a form to fill in.
Write a short spec only for billing/provider path, credentials, persistent access
or destructive changes ([specs/README.md](specs/README.md)). Toby's per-action
approval still applies to new credentials, persistent grants, legal terms and
destructive actions. Decision [061](docs/architecture/decisions/061-lean-dev-cycle.md)
retired the structural spec and review gates on October 23, 2026.
