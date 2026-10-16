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
are not current execution authority. No further attempt is authorized.
Read runner/STARTUP-CONTRACT.md before any runtime work. The nonlive audit fixes
three startup contract mismatches but identifies a remaining hard incompatibility:
Codex 0.157.1 rejects Fort's restricted-read turn policy. Do not remove that
restriction to make a run pass. A named permissions-profile migration needs
separate source-grounded implementation and validation before live preparation.
The current manual still points to consumed third-attempt pins; leave it blocked.
Preparation/tests must not start the live runtime or read credentials/settings.
Broader provider activation, external account enrollment, permanent services and
public ingress still require their exact authority and acceptance gates.
No imports from ../foundation or old Fort implementation belong in this graph.
Native iPhone and Mac are required alongside the versioned service contract.
Run targeted behavioral tests before claiming progress; distinguish Swift HTTP
conformance, native builds, Simulator UI, physical devices and real providers.
