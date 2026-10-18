# Durable Mac host removal progress

Spec: [059](../../../specs/059-modular-node-v1.md); base checkpoint
`8aad246dacf8d415fa8420eb3da5f0c795788d5d`. This implements the local stop/revoke/
unregister/absence operation under decision 059. It is not an installed removal
adapter, credential-cleanup action or v1 completion claim.

## Behavior and ownership

`MacHostRemoval.stopAndRevoke()` binds to one named-agent endpoint, installation,
device and existing private client directory. Construction does not read credentials,
connect to a host or call ServiceManagement. Its production composition uses the
exact pinned XPC transport and `MacHostStartup`; tests inject the startup/recovery
boundaries. No actual system backend ran in this slice.

The operation acquires the existing owner-file lease and refuses pending setup,
missing/foreign connection metadata, changed records or unsafe storage. It durably
writes `host-removal.json` in three phases:

1. `requested`, before an owner-recovery request.
2. `recovered`, containing the exact validated receipt and OS process observation,
   before unregistration.
3. `confirmed`, only after startup removal, original helper exit, a fresh startup
   status check and unchanged connection/progress checks.

The file holds version, endpoint/installation/device identity, receipt and process
PID/original user/start timestamp. It contains no token or provider credentials.
Writes reuse the 16-KiB bounded descriptor-relative temporary-file/F_FULLFSYNC/rename/
directory-fsync implementation. The lease is released explicitly, including failure.
The shared owner-file wrapper now rejects extended ACLs on the directory, lease file
and records. Directory and lease metadata are rechecked during the operation; a
changed ACL or removed/replaced lease cannot silently preserve authority. Nothing
repairs permissions, strips ACLs or deletes an unknown file.

An explicit retry contacts the exact current peer again if the agent remains
registered. If unregistration already finished and a recovery receipt was saved,
retry restores the process observation without XPC activation. A confirmed record
never supplies authority to unregister a newly enabled host: unexpected startup
state fails, including a change between status reads. OS inspection still accepts
only absent PID or different start timestamp as original-process absence. Errors
remain unconfirmed; the original recorded user must match the current user.

Cancellation waits for an already-dispatched startup callback through the existing
wrapper, then leaves the recovered record for retry. Timeout, failed recovery,
failed unregistration, malformed proof or changed files cannot return successful
removal. Concurrent operations and new owner setup are refused under the shared
lease/removal marker. A saved locally-forgotten connection may finish recovery, but
local forgetting alone never supplies a revocation receipt.

The operation returns the three `OwnedHostStop` assertions. It does not delete the
credential, history, app or record. Its process-record codec is internal; the public
process observation remains non-Decodable and has no public initializer. Private
owner metadata is not a cryptographic attestation against a hostile process with
the same user's access. New-installation reconciliation, app-level composition and
explicit history deletion remain separate work.

## Verification

Environment unchanged: MacBookPro18,2/M1 Max, 64 GiB, 10 CPU; macOS 27.2 (26B5112a),
Xcode 27.1 (27A9275), Swift 6.4, Node 26.5.0, TypeScript 6.0.3.

Passed:

- Eleven removal behavior functions cover restart after unregistration, durable
  ordering before recovery/unregistration, cancellation and competing callers,
  unavailable registration without invented revocation, failed/foreign/partial
  recovery, changed metadata, initial and mid-operation ACL changes, corrupt records,
  failed unregistration with refreshed recovery, and locally-forgotten/foreign
  connections. Corrupt-record cases include version/phase mismatch, inappropriate
  proof fields, foreign connection/user, missing process evidence, invalid PID/time,
  incomplete core-stop receipt and nonprivate file mode.
- One new process-observation restore test covers encoded/reopened identity before
  and after actual synthetic-child exit. One owner-setup test proves a removal
  marker blocks provider/setup submission and credential generation.
- Full native suite reports **191 passing test functions**, with four isolated
  timing functions deliberately excluded. The new timing function passed separately.
  The unchanged storage/startup/process-inspection timing records remain distinct.
- Signed embedded-XPC scenario passed with zero skips in 39.822 s including build,
  signing and the existing crash/restart journey. It encoded/restored the process
  record from the authenticated peer, matched its PID and confirmed exit within the
  existing three-second bound. Signed-peer mismatch, fabricated reply PID, malformed
  receipt, setup/cancel/restart, sandbox negative controls and exact process/group/
  container cleanup assertions still passed with no forced cleanup. Signing identity
  `58F40D1AB480565B89109B853235AC3A7B2E7AD1`, team `T3JB5MYZ93` and all entitlements
  were unchanged. This checks the persisted identity primitive; the complete removal
  coordinator still uses a fake startup backend. The successful temporary artifact
  was cleaned, not installed or retained as a release build.
