# 058: Native-first owned Agent MVP

Status: Proposed product and implementation contract. Awaiting Toby's approval before implementation.

Decision owner: Toby. Written: 2026-10-12.

Read the [executive decision page](decisions/058-executive.md) for the product choices and [validation record](decisions/058-validation.md) for evidence and limits. The [architecture diagram source](diagrams/058-architecture.mmd) and [delivery diagram source](diagrams/058-delivery.mmd) describe this proposal.

## 1. Goal and authority

Fort gives its owner a polished messenger for stable named Agents, with owned model choices, reviewed skills, and explicit workflows. Conversations continue across a native iPhone app and a native Mac app. The owner chooses a local execution host or an always-on server they control. Fort uses official agent runtimes behind its own bounded contract and reports what actually happened, including interrupted or uncertain work.

The latest direction is: "Go ahead and put a spec together based on the new implementation, but make native a first class deliverable for MVP."

**Interpretation:** native means both a SwiftUI iPhone app and a SwiftUI Mac app, sharing FortKit. Both are required for MVP and participate in the first runnable vertical slice. Native clients are not deferred wrappers around a web MVP. Watch and CarPlay remain later decisions. This interpretation follows the earlier iPhone and Mac goals and needs no invented visual style, navigation preference, or delivery date.

This is the proposed successor contract for the October 11 selective-extraction direction. Writing and committing it does not authorize product implementation, runtime unlock, installation, login, credential changes, provider cleanup, deployment, publication, or billing. Approval of the spec is required before a bounded implementation slice. Distribution and infrastructure work still need their applicable authorization.

### Relationship to earlier decisions

The October 11 fork decision and latest user direction supersede the Hermes/OpenClaw-only restriction and exclusions of official native runtimes, model choice, skills, and routines for this proposed product. Spec 058 also replaces the review's later-native sequence with native delivery from the first slice.

The historical Specs 054, 055, 056, and 057 in `/Users/tobiasgunn/dev/fort/specs/` remain unchanged. Their relevant intent continues: one owner, exact identity, durable ordered messaging, privacy, explicit delivery uncertainty, native Apple clients, and installed-product acceptance. Spec 058 proposes these specific changes to 054/056:

| Earlier constraint | Proposed decision |
| --- | --- |
| Hermes/OpenClaw Contacts only; no native CLI delivery | Stable Fort Agents bound to selected official Codex or Claude Code runtime versions |
| No owner model choice, skills, or routines in version one | Explicit model selection, reviewed skill bundles, manual routines, then durable schedules within MVP |
| Managed service and Postgres only; no local service | One Fort-owned service and transactional ledger, placed on the owner's Mac or self-hosted server |
| Full web, voice, Watch, and CarPlay scope | Native iPhone and Mac text messaging first; bounded web administration; voice, Watch, and CarPlay need later scope decisions |
| Clean-slate replacement of the full product | A separately approved, narrowly extracted successor; no broad rewrite or wholesale donor unlock |

Spec 055 retains authority over teardown and clean-baseline acceptance; Spec 057 retains its historical progress role. The owner reported local teardown complete, but this spec does not certify remote closure, resurrect old resources, or change teardown scope. Isolated development must not reuse old data, listeners, credentials, or hosted resources. Integration or release must reconcile the applicable clean-baseline evidence before proceeding. Existing Go seam and determinism rules remain applicable to any existing Go code touched by a later approved change.

## 2. Verified starting point

The branch is `codex/fort-core-openmaus-20261011`. Its product-source checkpoint is `2dfe43743a35d0a5e2639870c58055f17f95a1bc`; the donor is OpenMausBot `8b53c0323dba2099a953ee2d77ab9330cd5acaa7` under `foundation/openmaus-core/`.

The [fork decision](../foundation/openmaus-core/FORK-DECISION.md), [validation note](../foundation/openmaus-core/VALIDATION.md), [import manifest](../foundation/openmaus-core/IMPORT-MANIFEST.json), and [entrypoint inventory](../foundation/openmaus-core/QUARANTINE-ENTRYPOINTS.json) describe a source-only quarantine. All 72 inventoried entry modules are refusal-only stubs. The 73 archived source files and 2,509 manifest hashes were independently verified during review. The dependency-free checks pass 7/7, including 324 authorization combinations. These checks establish the inventoried refusal and authorization behavior, not a running product or an operating-system sandbox against arbitrary library invocation.

The imported manifest declares Node >=24 and pnpm 10.33.0; the inspected host has Node 22.22.3. The donor lock is provenance, not a validated successor lock. Full dependency resolution, typecheck, build, actual HTTP handlers, providers, native clients, packaging, and installed-product acceptance remain unrun. Dormant provider adapters still inherit process environment values; the archived scheduler and dispatch persistence do not meet the new transaction contract.

This spec adds no implementation. The donor stays quarantined. A later implementation creates a new bounded entrypoint with an explicit dependency list, fresh data root, and separate acceptance evidence.

## 3. MVP outcomes and limits

MVP is complete only when the following outcomes work together:

