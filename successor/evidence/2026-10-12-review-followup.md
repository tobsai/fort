# Review follow-up and native evidence — 2026-10-12

Branch `codex/fort-core-openmaus-20261011`; isolated worktree
`/tmp/fort-core-openmaus-20261011`. This follows the historical first-increment
record. The donor remains quarantined and unchanged at upstream
`8b53c0323dba2099a953ee2d77ab9330cd5acaa7`.

## Four reproduced review defects

Independent Astra review target: `f2a520660d17b84a743ab8de4a447aaeb573a663`.
Review source: `/tmp/fort-stage1-astra-w_km0rgn/REVIEW.md`.

| Defect | Fix and regression evidence |
| --- | --- |
| Revoked/expired credentials could finish delayed mutations | `d8085f2ed8f773635bb0b14ec617bcc6de7c54c8`: authority is checked within the SQLite mutation transaction after body arrival. Twelve delayed-upload cases cover revoke/expiry across submit, read, revoke, approval, cancel and close-unknown. |
| Native enrollment/authenticated requests followed redirects to disallowed origins | `86137fbd881cb842b24d762eea6325c8437489b9`: a task delegate refuses every redirect. Actual loopback tests for 301/302/303/307/308 verify zero forwarded requests to disallowed host `127.1`. No real TLS downgrade server was used; the policy refuses redirects regardless of destination scheme. |
| Cold offline launch hid history and pending work | `86137fb`: installation-scoped navigation, Agent/Home, snapshot and pending state load before network access. Model regression first failed, then passed. |
| Accepted message vanished locally if the next read failed | `86137fb`: validate and atomically save the acceptance snapshot before removing its pending request. Model regression verifies live state, on-disk cache and cold offline restart after immediate disconnection. |

A further model regression reproduced Send being unavailable during a pending
background refresh. Local enqueue is now independent of refresh; network submission
continues through the same immutable request. Pending conflicts no longer prevent
transcript reads. New local messages scroll into view.

Native UI/test and send-control increment:
`7365dc2140a452a913fd34e2cf4f636161eec04c`.

## Checks completed

- Node 26.5.0: **45/45** HTTP/SQLite/runner tests, including acceptance failpoints,
  real process-kill boundaries, approvals, decision races, restart fencing,
  cancellation and the twelve delayed-authority cases.
- Swift 6.4: **13/13** tests, including model recovery and send-during-refresh.
- Strict TypeScript: passed with TypeScript 6.0.3; no suppression or skipLibCheck.
- Native HTTP checks: exact text, two-client snapshots, stable retry, service/client
  restart, event-page retrieval and shared read state; separate approval checks
  prove parked restart, decision winner, continuation and cancellation.
  These use the actual Swift library/HTTP/SQLite path, not installed apps.
- Native redirect checks: 301/302/303/307/308 all pass for enrollment and authenticated
  reads with zero destination requests.
- Mac and iPhone Simulator build-for-testing targets compile. Simulator execution
  needs local ad-hoc signing for Keychain identity; this is now the explicit
  Simulator-only project default, with no development team; unsigned builds compiled but
  failed secure enrollment. No certificate, account or sign-in was added.
- Installed iPhone 18 Pro / iOS 27 fixture app: enrollment through accessibility,
  Keychain storage, send/reply and application restart **passed**. XCTest bundle:
  `/tmp/fort058-ios-journey-4.xcresult`. Screenshots were exported and inspected:
  [conversation](native-2026-10-12/iphone-conversation.png),
  [restart](native-2026-10-12/iphone-restarted-conversation.png).

The extended installed iPhone recovery journey **passed**, including cold offline
history/pending visibility, offline Send, loss of the acceptance response, graceful
service restart, app restart, and reconciliation of the original request. The harness
verified one dropped response, exactly three user messages with the expected text,
three replies and three completed runs. Result bundle:
`/tmp/fort058-ios-recovery-3.xcresult`. Inspected screenshots:
[offline pending](native-2026-10-12/iphone-offline-pending.png),
[recovered acknowledgement](native-2026-10-12/iphone-lost-ack-recovered.png).
This proves the injected outage/restart path; it is not physical network/TLS evidence.

The installed iPhone approval/cancellation journey also **passed**. The UI approved
one fixture reply and canceled another parked run; the canonical ledger contains
exactly two user requests, one reply and states `completed,canceled`. Result bundle:
`/tmp/fort058-ios-approvals-2.xcresult`. Its
[approval/cancel screenshot](native-2026-10-12/iphone-approval-cancel.png) was inspected.
The checked-in Simulator project defaults build successfully without CLI overrides.
The installed executable, debug dylib and Info.plist hashes match that build;
[exact hashes](native-2026-10-12/installed-build-hashes.json) identify the inspected
app. XCTest stopped the fixture app and Simulator after its runs.

The UI harness supplies synthetic one-use enrollment in a DEBUG-only unique test
storage directory, then presses the normal enrollment button. It does not bypass
Keychain or authentication. Its fault proxy and separately authenticated test-control
listener bind only ephemeral loopback ports; neither exists in the application or
production API. All fixture listeners and temporary configuration are closed/removed
when the harness exits. No provider credential was read or copied.

## Failed attempts and remaining gates

The Mac XCTest runner failed before connecting to tests. A local ad-hoc build passes
`codesign --verify --deep --strict`, but the system's `amfid` log reports an ad-hoc or
unknown certificate chain and `syspolicyd` explicitly reports **Gatekeeper rejection**.
This protection was left in place. The exact local artifact is
`/tmp/fort058-mac-build/Build/Products/Debug/FortMacUITests-Runner.app`.
A trusted/owner-approved launch or appropriately signed development build is needed.
No tool approval is pending; this is an OS launch blocker, not an auto-review denial.
CUA transport was unavailable and is no longer exposed in the current tool catalog.

Initial Simulator tests exposed missing Keychain identity in unsigned builds; local
ad-hoc signing resolved it. The extended recovery test then exposed Send being
suppressed during background refresh. An exact-input UI assertion also caught an unwanted keyboard prediction; the
plain-text composer now disables autocorrection. Compilation-only test harness mistakes were
corrected before successful runs; a stale-binary attempt was rejected by the harness
because its required dropped-acknowledgement phase had not run. Failed checks are not
counted as passes.

Still open: Mac installed UI acceptance; physical iPhone/TLS enrollment; installed
UI unknown-outcome closure and two-device races; full event-driven transcript rebuild,
pagination and compatibility; backup/restore fencing; hostile-runner OS isolation;
official-provider/subscription/topology cells; skills/routines/schedules/APNs and
release distribution. Fixture approval/cancel proofs do not prove official-runtime
controls. Node permission checks do not prove an OS security boundary. No deployment,
remote push, provider sign-in, startup service or public ingress was performed.
This remains a tested fixture foundation; Stage 1 and the whole MVP are not signed off.
