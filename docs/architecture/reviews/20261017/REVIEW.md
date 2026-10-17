# Fort foundation: independent architecture assessment

**Decision proposal — 17 October 2026. No redesign implementation is authorized by this report.**

Reviewed source: `e02eb571e66620e407a360584817616074730b55`, branch `codex/fort-openai-rebuild-20261016`, worktree `/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`. The current runtime is the new `successor/` implementation. The original OpenMaus extraction is historical context, not the current runtime graph. This assessment supersedes earlier findings only where the newer source changed the behavior.

The source links below point to preserved copies of these exact Git blobs. [PROVENANCE.json](PROVENANCE.json) records paths, blob IDs and SHA-256 hashes. Line references are to that revision. The original checkout remains untouched. The requested exact Astra model identity is not independently exposed to this session; this report does not claim to attest it.

## Decision for the owner

**Use one modular Fort service, one authoritative ledger and a separately isolated executor. Keep Swift for the Apple experience. Treat Go as the leading candidate for a portable service, subject to packaging and workload evidence. Preserve the working TypeScript behavior until that decision is proven.**

The most consequential decision is the execution and distribution boundary. An all-TestFlight local product must prove that its embedded official agent runtime, account lifecycle and workspace access work inside the chosen Apple sandbox. A Swift background helper does not settle that question. A separately controlled executor is an alternative product topology that needs an explicit owner decision; it must not quietly replace the requested local experience.

The present implementation is useful: it has stable identities, durable request handling, explicit unknown outcomes, native clients, reviewed skill revisions, routines and calendar decisions. Its organizing module and public types still describe the Codex prototype. Extending them directly with connectors, voice and VM management would deepen coupling. Separate those responsibilities before growing the feature set.

| Decide now | Prove before commitment | Retain while proving |
| --- | --- | --- |
| Domain ownership, versioned contracts, removable capability boundaries, one ledger writer, isolated execution | All-TestFlight runtime feasibility; local/iPhone connectivity; comparable build/runtime measurements; migration safety | SwiftUI client, durable intent and identity semantics, protocol fixtures, routine/calendar behavior, bounded process supervision |

## Evidence and limits

This review executed read-only Git inspection, static dependency/test-inventory checks and primary-source research. It did **not** run new product builds or behavior tests, benchmarks, services, enrollment, provider login, uninstall, signing or deployment. After the policy files appeared, it separately ran the structural gate suite and its positive/negative controls; see [policy review](POLICY-REVIEW.md). Two independent code-review passes examined the pinned source. Recorded checks from another worker remain recorded evidence, not a new pass here.

The current handoff records 120 service/runtime tests, 56 native tests and successful QA/Mac/Simulator builds plus Release FortKit compilation. Installed UI evidence, real owned-host recovery/removal and TestFlight delivery remain open. See [QA evidence](source/successor/evidence/2026-10-17-interactive-qa/README.md) and [delivery ledger](source/successor/TESTFLIGHT-DELIVERY-20261017.md). A single recorded 17.87-second Release compilation is not a controlled build benchmark.

The dedicated principles, structural gate and Spec 058 gap assessment were separately reviewed at local commit `1a864996e4e4f72bca99d22d745f891c3ff8ffe9`. All reviewed file bytes were verified against that commit. See [POLICY-REVIEW.md](POLICY-REVIEW.md). Their structure and explicit limitations are sound; the implementation remains noncompliant with several new requirements. The gate suite passed 10/10, its full scan passed, and Spec 058 correctly failed the implementation gate. Earlier AGENTS handoffs remain historical, consumed session authority.

## Current architecture and priority findings

The backend runtime composition is small: native SwiftUI → FortClient → HTTP/JSON → `service/openai.ts` → `ChatRuntime` → Codex protocol/process modules. Non-test TypeScript static imports in this graph are relative or `node:*`; the package declares no production npm dependencies. Imported donor code, managed-cloud SDKs and a VM manager are absent from this runtime composition. This is a useful starting point for ownership, but a small dependency list is not the same as modularity.