1. The owner enrolls iPhone and Mac clients, selects a named Agent, and sends text into its permanent Home Conversation. Both clients show the same ordered durable transcript, read position, and actionable delivery state.
2. The owner sees and chooses an available official runtime, exact model identifier, and execution host. Changes create a new binding revision. Existing messages retain their original execution evidence.
3. The owner reviews a versioned skill bundle, explicitly enables it, and invokes it for a turn or a saved routine. Routine authoring uses native forms and plain instructions. A structured file export is available but optional for the owner to use. TypeScript is never required for ordinary authoring.
4. The owner can run a routine now, review its exact destination and authority, then enable a schedule with explicit timezone, overlap, and missed-run rules. Each result returns to the selected Conversation.
5. A restart, connection loss, sleeping execution host, expired login, rejected approval, or ambiguous external effect has a visible recovery path. Fort does not silently resend, switch providers, change models, or move to API billing.
6. The same clients and contract work with a local Mac installation or an always-on self-hosted installation. Both topologies require their own acceptance evidence. Official OpenAI and Anthropic subscription paths are MVP goals subject to the feasibility gates in section 9.

MVP includes multiple named Agents and one permanent Home Conversation per Agent. It serializes work within each Conversation. Secondary Conversations, Groups, Handoffs, agent-initiated delegation, and automatic host failover are deferred; the identity scheme must not prevent adding them later. Their historical vocabulary is not authorization to implement them now.

Also deferred: voice sessions, speech vendors, Watch, CarPlay, attachment messaging, a plugin marketplace, arbitrary MCP servers, computer-use automation, a general visual DAG editor, event-trigger integrations, Google, multi-user hosting, and managed OpenMaus services. Selected workspace tools are possible only within the reviewed runtime authority. Importing a skill does not enable new tools or integrations.

## 4. Native journeys and web role

Both Apple apps use SwiftUI navigation, lists, text input, and approval controls with platform accessibility semantics. FortKit supplies transport, contract decoding, durable client requests, event reduction, and local cache behavior. It does not execute agents. Neither app embeds the donor's React app as its primary experience.

| Journey | Required native behavior on iPhone and Mac | Acceptance evidence |
| --- | --- | --- |
| Connect | Show the Fort installation identity and host address, enroll a device, store its credential securely, and distinguish unreachable service from expired authorization | Physical iPhone and Mac connect to the same service; revoked credentials fail; switching installations never merges caches |
| Start a conversation | Create or select an Agent; show Home, selected provider/model, execution location, and readiness; send exact text | One durable acceptance and one reply appear on both devices; a request replay after a lost response creates no duplicate |
| Continue elsewhere | Open the same Home Conversation on the other device; restore scroll context without manufacturing messages; update the read cursor only for content actually viewed | Kill each app and reconnect after events accumulate; ordered replay and monotonic read position converge |
| Change a model | Select a supported model and see the new binding apply to future accepted work; show queued work's pinned model | A queued request remains on its recorded binding unless explicitly canceled and resubmitted |
| Approve or stop | Show the actual pending action, Agent, host, relevant workspace, and authority; approve once, reject, or request cancellation | Conflicting device answers have one durable winner; stale approval cannot authorize a later attempt |
| Use a skill or routine | Inspect bundle source, files, requested tools, and changes; explicitly invoke; create/edit a routine with forms; preview its input, destination, and schedule | Both apps can complete the workflow without code or a web-only control; recorded revisions match execution |
| Recover | Explain pending-on-device, waiting-for-host, needs-sign-in, failed, and outcome-unknown states; offer only valid actions | Airplane mode, host sleep, lost acknowledgement, expired login, and interrupted effect tests produce distinct states |

The Mac app is a native client even when it shares a machine with the host service. Closing its window does not terminate accepted work when the separately managed host is running. Installation, service start/stop, data location, and uninstall behavior must be explicit. A visible host status must distinguish the app, service, runner, and provider. The local service must not inherit a UI process's credentials.

The iPhone is a client. Fort does not promise independent agent execution, a reliable scheduler, or a permanent stream in iOS background time. On foreground resume it reconciles durable requests and replays events. The service and execution host do the work. Apple's [background execution guidance](https://developer.apple.com/documentation/backgroundtasks/choosing-background-strategies-for-your-app) is a release-validation input, not evidence that a particular Fort background mode works.

Native notification handling is part of MVP. With owner consent, a self-hosted or awake local service may use direct APNs delivery for completion and approval wakeups. Payloads omit conversation text and credentials; the app fetches authorized state before showing or acting. Push is optional to use, never the ledger or a guarantee of timely execution. Denied notification permission, delayed or duplicate pushes, and an unreachable service must leave foreground replay fully usable. Apple push is a disclosed external transport dependency. No managed OpenMaus notification relay is required.

The web role is bounded owner administration and diagnostics, including service readiness and recovery inspection. A web composer may help development, but cannot satisfy native acceptance. No essential daily MVP task may require switching from either native app to a Fort web page. Provider-owned browser sign-in is a separate provider flow, not a substitute for native Fort controls.

## 5. Target architecture and language decision

Recommend a small TypeScript service with selective reuse of official-provider protocol logic. Backend language, extension interface, and workflow authoring are separate decisions:

| Layer | Proposed interface or implementation |
| --- | --- |
| Native clients | SwiftUI apps plus a shared Swift FortKit package |
| Client/service boundary | Versioned language-neutral HTTP commands and ordered event replay; JSON Schema/OpenAPI contracts and shared fixtures |
| Fort domain and persistence | Small TypeScript modules, one authoritative transactional ledger, deterministic dispatch and scheduling |
| Runner boundary | Authenticated, versioned command/event protocol to an isolated host runner; no direct database access |
| Official runtime adapters | Selected Codex app-server and Claude Code protocol logic behind Fort's Runtime contract |
| Skills | Reviewed versioned bundles with Markdown, a manifest, and optional explicitly permitted assets/scripts |
| Routines | Native form authoring of instructions, selected skills, typed steps, destination, and optional schedule; round-trip YAML/JSON export |

