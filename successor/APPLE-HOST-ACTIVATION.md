# Apple host activation gate

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


## G2 CSR local; upload approval review remains open

October 19 continuation: the separately approved G2 workflow created one local
2048-bit RSA CSR through Apple Certificate Assistant. Its signature verifies; the
private key was not read or exported. Automatic approval review rejected uploading
the public CSR because it could not verify authorization for that specific identity/
public-key disclosure to Apple. The file picker was canceled and the portal still
shows no file selected. A precise user approval request is pending; do not retry
through a workaround. No G2 leaf was created or installed, and the four existing
signing identities remain. [Exact hashes, scope and failure](evidence/2026-10-17-node-foundation/g2-certificate.md).

The signed development package below remains unchanged and unlaunched. The separate
[static privacy inventory](evidence/2026-10-17-node-foundation/privacy-runtime-inventory.md)
covers all 31 Mac/iPhone binary objects; complete runtime disclosures, installed
App Group/XPC/provider behavior and TestFlight acceptance remain open.

## Approved provisioning completed; signed package remains unlaunched

October 19, 2026. The owner approved the exact consolidated scope at 17:41:48 UTC,
message `Sentinel_0027f013f6888191af17a943d64b6802`, answering request
`Sentinel_d35153d258108191af6a5a13a439515b` at 17:38 UTC. The coordinator supplied
both references. Do not request that provisioning approval again.

Both exact Mac App IDs and `group.io.mtree.fort.mac` are now registered, with only
the two approved app associations. Both development profiles were generated and
downloaded, then checked against the exact existing certificate, team, app IDs and
single registered Mac. The existing Fort iOS record and unrelated records remain.
[Complete provisioning and signing evidence](evidence/2026-10-17-node-foundation/mac-development-provisioning.md).

Current signed development artifact:
`/tmp/fort059-development-signed-20261019/Fort.app`.
Its report SHA-256 is
`20ed08cc2df56549234a63f09c3da9b1854cc05a08958cbdf18301cdaa03b559`.
All 206 files, 30 code signatures, strict nested verification, exact child/app
entitlements, profiles and Fort's native read-only bundle inspector passed.
The helper retains no outgoing-network entitlement. Neither app nor helper has
been launched; no service was registered and no provider credential was created.

The next action-specific gate is installed operation and any required host-network/
provider access, using the exact reviewed signed package and explicitly selected
account. This provisioning approval did not add those grants. Do not infer runtime
access from a signature, profile allowance or the successful read-only inspector.
TestFlight distribution remains separate. G2 replacement is separately approved;
the later local-CSR/upload-review status is recorded above. It is not a
development/TestFlight prerequisite.

The earlier request and inventories below are preserved as historical checkpoints;
this section supersedes their missing-profile/signing status.


## Apple access reconciled and minimum Mac provisioning request

October 19, 2026, after the owner's 17:14 UTC "Updated" message. The existing
Safari session is authenticated to Apple Developer and App Store Connect.
The portal confirms Maple Tree Enterprises LLC, team `T3JB5MYZ93`, with the
Account Holder session. No credential value was read or submitted.

The coordinator supplied Apple confirmation email `1a155282da6a6b05`, received
17:14:14 UTC, that the Program License Agreement was signed for this team. Its
version is not named. Do not request acceptance of that same agreement again.
App Store Connect Business independently lists Free Apps Agreement as Active,
October 19, 2026 through September 11, 2027. Paid Apps Agreement is Active
with New Agreement Available, September 12, 2026 through September 11, 2027.
The Schedule 2 update banner is separate; it does not by itself prove that a free
beta is blocked. No new agreement was accepted during this work.

Read-only inventory found:

- Existing Fort app record `6786748810`, named Fort Orchestrator, bundle ID
  `io.mtree.fort`, SKU `fort-ios`; observed iOS 1.0 is Prepare for Submission.
  Preserve it. No remote rename, platform addition or replacement is authorized
  by this record. No separate Fort Mac record was verified.
