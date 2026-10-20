# Fort delivery through TestFlight

## Current checkpoint: explicit runtime migration passes source validation

October 20, after offline runtime commit `3f8ea843`. The owner approved the bounded
migration/install and three-prompt attended verification at 12:03:35 UTC. The
separate offline operation preserves history and model selection, creates current
binding revisions atomically, retries safely, and rolls back only before later
ledger changes. Normal runtime mismatch refusal remains. History deletion supports
the retained migration journal. Four process-crash boundaries pass.

Focused migration/history/package: 80 passed, two optional skips. Full portable:
1,398 passed, 16 skipped; strict typecheck passed. Its first run exposed a temporary
build-fixture module-setting omission, now fixed and passing. The 10,002-message
maintenance sample took 55.996 ms; this is one synthetic sample, not production
performance acceptance. Fresh resources and the 204-file unsigned app are prepared.

Read-only installed checks match all 206 prior files and confirm stopped/disconnected
state. Retained history has six messages/three completed runs and no pending work.
The installed app/ledger still use 0.157.1; no real migration, activation, sign-in or
prompt has happened at this source checkpoint. Next: closed-copy rehearsal, exact
signatures/install, normal personal-account sign-in, native verification and local
cleanup. Effective model, physical iPhone, TestFlight and G2 remain separate gates.
See [migration evidence](evidence/2026-10-17-node-foundation/runtime-migration-0163.md).
Older checkpoints below retain their observations.

## Current checkpoint: Codex 0.163.0 prepared offline; installation held

October 20, after Quit repair commit `3aba02fd`. The 0.163.0 adapter, startup
controls and legal/package pins are prepared against source
`83086188ae0614bd882818ff1aa23f3194962012`. The actual unauthenticated offline
runtime accepts an empty ephemeral thread with exact `gpt-6.1-sol`; no login or
inference occurred. Catalog presence does not establish account entitlement.
Four new opt-outs are enforced in global and thread scope. The diagnostic
preferences IPC exception is documented and changes no production permission.

Portable: 1,363 pass, 16 skips. Typecheck passes. Compiled Mac executor: 29 pass,
2 benchmark skips. Assembly: 21 pass. Focused signed offline version/provenance
and wrong-payload checks: 2 pass. Fresh helper Release build and 202-file package
readback pass. The review app is unsigned, uninstalled and unlaunched; report SHA256
`3ecd55ce2c16a6a3b0f23893ceb9ebad454283834984613599c51df631ecfae6`.
The original notices and covered-source copies are preserved in a new legal payload.

**Installation blocker:** the retained ledger is bound to 0.157.1. A regression
proves the new runtime is refused while the original exact history/bindings remain
recoverable. Owner: runtime/core implementation. Next gate: explicit reviewed,
history-preserving binding migration with recovery/rollback tests, then an exact
signed package and bounded installed verification scope. Do not reset history,
rewrite historical versions or silently change pending work to bypass this gate.

The installed Quit-fixed app remains on 0.157.1. At 04:06 UTC all 206 files matched
its prior signed report; no client, host, offline runtime, startup job, credential
file or active connection remained. History is retained. No process is stalled.
The paired installed Quit journey, live 6.1 account/inference, physical iPhone,
TestFlight and G2 remain separate open gates. No new activation, sign-in, pairing,
provider turn, upload, push or merge occurred. See [exact evidence, failures and
remaining scope](evidence/2026-10-17-node-foundation/codex-0163-offline-update.md).
Older checkpoints below preserve their original observations.

## Current checkpoint: Pairing navigation Quit repaired

October 20, after source 43bd032. Entering Pairing reset the sheet termination flag;
confirmation dismissal was not required. The destination now reuses the existing
window-local policy. A rendered-view regression failed first and now preserves the
draft and connection. The actual MessengerView/Settings/Pairing/Stop/Cancel flow in
a temporary synthetic FortKit app permits normal foreground Quit.

Complete native suite: 342 pass, 16 skips. Fresh Mac and iPhone Simulator Release
builds pass. The installed client-only update changes one executable; all 206 file
records, 30 signatures, entitlements and native bundle inspection pass. Disconnected
setup accessibility/screenshot and normal Quit pass. Final 03:40:12 UTC checks confirm
no owned processes, launchd job, provider credential file or active connection.
Retained history is untouched. Candidate report SHA256 is
`5d24d0c8e3595334e2bb0b0becb10b84d698b7516eb25c22f4802de0017b98f8`.