TypeScript is a recommendation to validate with the first slice, not an approved broad rewrite. A small Go service remains a viable alternative if the same contract tests demonstrate a concrete distribution or maintenance benefit. Replacing provider parsing merely to change language is not justified before those tests exist. Existing Go code is historical implementation evidence and is not silently adopted, deleted, or run beside the successor.

The proposed module boundaries are:

- Contract definitions depend on no service, client, or provider implementation.
- Domain rules consume validated commands and return transitions. Routing is deterministic. Only a task execution invokes Runtime; scheduling, authorization, transforms, and approval gates make no model calls.
- Persistence owns transactions, sequence allocation, the outbox, leases, and recovery. Providers cannot write tables.
- API adapters authenticate the caller and translate the public contract. They cannot spawn runtimes directly.
- The runner accepts only a bounded dispatch for its enrolled host and exact binding. Provider adapters normalize protocol evidence rather than deciding product state.
- FortKit and the web client use only the public contract. Neither imports server models or provider wire types.

Use one authoritative service writer and SQLite on a durable local volume for the initial single-owner installation, including a single self-hosted server. Enable transactional durability and foreign-key enforcement; prove crash recovery and backup through tests. The exact SQLite binding, synchronous settings, and pinned toolchain belong in the approved extraction manifest. Do not put SQLite on a network filesystem or share its file among active writers. Postgres is a later operational substitution if measured requirements justify it; it is not a second live store or a prerequisite for the first local slice.

## 6. Identity, binding, and public contract

Use the [Fort glossary](../CONTEXT.md). An Agent is a durable Fort identity; a provider session, display name, model, process, and machine are not its identity. Creation atomically makes the Agent and its sole Home Conversation. Provider/session changes never replace Home or split history.

An immutable Agent Binding Revision pins the Agent, Behavior Revision, provider adapter and official runtime version, provider account reference, exact selected model, host, workspace, tool policy, skill revisions, and authority. Credentials are referenced by opaque identifiers, never embedded. Every accepted turn and routine run pins the binding and input digest. A changed default cannot mutate pending work. A disabled binding blocks dispatch; the owner can cancel it and submit a new request under a new binding.

For a new chat submission, the server resolves the Agent from the Conversation and checks that the submitted binding belongs to that Agent and is its enabled current revision. Acceptance and that revision check share one transaction. A concurrent edit either follows acceptance of the exact submitted revision or makes the submission conflict. It never silently substitutes a new revision. Duplicate accepted requests return their original receipt before applying current-revision checks. A reviewed Routine intentionally pins an older enabled revision; its explicit invocation authorizes that exact revision. Disabling a revision blocks both queued dispatch and future routine invocation.

Record selected model, provider-reported effective model, and the evidence source separately. If the provider does not disclose the effective model, show that fact. Prompt text saying "I am model X" is not identity evidence. If the exact selected model is unavailable, fail visibly before dispatch or hold the existing request; no silent replacement.

Define contract version 1 before building the first clients. Include these operations and their behavioral fixtures:

| Contract area | Required behavior |
| --- | --- |
| Installation and device | Authenticated capabilities/version response, installation identity, enrollment, credential revocation, and readiness |
| Agents and bindings | List/create Agent, fetch Home, review and select a binding revision with optimistic concurrency |
| Conversation | Paginated durable transcript; submit text with a stable client request ID and immutable intended binding; lookup that request after lost acknowledgement |
| Events and read state | Cursor replay, snapshot at a stated cursor, live HTTP event stream, monotonic last-read update |
| Run and approval | Read exact state/evidence, request cancellation, resolve a bound pending approval with compare-and-set semantics |
| Skills and routines | Review/version/enable skill bundle, validate/save/run routine, explicitly enable/pause schedule, inspect occurrences |

The server durably commits acceptance before acknowledging it. Text is preserved as exact UTF-8, including whitespace, newlines, Unicode form, and slash-prefixed text. Fort does not trim, normalize, parse text as a command, or silently truncate it. The v1 text limit is 65,536 UTF-8 bytes; reject larger input before acceptance. Validate the bound provider's smaller declared limits before dispatch and report incompatibility rather than altering content. Skill/routine controls are structured commands separate from chat text.

A duplicate request ID with the same validated command fields and exact text bytes returns the original receipt and snapshot. A reused ID with different content returns a conflict without external I/O. The client persists its request ID, content, destination, intended revision, and recovery epoch before sending; an app kill or transport retry cannot change them. Pressing Send while disconnected creates an explicit pending submission that retries that same request on reconnect. Editing a draft alone never queues work. Pending submissions can be canceled locally before acceptance is confirmed; an uncertain acceptance requires receipt lookup and, if accepted, server cancellation. Bounded client retries do not create another turn and obey the restore rule in section 8.

Events carry schema version, installation ID, recovery epoch, unique event ID, monotonic sequence within that epoch, entity ID, and durable timestamp. The service orders events, not device clocks. Snapshot plus cursor establishes an atomic replay boundary. Duplicate events are idempotent. A cursor gap triggers bounded replay or a full snapshot at a new boundary. Terminal output is recoverable after stream loss; a transient animation or token delta is never the only stored copy of a message.