| Priority | Verified source fact | Consequence and proposed response |
| --- | --- | --- |
| Gate | The real `OwnedMacInstallation` adapter is absent; delivery document explicitly leaves sandbox/host and physical-iPhone connectivity unresolved. | Prove install/start/stop/update/recovery through the intended TestFlight topology before treating this as a usable product foundation. [OwnerLifecycle.swift:31–39](source/successor/apple/FortKit/Sources/FortKit/OwnerLifecycle.swift), [delivery:48–59](source/successor/TESTFLIGHT-DELIVERY-20261017.md). |
| High | `Installation.mode`, `Binding.provider`, `ModelCatalog.provider` and native branches encode OpenAI/Codex. Agent creation embeds a runtime version and local host. | Provider independence needs a domain/wire contract change, not a second class named Adapter. [v1.ts:2–7](source/successor/contracts/v1.ts), [openai.ts:686–687](source/successor/service/openai.ts), [Contract.swift:11](source/successor/apple/FortKit/Sources/FortKit/Contract.swift). |
| High | Service dispatch allocates an attempt ID, `ChatInput` omits it, and CodexChat creates another. | Before an executor crosses a process/machine boundary, make the service own attempt/generation identity and accept correlated, idempotent evidence. [openai.ts:387–391](source/successor/service/openai.ts), [chat-runtime.ts:3–10](source/successor/runner/chat-runtime.ts), [codex-chat.ts:142](source/successor/runner/codex-chat.ts). |
| High | Every acceptance stores a complete conversation snapshot; each dispatched run also stores rebuilt history. | A growing conversation repeatedly persists historical prefixes. This creates quadratic growth in stored message copies as accepted turns increase, even before model latency. Store compact receipts and immutable context references/digests; avoid repeating complete snapshots. [openai.ts:250–260,384–391](source/successor/service/openai.ts). |
| High | Snapshot reads deserialize all messages/runs of their kind before filtering by conversation. History assembly repeatedly searches messages. | Add indexed, bounded queries and paged transcripts. Fix the data-access shape before replacing SQLite. [openai.ts:182,218–222,389](source/successor/service/openai.ts). |
| High | Enabled schedules write `checkedThrough` on every one-second tick; the timer catches an exception by stopping itself. | Reduce idle commits, persist meaningful scheduling progress and expose a durable scheduler fault/recovery state. A silent stopped timer is not dependable routine delivery. [openai.ts:320–352,755](source/successor/service/openai.ts). |
| High | The owned runtime uses a pinned Homebrew executable and the owner's HOME. | This is a reviewed machine-specific launch, not a portable credential or filesystem isolation boundary. Replace the composition with scoped host capabilities; never copy the whole HOME into a VM. [owned-codex.ts:10–20](source/successor/prototype/owned-codex.ts). |
| Medium | Memory participates in snapshots, command validation, runtime input and native send eligibility. | Optional data is not yet removable implementation. Move context enrichment behind a capability boundary; absence must leave ordinary chat usable. [v1.ts:10–11,29](source/successor/contracts/v1.ts), [openai.ts:180,215,287–289](source/successor/service/openai.ts), [MessengerModel.swift:93–96](source/successor/apple/FortKit/Sources/FortKit/MessengerModel.swift). |
| Medium | Live event rows carry empty payloads. The older fixture service exposes event replay behavior that the live service does not share. | Specify snapshot/delta semantics and use one application implementation with fixture adapters. These rows cannot reconstruct state independently. [openai.ts:204–206](source/successor/service/openai.ts), [service.ts](source/successor/service/service.ts). |
| Medium | The default test script names 21 of 28 tracked TypeScript test files; agent settings and owner pairing are omitted. No recognized root CI or benchmark definition was found. | Define a safe, complete fixture suite and separate explicit live suites. Add test-discovery coverage and CI. This is a coverage-entrypoint finding, not proof that the omitted tests fail. [package.json:10](source/successor/package.json). |

The 771-line `openai.ts` combines authentication, HTTP, persistence, configuration, routine scheduling, dispatch and shutdown. `MessengerModel.swift` has 529 lines and coordinates network refresh, cache, sending and lifecycle. Line count alone is not a defect; the number of unrelated reasons to change these files is the problem. Native refresh polls every two seconds and sequentially fetches multiple collections plus a full snapshot, then rewrites caches. This is a source-derived amplification risk, not measured latency. [MessengerView.swift:75–82](source/successor/apple/FortKit/Sources/FortKit/MessengerView.swift), [MessengerModel.swift:254–291,326–333](source/successor/apple/FortKit/Sources/FortKit/MessengerModel.swift).

Prior findings about failed shutdown retry, FIFO reads and unpersisted disconnect intent have changes in `e30f116` and `eeb88f4`. They are not repeated here as open bugs. The distinction between local forget and verified revocation is worth preserving.

