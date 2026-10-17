# Fort — directives for every coding agent

## Current authority

[ARCHITECTURE.md](ARCHITECTURE.md) is the single authoritative design constitution:
vendor independent, modular, fast, and easy to install/remove. Toby requested this
reassessment on October 17, 2026. Read it before new requirements, design choices,
dependency changes or implementation. Its linked spec workflow is mandatory.

The isolated successor checkpoint is
`e02eb571e66620e407a360584817616074730b55`; product source is
`09788c132647458cbdc683506c8bd5622f0be1af`. Feature and TestFlight expansion are
paused while the replacement design is researched and reviewed. This task
permits documentation, review gates and safe verification, not a wholesale rewrite.
[Spec 058](specs/058-native-first-owned-agent-mvp.md) needs reassessment. Its older
approval does not settle the new principles. No backend language is prescribed.
Scope remains single-owner; shared or multi-user hosting is not authorized.

Earlier Go architecture and Specs 021/047/048, the 054/056 messenger direction,
and the provider-specific 058 prototype are historical inputs when they conflict
with the constitution. Preserve them and state conflicts; do not silently adopt
or delete their implementations. Specs 054–057 remain in the primary checkout.
Keep teardown/recovery evidence: local teardown was reported complete, full
provider closure is unverified, and this work authorizes no provider cleanup.
The donor under `foundation/` stays quarantined with its license/provenance intact.

## When work changes

1. Before a new/materially revised requirement, read [the spec workflow](specs/README.md),
   write the four-principle review and name the proposed files and acceptance tests.
   Supporting design documents point to a reviewed spec before implementation.
2. Run `node ops/architecture/validate-specs.mjs`. Before implementing a product
   slice, also run it with `--implementation specs/<NNN>-<slug>.md`. Both must
   pass; a human must assess the evidence and actual owner decision separately.
   Reuse existing authorization for unchanged scope; do not invent approval.
3. Use test-first development: one failing behavioral test at a confirmed public
   interface, then the smallest passing change. Run relevant checks; report
   passed, failed and unrun checks without conflating fixtures with real operation.
4. Before UI/release completion, inspect exact installed builds through accessibility,
   screenshots and authorized real-provider behavior. TestFlight is required for
   user delivery; a local build or upload receipt does not establish acceptance.

## Working discipline

- Inspect instructions, branch/worktree, dirty changes and applicable evidence first.
  Name exact files before edits. Preserve unrelated work and history; keep changes
  on the isolated branch. Commit only authorized work; do not push/merge by inference.
- Use small Fort-owned contracts. Keep exact identities and explicit capability
  differences. Never silently change provider, model, host, account or billing path.
- Treat the agent host and computer-tool sandbox as different security boundaries.
  A tool sandbox does not confine the host process. Do not weaken permission checks.
- Keep credentials and message bodies out of evidence. Do not replay historical
  demos, reset consumed authority, create credentials or enable startup services.
- Prefer simple behavior and removable dependencies over speculative infrastructure.
  Retain useful tests/contracts by behavior; do not wholesale copy old packages.
- Source, build, fixture, provider, installed-product and release evidence must be
  labeled separately. Explain uncertainty and unresolved decisions plainly.

## Where to continue

- [Current conflicts and keep/adapt/replace assessment](docs/architecture/reassessment-20261017.md).
- [Successor instructions](successor/AGENTS.md) before reading/changing its prototype.
- [TestFlight evidence and remaining gates](successor/TESTFLIGHT-DELIVERY-20261017.md)
  before delivery planning; no release is authorized by that plan.
- [Architecture gate](specs/README.md) before any new or changed specification.