- The unfiltered App IDs list contains `io.mtree.fort`, `io.mtree.fort.watch` and
  `io.mtree.fort.watch.widgets`. It has neither proposed Mac identifier below.
  The App Groups list contains no group identifier or Fort group.
- The unfiltered Profiles page shows five iOS profiles, no Mac profile and no
  Fort profile. The older local cache's historical Fort iOS profiles are not
  evidence of a current compatible Mac profile.
- This Mac's provisioning UDID `00006001-000429402202801E` is already in Devices.
  No new device registration is needed. Existing Apple Development identity
  `58F40D1AB480565B89109B853235AC3A7B2E7AD1` remains the selected development
  signing candidate. No new development key is indicated.

The concrete remaining provisioning request is limited to team T3JB5MYZ93:

1. Register explicit Mac app IDs `io.mtree.fort.mac` and `io.mtree.fort.mac.host`.
2. Register `group.io.mtree.fort.mac` and associate only those two app IDs with it.
3. Create Mac development profiles `Fort Mac Development 20261019` and
   `Fort Host Development 20261019` for their matching IDs, the existing development
   certificate above and the already-registered exact Mac; download and use those
   profiles for the local signed build.

This scope needs one explicit approval because the repository's current authority
retains "Mandatory per-action security, credential/persistent-access approvals".
It changes persistent Apple provisioning. It does not create a key, alter an
existing app, accept an agreement or authorize host/network/provider/startup
activation. Prepare the resulting exact signed artifact before requesting its
separate installed-operation grant. TestFlight distribution profiles/app record,
archive/upload and physical acceptance remain later, distinct steps.

The already approved G2 Developer ID Application replacement remains valid and
separate from this request; no new certificate was created or installed here.
It is not a development or TestFlight prerequisite. Existing certificates remain.
The [new branded review package](evidence/2026-10-17-node-foundation/release-assets.md)
contains 202 verified files but is still unsigned, unprovisioned and unlaunched.
Historical sign-in/agreement blockers below describe earlier checkpoints and are
superseded by this current access record.

## Synthetic cold-launch diagnosis — source c906b00

