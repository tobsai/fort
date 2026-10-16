# Spec 058 successor

Spec 058 implementation was approved by Toby on 2026-10-12. This directory is a
new isolated successor, separate from both historical Fort and quarantined donor.
Read ../specs/058-native-first-owned-agent-mvp.md and IMPLEMENTATION.md.

The bounded Candidate B test was approved on 2026-10-16, including its minimal
activation revision; see evidence/2026-10-16-candidate-b-activation/approval.json.
Its first authorized start failed closed with zero receipts. That original packet
remains blocked and must not be retried, reset or adopted by another attempt.
The separately approved one-start replacement also failed closed with zero
receipts, at features/memory_tool. Both consumed attempts remain blocked and
must not be reset or reused. The owner approved one separate third attempt on
2026-10-16 at 20:17 UTC after the proven canonical-feature compatibility fix.
See ops/candidate-b-third-20261016/MANUAL.md and the linked approval/pins.
Only the designated UI operator runs it: one start, five exact samples, no
restart, 292 seconds work plus eight seconds cleanup. Preserve both old ledgers,
all prior evidence and the canonical explicit-false checks. Preparation/tests
must not start the live runtime. No further attempt is authorized by this record.
Broader provider activation, external account enrollment, permanent services and
public ingress still require their exact authority and acceptance gates.
No imports from ../foundation or old Fort implementation belong in this graph.
Native iPhone and Mac are required alongside the versioned service contract.
Run targeted behavioral tests before claiming progress; distinguish Swift HTTP
conformance, native builds, Simulator UI, physical devices and real providers.
