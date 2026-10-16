# Spec 058 successor

Spec 058 implementation was approved by Toby on 2026-10-12. This directory is a
new isolated successor, separate from both historical Fort and quarantined donor.
Read ../specs/058-native-first-owned-agent-mvp.md and IMPLEMENTATION.md.

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
and the completed native cache archive. Preparation must not start this trial.
Read runner/STARTUP-CONTRACT.md and runner/PERMISSION-PROFILE.md before runtime
work. The nonlive migration replaces the rejected legacy restricted-read selector
with a named macOS profile denying all computer-tool file reads/writes and tool
network access. Source/fixture contracts and one real native text turn passed;
complete OS enforcement remains unverified. Do not weaken that profile, enable tools/environments or treat its
identity/legacy sandbox projection as a complete effective-policy attestation.
The current manual selects only owner-trial-20261016 and its sealed source/app pins.
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