October 19, 2026. The earlier windowless QA launch follows the harness argument
form. With unchanged production app source, explicit `--qa-account YES` followed
by `--qa-session UUID` passed five cold-launch and close/reopen trials; two ordinary
no-argument QA launches and a separate cold-launch AX/screenshot check also passed.
The original argument form still reproduces the failure. All exact QA processes
were normally stopped and independently confirmed absent. See
[the diagnosis and exact artifact](evidence/2026-10-17-node-foundation/mac-review-package.md#synthetic-cold-window-diagnosis--source-c906b00).

No production lifecycle fix was warranted by this evidence. The final signed app's
cold launch, host/provider behavior and TestFlight remain unverified. The production
review package is still unsigned/unlaunched; Apple sign-in, matching provisioning,
credentials, entitlements and activation scope are unchanged. No sign-in retry,
agreement acceptance or new permission occurred.

## Current signing and UI check — source 81938a4

October 19, 2026. [Exact metadata, package checks and synthetic UI evidence](evidence/2026-10-17-node-foundation/mac-review-package.md#signing-and-selected-account-ui-readiness--source-81938a4)
confirm four existing valid signing identities, including Apple Development through
September 29, 2027 and Apple Distribution through May 25, 2027 for team T3JB5MYZ93.
All 91 local profiles decoded (50 unique UUIDs); none authorizes a Mac client/helper
App Group for this team. The three historical Fort profiles are iOS-family only.
Matching Mac profiles/verified remote records are the immediate dependency. Creating
another development key is not indicated by this inventory. G2 Developer ID repair
remains separately approved/unexecuted and does not unblock TestFlight provisioning.

All 21 assembly checks passed; the unchanged 200-file review app and 198 inputs
matched their hashes. It remains unsigned, unprovisioned and unlaunched. The new
account-selection UI passed actual accessibility/screenshots in a separate ad-hoc,
synthetic-only QA app with unchanged production views; that app is now stopped.
First launch had no window and required ordinary reopen, so cold launch is unverified.

The smallest next user action is the existing secure Apple sign-in handoff, followed
by read-only verification/reuse of the exact Mac IDs, App Group and matching profiles.
Only confirmed missing records/profile associations should enter a consolidated
provisioning approval. No new credential, legal acceptance or activation is implied.
Final signing/installed-provider/TestFlight acceptance still requires the completed
artifact and exact grants. No independent packaging or existing UI check remains
identified for this bounded continuation; no product feature was added.

## Current account correlation implementation — after 28dfa3b

October 19, 2026. [Account correlation evidence](evidence/2026-10-17-node-foundation/account-correlation.md)
closes the nonsecret selected-account source gap. The existing adapter carries
reported account label/workspace ID; native controls require explicit primary-owner
selection. The ledger retains it across restart and pins accepted turns. The executor
and Codex adapter reject missing/changed identity before discovery or dispatch.
All account evidence is synthetic; no real credentials or account lookup occurred.
Earlier unresolved-source statements below are historical. Real-account and exact
signed-installation verification remain required.

1,326 portable and 328 native tests passed; 16 skips in each suite. The guarded
executor passed 19 tests with two opt-in skips; exact package refusal and isolated
selection p95 2.222 ms passed separately. Fresh unsigned Mac/helper/iPhone builds
and the 200-file package are verified. Current inert app:
`/private/tmp/fort059-account-correlation-review-20261019/Fort.app`, report SHA256
`f28747be6e5fb4ff8ba1088d7c8fc8e74582008d72b0804271a990e5ce7fb12d`.
No profiles, activation, new entitlement or release identity adoption occurred.
Do not seek activation approval for this unsigned artifact.

The thirty-minute/six-turn suggestion below describes only a possible bounded
verification session. It imposes no product restriction or pairing expiry.
The actual consolidated approval still needs the final signed artifact/path,
verified identities/profiles and real ChatGPT account/workspace/model. Nonsecret
identity pins remain in private metadata and retained turn history; local credential
cleanup does not erase that history or revoke provider-side grants.

Apple's public [Program License Agreement PDF](https://developer.apple.com/support/downloads/terms/apple-developer-program/Apple-Developer-Program-License-Agreement-English.pdf)
was read without acceptance: LYL260, October 13, 2026. Its correspondence to the
pending authenticated-account agreement is unverified. The existing G2 approval
remains valid, the secure Apple handoff remains pending and legal acceptance is
unapproved. Ordinary source/test work remains authorized.

## Current cleanup implementation — after 41b650a

October 19, 2026. [Provider-session removal evidence](evidence/2026-10-17-node-foundation/provider-session-removal.md)
now closes the local credential-removal source defect: ordinary uninstall preparation
requires confirmed host/runtime exit and startup unregistration, then removes only
Fort's private `Runtime/state/auth.json` using metadata-only no-follow operations.
A required provider-cleanup receipt precedes device-token cleanup and app handoff.
Unsafe files/directories are refused; failures remain retriable. History and other
runtime files are kept. This is local credential absence, not provider-side revocation.
Only disposable synthetic credentials were used; no real cleanup occurred.

326 native tests passed, 16 skipped; the isolated cleanup budget and exact package
probe passed separately. Fresh unsigned Mac/helper/iPhone Simulator builds passed.
The current 200-file review app is
`/private/tmp/fort059-provider-cleanup-review-20261019/Fort.app`, report SHA256
`dabd400861db883297722a8a647a383dd0d8af5c138ba5165822c703bea91caf`.
It is unsigned, unlaunched, contains no profiles and has unchanged engineering IDs
and outbound-network-disabled helper. Earlier packages and findings stay historical.

**Selected-account correlation remains unresolved.** The supported pinned protocol
has optional email and experimental workspace account ID, but Fort's current result
contract does not carry/compare them. Native signed-in status now explicitly labels
account identity unverified. An account read can perform routing discovery/auth
recovery despite `refreshToken:false`; no live lookup was attempted. The intended
ChatGPT identity must not be inferred from Apple login or historical demo grants.

The consolidated proposal below remains preparation only. Its credential-cleanup
implementation gap is superseded by this source result; actual signed-installation
cleanup, provider grant revocation, identity correlation and external activation
approval remain outstanding. Apple sign-in is still awaiting the owner. Existing
G2 certificate approval remains valid and unexecuted; the agreement is unapproved.

## Consolidated Mac demo grant review — source 7d528ba

October 19, 2026. Read-only trace of the packaged host, official-runtime account
flow, native startup and removal at `7d528baeb176d7827f60a7fa61fedb9c620b853c`.
This is an approval description for the coordinator to complete with verified
identities and artifact evidence. **It is not an approval or an activation request.**
The secure Apple/1Password handoff remains pending and was left undisturbed.

### Identities that must be resolved before asking

- Apple organization: **Maple Tree Enterprises LLC**, team **T3JB5MYZ93**.
  Selected Apple login: **tobias@mtree.io**; its current authenticated role is
  unverified. The earlier Admin session and named Account Holder do not prove this
  selected session's authority. Existing G2 certificate approval remains valid;
  the Program License Agreement remains unapproved.
- Intended Mac app/helper/group: **io.mtree.fort.mac**,
  **io.mtree.fort.mac.host**, **group.io.mtree.fort.mac** are proposals pending
  read-only record lookup. They are not the current engineering artifact's IDs.
  Reuse compatible existing records; request only verified missing registrations,
  group associations and matching profiles. No account role change is needed in
  the proposed scope. G2 Developer ID work and TestFlight distribution are separate.
- Provider: the bundled official **Codex 0.157.1**, source
  **36650394c5b38c2990ccf2a3457165ca3e9d9726**, provider `codex`, host `local-codex`,
  using an owner-selected existing **ChatGPT subscription**. The ChatGPT account
  email/workspace and exact model are **not yet established for this new grant**.
  The Apple email is not evidence of the ChatGPT identity. Do not reuse consumed
  demo grants or infer a model from historical sessions. The owner must identify
  the intended account, verify it in the official authorization UI, and select
  the exact available model before any chat turn.
- The final signed app's installation path, signatures, profiles, entitlement
  inventory and package digest must replace the current unsigned review-artifact
  reference in the actual approval. Public inspection checks profile presence,
  with `presentUnverified` evidence; profile contents, purpose, expiry, team and
  group agreement require separate inspection. No installation path is approved
  by this note.

### Minimum host and provider scope

| Grant | Concrete action and scope | Persistence / reversal |
| --- | --- | --- |
| Local host and App Group | Run the verified Fort client and its nested `Contents/Helpers/FortHost.app` as the current Mac user. Both use their own App Sandbox and the single verified Fort App Group. Node, guardian and Codex inherit that helper sandbox; no root daemon. | App Group storage and client cache survive quit/restart. Remove only the exact Fort installation through its verified lifecycle path; retain history by default. |
| Per-user background registration | One explicit `SMAppService.agent(plistName: "io.mtree.fort.mac.host.plist").register()` for the verified installation, if these proposed IDs are adopted. Honor any macOS `requiresApproval` prompt. | Registration persists until unregister. The packaged plist has only Label, BundleProgram and MachServices: no RunAtLoad or KeepAlive. It permits on-demand launch through `group.io.mtree.fort.mac.fort.host`; quit of the client does not revoke registration or prove the helper stopped. |
| Provider outbound access | Add `com.apple.security.network.client` to the exact standalone helper, currently disabled, then rebuild/sign/review that artifact. Permit the bundled runtime's official ChatGPT device authorization, model discovery and selected text requests. | This is an outbound-network entitlement, not a domain allowlist. The launch plan pins normal OpenAI routing and rejects endpoint overrides. Expected service paths include `auth.openai.com` and `chatgpt.com`; the actual runtime's network behavior still needs inspection. Remove the grant by stopping/unregistering and removing or replacing the exact signed app. |
| Fort-owned provider sign-in | Start one explicit device-code flow from Fort and have the owner authorize the named ChatGPT account at `https://auth.openai.com/codex/device`. No automatic browser opening or callback listener. Use no API key and copy no credential from another installation. | The official runtime writes its credential into Fort's private `Runtime/state/auth.json` and may refresh its session during later authorized use. It survives process restart. Canceling a sign-in attempt does not revoke a login that already completed. Provider credential removal is a separate unresolved operation below. |
| Primary local pairing and demo | Create the primary device token in this app's Keychain namespace; authorize only native launch, plain-text chat, quit/relaunch, explicit runtime stop/resume and reconnect to the same installation. A proposed attended acceptance limit is 30 minutes and at most six short text turns, followed by confirmed shutdown/unregistration; this is a proposed operator limit, not a built-in expiry. | Pairing has no expiry by default. It therefore needs explicit revocation at cleanup. A disconnect/re-pair check requires new explicit local pairing while preserving history; it does not authorize another provider account or another device. |

The minimal Mac path uses signed Mach XPC, a private pipe and a private Unix socket
`<App Group>/FortHost/core.sock`. The ordinary bootstrap omits optional network
control: no TCP/LAN listener, phone pairing, cloud tunnel or shared hosting is needed.
No incoming-network entitlement is requested. No routines or schedules need to be
created for this acceptance journey.

The agent host and computer-tool policy are different boundaries. The helper's
App Sandbox plus its explicit outbound entitlement contain the host and inherited
processes. Codex's `fort_candidate_b_no_files` profile separately denies computer-tool
filesystem access and disables tool networking. Shell, browser/computer tools,
plugins, hooks, MCP, cloud skills, remote control, telemetry and upstream auto-update
are disabled and checked. No workspace/file picker, Full Disk Access, Accessibility,
screen capture, microphone or sensitive-account grant is part of this proposal.
The source controls and earlier synthetic probes do not prove confinement of the
final signed provider session; inspect that exact installation after approval.

### Stored data and removal limits

Host storage resolves through Apple's URL for the verified group, then `FortHost`.
The private ledger is `openai-candidate-b.sqlite` with its SQLite sidecars. Runtime
directories are `Runtime/home`, `Runtime/state`, `Runtime/workspace` and `Runtime/tmp`,
each checked as private owner directories. Codex uses only these selected HOME,
CODEX_HOME and TMPDIR values with a restricted environment. The normal client uses
its system Application Support directory's `FortCandidateB` child for connection
metadata, pending work and cached history. Actual sandbox-resolved paths must be
recorded at installation; do not guess an unsandboxed user-library path.

The primary token is stored as a this-device-only Keychain generic password under
service `Fort.Spec058.Device`, account `<installationID>/<deviceID>`. Its name is a
historical internal namespace, not a license to adopt another installation's token.
Evidence records identifiers and credential classes only, never token or auth-file
contents. Provider requests transmit the owner-selected conversation/context; local
uninstall cannot undo delivery or promise deletion from the provider's systems.

The source removal path confirms exact owner revocation, runtime disposal,
SMAppService unregistration and helper exit before deleting the local pairing
credential. It then prepares the exact app URL for removal. Unconfirmed steps keep
the recovery journal and block a success claim. History stays unless separately
confirmed deletion is chosen.

**Two review limits must remain visible:**

1. `codex-account.ts` and `codex-chat.ts` verify the account **type** is ChatGPT;
   they do not expose or pin an email/account/workspace ID. Native sign-in status
   therefore cannot independently prove that the selected human account is the one
   authenticated. Before use, verify the exact account in the official device
   authorization UI and establish how that identity will be correlated with the
   private session. No credential inspection is authorized as a shortcut.
2. The native removal callback deletes the **Fort pairing token**, not
   `Runtime/state/auth.json`. No current production `account/logout` operation or
   provider-credential cleanup was found. Host/client history deletion also keeps
   runtime sessions. Therefore full credential-free uninstall is **not implemented
   or proven**. Before a cleanup-complete demo is approved, prepare and test the
   smallest exact private-session sign-out/local credential-removal action with
   confirmed process absence and history preservation. State separately any provider
   grant that needs revocation; never claim remote revocation from local deletion.

The next concrete approval must include the resolved Apple/app/group/profile and
ChatGPT identities, signed artifact/path, the proposed attended limits, the outbound
host entitlement, primary token/provider-session persistence, per-user registration
and its exact tested cleanup. The current facts do not yet support that complete
approval. The security/credential/persistent-access requirement is explicit in
[CLAUDE.md](../CLAUDE.md): "Mandatory per-action security, credential/persistent-access
approvals, legal agreements and destructive-action authority remain necessary."
This turn made no provider/Apple mutation, credential access, entitlement change,
registration, app launch, legal acceptance or fresh runtime test. Source review is
the new evidence; prior passing tests and unsigned package checks are unchanged.

## Current Mac demo handoff — October 19, 2026

Current source `1f3cda5dc473b4960c397b4a9a20915d2a262d64`; prioritize installed
Mac launch/actual-provider chat/restart/reconnect, then TestFlight, ahead of phone UI.
[Fresh exact package and checks](evidence/2026-10-17-node-foundation/mac-review-package.md#mac-demo-readiness--source-1f3cda5)
record 1,308 portable passes, 317 native passes, fresh Release client/helper builds
and a 200-file unsigned review app. No embedded profile, startup activation or
actual provider session exists for that package. Public signature inspection refuses
it; the synthetic signature check reaches the missing-profile gate.

Read-only Apple access previously confirmed Maple Tree Enterprises LLC, team
`T3JB5MYZ93`, role Admin, with Account Holder Tobias Gunn. Developer ID Application
creation was disabled. The owner selected `tobias@mtree.io` in source message
`Sentinel_abde69899a008191b033d31fb1194848` (October 19, 00:38:56 UTC), relayed by the
parent. The existing Admin browser session was signed out using ordinary Sign out,
without Trust and sign out. The selected account identifier was verified exactly.
The existing unlocked 1Password item was selected and secure Autofill invoked, but
Apple's password-step Sign In remains disabled. The secure sign-in/device-verification
handoff is pending. No password was read, revealed, copied or transmitted by tools.
The selected account's current role is **not yet verified**.

The separate approved G2 Developer ID Application action remains pending, preserving
all existing certificates and keeping its private key on the signing Mac. No key,
CSR or certificate has been created/installed/exported/revoked. The earlier account
banner reports an updated Program License Agreement; it remains unaccepted and needs
its verified link plus separate approval before acceptance. Xcode's system-component
Install prompt remains untouched. G2 direct distribution is separate from TestFlight.

Next: finish secure Apple sign-in, verify the exact existing Mac app/helper/group
records, then request only any confirmed missing provisioning changes. The proposed
release IDs below remain unadopted until that lookup. After signing/provisioning,
present exact helper outbound-network, provider session and per-user startup scope
for approval before installed execution. Existing certificate approval does not
authorize those grants. The current helper still has outbound networking disabled.
The source provider-account/native controls are implemented; the older "not
implemented" statements below describe their historical checkpoints only.


October 18, 2026: **not provisioned, registered or activated**. Governing design:
[Spec 059](../specs/059-modular-node-v1.md) and its
[owner decision](../docs/architecture/decisions/059-node-foundation.md).
Source checkpoint: `9c3d17d06e9010147c2448be1a25c6962de0efbd` on
`codex/fort-openai-rebuild-20261016`. This records the missing external steps;
it does not authorize them. Independent implementation continues. The current
source/package checkpoint is `1b08c224a497974f09c39b3af9c1f99c4be39cc0`; see the
[190-file review package and Mac test path](evidence/2026-10-17-node-foundation/mac-review-package.md#current-mac-test-milestone).
Its public inspector correctly refuses unsigned code. No profile was fabricated.

Read-only recheck on October 18: the existing App Store Connect window still shows
sign-in text and an email-entry field. Existing accessibility permission was used
to read labels only; no field value was read or control pressed. Current team,
role, app/helper/group records, profiles and agreements remain unverified. The owner
sign-in question is pending; this is an access prerequisite, not evidence that the
records are missing. No credentials, new access or Apple settings were changed.

## Exact identities and missing profiles

Current clients use engineering IDs `io.fort.spec058.fixture.mac` and
`io.fort.spec058.fixture.ios`; QA uses `io.fort.qa.firstwave.20261017`.
None is verified as the intended App Store Connect release identity. Existing
signing metadata verifies team **`T3JB5MYZ93`**, not its current account role,
app records, enabled capabilities or agreements.

The historical delivery note references the two client IDs below. This is a
**proposed identity set for account verification only**. These values have not been
adopted in release configuration, registered, granted or remotely verified.

| Component | Exact proposed identifier | Evidence / missing item |
| --- | --- | --- |
| iPhone client | `io.mtree.fort` | Historical reference and local iOS Store profile; current app record/build/testers unverified |
| Mac client | `io.mtree.fort.mac` | Historical reference; current Mac app record and matching Mac profile unverified |
| Nested Mac agent | `io.mtree.fort.mac.host` | New proposal; no registration or matching local Mac profile verified |
| Shared Mac App Group | `group.io.mtree.fort.mac` | New proposal; no matching local entitlement/profile verified |
| Derived Mach service | `group.io.mtree.fort.mac.fort.host` | Derived from that group; no service registered or connected |
| Derived LaunchAgent label/plist | `io.mtree.fort.mac.host` / `io.mtree.fort.mac.host.plist` | Proposed bundle-local configuration; no job or plist installed |

The October 18 [local inventory](evidence/2026-10-17-node-foundation/mac-agent-boundary.md)
decoded 91 profiles. Fort matches were the historical iOS app, Watch app and Watch
widget, all without App Groups. No Fort Mac/agent profile was found. Local absence
does not establish remote absence. If compatible helper/group records already exist,
record their exact values and review the identity set before configuration; do not
create duplicates or silently adopt a historical identity.

## Required package agreement

Both Mac client and agent need exact code identities under the verified team and
matching profiles authorizing the same `com.apple.security.application-groups` value.
For this proposal, profiles must authorize application identifiers
`T3JB5MYZ93.io.mtree.fort.mac` and `T3JB5MYZ93.io.mtree.fort.mac.host`, using the
actual macOS entitlement keys, with group `group.io.mtree.fort.mac`. Inspect team,
identifier, group, purpose, expiry and signing match in the actual profiles.

The standalone per-user agent needs its **own** App Sandbox and App Group entitlements;
it cannot inherit the foreground client's sandbox just because it is bundled there.
Node and the guardian launched by that agent retain only App Sandbox plus sandbox
inheritance. Prevent debugger/base-entitlement injection into those inherited tools.
Provider/network and additional file-access capabilities remain separate reviews.

Proposed layout:

- Helper: `Fort.app/Contents/Helpers/FortHost.app`, with its own
  `Contents/embedded.provisionprofile`; the parent app also needs its own profile.
- LaunchAgent plist:
  `Fort.app/Contents/Library/LaunchAgents/io.mtree.fort.mac.host.plist`.
- Bundle-relative program: `Contents/Helpers/FortHost.app/Contents/MacOS/FortHost`.
- History: `<Apple's URL for group.io.mtree.fort.mac>/FortHost`, resolved through the
  system API and storage preflight. A returned URL alone is not entitlement evidence.

This is a per-user agent, not a root daemon or a separately published helper product.
The background helper, native lifecycle and runtime bootstrap now compile and the
complete unsigned payload is assembled. Signed package/profile integration and
actual runtime acceptance remain. The strict no-extended-ACL storage policy
needs compatibility evidence from the actual provisioned container; do not repair
unknown ACLs or delete retained history. The iPhone needs no Mac App Group merely
to pair over its future network transport; this group grants no cross-device access.

Sources: [Apple's named-XPC/profile guidance](https://developer.apple.com/forums/thread/835003),
[App Groups](https://developer.apple.com/forums/thread/721701),
[bundle locations](https://developer.apple.com/documentation/bundleresources/placing-content-in-a-bundle),
[container API](https://developer.apple.com/documentation/foundation/filemanager/containerurl(forsecurityapplicationgroupidentifier:))
and the installed SDK's `SMAppService.h`. These describe platform requirements,
not this account's authorization or approval of the proposed names.

## Smallest external action, in order

1. **Read-only account check.** An owner/admin with existing authorized Apple access
   verifies team `T3JB5MYZ93`, the two historical client records, and whether the exact
   helper/group proposals already exist. Retain only identifiers, platform, capability
   membership, role and profile purpose/expiry. The existing browser remains at
   sign-in and no cached authorized CLI session is available. The owner must sign in
   through the existing account before the permitted read-only check can continue.
   No API key is needed for that UI check; do not send credentials in chat.
2. **Conditional provisioning approval.** Reuse matching existing records/profiles
   when available. Otherwise, the smallest proposed mutation is to register only the
   missing verified Mac client/helper IDs and shared group, associate the group with
   those two IDs, and generate matching development profiles using existing signing
   material for the test Mac. The new proposed values are `io.mtree.fort.mac.host`
   and `group.io.mtree.fort.mac`; client reuse must be verified first. This has not
   been approved or performed. Role changes, keys/certificates, agreements and
   distribution changes require separate scope.
3. **Local activation approval, later.** First finish and verify the actual signed
   package, profiles, hashes, entitlements, program path and cleanup behavior. Then
   present one bounded per-user
   `SMAppService.agent(plistName: "io.mtree.fort.mac.host.plist").register()` action
   for that exact installation. If macOS reports `requiresApproval`, the owner must
   approve its background item in system UI. Do not bypass that gate with launchctl,
   manual LaunchAgents files or permission changes. Registration status does not prove
   helper health, pairing revocation or descendant absence.

The per-action security/persistent-access requirement comes from
[CLAUDE.md](../CLAUDE.md). **No approval request is pending here:** the production
package is not yet ready for activation review. No provisioning/account mutation or
registration occurred. These Apple actions would not authorize provider sign-in,
credential copying, public ingress, another device's access, deployment or upload.

## Local code and verified checkpoint

Earlier local preparation added a background-only engineering helper target,
`io.fort.spec058.fixture.mac.host`. It is not a release identity or a registered
capability. That initial standalone build lacked the runtime payload. The current
composed review app now contains manifest, Node, guardian and bootstrap, but still
has no embedded profile or valid final signature. No activation request is ready.
The helper still has outgoing networking disabled and native provider sign-in is
not implemented; these are separate prerequisites for real-provider chat.
Historical source/tests/build evidence is in
[helper program preparation](evidence/2026-10-17-node-foundation/mac-host-program.md)
and the [v1 ledger](V1-ACCEPTANCE.md); the initial checkpoint table below remains
historical evidence rather than the latest build inventory.

Independent work includes wrapper/configuration and signature validation, exact
owned-installation cleanup, provider adapter containment, synthetic recovery tests
and engineering builds. Actual named-agent/App Group execution waits for matching
identities/profiles; persistent registration waits for its bounded approval. Provider
acceptance, reachable iPhone transport and TestFlight delivery remain separate gates.

Client evidence is from `51197365210f643d45a5eb73519c11dcb4eac7f8`; client product
source is unchanged by `9c3d17d`:

| Check | Actual result |
| --- | --- |
| Native regression | 173 functions executed and passed; 2 timing functions excluded, new storage timing passed separately |
| Fresh unsigned Mac Debug build | Passed: `/tmp/fort059-agent-storage-mac-build/Build/Products/Debug/FortMac.app` |
| Fresh unsigned iPhone Simulator Debug build | Passed: `/tmp/fort059-agent-storage-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app` |
| Simulator UI | Unrun; CoreSimulator 1171.7.0 is older than required 1174.9.2 |
| Latest packaging slice | 6 affected checks and strict TypeScript passed; retained Codex version-only artifact independently verified |
| Installed activation, accessibility/screenshots, real provider and TestFlight | Unrun or unverified; no completion claim |

See [storage evidence](evidence/2026-10-17-node-foundation/mac-agent-storage.md),
[Codex packaging evidence](evidence/2026-10-17-node-foundation/codex-bundle-version.md)
and the [v1 ledger](V1-ACCEPTANCE.md). Historical specs, unrelated work, consumed
demo grants and the unresolved full-provider-closure status remain preserved.