Each read acknowledgement identifies the Conversation and an existing message at its replay cursor. Reject a future cursor, wrong Conversation, or wrong epoch. Clients acknowledge only messages actually presented as read; the server stores the maximum valid read position per Conversation so an older device cannot move it backward. A global event sequence cannot mark later or unrelated messages read.

Support current and immediately previous contract minor versions during rolling client updates. Additive unknown optional fields are safe; unknown state/action variants display a bounded unsupported state and disable unsafe actions. Breaking behavior changes require a major version and explicit upgrade gate. Test Swift and TypeScript decoders against the same committed positive and negative fixtures. Installed old clients must remain read-only and informative when incompatible, rather than silently losing history.

## 7. Transactional delivery and recovery

One transaction records an accepted command, its immutable request/binding snapshot, the user message, the initial run, an outbox item, and ordered events. A second transaction persists each accepted provider event and its derived visible state. Publishing an event before commit is forbidden. Projection rebuilds must reproduce the transcript without running a provider or repeating an external effect.

Dispatch claims use a transaction, an attempt identifier, an expiring lease, and a fencing generation. A runner must acknowledge the exact dispatch and generation. Late events from superseded attempts cannot overwrite canonical state; retain them as diagnostic evidence. A lease is not proof that the former process stopped. Loss of runner contact first creates an uncertain state; no replacement dispatch occurs until the adapter proves the prior attempt did not start or reconciliation proves a safe continuation. A restored database also invalidates old runner sessions and reconciles in-flight work before dispatch.

Waiting for approval and outcome-unknown retain the Conversation's execution barrier. Subsequent messages may be durably queued but cannot launch around unresolved earlier work. An explicit owner recovery decision can close the barrier with its uncertainty recorded; it does not turn an unknown effect into success or failure.

| Durable delivery state | Meaning and allowed recovery |
| --- | --- |
| Accepted/queued | Fort has committed the request; provider execution has not been confirmed |
| Waiting for host or sign-in | The accepted request is retained for its exact binding; no rerouting |
| Running | Correlated runtime evidence confirms this attempt began |
| Waiting for approval | A persisted action requires owner authority; a schedule cannot answer it |
| Completed | A correlated terminal result and resulting messages are durably committed |
| Failed before start | Evidence proves no execution began; a bounded retry may use the same request and a new recorded attempt |
| Cancel requested/canceled | Requesting stop is not confirmation. Canceled requires runtime evidence or verified process termination; prior effects may remain |
| Outcome unknown | Execution or an external effect may have occurred; reconcile or request an informed owner decision, never blind resend |

Do not infer completion from process exit zero alone. Validate terminal protocol evidence. Separate run completion from the success of a claimed external action. Fort cannot promise exactly-once email, file mutation, payment, or another external effect when that system lacks an idempotency/reconciliation contract. Within the Fort ledger, request acceptance, event ingestion, and routine occurrences are deduplicated. Across an external system, the product preserves evidence and uncertainty.

An owner choosing to retry uncertain work sees the risk of repetition and creates a new linked request. That choice does not rewrite the original result. Safe resume requires provider support and evidence that resuming the exact session/attempt will not replay already performed effects. Missing resume support is a visible limitation, not a reason to start the prompt again invisibly.

## 8. Placement, offline behavior, and data ownership

Each Fort installation has one ledger, one installation identity, and one active service writer. The default first slice uses one local Mac host. The self-hosted variant uses the same service and contracts on an owner-controlled always-on server with persistent storage. Remote execution adds an explicit host binding; it does not replicate writable ledgers among devices.

| Situation | Required behavior |
| --- | --- |
| iPhone/Mac cannot reach the service | Preserve a local draft or explicit pending submission; show "not yet accepted by Fort". No server-delivery promise |
| Service is reachable, selected runner is offline | Commit acceptance and show waiting for that host. Preserve the exact binding |
| Local Mac service host sleeps or powers off | No service acceptance, execution, schedule, or notification guarantee while unavailable. Local clients reconcile after wake |
| Self-hosted service and runner remain online while owner devices sleep | Accepted runs and schedules may continue within their authority; gated work waits for the owner; clients replay on return |
| Provider is unavailable, signed out, or quota-limited | Report the exact readiness problem; retain queued work or request owner recovery; never change billing or provider |

A local installation supports the Mac client on the host and the iPhone over an explicitly configured reachable private network endpoint with authenticated TLS. A Mac's loopback URL is not an iPhone endpoint. Enrollment verifies installation identity and certificate trust. Public exposure, automatic tunnels, and unauthenticated LAN discovery are not defaults. Local remote access beyond the configured network needs an explicit connectivity setup; Fort does not imply that a sleeping laptop is an always-on server.

Self-hosted means the owner controls Fort's service, storage, runner, backups, and updates. Official provider inference remains external, and optional Apple push remains external. No managed OpenMaus account, worker, cloud identity, telemetry endpoint, updater, or tunnel is needed. Startup and idle network tests must prove this claim for the shipping dependency graph.

Use a fresh, explicit Fort data directory with service-only access. Never discover, migrate, or read legacy `.fort`, `.openmausbot`, provider homes, or arbitrary host directories automatically. Store canonical history and redacted audit data in the ledger. Native caches use platform data protection and credentials use Keychain; separate caches and secrets by installation and device identity. Cache clearing and device revocation do not delete canonical history. Owner-authorized full export and deletion must have defined scope and confirmation; automatic retention deletion is off by default.

