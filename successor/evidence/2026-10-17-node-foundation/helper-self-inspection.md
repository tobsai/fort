# Helper self-inspection repair

Spec 059 / decision 059. October 19, 2026. Base
4ca08324a851b37cfab0dcc89bba105ac755649b. The first installed attempt ended after cleanup;
its [failure and confirmed cleanup](installed-host-test.md) remain separate evidence.
No registration, pairing or provider activation occurred during this repair.

The later [extended-time continuation](installed-host-test.md#extended-time-continuation)
installed this exact candidate after repeat verification. Automatic approval review
blocked activation; the client quit normally and exact absence checks passed.
The owner extended the test duration, so time is no longer the blocker. The
uninstalled/old-deadline statements below describe the repair checkpoint.

## Cause and change

MacHostProgram called the full enclosing-app inspector from the sandboxed helper.
The canonical-path signed read-only probe returned unsafeFile. A metadata/open probe
reported EPERM for the enclosing client manifest, LaunchAgent and profile, while
all corresponding files inside the helper opened successfully. Canonical and
standardized paths matched in the Fort-local reproduction. An initial /tmp probe
also had path-normalization differences and was not used as the decisive case.

The client still checks the entire package, both signatures, matching strict
manifests, exact LaunchAgent and profiles before enabling. Helper startup now checks
its own exact nested layout, signed manifest, signature/team/App Group, profile
presence, executable and all Node/guardian/Codex signatures and inherited-sandbox
entitlements. Both inspectors use the same strict manifest parser. The signed
helper manifest supplies the existing exact signed-client XPC requirement. History
maintenance still runs full client verification before its helper program check.
No filesystem or network entitlement was added, no requirement was replaced with a
fallback, and no sandbox was disabled. Host and computer-tool boundaries remain
separate. This fixes the observed inspection failure, not all unrun startup phases.

## Red, green and package checks

The new behavior regression removes enclosing client manifest/startup/profile and
requires helper inspection using only owned resources. It first failed unsafeFile.
It now passes; negative cases reject invalid own manifest/profile/signature/layout,
wrong sandbox/group and altered child code requirements. Existing full-client
negative controls remain passing.

The repaired signed sandbox probe printed PROGRAM_INSPECTION_PASS for the exact
helper identity. Existing storage inspection returned NSPOSIXErrorDomain 2 because
FortHost was absent; it created no storage. Both client and helper in that isolated
probe lacked outgoing-network entitlements. The probe starts no XPC listener,
Node, guardian, Codex or service and does not establish working-host acceptance.

Final configured native suite: 133 passing functions, zero failures, 11 optional
checks skipped, 144 reported including skips. It covers storage, startup, owner
setup, host service, full/self package inspection, fresh cleanup and paired removal.
The narrow package run separately passed 17 functions with four optional skips.
Release FortMac/FortHost builds passed in 27.496/3.005 seconds. iPhone Simulator
Release compilation passed; simulator execution was unrun, with Xcode warning that
installed CoreSimulator services are older than its build. Durations are single
build samples, not performance benchmarks. Broad portable tests were not repeated.

## New inert candidate

/tmp/fort059-self-inspection-signed-20261019/Fort.app

- 206 files, 406,612,722 bytes, 30 Mach-O objects.
- Report SHA256 b887864ad524a4cd312130b1f55cc2eb7abfec666a19dcaeb50ad29541fbecae.
- Payload manifest SHA256 aac1011e317607d5b56a293ffdd237ffbe7ad16b328eb894e1676b015cda024f.
- Source base 4ca08324a851b37cfab0dcc89bba105ac755649b; report pins both changed
  product source hashes. Tests/spec/evidence do not alter packaged runtime inputs.
- Relative to the original approved report, only FortMac, FortHost and the outer
  CodeResources seal differ. Node, guardian, Codex, profiles, libraries, legal
  attribution and all entitlements match. Existing approved development certificate
  SHA1 58F40D1AB480565B89109B853235AC3A7B2E7AD1, team T3JB5MYZ93.
- Every file membership/hash/mode/size, all 30 strict all-architecture signatures,
  all public signing leaf fingerprints, exact entitlements, strict nested signature
  and native full-package inspection passed. Profiles are byte-identical to the
  previously verified original report.

The candidate remains uninstalled and unlaunched. The installed cleanup-only app
remains stopped at /Users/tobiasgunn/Applications/Fort.app with report
9e401469e60006e14f063718be077c4aed14f0719c13e3b7d145a2c68b215453. Its registration,
owned processes and host/provider storage are absent. Nothing reset the original
19:18 test clock, generated credentials, signed into ChatGPT or uploaded the G2 CSR.

The coordinator clarified at 19:41 UTC in source thread
01a0f307-3940-7333-96b0-a3b0b854e0e8 that existing approval covers targeted fixes
and retries when scope/risk are unchanged. The changed build hash alone is not an
approval gate. At 19:41:18, 104 seconds remained before the 19:43:02 cleanup reserve
and 404 seconds before the original 19:48:02 deadline. That was insufficient for
installation, connection, personal-account sign-in and safe testing, so no retry
was activated. Further execution needs enough authorized time; do not silently
reset the original deadline. Personal ChatGPT remains the owner's selection. The normal sign-in/account UI must establish
that identity; no company workspace, ambient credential import or model fallback.
Real helper startup/XPC, provider controls, Astra discovery/replies/cancellation,
paired Stop/Resume/removal, TestFlight and physical acceptance remain open.

Raw source probes, logs and exact verification reports are under
/tmp/fort059-attended-20261019 and /tmp/fort059-self-inspection-candidate-verification.json.
Stopped read-only diagnostic bundles remain under
/Users/tobiasgunn/dev/fort/.local/fort059-readonly-inspection-20261019.
