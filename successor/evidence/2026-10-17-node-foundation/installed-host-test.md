# Approved installed host test, October 19

Spec 059 / decision 059. Base source 092dc52ea62f979f6d5b935371fd0fe95e28c63e.
The owner approved the exact one-time test in Sentinel_5ca1f5e4bb788191b7a2470b8b42a9dd
and selected personal ChatGPT in Sentinel_4d576ff6134481919bb4eaaa71450b8c. The
proposal was Sentinel_1d0357e845108191bda2c580952409d5. No G2 upload approval follows.

## Outcome

The original candidate was verified against report SHA256
983ccb10307b07256daba171bee084568fc82d0e8c1772d4c3dffec0a7d1e439 before installation.
All 206 file records, all 30 code signatures/public signing leaf fingerprints,
exact entitlements/profiles and native client-side bundle inspection passed.
The earlier installed app was preserved. No original report was rewritten.

Enable succeeded at 19:18:02 UTC through the installed UI. macOS reported enabled
registration for io.mtree.fort.mac.host. One Connect action then failed: launchd
repeatedly attempted the helper, which exited with code 1 before creating FortHost
storage. The client timed out without pairing. No second Connect action occurred.
The client quit normally; scoped launchctl bootout stopped the exact failed job.
Fort still reported enabled registration afterward. A loaded-job absence check alone
would therefore have been insufficient cleanup evidence.

The bounded test ended at 19:29:26 UTC, before the 19:43:02 cleanup reserve and
19:48:02 deadline. Zero provider sign-ins, zero model requests, zero completed
pairings. The no-default-expiry pairing policy did not change. Personal ChatGPT,
Astra discovery, effective provider controls, replies, cancellation, Stop/Resume
and conversation replay remain untested. This consumed test is not replay authority.

## Cleanup repair and installed proof

The prior UI exposed removal only after pairing. The repaired client offers Remove
unused startup registration when fresh setup has no pairing. The controller holds
the existing setup lock, checks all eight setup/pairing/removal record kinds are
absent, revalidates exact signed bundle identity, proves the exact App Group's
FortHost child is absent, calls SMAppService.unregister, and rechecks absence.
Existing host storage, any saved record, changed identity, failed unregistration
or uncertain storage prevents success. No file, history or credential is deleted.
Existing paired installations continue through durable host removal.

The cleanup-only candidate is /tmp/fort059-unpaired-cleanup-signed-20261019/Fort.app.
Its report SHA256 is
9e401469e60006e14f063718be077c4aed14f0719c13e3b7d145a2c68b215453;
payload manifest SHA256 is
39375f91f994c8f68accb6d05af5d47c3b07f458e772964768c254640ea10340.
Only Contents/MacOS/FortMac differs from the original signed payload. Helper,
Node, guardian, Codex, profiles, resources and entitlements are unchanged. It was
installed only for the authorized cleanup, never connected or used for sign-in.
The original installed candidate remains preserved at
/tmp/fort059-installed-approved-failed-preserved-20261019.app.

The normal removal confirmation completed. Its AX press returned -25205 while the
app actually completed removal; state was inspected before any retry. Fort reported
notRegistered and disabled Connect. Normal Quit/reopen retained notRegistered.
The final client quit normally. All 206 installed file records still match.
Independent launchctl exit 113, exact process-path checks and metadata confirm
job/process absence, no FortHost directory/provider credential path, and only the
zero-byte owner-setup.lock in native storage. No setup offer or token was generated.
Keychain contents were not read. App/Group containers created by macOS are retained.

![Cleanup confirmed](installed-host-test-cleanup.png)

## Validation and limits

The new controller behavior test first failed with unavailable, then passed.
The configured focused suite passed 83 test functions, zero failures, six optional
timing skips. It covers fresh cleanup, eight saved-state refusals, existing host
storage, changed signatures/identity, ignored unregistration and storage absence
with files/directories/symlinks. The initial restricted run had three local IPC
fixture errors in two existing functions; normal local IPC permission and explicit
fixture paths passed. Release FortMac built in 31.118 seconds, a single build
sample, not a benchmark. Signed cleanup artifact verification passed. Broader
portable tests, iOS/older-macOS execution and paired installed removal are unrun.

Read-only signed diagnostic copies, with outgoing-network entitlements removed,
reproduce helper bundle inspection failure without service/runtime/provider launch.
A canonical-path probe shows sandbox EPERM when the helper opens its enclosing
client manifest, launch-agent plist and profile; its own corresponding files open.
The helper's current inspection assumes it can read the enclosing app. This remains
an open startup defect here; a later change must preserve exact signed identity
checks without broadening filesystem entitlements. No successful host/provider
or TestFlight acceptance is claimed. G2 upload remains held separately.

Raw redacted controls, source probes, build/test logs, original clock/outcome and
final absence metadata are retained under /tmp/fort059-attended-20261019. Probe
apps are stopped under .local/fort059-readonly-inspection-20261019. They have no
startup registrations and cannot be treated as tested application builds.
