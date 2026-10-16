# Spec 058 successor

Spec 058 implementation was approved by Toby on 2026-10-12. This directory is a
new isolated successor, separate from both historical Fort and quarantined donor.
Read ../specs/058-native-first-owned-agent-mvp.md and IMPLEMENTATION.md.

The bounded Candidate B test was approved on 2026-10-16, including its minimal
activation revision; see evidence/2026-10-16-candidate-b-activation/approval.json.
Its first authorized start failed closed with zero receipts. That original packet
remains blocked and must not be retried, reset or adopted by another attempt.
The separately approved one-start replacement also failed closed with zero
receipts, at features/memory_tool. Both attempts are consumed and blocked;
no additional start, reset or new attempt is authorized. The current correction
is nonlive feature-inventory compatibility only. See
evidence/2026-10-16-codex-feature-alias-compatibility/README.md for exact pins,
upstream proof and tests. Preserve both state/ledger records and all prior
approvals/evidence. No model, launch flag, deadline or credential change is made.
Broader provider activation, external account enrollment, permanent services and
public ingress still require their exact authority and acceptance gates.
No imports from ../foundation or old Fort implementation belong in this graph.
Native iPhone and Mac are required alongside the versioned service contract.
Run targeted behavioral tests before claiming progress; distinguish Swift HTTP
conformance, native builds, Simulator UI, physical devices and real providers.
