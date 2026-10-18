# Native phone setup controls — October 18, 2026

[Spec 059](../../../specs/059-modular-node-v1.md), based on
`b0caff657c328b345c29d18456ef60f6948c6aab`, under unchanged-scope
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
This is native source, build and synthetic UI evidence. Actual phone, provider and
TestFlight acceptance remain open.

Ordinary iPhone setup now uses the existing companion coordinator. The user enters
an exact HTTPS host, installation, invitation, code and phone name. Only Pair sends
the join. Pending restore reads the protected journal without a network request,
new credential or retry. The entered code is cleared after an attempt and is not
persisted. Recover checks the prepared credential; retry requires the same code.
Pending cancellation and separately confirmed local forgetting remain distinct.
A stale confirmation cannot select a changed preparation. Unconfirmed forgetting
blocks another pairing. Confirmed cancellation permits it.

Publishing a verified connection clears the earlier draft and in-memory selection;
durable history stays intact. A late result cannot replace another connection.
Expired-phone disconnect is visible independently of ordinary status access and
retains phone-only authority. Expiry edits require a verified connected status;
unconfirmed access no longer shows an expiry editor or a misleading pending-chat
message. Mac primary setup is unchanged. The older enrollment journey is retained
only through an explicit DEBUG initializer in a UITests/UUID directory.

## Passed checks

- Final full native suite: **307 functions passed, 14 skipped, zero failed**;
  Swift Testing reports 321 total including skips, 52.925 seconds. Command:
  `swift test --no-parallel --package-path successor/apple/FortKit`, with
  FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node,
  FORT_TEST_GUARDIAN=/tmp/fort059-assembly-guardian,
  FORT_TEST_CORE_ARTIFACT=/tmp/fort059-owner-account-host-resources/core and
  FORT_TEST_HOST_RESOURCES=/tmp/fort059-owner-account-host-resources.
  Log: `/tmp/fort059-phone-controls-reviewed-full.log`.
- Phone/model/legacy-construction focus: seven passed, one benchmark skipped.
  After the observed expired-state correction, 14 affected phone/owner lifecycle
  functions passed. The full suite above ran after that correction.
- Final isolated benchmark: 20 prepare/lost-reply/reopen/recover samples after five
  warmups, **p50 23.772 ms, p95 25.541 ms, maximum 26.238 ms**, under 100 ms.
  Synthetic in-process transport and memory credentials; real network, Keychain and
  human time excluded. No QA process or build overlapped samples. Existing hardware
  inventory: MacBookPro18,2, 10 CPUs, 64 GiB; macOS 27.2, Xcode 27.1, Swift 6.4.
  `/tmp/fort059-phone-controls-reviewed-samples.json` and `-reviewed-budget.log`.
- Fresh QA, unsigned Mac and unsigned iPhone Simulator builds passed, then each
  rebuilt successfully after the expired-state correction. Final logs:
  `/tmp/fort059-phone-controls-{qa,mac,ios}-final-build.log`.
- FortKit Release compilation passed after the final correction; DEBUG-only QA and
  legacy test construction are excluded by compile guards.
  `/tmp/fort059-phone-controls-reviewed-release.log`.
- Final QA strict/deep signature verification passed. Its only entitlements are
  App Sandbox and debug get-task-allow. No outgoing network or App Group entitlement.
- Exact synthetic QA window accessibility and all four screenshots were inspected:
  [prepared](phone-controls-prepared.png), [cold reopen](phone-controls-reopened.png),
  [recovered](phone-controls-paired.png), [expired](phone-controls-expired.png).
  The expired disconnect confirmation names only this phone and retains host work
  and history. The confirmation was not submitted.

Observed red cases included the missing model/factory interfaces, placeholder model
actions, cancellation availability after confirmed removal, missing explicit DEBUG
test construction, and the expired access editor/message. Those tests passed after
implementation. Earlier passing full runs (320 and 321 reported) are superseded by
the final run above. No test threshold was widened or test removed.

The 14 full-suite skips are 12 isolated performance checks plus review-package and
retained-signed-probe checks. This slice's performance ran separately. Portable
service checks were not repeated for these Swift-only changes; their latest result
belongs to a522d2a. Real account, credential, LAN, TLS/trust and physical-device checks
were not run. CoreSimulator 1171.7.0 remains below Xcode's required 1174.9.2, so iPhone
Simulator compilation passed but execution remains blocked; no system change occurred.

## Exact artifacts and UI session

QA: `/tmp/fort059-phone-controls-qa/Build/Products/Debug/FortQA.app`,
identifier `io.fort.qa.phone.20261018`, flags `--qa-phone`, optionally
`--qa-phone-expired`. Final session: `f73f0402-7301-4600-acad-f382a49a1e36`.
Its DEBUG factory is confined to a named temporary directory, uses synthetic
constants and an in-process responder, and never uses Keychain or a network transport.
It reconstructs the prepared synthetic identity on cold reopen without a new join.

Final screenshot process/window pairs: prepared 20485/2556; reopened and recovered
20610/2571; expired 20948/2590. Inspectors checked the exact bundle path and ID and
only read the matching window, excluding menus and other applications. Cold `open -n`
started the process with zero windows; a normal reopen event to the same exact bundle
showed its window. This is recorded behavior, not ordinary installed-launch acceptance.
The alert's AX Cancel press returned -25205; it was not reported as a successful
cancel. Normal termination then succeeded. Final exact QA process count was zero.

Final file inventories in `/tmp/fort059-phone-controls-artifacts.json` bind sorted
relative paths, modes, lengths and SHA256 values. Inventory SHA256 values use compact,
sorted-key JSON serialization of each file list:

| Artifact | Files | Inventory SHA256 |
| --- | ---: | --- |
| QA | 12 | ad08ca60c643179cdbbedfa8d79f7fd62ca49bc908ae7222c47ebf0191a50ae4 |
| Mac | 7 | a71a8d0c6654dfaf8f1a7200f94dd30707ad23e41e3a32b30e93810ead4f2a34 |
| iPhone Simulator | 7 | e1a1d030244155db404a8df126cfcf8ee0871d42aba9d222cfdbf53d192f836a |

Mac path: `/tmp/fort059-phone-controls-mac/Build/Products/Debug/FortMac.app`.
iPhone path: `/tmp/fort059-phone-controls-ios/Build/Products/Debug-iphonesimulator/FortiOS.app`.
Ordinary apps were not launched. The prior 200-file inert Mac review package remains
at its account-controls revision; these new builds do not silently replace its evidence.

## Remaining gates

Ordinary Mac invitation and secure-listener lifecycle composition is the next source
slice. Actual HTTPS identity/trust, local-network access, protected pairing credentials,
background startup, installed Mac/phone/provider use and TestFlight remain separately
unverified. No real credential, sign-in, certificate, trust, provider, startup, listener,
push, deployment or cleanup action occurred. The pending Developer ID replacement
approval remains unconsumed. Agent-host and computer-tool sandbox boundaries remain
separate; this phone UI does not add computer-tool authority.
