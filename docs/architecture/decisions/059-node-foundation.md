# Decision 059: implement the modular foundation in Node

Status: adopted by Tobias on October 17, 2026. Governing spec:
[059](../../../specs/059-modular-node-v1.md).

## Source decision

Source thread: `01a0f307-3940-7333-96b0-a3b0b854e0e8`.
The implementation coordinator supplied this transcript evidence:

- `Sentinel_9d1ca780b6d48191b1e19cdea66dc56e`, 23:21 UTC: “Let’s stay with node
  for now. Implement your last design as specified. Update all relevant repository
  specs and begin dev with a goal to have a fully working project before stopping
  with no approval requests”.
- `Sentinel_667fda138e308191a979ce2aad0906f2`, 23:22 UTC: “Don’t stop for any
  reason until v1 is complete”.

These are relayed source-message references, not an authorization claim generated
by a status string or spec digest. They authorize ordinary implementation, tests,
routine fixes, CI files and requested delivery work within the adopted design.
They do not waive per-action security/credential/persistent-access approval,
legal agreements, denials or deletion authorization. Expired demo grants remain
consumed. Reuse this decision for ordinary implementation refinements inside the
accepted scope; do not interrupt for routine file lists, tests or commits.

## Adopted design and differences from the proposal

Preserve the [independent review](../reviews/20261017/REVIEW.md),
[policy review](../reviews/20261017/POLICY-REVIEW.md) and
[visual brief](../reviews/20261017/visual-brief.md) as original evidence. Their
source copies/provenance remain intact. The owner chooses **Node/TypeScript**
for the portable service, replacing their tentative Go comparison. Swift remains
native UI and Apple lifecycle. This is a decision to retain Node, not a claim
that its performance has already been established.

Adopt one modular service, one indexed transactional ledger/outbox, service-owned
attempts/generations and separately contained execution. Provider-neutral public
contracts and capability-based adapters own runtime differences. Memory, voice,
connectors and VM placement are removable boundaries; future implementations are
not unlimited v1 scope. V1 retains the user's identity/model/avatar/settings/image
display and easy unpair/removal requests, native iPhone/Mac, reviewed skills and
routines, durable messaging and TestFlight delivery.

Start with the all-TestFlight local host feasibility proof using synthetic data.
Do not silently replace it with a separately hosted executor if Apple/runtime
constraints prevent that topology. A verified packaging or account blocker is
reported precisely while independent implementation continues. A local build,
synthetic turn or archive does not imply provider or Apple acceptance.

GitHub Actions owns portable quality checks; Xcode Cloud is the proposed sole
signed Apple lane, subject to existing authorized access and actual setup. Do not
create credentials or agree to new terms to make a pipeline appear complete.

## Implementation authority and evidence

Spec 059 describes accepted design and engineering acceptance targets, not
verified implementation. Its file list may be refined under this unchanged-scope
owner direction; record the revision, refresh its digest and keep staged-path
coverage. New external authority or a material product/topology change must not
be inferred from implementation autonomy. The [v1 checklist](../../../successor/V1-ACCEPTANCE.md)
tracks implementation, test, build, archive, upload, availability and physical
acceptance separately. Continue necessary work until v1 passes or only real
external blockers remain, with a concrete handoff for any blocked action.
