# Spec 058 successor

## Current handoff — architecture reassessment

Read the root [directives](../CLAUDE.md), [constitution](../ARCHITECTURE.md) and
[assessment](../docs/architecture/reassessment-20261017.md). Feature and TestFlight
expansion are paused at `e02eb571e66620e407a360584817616074730b55` while a new design
is researched/reviewed. Product source remains `09788c132647458cbdc683506c8bd5622f0be1af`.
[Spec 058](../specs/058-native-first-owned-agent-mvp.md) now needs reassessment;
its four blocked principles are negative controls for the review, not accepted
architecture. Do not continue a historical handoff as current implementation authority.

Run `npm run check:specs` and `npm run test:specs` here for the structural gate.
Before any later product implementation, require the full check plus the relevant
spec's accepted `--implementation` gate and the actual human design decision.
Before committing a product slice, also run that gate with `--staged`; the spec's
reviewed `Implementation paths` must cover every staged change.
Passing a form does not establish vendor independence, removability, speed or
installed-product correctness. No tests/builds were left running at the pause.

Preserve all prototype code, engineering artifacts, old decisions and consumed
demo ledgers. No new credentials, provider access, persistent service activation,
cleanup, runner setup, distribution or release is authorized by this reassessment.
The handoffs below are chronological evidence, subject to this current pause.

## Previous handoff — interactive engineering QA, review fixes and TestFlight

Read [the interactive QA handoff](evidence/2026-10-17-interactive-qa/README.md)
and [TestFlight delivery plan](TESTFLIGHT-DELIVERY-20261017.md). Current source is
`09788c132647458cbdc683506c8bd5622f0be1af`. The dedicated `FortQA` target uses
`io.fort.qa.firstwave.20261017`, a labeled interactive fixture and temporary session
storage. The exact-path launcher preserves a QA session across relaunches. Normal
setup is excluded from that target; no credential, provider or network path is enabled.
Use this unique identity for engineering UI inspection, not the old shared Mac ID.
The known old read-only preview PID 29922 was verified and stopped; other apps/data
were not removed. Screenshots/accessibility of the new QA artifact remain unverified.

120 service/runtime tests and 56 native tests pass. QA, Mac and iPhone Simulator
builds plus Release FortKit compilation pass. Review corrections cover management
retry after failed runtime disposal, nonblocking FIFO rejection, image cache reuse,
JPEG/orientation/animation/pixel/capacity coverage and persisted disconnect intent.
Lost replies/expired credentials remain explicitly unconfirmed until verified
recovery; local forget never claims revocation. The owned-host recovery/removal
adapter is still only a protocol with fixture implementations, not installed.

The owner now explicitly requires **TestFlight**, not standalone user delivery.
Local apps are engineering artifacts only. Apple Distribution signing metadata for
team T3JB5MYZ93 exists, but the successor's real app IDs/profiles/App Store Connect
records, access, versions and tester groups remain unverified. Do not reuse the
historical Developer ID Mac export path as TestFlight delivery. Archive, upload,
processing, tester availability and physical acceptance must be reported separately.
No new credentials, agreements, security changes, persistent activation, actual app
removal, provider cleanup or publication are authorized by this preparation. Keep
all prior reviewed commits, demo ledgers, pins and unrelated work.

## Earlier handoff — first-wave source and fixture lifecycle

The first-wave source is committed through
`1a3e5bdc4902ba7fdf4c936730174754f6a6b2e6`; its settings/image predecessor is
`60675b5fd8b4cc6c9e86cca188ee684628c84e0a`. Read
[FIRST-WAVE-20261017.md](FIRST-WAVE-20261017.md) and
[the evidence](evidence/2026-10-17-first-wave/README.md).
119 service/runtime tests, 50 Swift tests, strict typechecking, fresh Mac/iPhone
Simulator builds and signature checks pass. Installed screenshot/accessibility,
file-picker behavior and actual model discovery remain open. The new Mac build is
ad-hoc signed and has no user-selected-file entitlement; do not change permissions
or treat fixture image tests as installed file-picker acceptance.

