# Spec 058 successor

## Current handoff — completed batch and offline preview

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

Spec 058 implementation was approved by Toby on 2026-10-12. This directory is a
new isolated successor, separate from both historical Fort and quarantined donor.
Read ../specs/058-native-first-owned-agent-mvp.md and IMPLEMENTATION.md.

## Current work — October 17

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