## Domain model and ownership

Use business concepts to define modules before selecting a framework. Ports and adapters keep use cases independent of devices and technology-specific protocols; they are useful here because clients, runtimes and placement genuinely vary. This does not call for an interface around every function. [Cockburn's original ports-and-adapters article](https://alistair.cockburn.us/hexagonal-architecture/). Use a shared vocabulary and explicit model boundaries as design tools; the following ownership is a Fort-specific proposal, not a claim that a general pattern dictates these exact modules. [Evans' DDD reference](https://www.domainlanguage.com/ddd/reference/).

| Domain/module | Owns | Must not own |
| --- | --- | --- |
| Agent configuration | Stable agent ID, Home reference, preferences, immutable behavior/binding revisions | Provider session identity, credentials, UI state |
| Conversation ledger | Messages, request deduplication, ordering, read position, export identity | Retrieval ranking, provider thread persistence |
| Execution coordination | Runs, service-owned attempts/generations, dispatch intent, cancellation and unknown outcomes | Vendor SDK objects, raw tokens, OS process creation |
| Authority | Device grants, resource scopes, approval binding, expiry/revocation | Model reasoning as authorization |
| Routines and scheduler | Versioned routine definitions, explicit invocation, steps/gates, calendar occurrences, recovery policy | User-authored unrestricted code or provider-specific scheduling |
| Context module (optional memory) | Index/retrieval, bounded excerpts, provenance and deletion policy | Canonical conversation history, ability to approve actions |
| Service connector | Typed service operations, response normalization, connector-specific credential access | Fort ledger writes, global agent authority |
| Runtime adapter | Official agent protocol, capability discovery, model/account readiness, runtime evidence | Fort identity or scheduling truth |
| Execution placement | Approved process/VM/remote lifecycle, resource limits, cleanup proof | Reinterpretation of requested model or permissions |
| Storage adapter | Migrations, indexed queries, transactions, backup/export mechanics | Independent competing interpretation of domain rules |
| Native client/platform host | Presentation, durable local intent, Keychain, Apple lifecycle and approved transport | Authoritative execution status inferred from animation or a timeout |

Core modules belong in one process and one transaction boundary initially. The executor is separated because it handles agent processes and untrusted work. Sensitive connector workers may require another process for containment; each feature does not need its own service, queue server or database.

Define the core vocabulary precisely:

- **Agent** is an owner-defined identity and behavior, not a provider thread.
- **Binding** is a revisioned choice of agent runtime/provider, selected model, account reference, execution host and policy. Availability and provider-reported effective model are separate facts.
- **Run** is accepted intent; **attempt** is one authorized dispatch of that intent. Retrying a transport does not create a new user command.
- **Routine** is an explicitly invoked or explicitly enabled structured workflow. **Occurrence** is a durable calendar decision. A scheduler is not a cron string hidden in model instructions.
- **ModelProvider** offers inference. **AgentRuntime** supplies sessions, tools, approvals and vendor agent behavior. Subscription-backed official agents belong behind AgentRuntime; a raw API-key integration must not be presented as equivalent billing or authority.
- **Capability** is a versioned operation contract. **Grant** permits a principal to use a capability within a scope. **Approval** authorizes an exact proposed action. Advertising a capability grants no permission.

## Contracts, execution and durable recovery

Keep a small versioned Fort protocol separate from the Swift or backend language. Specify wire schemas, error codes, absent/null meaning, pagination, sequence/cursor behavior, maximum sizes and capability versions. Share positive and negative fixtures across implementations; do not make a generated type file the only compatibility proof.

A proposed dispatch envelope carries installation ID, recovery epoch, run ID, attempt ID, generation, immutable binding and policy revisions, context/input digest, deadline, output/resource limits and opaque credential/grant references. The service creates it. The executor cannot invent a competing attempt ID or write the database.

Executor evidence is bounded and sequenced: accepted/start, output, action request, resource evidence and terminal result. The service returns idempotent receipts; duplicate evidence cannot duplicate a message or action. Stale generation, wrong installation, wrong digest or expired dispatch is rejected. Report selected model and provider-reported effective model separately. The current `effectiveModel: null` is honest and should remain unknown until attested; echoing selection is not attestation.

Use ordinary relational state plus an append-only event/outbox record in the same transaction. Full event sourcing is unnecessary. A notification event may tell a client what to refresh; a delta event must carry enough versioned data to apply correctly. Do not conflate these. Keep snapshot recovery for expired cursors. One writer with SQLite WAL is sufficient until measured load or multi-host availability requirements justify a different store.

Cancellation is a persisted request targeting the exact attempt. A lost reply after dispatch is `outcomeUnknown` unless reconciled with that attempt. Killing a process proves neither that an external write was canceled nor that it was undone. Idempotency keys help only where the downstream service honors them. At-most-once acceptance does not establish exactly-once external effects.

Replace the global work pump with explicit bounded queues when actual concurrent agents are enabled: preserve per-conversation order, cap total/provider/connector concurrency, bound output and queue size, and expose backpressure. Start conservatively. A durable outbox/claim protocol is needed before remote execution; there is no need for a separate queue vendor initially.

## Removable modules and independent credentials

A feature is removable when its implementation can be excluded from the build/composition, its grants and background work can be disabled safely, and the remaining product still works. A feature flag that hides a button does not prove this.

**Memory.** Keep messages, identity and accepted configuration as mandatory durable records. Optional semantic memory supplies bounded, revisioned excerpts with source references and retention policy. Freeze selected context in accepted work, preferably by immutable references plus content digests. Removing memory stops indexing/retrieval and leaves messaging and routine execution intact. Conversation notes are a valid simple capability; do not mislabel them as a complete semantic memory system.

**Voice.** Put capture, transcription and playback at the client boundary or behind explicit service ports. Transcription produces an editable draft and submits through ordinary command authority. Audio retention is an explicit artifact policy. Removing voice must not change message identity or text controls.

**Service connectors.** A reviewed manifest declares capability versions, operations, argument/result schemas, resource scopes, effect semantics, byte/time limits and credential needs. The authority module approves exact arguments/destination/digest/expiry outside the model. A connector receives only a scoped handle; credentials never become prompt text, ordinary ledger events or client cache. Treat retrieved mail, pages and tool output as untrusted data even when the account is trusted. The tool protocol—MCP or otherwise—does not supply isolation or authorization by itself.

Use separate credential classes: Fort device credentials; provider-owned account state; connector credentials. Revoking one connector must not sign the user out of unrelated providers or delete conversation history. Disabling a module blocks pending work requiring it and revokes its grants. An ambiguous external action remains visible. Module data deletion is a separate owner choice. Use trusted static module registration first; add out-of-process extension protocols where there is an actual integration need, not an unrestricted downloadable plugin platform.

**VM placement.** VM is an execution-host adapter, not a second agent domain model. A binding that requires VM isolation must fail visibly when VM support is absent; it must not silently run on the Mac host. No whole-HOME credential mount. Require explicit image/version provenance, scoped storage/network, resource budgets and cleanup/recovery behavior. A VM changes host containment, not the sensitivity of data sent to a cloud AI provider.

**Skills and routines.** Keep immutable, reviewable skill bundles with provenance, hashes and explicit enablement. Instruction content and executable extensions are different capabilities. Current text-only handling is an intentional boundary. A skill may suggest tools but cannot expand its grant. Preserve named routine invocation, native forms, inputs, steps, approval gates and immutable revisions. Neither backend TypeScript nor proposed Go requires users to author routines in that language. Optional future code authoring belongs behind a separate reviewed execution capability.

## Local, cloud and TestFlight decisions

The service/ledger has one authoritative home at a time: approved Mac or owner-controlled server. A placement choice must show where history, files, credentials and execution reside. Relocating a service is an export/migrate/rebind operation with stable identity and an explicit recovery epoch; it is not two simultaneous writers sharing a database file. A fully self-hosted Fort control plane can still send selected content to an external subscription agent provider.

The current listener is `127.0.0.1`; an iPhone cannot reach the Mac by using its own loopback. HTTPS support in FortClient is useful, but reachable authenticated transport and device pairing remain a separate acceptance task. No public ingress, tunnel or account change is implied by this report.

Apple's current guidelines apply to TestFlight beta apps and place sandbox, bundle, background-execution consent and code-distribution requirements on the shipping design. Read sections 2.2, 2.4.5 and 2.5.2 against the actual package. This review predicts neither rejection nor acceptance. [Apple App Review Guidelines](https://developer.apple.com/app-store/review/guidelines/).

| Delivery option | Proposed structure | Unresolved gate |
| --- | --- | --- |
| All-TestFlight local | SwiftUI app, approved per-user sandboxed helper/host, bundled signed pinned runtime, scoped workspace; XPC for native local control | Runtime embedding/licensing, sandbox behavior, login/refresh/logout, work access, subprocess denials, helper/update lifecycle and actual Apple acceptance |
| TestFlight client + separately controlled executor | Same Fort contracts; service/ledger/executor on an owner-controlled host | Owner agreement that this satisfies local/self-hosted goals; lawful supported provisioning, secure connectivity and operational ownership. This is an alternative, not an automatic fallback. |

A Swift `SMAppService` helper is the native lifecycle candidate. Keep platform lifecycle out of the portable domain. Embedding and inheritance must follow Apple's helper guidance; registration alone is not sandbox enforcement. Test the exact signed bundle and entitlement relationship. [Apple helper documentation](https://developer.apple.com/documentation/xcode/embedding-a-helper-tool-in-a-sandboxed-app).

The first feasibility spike, after separate execution authorization, should test the embedded pinned official CLI, supported managed login/refresh/logout, a scoped workspace, prohibited subprocess/file/network paths, sleep/wake, app/helper restart, update and removal. It must not depend on the developer's Homebrew path or pre-existing owner HOME. Official app-server/ChatGPT support is not certification for this packaging. Anthropic and possible Google runtime integrations require their own current official authentication, embedding and capability validation; no subscription portability promise is made here.

For removal, distinguish disconnect, stop/background disable, app removal and history deletion. Preserve history by default. Persist removal intent before network operations, confirm revoked pairing and stopped owned processes, unregister the owned job, and let the supported OS/TestFlight lifecycle remove the app. Do not assume an App Store app should implement arbitrary self-deletion. Interrupted removal must remain recoverable without resurrecting work or erasing unrelated credentials.

## Language decision: separate three concerns

1. **Runtime implementation:** the service, ledger and execution coordination.
2. **Extension interface:** versioned capabilities and wire schemas; adapters may use different languages where justified.
3. **Owner authoring:** native routine forms, named skills and explicit invocation. No required TypeScript, Go or Rust programming interface.

| Choice | Why it fits | Costs and evidence needed | Recommended role |
| --- | --- | --- | --- |
| TypeScript/Node | Existing behavior and tests; small dependency graph; rapid edit/check cycle; direct official protocol work | Node/runtime packaging, memory/idle behavior, lifecycle ownership; current structure still needs refactoring. Type stripping does not typecheck. | Keep as the working reference and a valid shipping option if measured budgets and bundle feasibility pass. |
| Go | Portable service, standard networking/concurrency, compiled deployment unit; aligns with fast iteration and operational simplicity goals | GC behavior, native binding/toolchain choices, SQLite dependency/linking, Apple IPC/embedding and full workload measurements | Leading candidate for a portable control service; prove before migrating. Avoid reviving old Fort's vendor/cloud architecture merely because it is Go. |
| Swift | Native UI/lifecycle/security APIs; one language can reduce Apple integration friction; Linux service deployment is possible | Cross-platform package availability, toolchain/pinning, storage/runtime interfaces and build measurements; Foundation domain must stay separate from Apple frameworks | Keep for clients and platform host. A cohesive Swift service is a credible alternative if all-TestFlight embedding makes a Go bridge more complex than its benefit. |
| Rust | Strong ownership/resource modeling, memory management without a tracing GC; suitable for strict process/protocol boundaries | Compile/edit cost, async/process ecosystem integration, implementation complexity and migration cost; no measured Fort advantage yet | Select only for a demonstrated containment/performance need or team preference, not a blanket rewrite. |

Go documents compilation speed and concurrency as design goals, not a benchmark of Fort. [Go FAQ](https://go.dev/doc/faq). Rust's ownership model checks memory rules at compile time; it does not make application authorization correct. [Rust ownership](https://doc.rust-lang.org/book/ch04-01-what-is-ownership.html). Optimization, LTO and incremental compilation involve tradeoffs. [Cargo profiles](https://doc.rust-lang.org/cargo/reference/profiles.html). Swift's Static Linux SDK supports portable Linux binaries but constrains dependency linking; this is not the Apple packaging model. [Swift Linux SDK](https://www.swift.org/documentation/articles/static-linux-getting-started.html). Node's built-in TypeScript execution strips types and does not perform type checking. [Node TypeScript documentation](https://nodejs.org/api/typescript.html).

**Recommended selection process:** keep Swift at the platform edge and specify the portable domain first. Compare a single small Go candidate with the existing TypeScript slice only after the distribution experiment establishes acceptable packaging. Select one core implementation. If the all-TestFlight local host is materially simpler as Swift, evaluate that cohesive option before committing to an FFI or extra process bridge. Do not build three production backends or keep dual writers.

Measure identical semantics and fixtures: clean/incremental check/build, packaged size including runtime, cold start, idle RSS/CPU/wakeups, local acceptance latency, large-history reads, database growth, cancellation and restart recovery. Record hardware, OS/toolchain, optimization, history sizes, concurrency, warm/cold conditions and variation. Separate Fort overhead from provider latency and process memory. Set numeric budgets with the owner before the comparison; none are invented in this report.

## Delivery automation and maintainability

Use **GitHub Actions for portable quality/security/contract checks and Xcode Cloud as the sole signed Apple archive/TestFlight lane**, subject to authorized account setup. Keep quality commands runnable locally so CI remains replaceable. Xcode Cloud integrates builds/tests and TestFlight delivery; its adoption does not prove this application's packaging. [Xcode Cloud](https://developer.apple.com/xcode-cloud/). Bitrise is a possible alternative release owner if Xcode Cloud cannot meet the actual workflow; do not maintain competing release pipelines by default.

PR checks should discover the complete safe fixture suite, typecheck, exercise migration/contract/removal matrices, validate dependency directions, scan secrets/licenses and record source/artifact hashes. Isolate untrusted PR execution from signing and release authority. Pin workflow dependencies with the supported lockfile mechanism, or verified full commit SHAs where lockfiles are unavailable, use minimal token permissions and review workflow changes. [GitHub secure-use guidance](https://docs.github.com/en/actions/reference/security/secure-use). Do not put personal subscription credentials on a general runner.

Release records must distinguish source checks, signed archive, upload, Apple processing, tester availability and physical acceptance. VM integration tests need infrastructure that actually provides the required virtualization; hosted Apple Silicon CI must not be assumed to support nested VMs. Keep hardware/VM acceptance separate from ordinary unit tests.

Add a small dependency rule set: core imports no provider/platform/connector implementation; adapters import core contracts; composition alone chooses implementations. Test two independently shaped runtime fixtures and remove each optional module from a build. Static scans prove import constraints, not appropriate domain ownership or runtime containment. Keep architectural decisions, release authorization and consumed session history in separate documents with clear pointers.

## Keep, replace and remove from the product graph

| Keep and strengthen | Replace or reshape | Keep outside the shipping graph |
| --- | --- | --- |
| SwiftUI native experience, scoped Keychain device storage, durable local intent, stable Agent/Home identity | `openai.ts` as organizing module; provider literals in public domain contracts | Historical Fort cloud/gateway deployment paths |
| Atomic acceptance, immutable revisions, honest unknown/cancellation semantics | Generic whole-kind scans, repeated snapshot/context copies, polling-heavy refresh | OpenMaus donor/mascot/updater/managed-worker dependencies |
| Reviewed skill bundles, explicit routine invocation, deterministic calendar rules | In-process final-result-only runtime seam; competing attempt IDs | Demo packet launchers and consumed activation authority |
| SQLite single writer, process bounds, useful protocol fixtures | Fixture/live application duplication; per-second no-op schedule writes | Broad plugin framework, microservices, Kubernetes and mandatory WASM until justified |
| Strict startup/control validation and targeted safety behavior tests | Machine-specific executable/HOME composition; absent install/remove adapter | QA resources/entrypoints from final release artifacts |

“Outside the product graph” means preserve historical source/evidence while excluding it from packaging and runtime dependencies. It does not authorize deleting the user's files. Upstream protocol changes should land in narrow runtime adapters with contract fixtures; do not rebase the whole product onto an upstream agent UI. Preserve notices and provenance for any copied code, bundled CLI and artwork. No production npm dependencies does not mean no license obligations. Packaging provenance must include native/runtime transitive components and the exact shipped versions. No new network telemetry or self-updater is needed; make diagnostics local/redacted by default and update through the chosen distribution channel.

## Acceptance grid and staged work

Each requirement needs: principle → owned boundary → removal/dependency impact → behavioral test or measurement → exact evidence → status → unresolved judgment. Use `planned`, `implemented-unverified`, `verified` or `blocked`, with justified `not applicable`. A template checker cannot mark an architecture approved.

| Gate | Concrete acceptance | Current status |
| --- | --- | --- |
| Delivery topology | Exact signed bundle/helper/runtime exercises supported auth, constrained workspace, sleep/restart/update/remove; owner accepts topology; TestFlight physical install works | Blocked on feasibility and authorized delivery work |
| Provider independence | Same core/Swift contract suite against Codex-shaped and independently shaped non-OpenAI runtime; no vendor imports/literals in core; real adapter validation recorded separately | Implemented seam, neutrality unverified |
| Optional modules | Build/run text chat with memory, voice, connectors and VM implementations absent; removed requirements become explicit blocked work | Planned; memory currently coupled |
| Durable execution | Crash before dispatch, after dispatch, after external success and before terminal commit; duplicate/stale evidence; exact-attempt cancel; no replay of ambiguous side effects | Useful current semantics; cross-process protocol planned |
| Connector authority | Wrong device/scope/digest, changed arguments, expired approval, revoked handle and prompt-injected result cannot perform external writes; lost write result stays unknown | Planned |
| Scheduler | DST gap/repeat, clock jump, downtime/catch-up, duplicate occurrence, crash/restart and unavailable runtime; visible scheduler failure; bounded idle writes | Existing behavior/fixtures; daemon and performance incomplete |
| Lifecycle | Disconnect lost response, expired pairing, partial cleanup, repeat uninstall, process/socket exit, keep-history reinstall and separately approved deletion | Intent/retry improvements present; real adapter absent |
| Portability | Export/import preserves identity, history, routine/skill revisions and unknown outcomes; secrets excluded; backup restore proves recovery | Not demonstrated |
| Performance | Comparable measured build/start/idle/history/storage/cancel workloads against agreed budgets, including packaged runtime | No reproducible benchmark |
| Release quality | Complete safe suite; contracts/migrations/removal checks; one release owner; archive/upload/processing/availability/physical evidence separated | Recorded engineering checks; no current pipeline/release |

**Stage 0 — settle the design.** Review this proposal with the owner, use the principles/spec grid committed at `1a864996e4e4f72bca99d22d745f891c3ff8ffe9` and record the chosen all-TestFlight or separately controlled execution path. Inventory exact module contracts and acceptance criteria. No replacement code is required to make these decisions concrete.

**Stage 1 — prove delivery feasibility.** After its own authorization, test the smallest signed host/helper/runtime arrangement with no new product features. Fail explicitly if the official runtime or credential flow cannot meet the chosen sandbox/distribution constraints. Do not compensate by widening privileges silently.

**Stage 2 — prove one portable vertical slice.** Native client sends one durable text command → core records request/run/configuration → executor receives a service-owned attempt → bounded result is committed → client reconnects/replays → cancellation and process restart retain honest outcomes. Use synthetic runtimes first. Memory, voice, connectors, VM and scheduling can all be absent. Add one explicitly approved official subscription turn only after the fixture and packaging gates. This is the smallest slice that proves the proposed foundation.

**Stage 3 — choose implementation and migrate.** Compare Go with retained TypeScript using the same slice and measured budgets. Consider cohesive Swift if the local TestFlight host favors it. Select one backend; migrate a copied ledger with stable IDs, backups and explicit rollback. Do not dual-write or discard historical unknown outcomes. Carry over behavior tests, not just function names.

**Stage 4 — prove optionality and automation.** Add one memory implementation and one service connector through capability ports, then remove them again. Add routine/scheduler operation with durable dispatch and visible recovery. Voice and VM should be independent later additions, each passing its own absent-module and lifecycle tests.

**Stage 5 — deliver and maintain.** Configure the chosen CI/CD ownership, finish real lifecycle behavior, produce the exact TestFlight artifacts and collect physical Mac/iPhone acceptance. Expand only when recorded evidence supports the next requirement. These are dependency stages, not invented calendar estimates.

## Notes for the visual designer

Use [target-architecture.mmd](target-architecture.mmd), [placement-variants.mmd](placement-variants.mmd), [dispatch-lifecycle.mmd](dispatch-lifecycle.mmd) and [visual-brief.md](visual-brief.md). Solid arrows denote commands/data, dashed edges optional capabilities or proposed placement. Separate owner device, Fort authority, executor containment and external vendor trust boundaries. Mark every target diagram **proposed**, and TestFlight feasibility **unverified**. Do not show an existing VM, connector platform or Apple approval badge.
