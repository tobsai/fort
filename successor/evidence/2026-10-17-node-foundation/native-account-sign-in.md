# Native primary-owner account controls — October 18, 2026

[Spec 059](../../../specs/059-modular-node-v1.md), based on
`a522d2a266efcd21366b82313fa2f6a5895e5c19`. Ordinary implementation authority is
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
This is native source and synthetic UI evidence, not actual provider sign-in.

The primary owner-local Mac can explicitly start, inspect, cancel or retry one exact
sign-in request. A protected intent is written before POST and stores only the
connection, epoch, operation ID, revision and absolute deadline. Restore only reads
status. Lost replies preserve uncertainty; retry retains the same deadline. Expired
unaccepted requests need explicit discard after the cleanup reserve and a fresh
status. Codes/URLs/account credentials are not persisted. The exact runtime name is
shown. Verification links open only on a user click. Companions cannot manage accounts.

Review regressions observed red and fixed: additional saved-intent fields were
silently accepted; confirmed disconnect left its account intent; an older confirmed
removal receipt blocked a later pairing; a delayed challenge could return after Stop.
Strict decoding, exact retirement, historical-receipt checks and response-generation
invalidation correct those cases. Unconfirmed/foreign removal records remain blocking.
Starting/canceling checks its generation again before POST; all account replies check
it after awaiting. Stop/removal invalidate earlier replies and hide their codes.

## Verification

- Final full native run: **284 functions passed, 12 optional checks skipped, zero
  failed**. Swift Testing reports 296 total including the skipped functions.
  Command: `swift test --no-parallel --package-path successor/apple/FortKit` with
  FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node,
  FORT_TEST_GUARDIAN=/tmp/fort059-assembly-guardian,
  FORT_TEST_CORE_ARTIFACT=/tmp/fort059-owner-account-host-resources/core and
  FORT_TEST_HOST_RESOURCES=/tmp/fort059-owner-account-host-resources.
  Log: `/tmp/fort059-native-account-final-suite.log`.
- Account focus: 13 ordinary functions passed, one benchmark opt-in skipped.
  Two public offline-factory tests passed again after the final QA navigation wrapper.
  Native start/lost-response/cancel/relaunch, exact receipt/deadline, companion refusal,
  changed pairing/intent, expired code, symlink/hardlink, removal and decoder cases ran.
- Isolated start/cancel measurement: 20 serial samples after five warmups, **p50
  7.292 ms, p95 8.005 ms, maximum 8.372 ms**, budget 100 ms. Real provider/network/user
  latency excluded. No build/test work overlapped the sample. MacBookPro18,2,
  macOS 27.2, Xcode 27.1 / Swift 6.4; existing hardware inventory 10 CPUs / 64 GiB.
  Samples: `/tmp/fort059-native-account-samples-final.json`.
- Initial ordinary parallel Swift Testing execution had two timing-only failures:
  removal projection and bundled host journey. Their isolated reruns passed at
  0.348 ms p95 under 50 ms and 1.717 seconds under five seconds, respectively.
  Explicit `--no-parallel` full runs passed; no threshold was widened or test removed.
- Initial fixture missing-interface/stub runs failed; implemented public factory tests
  passed. A request-method enum compile error was corrected before the passing run.
- Fresh QA, unsigned Mac, unsigned iPhone Simulator and unsigned helper Debug builds
  passed. FortKit Release compilation also passed with the DEBUG-only fixture excluded
  by its compile guards (`/tmp/fort059-native-account-release.log`). Simulator execution remains unrun: CoreSimulator 1171.7.0 is older than
  Xcode's required 1174.9.2. No system update was attempted.
- Exact new review-package inspection passed: ordinary inspection rejects its unsigned
  signature; synthetic signature injection reaches the absent-profile guard. This is
  refusal/layout proof only. `/tmp/fort059-account-package-refusal.log`.

Full-suite optional skips: the account/companion and eight other isolated performance
checks, review-package inspection and retained-signed-probe signature check. Account
performance and exact review-package inspection ran separately here. Prior portable
service results belong to a522d2a; this Swift-only slice did not rerun the portable suite.

## Exact running QA

`/tmp/fort059-account-qa-20261018/Build/Products/Debug/FortQA.app`, identifier
`io.fort.qa.account.20261018`, explicit `--qa-account`, session
`7f73c86b-d1f6-44fb-9cef-4230bd60f3da`. The dedicated DEBUG factory uses the ordinary
model/PairingSettings view, an in-process responder, temporary storage and an in-memory
synthetic token; no Keychain, network, provider process or startup registration.
On cold reopen it derives an unconfirmed synthetic receipt from the saved intent;
it never initiates a replacement sign-in.

Strict signature verification passed for the final artifact. Entitlements are App
Sandbox and debug get-task-allow only, with no App Group or outgoing-network entitlement.
The first build inherited disabled signing and failed verification. Its launch command
mistakenly continued after that failure; the exact synthetic app was normally stopped
without using account controls, then rebuilt with CODE_SIGNING_ALLOWED=YES. Subsequent
verification and launch were separate. No signing identity/certificate was changed.

Accessibility verified the actual native window title `Pairing and local host`, exact
bundle path/PID and the visible label `Synthetic account fixture · no provider`.
Initial title-only selectors refused because child navigation overrides WindowGroup's
requested title; the inspector was corrected to the observed title and retained the
exact bundle identity. An early generic app dump included menu labels; those incidental
lines were removed and are not included in retained evidence. Final inspection was
restricted to the exact QA window.

Actual AX presses exercised idle → synthetic challenge → canceled, then a second
explicit start followed by normal termination and cold reopen → unconfirmed. The code
was visible only during the challenge. The verification link was never followed.
Screenshots were captured by verified window IDs 2475/2496 and visually inspected:
[idle](account-controls-idle.png), [challenge](account-controls-challenge.png),
[canceled](account-controls-canceled.png), [reopened](account-controls-reopened.png).
Final normal termination confirmed **zero** processes with this QA identifier.
No real pairing, stop, removal, history deletion or provider control was activated.

## Exact build inventories

`/tmp/fort059-account-artifacts.json` records paths, modes, sizes and SHA256 values.
Hashes below cover sorted-key compact file inventories plus newline.

| Artifact | Files | Inventory SHA256 |
| --- | --- | --- |
| qa | 12 | `eda65bb3d15cc35754b72c4c8801cdd5140242653990baf45621e6468cd1b4f2` |
| mac | 7 | `0b49b9a031445a250f8b580d37f3fcc6573677f79143667bf821bcc839e4ff2e` |
| ios | 7 | `055ae4e76e1d8e2824b2250f838d6e5c9d59b7e9b231585370732e4589f7f560` |
| helper | 7 | `ce82de7a824253ed459c63bb9015fd42d6d1f3eae0ac277a999d656aa728698c` |

Paths use `/tmp/fort059-account-{qa,mac,ios,helper}-20261018/Build/Products/`;
Debug for Mac variants, Debug-iphonesimulator for iPhone. Ordinary bundles were not
launched. The current composed review bundle and every input/output are recorded in
[Mac package evidence](mac-review-package.md#native-account-checkpoint).

## Remaining gates

Real provider sign-in/send/replay/stop, exact signed installed lifecycle, Apple records
and profiles, helper networking/startup authority, physical iPhone and TestFlight
archive/upload/processing/tester availability remain unverified. The parent is handling
specific G2 Developer ID Application replacement approval; no certificate action was
performed. Existing ASC sign-in remains pending. No old demo authority was reused.
The agent host and computer-tool sandbox remain separate security boundaries.
