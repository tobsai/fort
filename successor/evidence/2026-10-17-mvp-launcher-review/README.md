# Launcher review fixes — October 17, 2026

Corrected source: `2cbe35323b6aa3b3b5ad35e20ced8c71b7bf5ce8` on
`codex/fort-openai-rebuild-20261016`. Both findings in the independent review of
`a8a14c8` are fixed with focused regressions. The candidate is resealed across 111
product source files; the original proposal and absent owner-approval record are
unchanged. This is prepared source with bounded validation, not installed acceptance.

## Changes and observed behavior

The actual HTTP service tracks accepted sockets, stops listening and destroys and
joins those connections during shutdown. It does this even when the external
listener signal was already aborted or close races listener startup. New work is
blocked and bounded credential revocation retains its prior ordering. Runtime and
HTTP cleanup both finish before database close; a cleanup failure blocks restart.

Six no-enrollment service regressions cover idle, incomplete-header and incomplete-
body peers during normal and aborted-listener shutdown, plus never-started,
already-aborted, abort-while-starting and close-while-starting cases. They assert
peer closure before any client-side destruction, no remaining listener and zero
enrollment/device rows. They use only disposable ledgers and local sockets; the
incomplete requests never call an enrollment endpoint.

The production packet admission policy is now a separately testable pure function.
It recomputes the full canonical bundle digest and permits only Check marker's
single-file CHECKED version 1 and REVIEWED version 2 bundles. Exact entry bytes,
source, license, runtime list, file set, version and absent tool/script authority
are bound by that digest; enabled review and generated identifiers are checked.
Sixteen pure admission/context tests accept both editor bundles and reject extra
files, changed metadata/entry bytes, arbitrary identifiers and invalid reviews.
The extra-file regression also proves the general context builder still supports
reviewed auxiliary files; only this temporary packet's policy excludes them.

The existing launcher stalled-body test now asserts actual peer closure instead
of merely a fast finish promise. That enrollment-fixture test was edited and
typechecked but deliberately **not executed** in this pass.

## Exact validation

- **68 non-enrollment TypeScript tests passed**, zero failed/skipped: 44 adapter,
  two preparation, 16 admission and six actual-service socket tests.
- Strict TypeScript checking passed. Test-only narrowing/union errors found during
  implementation were corrected before the final passing check.
- Three preparation-checker tests passed. Both read-only entrypoints passed and
  reported activation blocked by pending owner approval.
- The socket and admission regressions failed before their fixes. The old socket
  listener kept the red run alive; that exact test process was interrupted. The
  partial red log is retained and is not counted as a completed suite.
- The full suite, launcher enrollment fixtures, native HTTP journey, Swift tests,
  native builds and installed/provider acceptance were **not rerun**. The earlier
  689-pass result belongs to `a8a14c8`; it is not a claim for this source revision.

From `successor`, the safe selected suite is:

```sh
/opt/homebrew/opt/node@26/bin/node --test runner/codex-chat.test.ts prototype/mvp-preparation.test.ts prototype/mvp-admission.test.ts service/openai-shutdown.test.ts
/opt/homebrew/opt/node@26/bin/node node_modules/typescript/bin/tsc -p tsconfig.json
```

`test:non-enrollment` records the same test selection. The socket tests require
local-listener permission on this host. The full `test` and `test:prototype`
scripts include enrollment fixtures and are outside this pass's execution scope.
Commands, counts, source identities and log hashes are in [checks.json](checks.json).

## Seal, preservation and remaining gates

The former seal was copied byte-for-byte to [previous-pins.json](previous-pins.json)
before the existing exclusive-create sealer generated the corrected
[pins.json](../../ops/mvp-native-acceptance-20261017/pins.json). Native source is
unchanged at `996e19849f5104e71b26b7c14d201aaa56b31c87`. All four existing app/runner
artifact file sets and hashes match prior evidence and all four strict signatures
reverify. These are reused artifacts, not fresh builds or fresh native test results.

[Preservation evidence](preservation-and-builds.json) confirms unchanged main
checkout status, all six historical packet files by bytes/size/mtime, and the
native cache's exact file set/bytes/mtime. The proposed batch root and owner-approval
file remain absent. Preexisting untracked `.build/` is retained. No real enrollment,
native app, official provider, Keychain access, cache archive, package install,
startup service, provider cleanup, push, merge or deployment occurred.

The corrected source has not had a new independent review. The next installed
Mac checklist remains in the [operator plan](../../ops/mvp-native-acceptance-20261017/PLAN.md),
under the existing pending owner decision and unchanged three-session limits.
Accessibility/screenshots and correlated real-provider receipts remain unrun.
The agent host has the Mac user's access; the computer-tool profile does not prove
host isolation. Broader MVP and physical-iPhone gates remain as previously recorded.

The reviewer reported that automatic approval review rejected the full launcher-
suite rerun because its temporary synthetic enrollment credentials conflicted with
the explicit no-enrollment instruction. That rejection was respected. Rerunning
those fixtures requires explicit authorization for synthetic enrollment; the
separate no-enrollment checks above do not bypass or satisfy that missing authority.
