# Pending request cancellation — October 17, 2026

Source: `8a877fff2730d7dd99bb45b9f47e31158182383c` on
`codex/fort-openai-rebuild-20261016`. This is local source, synthetic HTTP and
native-build evidence. No installed app, Simulator UI or real provider was run.

## Result

An authenticated request-ID cancellation transaction now either records a durable
cancellation before acceptance or reconciles the exact accepted receipt and requests
cancellation of its run. A later original submission cannot cross that cancellation
record. Scope conflicts and old recovery epochs are rejected. Accepted duplicate
submissions retain their original receipt. Active work stays cancel-requested until
the runtime supplies evidence; completion and unknown outcomes remain truthful.

The native pending store persists cancellation intent before network I/O. Relaunch
replays that intent, never the canceled submission. Cancellation during an in-flight
acceptance response is retained. Only an exact acknowledged scope can clear pending;
accepted results must include the matching receipt, run and snapshot. The UI displays
confirmation pending while offline. Older services without the capability keep
cancellation intent held. No credential, permission or runtime policy was expanded.

## Validation

- Full synthetic TypeScript suite: **654 passed, zero failed/skipped**.
- Strict TypeScript check: **exit 0**.
- Swift package suite: **33 passed**. Includes offline cancellation, lost acceptance
  and cancellation responses across relaunches, cancellation during submission, and
  rejection of a mismatched acknowledgement.
- Real Swift HTTP client against the local service: **ten synthetic runtime calls,
  six service restarts, two discarded acceptances and one discarded cancellation**.
  No duplicate or canceled dispatch; final pending queue empty. Schedule preview,
  enable and pause still make no runtime call.
- Fresh Mac and iPhone Simulator `build-for-testing`: **passed**. Four app/runner
  signatures verified with deep/strict checking; 26 files hashed, two test plans.
  Artifact paths, signatures and source identity are in `builds.json`.
- All six historical packets match file sizes, hashes and modification times;
  native cache file set also matches. Main checkout status matches the baseline.
  Preexisting untracked `.build/` is retained.

The first two service tests failed with missing-route 404s, then passed after the
implementation. The first Swift build failed because the new API/intent did not
exist. An intermediate offline fixture lacked a recipient, and an HTTP-checker
compile attempted an inaccessible memberwise initializer; both were corrected.
No final check remains failing. Persistent log paths and SHA-256 hashes are in
`checks.json`; the initial service and Swift red logs are retained. Intermediate
logs were superseded by the final successful runs and are not claimed as retained.

## Open gates

The three-session live enrollment proposal is still pending; no live token was
issued or consumed packet reused. Installed accessibility/screenshots, current
provider behavior, physical iPhone, host isolation, real sleep/reboot/schedule
execution, backup/restore and distribution are unrun. The agent host still has the
Mac user's access; computer-tool restrictions do not prove host confinement.
This milestone does not complete the MVP or change Spec 058's acceptance gates.
