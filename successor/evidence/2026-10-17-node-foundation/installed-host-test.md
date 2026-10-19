# Approved installed host test, October 19

Spec 059 / decision 059. Base source 092dc52ea62f979f6d5b935371fd0fe95e28c63e.
The owner approved the exact one-time test in Sentinel_5ca1f5e4bb788191b7a2470b8b42a9dd
and selected personal ChatGPT in Sentinel_4d576ff6134481919bb4eaaa71450b8c. The
proposal was Sentinel_1d0357e845108191bda2c580952409d5. No G2 upload approval follows.

The subsequent [self-inspection repair](helper-self-inspection.md) has source and
read-only sandbox evidence. It does not turn this failed live test into a pass.

## Extended-time continuation

At 23:43:54 UTC the owner said “Extend these as long as needed as a rule going
forward” (Sentinel_15479ff398d081918411da08295352de), in response to extending
this installed test. The original time record is preserved, not reset. Personal
ChatGPT, gpt-6-astra only if reported, six synthetic prompts total, unchanged
permissions and verified cleanup remain the bounded scope. G2 upload is separate.

The self-inspection candidate was verified again: all 206 files/modes/sizes/hashes,
all 30 strict signatures and public signing leaf fingerprints, exact entitlements,
strict nested signature and native full-package inspection passed. Report SHA256
b887864ad524a4cd312130b1f55cc2eb7abfec666a19dcaeb50ad29541fbecae;
payload SHA256 aac1011e317607d5b56a293ffdd237ffbe7ad16b328eb894e1676b015cda024f.
It was installed at /Users/tobiasgunn/Applications/Fort.app. The prior cleanup
app was preserved at /tmp/fort059-installed-cleanup-preserved-20261019.app.
The installed setup UI reported the host not enabled and Connect disabled.

Automatic approval review rejected the exact Enable local host action before
execution. Its reason: persistent startup conflicts with the original import-only
prohibition, and extending time alone is insufficient. Reconsideration after
checking the prior proposal, approval references and same-scope retry record was
also rejected: the review treats those files/assistant claims as untrusted evidence.
The source-thread read returned no turns, so it did not independently recover the
original human approval. This record does not override the rejection. No workaround,
registration, pairing, credential creation or provider authentication occurred.
The owner was asked for an explicit confirmation of the concrete installed test.

Normal client Quit passed. At 23:49:32 UTC, independent launchctl exit 113 and
process/path checks confirmed no exact host job or owned process, no FortHost
storage or provider credential path, and only the zero-byte mode-0600 setup lock
in native storage. Installed files still match all 206 manifest records. No token
was generated; Keychain contents were not read. Zero of six prompts have been used.
No provider authentication handoff has been reached. Full startup/XPC, account/model
controls, replies, cancellation and paired lifecycle remain unrun after the repair.

Continuation authority, exact action/result, redacted AX and final absence records
are under /tmp/fort059-attended-resume-20261019. No product source changed or
behavioral tests reran for this evidence-only checkpoint. Existing native test/build
results in the linked repair and original outcome remain historical proof with their stated limits.

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

The first attempt and its cleanup ended at 19:29:26 UTC, before the 19:43:02 cleanup reserve and
19:48:02 deadline. Zero provider sign-ins, zero model requests, zero completed
pairings. The no-default-expiry pairing policy did not change. Personal ChatGPT,
Astra discovery, effective provider controls, replies, cancellation, Stop/Resume
and conversation replay remain untested. The coordinator subsequently confirmed that same-scope retries are allowed within
the original window. A changed hash alone does not require another approval. At
19:41:18 only 104 seconds remained before the cleanup reserve, so the repaired
candidate stayed inactive. No deadline or request allowance was reset.

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
