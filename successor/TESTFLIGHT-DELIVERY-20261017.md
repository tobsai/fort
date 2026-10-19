# Fort delivery through TestFlight

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
