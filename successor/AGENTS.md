# Spec 058 successor

Spec 058 implementation was approved by Toby on 2026-10-12. This directory is a
new isolated successor, separate from both historical Fort and quarantined donor.
Read ../specs/058-native-first-owned-agent-mvp.md and IMPLEMENTATION.md.

The bounded Candidate B test was approved on 2026-10-16, including its minimal
activation revision; see evidence/2026-10-16-candidate-b-activation/approval.json.
Its first authorized start failed closed with zero receipts. That original packet
remains blocked and must not be retried, reset or adopted by another attempt.
The owner approved one separate five-minute replacement at 19:34:56 UTC on
2026-10-16. See ops/candidate-b-replacement-20261016/MANUAL.md and
its linked approval/pins. The replacement has its own fixed directory, permits
one start and the same five exact samples within one session, and cannot restart.
Only the designated UI task executes it; preparation/tests do not launch it.
Preserve original evidence, exact runtime controls and all failure stop checks.
Broader provider activation, external account enrollment, permanent services and
public ingress still require their exact authority and acceptance gates.
No imports from ../foundation or old Fort implementation belong in this graph.
Native iPhone and Mac are required alongside the versioned service contract.
Run targeted behavioral tests before claiming progress; distinguish Swift HTTP
conformance, native builds, Simulator UI, physical devices and real providers.
