# Native chat recovery milestone — October 17, 2026

Implementation source: `7c3287298bb0ee52c2eab8054465b9fdb9119b5f`, isolated branch
`codex/fort-openai-rebuild-20261016`, worktree
`/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.
This is source, synthetic behavior and build evidence. No app or live provider was
launched for this milestone. It does not replace the historical one-turn Mac
[provider acceptance](../2026-10-16-fourth-native-acceptance/README.md).

## Behavior proved

The actual OpenAI service exposes authenticated cancellation and explicit unknown
closure. Queued work, including work held after restart, can be canceled without
runtime dispatch. Active cancellation requests an interrupt and remains pending
until terminal evidence: confirmed interruption becomes canceled, a competing
completion stays completed, and missing evidence stays unknown. Later same-Home
work cannot cross the unresolved barrier. Closing uncertainty requires explicit
acknowledgment and the current recovery epoch; it neither retries the earlier turn
nor erases its receipt/history. Repeated controls are idempotent. Expired owner
authorization fails. Provider refusal before dispatch reports failure rather than
claiming the owner canceled it.

FortKit exposes these controls in the OpenAI mode. Cancellation waits for current
compatibility after a concurrent refresh. Unknown states have no actionable control.
The native Swift HTTP check exercised eight synthetic executions and three service
restarts, including a held queued cancellation, repeated uncertainty closure, one
committed acceptance replaced by a synthetic gateway error, exact pending/receipt
reconciliation, and continuation with the same Home's saved context. It verified
that the canceled queued turn never executed and the lost acceptance did not execute
twice. This is real HTTP transport with synthetic runtime responses, not a provider
or installed UI test. [Exact output](native-http.txt).

## Checks

| Check | Result |
| --- | --- |
| Complete existing TypeScript suite | 624 passed, 0 failed/skipped |
| Focused OpenAI service/CodexChat subset | 48 passed, included in the 624 |
| FortKit Swift tests | 25 passed, 0 failed |
| Strict TypeScript | Exit 0 |
| Expanded FortNativeCheck HTTP journey | Passed all five phases and final assertions |
| Mac build-for-testing | TEST BUILD SUCCEEDED; existing Apple Development identity/team |
| iPhone Simulator build-for-testing | TEST BUILD SUCCEEDED; existing local ad-hoc signing |
| Four app/test-runner signatures | Strict/deep verification passed using host trust service |
| Diff whitespace | Passed |
| Six prior attempt ledgers/state | Every recorded size/hash and recorded modification time preserved |
| Main checkout | Status matches the recorded pre-reconstruction baseline |

[checks.json](checks.json) records exact log paths and hashes, including deliberate
red tests. Queued cancel, active cancel, unknown closure, provider refusal,
correlated interruption and cancellation during native refresh each failed before
its corresponding implementation. The expanded native test initially failed to
compile because Receipt does not conform to Equatable; its assertion now compares
receipt identity fields and the build and behavior check pass. This compile error
is not counted as a behavioral red test.

The first signature verification inside the restricted sandbox returned
`CSSMERR_TP_NOT_TRUSTED`. The same artifact passed read-only verification using the
host trust service. No trust, certificate, signing setting or entitlement changed.
[builds.json](builds.json) records that limitation and all app file hashes,
signature metadata and entitlements.

## Fresh built artifacts

- Mac app: `/tmp/fort058-mvp-recovery-mac-20261017/Build/Products/Debug/FortMac.app`
- Mac test plan: `/tmp/fort058-mvp-recovery-mac-20261017/Build/Products/FortMac_macosx27.0-arm64.xctestrun`
- iPhone Simulator app: `/tmp/fort058-mvp-recovery-ios-20261017/Build/Products/Debug-iphonesimulator/FortiOS.app`
- iPhone test plan: `/tmp/fort058-mvp-recovery-ios-20261017/Build/Products/FortiOS_iphonesimulator27.1-arm64-x86_64.xctestrun`
- Synthetic native HTTP executable: `/tmp/fort058-mvp-swift/out/Products/Debug/FortNativeCheck`

These are new output directories; earlier accepted products are retained. The
project has no custom shell-script build phases, remote Swift dependency or plugin.
The Mac uses the same previously authorized development identity and outgoing-network
sandbox entitlement. Compilation/signing are not installed-product signoff.

## Consumed owner-trial reconciliation

The earlier owner-trial replacement record described preparation at its own commit.
Its later observed session state is now `finished`, one start, zero receipts,
credential cleanup `deleted`. The coordinator reports it opened at 22:16 UTC and
completed cleanup at 22:24:59 UTC on October 16; this record independently hashes
the resulting files and observes state, but does not independently measure those
times. [owner-trial-outcome.json](owner-trial-outcome.json) retains that distinction.
All six attempts and the current native cache remain untouched; the first five
match their previous manifests, and the sixth matches the newly observed outcome.
[preservation.json](preservation.json) contains the byte-only checks. No stored
message body, credential, provider setting or Keychain value was inspected.

## Unrun and remaining gates

Current-artifact installed Mac/iPhone UI, accessibility/screenshots, actual-provider
multi-turn/relaunch/restart/cancellation, physical iPhone, reachable authenticated
endpoint and complete Spec 058 acceptance remain unrun. Real runtime execution,
new device enrollment, provider sign-in, API billing, persistent service setup,
security changes, deployment, push and merge were not performed here.

The next live session requires the fresh temporary enrollment scope proposed in
[MVP-20261017.md](../../MVP-20261017.md). The existing operator remains the installed
UI owner through the parent. Complete skills, native routines/schedules, provider
and topology feasibility, OS isolation, lifecycle, backup/export and distribution
remain required work. Same-user agent-host access remains an explicit limitation;
the computer-tool profile is not proof of agent-host confinement.
