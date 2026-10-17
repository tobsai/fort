# Native routines and calendar schedules — October 17, 2026

Source `f6e4615c8ce962d4d90948613b6339381e3c533e` on
`codex/fort-openai-rebuild-20261016` adds local routine and schedule behavior to the
isolated Spec 058 successor. It follows complete reviewed text-bundle execution
in `ffeb3953c38d77a32a41280579263e0960795767`. This is synthetic/native-build
evidence, not installed-product, provider or complete-MVP acceptance.

## Routine contract

A saved immutable revision pins its Agent/Home/binding, instructions, typed inputs,
reviewed complete-bundle digests and at most sixteen ordered task or approval steps.
Tasks can read named inputs and explicitly selected earlier task outputs. Invalid
or forward/cyclic references, extra authority fields and arbitrary code steps fail
before saving. An owner command accepts Run now through the same message/run/event/
request transaction and durable dispatch queue as chat. An older enabled binding
can be deliberately pinned; later edits do not substitute current defaults.
Disabling a binding or selected skill revision blocks queued and future dispatch.

Each task attempt, output reference/digest and next-step eligibility commit together.
Restart retains completed tasks and pending gates. Only eligible tasks call the
runtime. An exact owner gate binds installation/epoch/run/attempt/binding/routine/
step/action digest/scope and expires after 24 hours. The first decision wins;
replaying an opposite decision returns that winner. Cancellation invalidates pending
continuation, while correlated completion can preserve the current task's output.
An unknown task holds successors and later Home work; explicit closure preserves
uncertainty and cancels remaining steps without retry.

Native forms create/edit routines, select reviewed bundles, declare typed inputs,
order tasks/gates and select earlier outputs. JSON import opens an inert draft;
export round-trips the definition. Run now stores an immutable pending command before
network I/O. Lost acknowledgement and app relaunch retain its request ID, epoch,
binding, routine revision and exact typed input. The UI renders checkpoints and
bounded continuation cards. Capability flags keep older services usable without
fetching unsupported routes. Native accessibility and visual inspection remain open.

## Calendar contract

Schedules pin a routine revision and typed input snapshot. Save/edit always produces
a paused revision. Preview shows timezone, next occurrences and policies; enabling
requires the exact unexpired preview and records authenticated owner consent.
Editing a schedule never carries enablement into its new revision. Pause prevents
future occurrences without canceling accepted work.

One-time, daily and weekly schedules use an explicit IANA timezone. Nonexistent
local times create skipped-gap records; repeated local times use their first
instant. Default downtime policy skips missed work. Optional catch-up admits at
most the latest occurrence, and one active run per Routine includes approval and
unknown-outcome barriers. Overlap creates a visible skipped record. Downtime longer
than a year has a visible skipped-range summary and bounded recent enumeration;
that long-range branch is implemented but not separately exercised in this record.

The routine revision plus intended instant identifies an occurrence globally. A
unique ledger entity and the ordinary Run now command commit in one transaction.
Duplicate schedules, competing in-process ticks and restart do not duplicate it.
SQLite remains one exclusive service writer; this is not a multi-writer deployment
test. The service polls only while explicitly running. No OS scheduler/startup
registration or external automation was created. Calendar tests use injected clocks;
no live-provider scheduled run occurred.

## Validation and artifacts

- **647 TypeScript tests passed**, zero failures/skips; strict typecheck exit 0.
- **28 Swift tests passed**, including routine pending replay after a lost response,
  stale-form revision handling, old-service capability compatibility and bounded
  approval scope recognition.
- The compiled Swift HTTP client completed **ten synthetic runtime executions and
  four service restarts**. It preserved receipt identity, canceled held work,
  explicitly closed uncertainty, recovered a discarded acceptance, preserved full
  bundle contents, resumed a task/gate/task routine, and previewed/enabled/paused
  a schedule with zero schedule executions. [Exact output](native-http.txt).
- New Mac and iPhone Simulator build-for-testing both succeeded. Host strict/deep
  signature verification passed for both apps and both test runners; 26 artifact
  files were hashed. [Build inventory](builds.json).
- All six consumed attempts, the native cache and main checkout remain unchanged.
  Checks used raw file hashes/stat and Git status, without opening historical
  databases or reading credential stores. [Preservation](preservation.json).

Red tests exposed the missing routine/schedule routes, unsupported multi-step
execution, revision-disabling routes, capability advertisement and native gate
recognition. Intermediate typechecking found a widened literal; the native checker
required Receipt equality before its new comparisons compiled. Both were corrected.
The final passed/failed/unrun ledger includes [log paths and hashes](checks.json).

Fresh artifacts:

- `/tmp/fort058-mvp-routines-mac-20261017/Build/Products/Debug/FortMac.app`
- `/tmp/fort058-mvp-routines-mac-20261017/Build/Products/FortMac_macosx27.0-arm64.xctestrun`
- `/tmp/fort058-mvp-routines-ios-20261017/Build/Products/Debug-iphonesimulator/FortiOS.app`
- `/tmp/fort058-mvp-routines-ios-20261017/Build/Products/FortiOS_iphonesimulator27.1-arm64-x86_64.xctestrun`

No app or Simulator was launched, no live enrollment or provider turn occurred,
and no credential, trust, startup, entitlement, remote, push, merge or deployment
change occurred. Existing signing settings were used without changing trust.

## Remaining gates

The consolidated [temporary test scope](../../ops/mvp-native-test-scope-20261017.json)
remains proposed and awaiting owner approval: three sessions, 600 seconds each with
60 seconds for cleanup, one fresh expiring Mac enrollment per session, twelve
synthetic text turns per session, existing ChatGPT/gpt-6-astra/Codex 0.157.1 and the
unchanged no-tool profile. Each session permits one explicit service/runtime restart
and at most three owned runtime launches, each at most 300 seconds within the shared
deadline. No consumed packet may restart. The existing UI operator is
`01a1451b-81d7-76cf-8e5c-a85d7164201c`, coordinated through the parent.

Current installed accessibility/screenshots, actual provider execution, physical
iPhone/shared history, Claude/provider/topology feasibility, lifecycle, encrypted
backup/restore, upgrade/uninstall and release remain open. Legacy instruction-only
authoring migration and broader bundle packaging/runtime support also remain open.
Binary/script/tool bundles are stored for review but refused by the current text-only
runtime before dispatch. The agent host retains the Mac user's access; the computer
permission profile is not agent-host confinement. No complete-MVP claim is made.