Backups must be consistent, encrypted, and owner-controlled, with a recorded recovery point. They cannot guarantee recovery of work acknowledged after that point. Restore to a stopped fresh service and retain Agent/Conversation identity, but create a new recovery epoch, invalidate prior runner sessions/leases, and rotate exposed service/device credentials as needed. Clients discard old replay cursors and reconcile against a new snapshot. A pre-restore pending request or receipt absent from the backup is recovery-unknown and cannot be automatically resubmitted under the new epoch. Recover its evidence or require an informed owner decision before a new linked request. Reconcile every in-flight effect. Never run a restored copy and its original concurrently. The distribution gate includes this actual restore drill, not a backup command's exit status.

Before resuming dispatch or scheduling, reconcile every restored nonterminal request and every potentially due occurrence between the recovery point and the restore cutoff. A queued item in the backup may have executed afterward, and a missing occurrence may already have caused an effect. Neither is proof that work never began. Unresolved cases remain recovery-unknown; automatic catch-up cannot replay that interval. Stop or fence the original service and runners, and retain uncertainty about any process whose termination cannot be confirmed.

## 9. Provider feasibility and credential boundaries

The target is official OpenAI and Anthropic subscription-backed agents for the single owner. Support is demonstrated per provider, selected version, account/plan, operating system, and local or server topology. It is not inferred from an adapter file or a successful login. Model inventory reflects actual runtime availability; there is no fixed promise of models, quotas, or plan entitlements.