- Fresh unsigned Debug Mac and iPhone Simulator builds passed. Artifacts:
  `/tmp/fort059-host-removal-mac-build/Build/Products/Debug/FortMac.app` and
  `/tmp/fort059-host-removal-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.
  Logs: `/tmp/fort059-host-removal-mac-build.log` and
  `/tmp/fort059-host-removal-ios-build.log`. Neither was launched or installed.
  CoreSimulator 1171.7.0 remains below Xcode's required 1174.9.2.
- Strict TypeScript checking and full/accepted architecture gates passed. Exact
  staged-path coverage and whitespace checks run before commit.

The native tests use temporary owner-only files, exact `/bin/cat` children with
empty environments and fake startup/recovery backends. Children exit when their
owned input pipe closes and are reaped; no actual provider, Keychain, Mach agent,
App Group lookup, SMAppService registration or unregistration is invoked.

Commands/logs:

- Targeted behavior: `swift test --package-path successor/apple/FortKit --scratch-path /tmp/fort059-owner-setup-xpc-native --filter 'hostRemoval|hostProcessRestores|ownerSetupCannotStartWhileAnOwnedHostRemovalRecordExists' --skip hostRemovalConfirmedRecordRetryBudget`
  — `/tmp/fort059-host-removal-behavior.log`; the final two retry cases also passed
  in `/tmp/fort059-host-removal-retries.log` before the full run.
- Full native command sets `FORT_TEST_GUARDIAN=/tmp/fort059-mac-core-host-guardian`,
  `FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node` and the retained
  `successor/.build/owned-core-059-20261018` artifact, excluding the four timing
  functions — `/tmp/fort059-host-removal-native.log`.
- `FORT_SIGNING_IDENTITY=58F40D1AB480565B89109B853235AC3A7B2E7AD1 /opt/homebrew/opt/node@26/bin/node --test --test-name-pattern='embedded XPC peers' successor/packaging/sandbox-feasibility.test.ts`
  — `/tmp/fort059-host-removal-xpc.log`.
- `PATH="/opt/homebrew/opt/node@26/bin:$PATH" npm --prefix successor run typecheck`
  — `/tmp/fort059-host-removal-typecheck.log`.

## Confirmed-record retry cost

Twenty isolated serial retries, constructing a fresh removal controller each time,
with warm filesystem/OS metadata caches and no concurrent verification work. The
sample includes opening/validating the private lease and records, ACL checks,
restoring/querying the absent synthetic process and asynchronous fake startup status
reads. Record creation/fsync, real XPC/ServiceManagement callback, process shutdown,
provider, UI and build time are excluded. The test confirms unchanged record bytes,
inode and mtime, one recovery call and one fake unregistration across all retries.

Target p95 <=50 ms; nearest-rank p50 **0.516917 ms**, p95 **0.740583 ms**. This is
local retry overhead, not installed removal latency or an OS callback budget.

Raw milliseconds:

```json
[0.740583,0.559291,0.537208,0.527625,0.522458,0.522458,0.516917,0.787708,0.593208,0.531416,0.526041,0.512875,0.511208,0.504,0.515042,0.501875,0.498541,0.508791,0.503041,0.500167]
```

Command: `FORT_HOST_REMOVAL_BENCHMARK=/tmp/fort059-host-removal-budget.json swift test --package-path successor/apple/FortKit --scratch-path /tmp/fort059-owner-setup-xpc-native --filter hostRemovalConfirmedRecordRetryBudget`.
Log: `/tmp/fort059-host-removal-budget.log`.

## Preserved failures

- `/tmp/fort059-host-removal-red.log`: missing operation plus an actor-property test
  expression error. `/tmp/fort059-host-removal-missing-red.log` isolated the missing
  implementation. `/tmp/fort059-host-removal-first-green.log` then found that the
  fake actor method needed an explicit Sendable async closure. Corrected syntax
  passed in `/tmp/fort059-host-removal-restart-pass.log`.
- `/tmp/fort059-host-removal-boundary-red.log`: setup proceeded despite the removal
  marker, and a confirmed retry unregistered a newly enabled fake host. The setup
  guard and mutation-free already-unregistered path fixed both; boundary pass log
  retains the result.
- `/tmp/fort059-host-removal-acl-red.log`: Unix-mode checks alone admitted ACL-bearing
  directory/lease/record paths. ACL checks fixed initial admission. The expanded log
  then caught a lease ACL added during recovery; revalidating the held lease fixed
  it. `/tmp/fort059-host-removal-behavior.log` confirms the passing cases. Tests
  remove only ACLs they added to their own temporary fixtures.
- A read-only feasibility query for `kern.bootsessionuuid` metadata was denied by
  the execution sandbox with EPERM; no value was read or grant requested. The design
  does not require boot metadata. Recorded start-time comparison remains conservative
  after PID reuse/reboot and never treats inspection denial as absence.

## Remaining gates

The production app still lacks its complete owned-installation composition, verified
package/profile and ordinary UI wiring. No actual credential removal, history deletion,
App Group access, startup activation, official provider, installed UI/accessibility/
screenshot, physical device, archive, upload or TestFlight acceptance ran. Existing
confirmed removal records intentionally block a new setup until explicit installation
reconciliation is implemented. This must not be mistaken for a completed reinstall
journey.

The portable Node runtime/core did not change; its 900-test evidence and unchanged
non-XPC/Codex packaging scenarios were not rerun here. The retained compiled core
remains 30 files/190826 bytes with manifest SHA256
`c1a19bd8e8f0def68efa45f2b913dc5b2123466a3da6a5c14ce0eb8fded7f527`.