The owner explicitly changed pairing to **no default expiry**, with optional expiry
and easy disconnect/uninstall. The thirty-day proposal is superseded. The new
owner-local policy and stop/revoke/removal ordering are fixture-tested only; no
persistent setup has been activated. The real owned-host launcher/removal adapter
is still absent and uninstall is disabled in ordinary native construction. Keep
history by default; deletion requires a separate explicit choice. Confirm exact
persistent access scope once when setup is concrete. No login startup, other-device
access or tool/network expansion is authorized by this preparation.

The coordinator relayed that the owner reported the separate demo working at
21:24 UTC. That is basic chat acceptance, not first-wave/MVP signoff. Its preparation
notes below are historical and do not authorize a replay; this implementation turn
has not independently verified its final cleanup. Preserve all demo records,
credentials' historical lifecycle evidence and old source/app pins. Do not start,
reenroll, reset or reseal any historical packet. Continue ordinary isolated source,
fixture and build work; no new provider/account access, persistent credential,
security setting, deployment or provider cleanup is authorized here.

## Historical preparation — separate approved interactive owner demo

The owner approved one new ten-minute interactive session at 2026-10-17
21:00:19 UTC, message `Sentinel_f671f601a3148191bae54d2de3b281c1`. Read
[the operator manual](ops/owner-demo-20261017/MANUAL.md), scope and approval
there. The source thread designated fresh UI operator
`01a14bac-6acb-70be-926a-23ab4584a0f1` after the prior operator lost CUA tool
exposure; no denial or permission change was involved. Only that operator starts
the new packet, when ready to pair and hand the normal composer to the owner.
Preparation does not start the clock, provider runtime or enrollment.

The separate root is `.local/fort-owner-demo-20261017` in the primary checkout.
One start, one runtime launch, no restart, twelve plain-text requests maximum,
540 seconds work and 60 seconds cleanup. Existing ChatGPT and exact Astra only;
unchanged no-tool profile. The launcher opens the current Mac build without the
read-only preview flag. Longer use needs separate duration/lifecycle authority.
The nine consumed packets and every historical approval/pin stay preserved.
Historical activation source pins are intentionally not updated to this revision.
The first fresh operator invocation failed before the root/session directory was
created. All native/cache evidence stayed unchanged; no runtime, enrollment or
session clock started. At that point the action remained unused. Read the startup-repair evidence
and current manual: use supported permission escalation for --check-writes and the
attended launcher from the operator workspace. A permission refusal before durable
session creation is repairable under the same approval, not a consumed credential.


## Historical handoff — completed batch and offline preview

Current native source: `6fe7120919b660ffc030521839cba8a8ba23706e`.

All three owner-approved Mac sessions are consumed. Session 3 finished with six
accepted turns: five completed and one canceled without a reply or replay. It used
two runtime launches and one explicit restart. Across the batch: ten accepted,
nine completed, one canceled. Sessions 1 and 2 retain their original blocked
records and separate cleanup reconciliations. Session 3 records confirmed native,
runtime and credential cleanup. No allowance, deadline, ledger or approval was reset.

The UI operator observed supported app quit, confirmed native exit, service/runtime
restart and retained history without reenrollment, followed by a new completed
Reviewer turn. Writer and Reviewer used the same shared-memory revision while their
Homes stayed separate. This does not prove user-initiated cancellation: the final
checklist completed before the operator could cancel it.

The saved session-3 transcript has eleven nonempty messages. Its old view placed
all run statuses after all message bodies. A failing presentation regression
reproduced that ordering; statuses now follow each request's last message without
reordering interleaved messages or losing approval controls. A Debug-only read-only
offline preview uses synthetic data, a separate temporary directory and no connection.