The production Pairing route is unavailable while disconnected, so that installed
flow remains unrun; the faithful synthetic actual-view flow is the fix evidence.
No activation, re-pairing, sign-in or prompt occurred. Codex 0.163.0 offline update
preparation continues separately. 6.1 inference, TestFlight, physical acceptance
and G2 upload remain unrun/held. See the [repair evidence](evidence/2026-10-17-node-foundation/model-selection-recovery.md#pairing-navigation-quit-repair-october-20).
Older checkpoints below retain their original observations.


## Model selection repaired; attended smoke test cleaned up

October 20: account confirmation is now available inside the agent editor and
refreshes the exact runtime catalog. Three public regressions cover confirmation
and both concurrent discovery/selection orderings. A separate cleanup regression
found an old confirmed pairing record incorrectly blocking removal after re-pairing;
that case is fixed while unfinished foreign intents still refuse before effects.

Fresh native checks: 341 pass, 16 optional skips, zero failures. Portable suite:
1,329 pass, 16 skips, zero failures. Mac and iPhone Simulator Release builds pass.
The exact installed client passed account/model persistence, accessibility and
screenshot inspection, two synthetic Astra replies, Stop/Resume, history/relaunch
and normal host-removal cleanup. Three of six prompt slots are accounted for,
including one additional observed turn whose contents were not inspected.
Effective provider model is not reported by the completed turn UI.

Final installed report:
b418cf5fb43b631339a9b1031ab162d3f0e8f774756e39b9352bfb96b50af67f.
Only the client executable changed; helper/runtime/resources/permissions stayed
identical and previous packages are retained. At 03:17:18 UTC the client and host
were closed, startup job/local credential/active connection absent, and history
retained. The attended session is complete; do not replay activation or sign-in.

Open: Quit after a Settings confirmation can fail until Settings is closed. Simple
alert fixtures do not reproduce it, so no speculative additional fix is claimed.
Owner: native UI implementation; gate: reproduce and repair the exact installed
modal lifecycle, then verify normal Quit. This prevents a full smoke pass.

GPT-6.1-Sol entered the official bundled catalog in Codex 0.159.1. Latest verified
stable is 0.163.0; Fort still bundles 0.157.1. A coordinated runtime/adapter/package
update is the next concrete step. Catalog presence is not entitlement proof; keep
managed credentials inside the official runtime. Offline update preparation is
within ordinary scope; new live activation/6.1 inference is not covered by the
completed session. TestFlight, physical iPhone and G2 remain separate open gates.
See [full results and exact update assessment](evidence/2026-10-17-node-foundation/model-selection-recovery.md).
Older checkpoints below describe earlier state.

## Native timing failures isolated; CI scheduling corrected

October 20: repeated controls reproduce the removal timing failure without the new
Settings view. The budget passes alone; unrelated concurrent main-actor tests add
scheduling delay to its elapsed time. The broader parallel suite also has three
synthetic account coordination timeouts and a host-journey timing failure. With
explicit --no-parallel, the unchanged full native bundle passes 337 functions with
16 optional skips and zero failures; the host journey takes 2.09 seconds against
five seconds. Three focused final-command runs each pass all 25 functions.

The minimal fix adds --no-parallel to the existing CI command and local instructions.
No product/test code, threshold, assertion or sample count changed. Validation uses
the existing compiled bundle: a fresh broad build stalled before test execution.
Remote CI remains unrun. [Exact controls, results and limits](evidence/2026-10-17-node-foundation/native-timing-isolation.md).
The Settings fix is still awaiting installed verification. The installed sign-in
UI and active host were untouched; authentication confirmation is awaited, zero of
six prompts used, and final cleanup remains pending. G2 upload is separately held.

## Settings Quit repaired in source; active sign-in left untouched

October 20: the Settings sheet now reuses the setup sheet's window-local termination
setting. The earlier fix covered only Connect to Fort. A hidden rendering test of
the real Settings view failed before this change and passes afterward, preserving
its unsent draft and empty isolated storage. The 25 focused native tests pass with
`--no-parallel`; two parallel runs each fail only the existing removal timing budget
(52.55 and 65.75 ms against 50 ms). Sequential p95 is 0.668 ms. The timing test's
sensitivity to shared-main-actor scheduling remains a validation limitation; its
threshold and implementation were not changed. Mac and iPhone Simulator Release
builds pass. No new artifact was signed, installed or launched.

The installed report remains 0ab05d939bcbdc7eb1d0905156790a42a1db4375b5bb1ac63d6106cb95d563fc;
this source slice did not touch the app, sign-in UI, active host or pairing. Owner
confirmation of personal ChatGPT authentication is still awaited. Zero of six
prompts used; account selection, Astra discovery, provider acceptance and final
cleanup remain pending. Normal Quit on the changed installed build also remains
unverified. G2 upload is still separately held. See the
[Settings repair evidence](evidence/2026-10-17-node-foundation/installed-setup-quit.md#settings-sheet-repair-october-20).

## Host paired; private runtime startup repaired; authentication handed off

October 20: the owner's explicit override in
Sentinel_cc6a553555a481919adde09325fb740c admitted the previously blocked activation.
The repaired host started and paired. A fresh private Codex configuration then failed
before authentication because it declared disabled MCP servers without transports.
The narrow fix requests an empty MCP map and retains all effective-control checks.
Two public regressions failed first; 561 focused tests now pass with five optional
benchmark skips, strict TypeScript checking passes, and the compiled executor passes
official Codex initialization/control checks in a signed no-network diagnostic.

The replacement report is
0ab05d939bcbdc7eb1d0905156790a42a1db4375b5bb1ac63d6106cb95d563fc.
All 206 installed files, 30 signatures, exact entitlements and native bundle inspection
pass. The previous copy is preserved. The first paired test's normal removal confirmed
shutdown, revocation, unregistration and credential removal; independent absence
passed. Settings must be closed before normal Quit; that UI defect remains open.

The replacement connected through Set up Fort again and reached the official device
challenge. The verification page was opened at 00:06:29 UTC; the owner was asked to
complete personal ChatGPT verification without sharing the code. Account selection,
Astra discovery and prompt tests await that interaction. Zero of six prompts used.
This checkpoint leaves the exact host/pairing active for the authentication handoff;
final cleanup is pending. G2 remains separately held. [Evidence](evidence/2026-10-17-node-foundation/installed-host-test.md#empty-mcp-repair-and-authentication-handoff).
Earlier activation-rejection and inactive-app statements below are historical.

## Repaired candidate installed; activation rejected by automatic review

October 19, 23:49 UTC: the owner extended test time as needed in
Sentinel_15479ff398d081918411da08295352de (23:43:54 UTC). The old clock remains
historical evidence; duration is no longer the blocker. Personal ChatGPT, Astra
only if reported, six synthetic prompts total, unchanged permissions and verified
cleanup remain the scope. The original attempt used zero prompts.

The repaired candidate is now installed at /Users/tobiasgunn/Applications/Fort.app.
All 206 file records, all 30 signatures/public certificate fingerprints, exact
entitlements and native full-package inspection passed again. The old cleanup
app was preserved. Automatic approval review rejected Enable local host before
execution, then rejected reconsideration using the earlier approval records:
it treats those records as untrusted and recognizes the original startup
prohibition. No workaround was attempted; an explicit confirmation was requested.
The client quit normally. At 23:49:32 UTC the exact job, owned processes, host
storage and provider credential path were absent; native storage held only its
empty setup lock. Zero pairings, sign-ins or prompts occurred. The application
remains installed and stopped. [Exact continuation evidence](evidence/2026-10-17-node-foundation/installed-host-test.md#extended-time-continuation).

Host/provider/paired lifecycle and TestFlight acceptance remain open. G2 CSR upload
is separately held. Earlier uninstalled/deadline statements below are historical.

## Helper inspection repaired; replacement candidate inert

October 19: the helper now validates its own signed bundle without reading its
sandbox-inaccessible enclosing client files. The client retains full-package
inspection. A signed sandboxed read-only probe passes, 133 affected native tests
pass with 11 optional skips, and Mac/helper/iPhone Simulator builds pass. The
[new exact 206-file candidate](evidence/2026-10-17-node-foundation/helper-self-inspection.md)
is uninstalled/unlaunched; all 30 signatures and full-package inspection pass.
The installed cleanup app remains stopped and unregistered. The existing approval
covers same-scope targeted repairs/retries through 19:48:02 UTC; a changed hash alone
is not a new approval gate. At 19:41:18 only 104 seconds remained before the reserved
cleanup period, so no retry was activated. A continuation needs enough authorized
time, with the original deadline preserved.
Personal ChatGPT remains selected, with identity verification still required at
normal sign-in. Provider, paired lifecycle, TestFlight and physical gates are open.
G2 CSR upload remains separate and held.


## Approved test stopped before pairing, cleanup confirmed

October 19: the approved exact installed candidate registered its helper, but helper
startup failed before host storage, pairing or provider sign-in. Zero prompts were
sent. A tested client-only cleanup repair removed the unused registration through
SMAppService; reopening confirmed notRegistered, and final process/job/storage
absence passed at 19:29:26 UTC. The app remains installed and stopped.
[Evidence and exact artifact pins](evidence/2026-10-17-node-foundation/installed-host-test.md).
Helper sandbox inspection needs repair. Real-provider and paired lifecycle acceptance
remain open; no retry can reset the original 30-minute deadline. G2 upload is
separate and still held. Earlier unapproved/unlaunched statements below are historical.


## Setup Quit and Close repaired — October 19, 2026

The [installed setup repair](evidence/2026-10-17-node-foundation/installed-setup-quit.md)
closes the ordinary client Quit issue below. A minimal SwiftUI reproduction isolated
the modal-window termination default. The setup sheet now permits normal Quit and
has an explicit Close action; durable setup and pairing policy are unchanged. Three
installed cycles passed six normal Quits, nine setup close/reopens and three window
close/reopens. Final AX/screenshot and normal cleanup passed. The client is stopped,
registration absent, host storage uncreated and all installed hashes unchanged.

Release Mac/helper/iPhone Simulator builds and the focused native run passed, with
66 passing functions and two optional timing skips. Both final packages passed all
206 file checks and 30 signatures. The [one-time provider-test proposal](evidence/2026-10-17-node-foundation/installed-host-proposal.md)
now pins the new artifact/report digest. Its 30 minutes/six requests constrain only
that attended test; pairing still has no default expiry. The provider candidate is
uninstalled/unlaunched; host/provider and G2 upload approvals remain pending.
Earlier known-Quit-blocker statements below describe superseded checkpoints.


## Installed client preflight — October 19, 2026

The exact task-installed development client now shows Enable for a verified fresh
host whose OS status is notFound; Connect remains disabled. Three source files and
three native test files cover the startup/controller/view correction. The focused run finished with
66 passing functions, zero failures and two optional timing skips; fresh Release builds, all 206
file checks and all 30 signatures passed for each final candidate. Installed AX and
screenshot passed without Enable/Connect/sign-in. Normal unpaired Quit did not exit;
exact-process cleanup was needed, so quit/reopen remains open. The client is stopped,
service absent and host storage uncreated. The installed helper still lacks network.

A separately signed **uninstalled/unlaunched** provider-test candidate with proposed
helper outgoing-network entitlement is pinned in the [consolidated authorization
proposal](evidence/2026-10-17-node-foundation/installed-host-proposal.md), including
account/model choice, persistence, transmitted/retained data and bounded cleanup.
[Full preflight evidence](evidence/2026-10-17-node-foundation/installed-host-preflight.md).
Do not activate it or configure credentials before that action-specific approval.
G2 CSR upload remains held pending the parent's exact disclosure approval; no retry
or new certificate occurred. Older “unlaunched client” statements below describe
previous checkpoints. Host/provider/removal/TestFlight acceptance remains unrun.


October 19 static-privacy checkpoint: all 30 signed Mac binary objects and the
unsigned iPhone binary were hashed and inspected; 31 symbol reads passed and no
privacy manifest is present. Native callers and remaining runtime/platform decisions
are recorded in the [inventory](evidence/2026-10-17-node-foundation/privacy-runtime-inventory.md).
Focused synthetic telemetry/update checks passed 407 tests with one optional budget
skip. No privacy label, encryption exemption or remote submission changed.

The separate [G2 continuation](evidence/2026-10-17-node-foundation/g2-certificate.md)
created a local CSR, but its upload was rejected by automatic approval review and
is awaiting a precise user approval. No G2 certificate was issued/installed; this
does not change development signing or close any TestFlight gate.

October 19 signed-development checkpoint: the approved two Mac IDs, shared group
and two development profiles are created and verified. The new unlaunched signed
package passes all 30 code-signature checks, 206-file inventory and Fort native
bundle inspection. [Exact artifact, authority and limits](evidence/2026-10-17-node-foundation/mac-development-provisioning.md).
No installed host/provider activity or TestFlight operation occurred. The helper
retains no outgoing-network entitlement. Distribution profiles, archive/upload,
processing, tester access and physical acceptance remain separate gates. Earlier
missing-profile/signing statements below describe historical checkpoints.

October 19 release preparation: ordinary native builds now display Fort and contain
the existing Fort icon; QA labels remain explicit. Fresh unsigned Mac/helper/QA/iOS
builds and inert assembly passed [asset checks](evidence/2026-10-17-node-foundation/release-assets.md).
[Local submission drafts](release/SUBMISSION-DRAFT.md) are not published metadata.
The [current Apple record](APPLE-HOST-ACTIVATION.md#apple-access-reconciled-and-minimum-mac-provisioning-request)
confirms authenticated team access, Program License Agreement confirmation, active
Free Apps terms and existing Fort iOS record 6786748810. The missing Mac provisioning
request is explicit. No signing, profile creation, upload or activation occurred.
Privacy/required-reason API and bundled-runtime inventory, final contact/URL decisions,
signed installed acceptance and TestFlight delivery remain open.

Current read-only checkpoint, October 19, source 81938a4: the normal Keychain context
lists an existing Apple Distribution identity for team T3JB5MYZ93, expiring May 25,
2027, plus an existing Apple Development identity through September 29, 2027. All
91 local provisioning profiles decoded; none targets macOS for this team or grants
the exact Mac client/helper group. The two Developer ID identities use the previous
Sub-CA and expire February 1, 2027; their separately approved G2 replacement is an
outside-Store path and does not resolve TestFlight provisioning. App Store export/
installer signing is unrun; no Mac Installer Distribution certificate was found in
the queried team certificate metadata. [Exact evidence and smallest next action](evidence/2026-10-17-node-foundation/mac-review-package.md#signing-and-selected-account-ui-readiness--source-81938a4).
The later implemented host/account source and synthetic UI supersede older source-gap
statements below, while archive/upload/installed TestFlight gates remain open.

Current Node-foundation continuation: [Apple host activation gate](APPLE-HOST-ACTIVATION.md)
records verified/proposed identities, missing profiles, the smallest account check
and conditional provisioning/startup actions. Those actions remain unperformed;
the engineering source/build checkpoint is separate from release readiness.

The owner requires TestFlight distribution, not a standalone binary (message
`01a14bf2-68bb-7211-896f-b813da65f78b` in UI task
`01a14bac-6acb-70be-926a-23ab4584a0f1`). Local Debug/QA artifacts remain engineering
validation tools only. They are not the final user delivery or a substitute for
TestFlight installation and acceptance.

Apple supports TestFlight for iOS and macOS. Eligible builds need application
identifiers in their provisioning profiles; builds are assigned to tester groups,
and external testing can require beta review. A successful upload still requires
Apple processing before the build appears. These are separate gates, not synonyms
for delivery. Sources: [TestFlight overview](https://developer.apple.com/help/app-store-connect/test-a-beta-version/testflight-overview/)
and [upload workflow](https://developer.apple.com/help/app-store-connect/manage-builds/upload-builds/).

## Read-only inspection

Inspected on October 17, 2026 without reading/exporting private keys, passwords,
provider tokens or API-key files:

| Item | Observed state |
| --- | --- |
| Signing identities | Normal-context `security find-identity -v -p codesigning` reports four valid identities: one Apple Distribution, two Developer ID Application and one Apple Development. Apple Distribution belongs to Maple Tree Enterprises LLC, team `T3JB5MYZ93`. The sandbox's initial zero-result was not treated as absence. |
| Current successor project | `successor/apple/project.yml` defaults `CODE_SIGNING_ALLOWED=NO`; Simulator builds use ad-hoc signing. No non-Simulator team/profile mapping or TestFlight export configuration is defined. |
| Current app identities | `io.fort.spec058.fixture.ios`, `io.fort.spec058.fixture.mac`; engineering QA is `io.fort.qa.firstwave.20261017`. Earlier local Mac builds override to `io.fort.candidateb.mac`. None is verified as the intended App Store Connect identity. |
| Current version/branding | Version `0.1.0`, build `1`, default display name `Fort Fixture`. No app icon asset catalog exists in the successor Apps target. These are prototype defaults. |
| Historical iOS export metadata | Primary checkout `ui/apple/ExportOptions.plist`: team `T3JB5MYZ93`, `method=app-store-connect`, `destination=upload`. It belongs to the historical tree and is not a successor release command. Export with that destination would upload; do not run it as an innocuous packaging check. |
| Historical Mac export metadata | Primary checkout `ui/apple/ExportOptions-mac.plist`: `method=developer-id`, automatic signing, team `T3JB5MYZ93`. This describes standalone distribution, not this TestFlight delivery requirement. |
| Historical app references | The explicitly historical `docs/notes/testflight.md` names `io.mtree.fort` and `io.mtree.fort.mac`. Their present App Store Connect ownership/platform/build state is unverified. Do not silently adopt them or restore their legacy gateway. |
| Tooling | Xcode/xcrun and fastlane are installed. No callable authenticated App Store Connect connector is exposed here. No account login/API credential was attempted or inspected. |
| App Store Connect state | Current app records, account role, provisioning, agreements, latest build numbers, tester groups and processing status have not been verified through an authorized session. Certificate presence alone does not prove any of these. |

No existing Apple credential was created, replaced, deleted or exported. No agreement
was accepted. No signing/entitlement/security settings were changed for release.

## Concrete packaging and access gaps

1. Verify the intended Fort iOS/macOS records and bundle IDs through existing
   authorized App Store Connect/Xcode access. Record only IDs, platform, team, role,
   latest version/build and tester-group metadata. Preserve existing app records;
   resolve reuse versus a new identity before modifying them. Creating credentials,
   accepting agreements or expanding access requires its own explicit authority.
2. Add successor Release signing/profile and App Store Connect export configuration
   for those verified identities. Keep QA outside the release graph. Add Fort display
   metadata and app icons, select a build number based on the actual remote record,
   and review required privacy/export-compliance metadata against the shipped behavior.
   Do not copy the historical runbook's answers or production gateway configuration.
3. Complete the real owned-host lifecycle and lost-response recovery adapter. The
   current Mac app is a native client; no managed host is installed or packaged by the
   QA build. Its Codex-host/credential/process boundary must work with the intended
   TestFlight app sandbox and update lifecycle. This remains unverified.
4. Resolve physical-iPhone connectivity before acceptance. The present service binds
   `127.0.0.1` on the Mac; the phone's own loopback cannot reach it. The native client
   supports HTTPS addresses, but no approved reachable host/transport has been deployed
   or enrolled. Do not add public ingress, vendor tunnels, other-device access or a
   replacement host merely to make a TestFlight upload look usable.
5. Inspect file-picker access and determine the required exact app entitlement before
   any approved change. The current Mac build lacks user-selected-file access. Bundled
   QA image rendering does not establish picker acceptance or permission scope.

## Delivery ledger

Record each stage independently for each platform and exact build:

| Stage | Required evidence | Current result |
| --- | --- | --- |
| Source/engineering build | Commit, tests, artifact hashes and signatures | Passed for the current local QA/Mac/Simulator source; engineering only |
| Archive | Release `.xcarchive`, app ID/version/build, distribution signing and profile validation | Not attempted for this successor release |
| Upload | Correlated App Store Connect upload receipt and exact app/build identity | Not attempted |
| Apple processing | The same build's processed/valid state and resolved metadata/compliance | Unverified; no successor upload |
| Tester availability | Exact build assigned and visible to the intended authorized tester/group; review complete if required | Unverified |
| Physical acceptance | Install/update through TestFlight on the owner's Mac/iPhone, exact version/build, accessibility/screenshots and real-provider/lifecycle evidence | Not performed |

A standalone app, archive, upload receipt or processed build must never be reported
as TestFlight availability or physical acceptance. The next delivery work is to
resolve existing app/account metadata and prepare the concrete release configuration,
while continuing ordinary source and fixture work. This plan authorizes no upload,
credential creation, agreement acceptance or security change by itself.
