# Memory and independent-review fixes — October 17, 2026

Source `996e19849f5104e71b26b7c14d201aaa56b31c87` on
`codex/fort-openai-rebuild-20261016`. This is local source, synthetic service/adapter
validation and fresh native-build evidence. No installed GUI, Simulator UI or live
provider was started. The pending three-session enrollment proposal is unchanged.

## Independent review findings

The read-only review of `a4a51a5ba0fcac30f851d223cfc631ee1bd1d6c5` reported three
reproduced/source-inspected defects. Original report:
`/tmp/fort-a4a51a5-review.cC2tOH/REVIEW.md`.

- **P1, partial scheduled acceptance:** composed Agent/routine configuration is
  validated before writes, and each acceptance has a savepoint. A scheduler may
  record its blocked occurrence without committing partial messages, runs, events
  or receipts. The regression uses two individually valid 9,000-byte instruction
  fields; manual and scheduled rejection leave the snapshot/cursor unchanged and
  readable after restart. A later valid occurrence is accepted once. Native pending
  treats the actionable `invalidConfiguration` response as requiring review.
- **P2, closed runtime:** the actual Codex adapter reports readiness and accepts an
  explicit replacement only within an injected connection count and absolute deadline,
  after confirmed old-connection cleanup. Setup cancellation, unavailable exact model
  and unknown-turn fixtures all recover for a new request without replaying the old
  one. Failed cleanup and expired budgets launch nothing. Authenticated service routes
  and native status/restart controls expose that operation. A real adapter plus owned
  synthetic child processes proves queued continuation. Historical launchers remain
  unchanged and do not supply this replacement factory; live activation needs the new
  bounded controller described below. This is not current installed recovery evidence.
- **P2, historical skill authority:** a routine opens its exact pinned revision with
  a fresh review state, and bundle review traverses prior bundle revisions. Disabling
  historical A leaves current B enabled. Native model and actual-service regressions
  prove A cannot dispatch queued or newly invoked routine work after revocation.

## Conversation memory

Native Memory edits owner-written plain Unicode notes up to 8,192 UTF-8 bytes in one
exact conversation. Revision compare-and-swap rejects stale edits. Each accepted
chat pins its note revision and contents; pending replay preserves that pin across
lost acknowledgement/relaunch, and the native run displays the memory used.
Clearing affects future turns; it does not erase historical configuration or transcript.

Homes remain isolated. Existing manually addressed shared conversations use their
own note for their selected participants; no automatic delegation was added. Notes
reach Codex as scoped user context, not developer instructions. Provider memory tools
remain disabled. Routines continue using their saved plan rather than implicitly
reading later chat notes. No automatic memory extraction or host access was added.

## Passed validation

- Full synthetic TypeScript suite: **665 passed, zero failed/skipped**.
- Strict TypeScript compilation: **exit 0**.
- Swift package suite: **36 passed**, including memory stale edits/clearing/replay,
  explicit runtime recovery, and historical revocation.
- Real Swift HTTP client against Fort's temporary local service: **10 synthetic
  runtime calls, six service restarts, two discarded acceptance responses and one
  discarded cancellation response**. No duplicate/canceled dispatch, final pending
  queue empty. Schedule preview/enable/pause made no runtime calls. This existing
  HTTP journey was not extended to the new memory/restart controls; those have the
  separate native-model and service/adapter regressions above.
- Fresh signed Mac and iPhone Simulator **build-for-testing passed**. Four app/runner
  signatures passed deep/strict verification; **26 files hashed, two test plans**.
  Exact artifacts and signatures are in [builds.json](builds.json).
- Read-only preparation checker: **three unit tests passed**; actual source, artifact,
  runtime-byte, preservation and absent-session-root checks passed. It reports
  activation blocked and performs no live actions.
- Six historical packets retain size/hash/modification time; native cache retains
  exact file set and bytes. Main checkout status matches its recorded baseline.
  Preexisting untracked `.build/` remains untouched.

Red regressions are retained for atomic admission, adapter continuation, service
restart routes, native memory and historical review. The initial sandboxed Swift
manifest invocation could not use compiler caches; the approved host invocation
passed. One earlier routine fixture used a future time and was corrected. No final
check remains failing. Persistent log paths and hashes are in [checks.json](checks.json).

## Acceptance preparation and open work

[Executable read-only check and exact session steps](../../ops/mvp-native-acceptance-20261017/PLAN.md)
cover same-Home memory/history/relaunch, bounded restart, complete text bundles,
historical revocation, manual shared recipients and cancellation. The plan preserves
all proposal limits and does not add provider routine/schedule execution.

**The live batch is not launch-ready:** owner approval is pending, and a separately
sealed three-session controller still needs implementation and synthetic lifecycle
validation. Do not reuse `prototype/manual.ts`, consumed sessions, old credentials,
or old artifact pins. No enrollment, token, provider sign-in or process was created
by this milestone. The instruction-skill migration is already committed at
`a524c4a6fbe4d53e9f8e514a5e89a71f8cda8968` and included in the current build.

Current installed accessibility/screenshots, real-provider acceptance, physical
iPhone, reachable authenticated endpoint, cross-device history/read state, real
sleep/reboot/schedules, notifications, backup/restore, complete host isolation and
distribution remain unrun. The agent host still has the Mac user's access; computer
tool restrictions do not prove host confinement. This does not complete the MVP.