Current evidence: 91 fixture service/adapter tests and 40 Swift tests pass; fresh
Mac and iPhone Simulator builds and strict signature checks pass. Mac UI automation
failed before test execution while enabling automation mode. Corrected installed
screenshots/accessibility remain unverified. The completed live batch used native
source `996e19849f5104e71b26b7c14d201aaa56b31c87`; the new UI fix has no live signoff.
Read [demo readiness](evidence/2026-10-17-mvp-demo-readiness/README.md).

Do not start another live session, reenroll, reset a consumed packet or reseal the
historical activation pins to the new native source. Routine local implementation,
fixture tests and offline preview preparation remain authorized. Skill B invocation
and export, actual-provider routines/schedules, deliberate user cancellation,
physical iPhone, effective-model reporting and broader lifecycle/isolation/release
acceptance remain open. No full MVP completion is claimed.

Historical context: Spec 058 implementation was approved by Toby on 2026-10-12. This directory is a
new isolated successor, separate from both historical Fort and quarantined donor.
Read ../specs/058-native-first-owned-agent-mvp.md and IMPLEMENTATION.md.

## Historical work — October 17 MVP expansion

The authorization described in this section is superseded by the architecture
reassessment pause above; it remains here only to explain the earlier work.

The owner asked at 2026-10-16 23:47:34 UTC: “Can you test the app? I want you to
build this thing until it gets to the Mvp”. The coordinator delegated end-to-end
Spec 058 implementation here. Continue routine code, synthetic tests, native
builds and local commits through the MVP milestones; the historical single-sample
and owner-typing limits below do not restrict that development work.

For the current acceptance gaps and repeatable live-test proposal, read
[MVP-20261017.md](MVP-20261017.md). Coordinate installed UI execution through the
existing operator `01a1451b-81d7-76cf-8e5c-a85d7164201c` via the parent; preserve
one implementation owner. Native iPhone and Mac, complete reviewed skill bundles,
native routines and durable schedules remain required. Spec 058 defers Groups and
agent delegation; preserve existing experiments without expanding that scope.

Every historical packet below is consumed, including the owner-trial replacement:
its observed state is `finished`, one start, zero receipts, credential cleanup
`deleted`. Preserve all six ledgers/state/evidence packets and the native cache.
See [the outcome record](evidence/2026-10-17-mvp-recovery/owner-trial-outcome.json)
for byte hashes and the distinction between observed state and operator timing.
The original preparation records remain historical evidence.

The historical schedule checkpoint is `ced1cc6ad74c6866c9fee353c8ce42b27e1ef1c6`;
read evidence/2026-10-17-mvp-schedule-recovery/README.md before further validation.
Its earlier native build source was `8a877fff2730d7dd99bb45b9f47e31158182383c`; its
evidence is in evidence/2026-10-17-mvp-pending/README.md.
Pending request cancellation now survives lost responses and client/service restart.
Native routines, schedules and complete instruction-bundle authoring have synthetic
and build evidence. Installed/provider acceptance remains open. The three-session approval was later granted and has now been fully consumed; it
is retained as historical authority, not permission for another live session.

Expired service enrollment is not renewed by this development authorization.
Identify the exact temporary credential scope before another live test. Keep
provider sign-in, API billing, persistent credentials, startup registration,
security changes, public ingress and deployment outside this local increment.
The agent host still has the Mac user's access; tool sandbox settings do not prove
host isolation. Continue independent implementation while those live gates wait.

## Historical bounded-session authority

