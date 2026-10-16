# Spec 058 successor

Spec 058 implementation was approved by Toby on 2026-10-12. This directory is a
new isolated successor, separate from both historical Fort and quarantined donor.
Read ../specs/058-native-first-owned-agent-mvp.md and IMPLEMENTATION.md.

The bounded Candidate B manual test was approved on 2026-10-16, including its
minimal activation revision; see evidence/2026-10-16-candidate-b-activation/approval.json.
Only that pinned manual entrypoint may use the approved official runtime. Wait for
the coordinator to relay the owner's readiness at the Mac before starting it,
enrolling or consuming either session. Preserve all budgets and preflight gates.
Broader provider activation, external account enrollment, permanent services and
public ingress still require their exact authority and acceptance gates.
No imports from ../foundation or old Fort implementation belong in this graph.
Native iPhone and Mac are required alongside the versioned service contract.
Run targeted behavioral tests before claiming progress; distinguish Swift HTTP
conformance, native builds, Simulator UI, physical devices and real providers.
