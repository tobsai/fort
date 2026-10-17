# NNN: Requirement title

## Goal

<Describe the owner-visible outcome and scope.>

## Architecture review
Review status: draft
Review owner: <actual decision owner>
Decision reference: pending
Reviewed content SHA256: pending

### Vendor independence
Status: planned
Affected boundaries: <name affected contracts and dependency direction>
Dependency/removal impact: <what can be omitted and what remains>
Acceptance criteria: <Assess AI/runtime, hosting, storage, optional modules and client/distribution separately. Name exact capability differences and subscription feasibility. Specify an independently shaped non-OpenAI fixture plus adapter-absent build; distinguish real-provider acceptance.>
Evidence: <actual source or artifact, result and limits; otherwise specific missing proof>
Tradeoffs: <alternatives, user cost, mitigation and revisit condition>
Unresolved judgment: <decision to resolve, or none>

### Modularity
Status: planned
Affected boundaries: <name affected contracts and dependency direction>
Dependency/removal impact: <what can be omitted and what remains>
Acceptance criteria: <Name memory, voice, execution environments/VMs and service connectors affected or justify each unaffected case. Define contracts/lifecycle/data ownership. Remove packages/artifacts and test core send/persist/replay and retained history.>
Evidence: <actual source or artifact, result and limits; otherwise specific missing proof>
Tradeoffs: <alternatives, user cost, mitigation and revisit condition>
Unresolved judgment: <decision to resolve, or none>

### Speed
Status: planned
Affected boundaries: <name affected contracts and dependency direction>
Dependency/removal impact: <what can be omitted and what remains>
Acceptance criteria: <Set numeric clean/incremental build, cold/warm startup and runtime budgets; for background hosts include idle CPU/RSS/wakeups and active load. Specify hardware, OS, toolchain, workload, cache state, repetitions and percentiles. Separate Fort overhead from network/provider latency and compare language/framework candidates.>
Evidence: <actual source or artifact, result and limits; otherwise specific missing proof>
Tradeoffs: <alternatives, user cost, mitigation and revisit condition>
Unresolved judgment: <decision to resolve, or none>

### Installation and removal
Status: planned
Affected boundaries: <name affected contracts and dependency direction>
Dependency/removal impact: <what can be omitted and what remains>
Acceptance criteria: <Inventory dependencies, permissions, startup ownership and data. Define local/self-hosted and TestFlight paths; no-expiry default/optional expiry, revoke versus forget, keep/delete history, interruption/retry, upgrade/rollback and actual uninstall absence proof. Distinguish CI verification, signing, upload, processing, tester access and installed acceptance.>
Evidence: <actual source or artifact, result and limits; otherwise specific missing proof>
Tradeoffs: <alternatives, user cost, mitigation and revisit condition>
Unresolved judgment: <decision to resolve, or none>

## Approach

<Describe the bounded design and alternatives; identify conflicts with earlier specs.>

## Affected files

<Exact paths for the next slice; preserve unrelated work.>

## Implementation paths

- `<exact/repository-relative/file-for-this-slice>`

## Test criteria

<Public behavior tests, removal/dependency checks, benchmark commands and installed proof.>

## Rollback

<Reversible recovery plan and data compatibility; no blanket deletion of existing work.>