The bounded Candidate B test was approved on 2026-10-16, including its minimal
activation revision; see evidence/2026-10-16-candidate-b-activation/approval.json.
Its first authorized start failed closed with zero receipts. That original packet
remains blocked and must not be retried, reset or adopted by another attempt.
The separately approved replacement failed at features/memory_tool. The third
attempt, approved at 20:17 UTC on 2026-10-16, failed at features/unified_exec.
All three starts are consumed, with zero receipts; preserve their ledgers,
state and evidence without resetting or adopting them. Historical manual packets
are not current execution authority. At 21:29 UTC on 2026-10-16 the owner approved
one separate fourth attempt after the startup/profile fixes and offline canary.
See evidence/2026-10-16-candidate-b-fourth/approval.json and
ops/candidate-b-fourth-20261016/MANUAL.md. Only UI task
01a1451b-81d7-76cf-8e5c-a85d7164201c executes it: one start, no restart,
five minutes total including cleanup, one exact Writer Home sample with no skill,
existing ChatGPT sign-in and exact gpt-6-astra, automatic local UI pairing.
That fourth attempt is now consumed and finished: one completed receipt, visible
`cedar`, scoped cleanup reported deleted. The designated UI task completed turn
01a146a9-2d59-729b-8481-21afbc8eb16b. Read
evidence/2026-10-16-fourth-native-acceptance/README.md for read-only receipt/pin/
residue verification and limits. Preserve all four state/ledger/evidence packets.
Do not restart the fourth packet or enroll/send in any consumed packet.
The owner separately approved a fresh owner trial at 21:52 UTC on 2026-10-16.
See evidence/2026-10-16-candidate-b-owner-trial/approval.json and
ops/candidate-b-owner-trial-20261016/MANUAL.md. Only designated UI task
01a1451b-81d7-76cf-8e5c-a85d7164201c may start it once: ten minutes total,
with the last minute reserved for automatic shutdown/cleanup, up to five
owner-entered plain-text messages in one Writer Home, no skill, the same existing
ChatGPT/gpt-6-astra setup and unchanged permission/host limitations. The operator
pairs automatically and immediately notifies the parent of readiness and exact
work cutoff/credential expiry. Only the human types/sends the messages; no operator
sample or corrective prompt may consume the budget. Preserve all four prior ledgers
and the completed native cache archive. That owner-trial start is now consumed:
it failed before READY at session/runtime-launch, with zero receipts and cleanup
reported deleted. The generated 540-second work plan exceeded the owned launcher's
unchanged 300-second ceiling. Preserve its ledger/state alongside the prior four.
The coordinator authorized a narrow timer repair and replacement preparation under
the same ten-minute/five-owner-message scope, with no permission changes.
See evidence/2026-10-16-candidate-b-owner-trial-replacement/README.md and
ops/candidate-b-owner-trial-replacement-20261016/MANUAL.md. Preparation must not
start it; only the designated UI task may execute on the coordinator's handoff.
The replacement has a separate fixed identity/root; never reset the consumed trial.
Read runner/STARTUP-CONTRACT.md and runner/PERMISSION-PROFILE.md before runtime
work. The nonlive migration replaces the rejected legacy restricted-read selector
with a named macOS profile denying all computer-tool file reads/writes and tool
network access. Source/fixture contracts and one real native text turn passed;
complete OS enforcement remains unverified. Do not weaken that profile, enable tools/environments or treat its
identity/legacy sandbox projection as a complete effective-policy attestation.
The current manual selects only owner-trial-replacement-20261016 and its sealed
source/app pins. The generic owned-process ceiling remains 300 seconds. Only this
exact packet budget admits up to 540 seconds of work in a 600-second total, retaining
a 60-second cleanup reserve and unchanged two-second TERM/KILL grace limits.
Any persisted owner-trial state prevents another start, even after a failure.
Preparation/tests must not start the live runtime or read credentials/settings.
The actual offline CLI parser accepted the profile; the separate deny-rule canary
passed pathname denials but not descriptor revocation, and nested sandbox application
was blocked. Full isolation remains unproven; the agent host has the Mac user's access.
Do not broaden the profile or weaken any startup/cleanup check to make the test pass.
No persistent settings, OS/network/security configuration or credentials were changed
by preparation. The separate owner-trial approval permits only its fresh packet;
no permission to reset any packet or broaden runtime controls is implied.
Broader provider activation, external account enrollment, permanent services and
public ingress still require their exact authority and acceptance gates.
No imports from ../foundation or old Fort implementation belong in this graph.
Native iPhone and Mac are required alongside the versioned service contract.
Run targeted behavioral tests before claiming progress; distinguish Swift HTTP
conformance, native builds, Simulator UI, physical devices and real providers.