OpenAI documents [ChatGPT subscription sign-in for Codex](https://learn.chatgpt.com/docs/auth) and [app-server integration](https://learn.chatgpt.com/docs/app-server). The first candidate uses the official CLI's own authentication lifecycle and bounded protocol. This does not certify every unattended or hosted deployment.

Anthropic's [authentication and credential-use policy](https://code.claude.com/docs/en/legal-and-compliance) distinguishes ordinary Claude Code subscription use from products routing users through subscription credentials; it directs product/SDK use toward API or supported cloud authentication. Fort must resolve the exact personal local-wrapper and unattended self-hosted cases before claiming support. A CLI wrapper is not automatically permission to use subscription credentials in any topology.

The release evidence matrix starts with four unproven cells: Codex local, Codex self-hosted, Claude Code local, and Claude Code self-hosted. Each cell requires:

1. Current official documentation and any necessary provider clarification for the exact personal use, automation, and placement.
2. Owner-authorized provider-native sign-in on that execution host, selected account/plan evidence, and confirmation of the intended billing mode without recording tokens.
3. Real exact-model send/reply, streaming normalization, tool approval, cancellation, session continuation or an explicit unsupported result, login expiry/refresh, quota behavior, and restart recovery.
4. Isolation and credential-canary tests for the selected official version and operating system.

Both providers are MVP goals. If a required subscription/topology cell is not feasible, mark it unsupported and present a concrete scope decision to Toby. Do not label the full matrix complete, swap in API billing, copy tokens to make a failing case work, or quietly ship a narrower MVP. A local provider result does not satisfy a remote cell. No provider login or policy waiver is authorized by this document.

Provider credentials stay in the chosen provider-owned store inside the designated execution environment. Fort clients and the ledger store no OAuth tokens, refresh tokens, or API secrets. The first implementation must not offer a Fort-owned Claude OAuth flow or extract a subscription token for a custom HTTP client. Codex external-token mode and managed-company access are not part of the first slice. Tool configuration must not accidentally select an ambient API key over subscription authentication.

## 10. Isolation and approval authority

Fort treats messages, retrieved content, repository instructions, provider output, and skill contents as untrusted task data. They cannot enroll devices, grant authority, change bindings, install tools, enable schedules, answer approval cards, or request credentials. These changes require authenticated owner commands checked outside the model.

Every owner device has a separately revocable credential. Bootstrap enrollment uses a single-use, short-lived code presented by the service owner and bound to the installation identity; it grants no unattended provider login. Native credentials stay in Keychain. Browser administration needs same-origin/CSRF protections in addition to authentication. Loopback, no Origin header, a local username, or an Ask/Full label confers no owner authority. All operations, including reads and event streams, enforce identity and scope.

The service process owns the ledger, device authorization, enrollment, approval decisions, and runner enrollment. An isolated runner has only dispatch-scoped authority, a bounded workspace, selected provider configuration, and required network access. It has no database connection, owner token, approval-signing key, deployment credential, global Keychain access, or host home mount. The initial implementation must choose an enforceable OS account, sandbox, or VM arrangement and prove its filesystem, process, IPC, and network boundaries. A separate process with the same unrestricted host-user privileges does not satisfy isolation.

Build child environments from explicit per-provider allowlists. Do not spread the parent environment and remove known secret names. Check unknown credential canaries, PATH/binary substitution, symlink escapes, inherited file descriptors, and access to the service API. The selected official runtime is trusted with its own provider authentication; this is a stated trust assumption. Whether its spawned tools can reach that credential scope must be tested. If the selected configuration cannot protect the required credential boundary, that provider/topology remains unsupported until corrected. No prompt or permission label counts as isolation evidence.

An approval record binds the owner, installation, run, attempt, binding revision, canonical action digest, requested resource scope, expiry, and pending state. Only an authenticated owner command can change it. The first valid decision wins atomically; a conflicting or stale answer returns the durable prior decision. Runner events can request an approval but cannot authorize it. Approved authority cannot widen at dispatch or on resume. Changes to action arguments require a new approval. If the provider offers only broader approval than Fort can truthfully express, hold that action as unsupported.

Preapproved routine authority is explicit, bounded, versioned owner consent. Every step still passes policy checks. A run cannot promote itself to full access, accept a skill update, or use a scheduled trigger to bypass a gate. Cancellation revokes pending authority; expiry and provider restart do not turn old approvals into reusable grants.

## 11. Skills, routines, and scheduler

A skill is a complete reviewed bundle, not only a copied `SKILL.md`. Its manifest records identity, source/provenance, license, version, content digest, entry instruction file, assets, optional scripts, requested tools, and supported runtimes. Validate path traversal, symlinks, archive sizes, and executable permissions before staging. Import is inert. The native review shows changed files and requested authority before enablement. Runtime-owned native packaging may differ, but it must preserve the reviewed bundle or reject unsupported content.

The owner explicitly selects a skill for a turn or a routine. Model-suggested skills remain suggestions. Pin the exact enabled revision into acceptance; later edits create new revisions and require review. No background discovery, auto-install, or silent bundle update enters the MVP. An executable script is optional expert content with its declared interpreter and resource scope, not a TypeScript requirement. Unreviewed scripts cannot run on import.

A Routine revision contains a name, owning Agent, exact output Conversation, binding revision, typed inputs, plain instructions, selected skill digests, a bounded ordered list of task and approval-gate steps, and optional schedule. A native form can author a single task or a short linear workflow. No loops, arbitrary code expressions, hidden delegation, or model-generated routing are required. A structured YAML/JSON export round-trips the same data for owners who prefer files. A future authoring language cannot become an MVP prerequisite.

Each routine run stores its ordered step IDs, step revisions, input snapshots, state, output references/digests, attempts, and pending gate checkpoint in the same ledger. A completed step and the next step's eligibility commit together. Restart resumes from persisted eligibility without rerunning a completed step or skipping a gate. Task outputs are untrusted data for declared later inputs; they cannot add a step or widen authority. A step with an unknown effect blocks successors until reconciled. The first implementation caps a Routine at 16 ordered steps and validates the whole sequence before saving.

"Run now" is a distinct authenticated command using the same acceptance transaction and outbox as chat. A schedule emits that same command with a unique occurrence ID; it has no separate task transport or transcript. Saving a routine does not enable a schedule. Enabling or editing a schedule previews its timezone and next occurrences and records explicit owner consent. Editing a binding or skill never silently updates an already enabled routine's pinned revisions.

MVP schedules cover one-time dates and daily/weekly calendar recurrence with an IANA timezone. More general cron and external events are deferred. Persist occurrence identity as routine revision plus the intended scheduled instant. Transactionally insert the occurrence and command once; a unique constraint prevents duplicates after restart or competing schedulers.

Default policies are one active run per Routine, skip an occurrence that overlaps, and skip missed occurrences after downtime with a visible summary. The owner may select one catch-up occurrence rather than an unbounded backlog. A nonexistent local time at a DST jump is skipped and recorded; a repeated local time executes once at the first occurrence. Timezone changes create a new schedule revision and do not reinterpret old occurrences. Approval pauses do not release overlap protection. Disable/pause prevents new occurrences but does not erase history or imply that in-flight effects stopped.

Retries are bounded and deterministic. Automatic retry is allowed only with evidence that the previous attempt never began or an explicitly idempotent step contract. External-effect uncertainty pauses the run. Calendar calculations, scheduling, skill selection, and authority checks invoke no model.

## 12. Extraction and affected areas

The following map guides a later approved implementation. Proposed directories are not created by this spec and do not authorize moving existing Fort code.

| Area | Keep, replace, or remove from the shipping graph |
| --- | --- |
| `foundation/openmaus-core/server/drivers/codex.ts`, `claude.ts` | Selectively retain protocol knowledge, parsers, and relevant fixtures after conformance review; replace ambient environment, credential discovery, install/update behavior, and product coupling |
| `foundation/openmaus-core/server/contracts.ts`, `shared/runtime-events.ts` | Use as reference for a smaller private runtime protocol; do not expose donor wire types to native clients |
| Archived HTTP composition root, store, permission, and routine code | Replace with Fort-owned auth, ledger/outbox, explicit approvals, and scheduler contracts; do not restore archived entrypoints |
| Donor React UI | Reference useful accessibility and rendering behavior selectively; do not repair the full donor app for native MVP |
| Enterprise, managed workers, tunnels, telemetry, Electron/updater, mascot | Keep excluded or outside the shipping graph; remove residual dependency declarations from the successor manifest |
| `contracts/`, `service/`, `runner/`, conformance fixtures | Proposed narrow successor modules, separate from the quarantined donor |
| `ui/apple/FortKit`, `ui/apple/FortiOS`, `ui/apple/FortMac` | Existing names are candidate locations; inspect retained source before reuse. Implement native clients against the new public contract without importing the old runtime |

Maintain an extraction manifest with upstream path/commit/hash, destination, license, Fort modifications, and rationale for every copied file. Preserve Apache-2.0 LICENSE/NOTICE and applicable third-party notices; the root MIT notice does not relicense donor code. A release needs changed-file attribution, a dependency license/SBOM review, and a branding/asset check. Merge selected upstream fixes through adapter conformance tests; do not wholesale merge the donor product.

The successor build must use a pinned supported toolchain and its own lockfile, explicit script policy, and reproducible artifact hashes. Runtime installation and updates are owner-controlled. Fort never silently downloads, upgrades, or switches an official runtime during a run. Network-denied startup and idle tests must detect residual vendor dependencies; explicitly selected provider traffic is tested separately.

## 13. Implementation sequence and acceptance tests

Every code slice starts with a failing behavior test at its highest useful public seam, observes the intended failure, and implements the smallest change. Go changes retain `go test ./...`, architecture dependency checks, and `-race` for concurrency. New TypeScript and Swift targets need equivalent behavioral, concurrency, and contract checks. Static source-shape tests supplement actual HTTP, process, persistence, and installed-client tests; they cannot replace them.

| Stage | Deliverable and required exit |
| --- | --- |
| 0. Approved contract and extraction plan | Approve this spec and its bounded implementation scope, select files/dependencies and isolation method, record provider feasibility questions, and preserve quarantine. Ordinary in-scope local builds/tests follow that implementation authority. Resolve only missing authority for external actions, new costs/scopes, or interactive provider steps; do not ask again for already-authorized work |
| 1. Native fixture slice | Fresh service, ledger/outbox, owner/device auth, one Agent/Home, fixture Runtime, FortKit, SwiftUI iPhone and Mac. Both clients send, display, reconnect, and recover after service restart against the same versioned contract. Fixtures are visibly non-production |
| 2. First official provider | Codex is the default first candidate. Both native apps perform a real selected-model turn through the isolated runner; persist terminal output, interrupt, disconnect, kill/restart, and replay. Prove provider-native subscription/auth behavior and credential confinement on the local host |
| 3. Second provider and owned work | Run the same provider tests for Claude Code; keep feasibility failure visible. Add complete reviewed skill bundles, explicit invocation, and native manual routine creation/run through the existing transaction path |
| 4. Scheduling and placement | Add durable calendar occurrences, overlap/missed-run recovery, then validate the complete native flow on the always-on self-hosted installation. Close every advertised provider/topology cell and prove restore and host outage behavior |
| 5. Native MVP release | Complete accessibility, notification lifecycle, distribution, install/upgrade/uninstall and real-device acceptance. Both native apps and both accepted topologies meet the declared matrix. This stage polishes the clients already used in stages 1-4 |

The smallest real vertical slice is **native iPhone + native Mac → authenticated v1 API → one Agent/Home → transactional acceptance/outbox → isolated official Codex runtime → committed reply → restart and cursor replay on both devices**. A web page is not a substitute at any arrow. The first provider may change only through a recorded feasibility decision; no provider promise is implied by its place in the sequence.

The following are executable acceptance requirements for the later implementation, not claims of tests already run:

| ID | Test and pass condition |
| --- | --- |
| T01 | Fail injection before and after every acceptance transaction boundary yields either no accepted request or one message/run/outbox/event set; lost HTTP acknowledgement and concurrent duplicate submit return the same receipt. Changed payload under the same key conflicts. Preserve leading/trailing whitespace, line endings, Unicode forms, and slash text; test 65,536-byte input and rejection one byte over |
| T02 | Kill service and runner around claim, start acknowledgement, provider effect, event receipt, and terminal commit. Leases never trigger blind resend; ambiguous execution remains outcome-unknown; delayed old-attempt events cannot overwrite a newer attempt |
| T03 | Replay duplicated, reordered, and gapped network deliveries into both native clients; snapshot/replay converges on canonical event order. Kill the app with an unacknowledged request and recover the same request ID and immutable binding |
| T04 | Concurrent model/binding change and turn submission either accept the exact current submitted revision or conflict. Foreign, disabled, and stale chat bindings fail before acceptance; receipt retries retain their accepted revision. A routine can use its explicitly pinned older enabled revision. Queued work stays pinned. Provider-reported model and unavailable/unknown evidence are rendered honestly |
| T05 | Actual HTTP fixtures deny absent, forged, expired, revoked, wrong-installation, runner, loopback, cross-origin, and wrong-scope owner authority on commands, reads, streams, and approval decisions. No credentials are renewed on denial |
| T06 | Two devices race approve/reject; only one answer commits. Wrong action digest, attempt, expiry, or resource scope fails. A prompt or runner calling the approval API cannot authorize itself |
| T07 | A malicious fixture tool attempts owner-token, ledger, sibling-run, provider-scope, home, Keychain, environment-canary, symlink, inherited-descriptor, and service-API access. The selected OS isolation enforces the declared boundary; residual privilege is a failed gate |
| T08 | For each supported provider/topology cell, record exact runtime version, subscription mode and model evidence, real text/tool/gate/cancel behavior, login expiry, quota exhaustion, resume limits, and restart/uncertainty behavior. No fallback billing or impersonated result |
| T09 | Import a bundle with nested assets, a script, traversal, symlinks, and a changed permission request. Unsafe content is rejected or held for review; import executes nothing; invoking a pinned revision remains unchanged after editing the source |
| T10 | Create and run a routine through each native form without TypeScript. Only enabled reviewed skills run; the exact Conversation receives its result; schedule or skill edits do not mutate an accepted run |
| T11 | Deterministic clock fixtures cover one-time, daily, weekly, DST gaps/folds, timezone edits, overlap during approval, pause, downtime, bounded catch-up, and competing scheduler claims. One occurrence creates one command; no model calls occur |
| T12 | Physical iPhone switches between reachable network, airplane mode, background, app termination, and foreground; local host sleeps/wakes; self-hosted host reboots. Pending-on-device, accepted/waiting, running, and unknown remain distinct and converge after recovery |
| T13 | Physical iPhone and Mac share Home history and monotonic read state. Race valid old/new read acknowledgements and reject future, wrong-Conversation, and wrong-epoch positions. Revoke one device, enroll another, then switch installation endpoints. Canonical history survives; caches and credentials never cross installations |
| T14 | Test VoiceOver, Dynamic Type on iPhone, keyboard and VoiceOver on Mac, focus after updates, reduced motion, long messages, large transcript pagination, dark/light system appearance, and error recovery in the exact installed builds |
| T15 | Grant/deny/revoke notification permission; delay, duplicate, and drop APNs notices; open a stale approval notice. Fetch current authorized state and never execute from a notification payload. Foreground recovery works with push disabled |
| T16 | Restore an encrypted consistent backup after effects from a request accepted after backup, a request queued in the backup, and a scheduled occurrence absent from the backup. Fence original runners and change the recovery epoch. Recover messages through the stated recovery point, reject old cursors, and prevent client retry, outbox dispatch, or schedule catch-up from repeating those effects. Unresolved cases remain recovery-unknown. Demonstrate owner export and scoped deletion |
| T17 | Network-denied startup/idle, recorded dependency graph and SBOM, explicit update policy, fresh data-root canaries, and omitted vendor routes prove no managed OpenMaus, telemetry, updater, legacy-data, or automatic-install dependency |
| T18 | Shared Swift/TypeScript fixtures and an old installed native build exercise additive changes, unknown enum/action states, expired cursors, upgrade-required responses, and authentication errors; no decoder crash or unsafe action |
| T19 | For the exact signed distribution artifacts, install on a physical iPhone and a Mac, run real send/reply, owner approval, cancellation and restart/replay, inspect accessibility and screenshots, then upgrade and uninstall without undocumented services or duplicate Fort apps |
| T20 | Restart a multi-step routine after a completed task, before/after a gate decision, and with an unknown-effect task. Completed steps never rerun, gates never disappear, and successors remain blocked when required. Queue a later chat turn while waiting for approval or reconciliation; it cannot bypass the Conversation barrier. Instrument Runtime calls to prove that only eligible task steps invoke a model |

## 14. Platform and distribution gate

The implementation records supported iOS/macOS versions, CPU targets, exact Xcode/Swift and backend toolchains, official runtime versions, signing team, bundle identifiers, entitlements, and contract versions before the first device build. Pick the owner's actual iPhone and Mac as mandatory devices; additional OS/CPU support requires its own matrix. Simulator runs and unit tests are necessary but do not replace physical-device acceptance.

The default proposed private delivery is an iPhone TestFlight build and a signed/notarized native Mac app. Confirm the appropriate Apple account, distribution eligibility, and current [TestFlight requirements](https://developer.apple.com/testflight/) before using that route. This spec requests no new entitlement, provisioning, signing action, or upload. The Mac host service has a separate explicit install/lifecycle contract even if delivered alongside the app; distribution testing must verify which component owns execution and persisted data.

For release, inspect the exact installed builds through accessibility and screenshots, identify their artifact hashes, and perform T19 with real provider traffic. A visible unexplained error/alert, raw diagnostic error code, non-ready selected Agent in the healthy smoke path, duplicate Fort app bundle, disabled primary action after valid ready input, or failed smoke action blocks signoff. Deliberate offline/failure fixtures must show the expected clear recovery UI. Backend health, build success, signing, notarization, and upload receipts are insufficient alone.

Release evidence must separate source tests, fixture tests, live-provider results, and exact installed-product observations. Document errors in plain language with a safe diagnostic reference. Never put credentials, full environment dumps, or private prompt content into generic diagnostics. There are no analytics transmissions by default.

## 15. Rollback and remaining decisions

The donor quarantine remains the fallback during development; rollback never means unlocking its archived application. Each approved slice has a reversible migration or a verified pre-migration backup and an explicit compatible binary version. A failed deployment stops new dispatch, retains the ledger and evidence, and restores only a compatible service after reconciling in-flight effects. Client rollback preserves installation identity and pending request IDs. Never dual-write to a legacy ledger or run two authoritative copies to reduce downtime.

The defaults in this spec allow bounded planning without another preference interview. Necessary decisions before their dependent work are:

- Toby approves or amends the native iPhone/Mac MVP scope and the selective TypeScript recommendation. This does not require the owner to learn TypeScript.
- Stage 0 selects an enforceable isolation method, exact supported toolchains, data-root locations, and the owner's test-device/OS matrix. Record these implementation choices without expanding scope.
- Each provider/topology feasibility cell resolves subscription suitability. A failed required cell needs Toby's choice to change the advertised scope or authentication model; no silent substitution is permitted.
- Before integration/release, reconcile the applicable clean-baseline record and record the concrete self-host host, connectivity, distribution accounts, and cost. Use existing authorization where it applies; obtain a decision only for missing authority or new scope/cost. Preserve Spec 055's separate authority.

Watch, CarPlay, voice, Google, broader workflows, and high availability require later decisions. Their absence is an explicit MVP boundary, not a removal of historical product aspirations.
