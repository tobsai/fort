# Fort — directives for every coding agent

## Current authority

[ARCHITECTURE.md](ARCHITECTURE.md) is the single authoritative design constitution:
vendor independent, modular, fast, and easy to install/remove. Toby requested this
reassessment on October 17, 2026. Read it before new requirements, design choices,
dependency changes or implementation. Its linked spec workflow is mandatory.

The active accepted design is [Spec 059](specs/059-modular-node-v1.md), adopted by
Tobias at 23:21/23:22 UTC on October 17, 2026. Read the actual
[decision references](docs/architecture/decisions/059-node-foundation.md).
Keep Node/TypeScript core, Swift native clients and Apple lifecycle. Implement
through a fully verified v1; fix recoverable issues and continue independent work
when one external requirement is blocked. Do not end at a build or ask whether to
continue. No Go rewrite. The [v1 ledger](successor/V1-ACCEPTANCE.md) tracks each
implementation/test/build/archive/upload/Apple/physical gate separately.

The preserved pre-redesign source is `09788c132647458cbdc683506c8bd5622f0be1af`;
`e02eb571e66620e407a360584817616074730b55` remains its evidence checkpoint. Spec 058
is superseded for current architecture, with history intact. Ordinary coding,
tests, spec refinements, routine fixes, CI files and requested delivery work are
authorized; no routine staged confirmations. Mandatory per-action security,
credential/persistent-access approvals, legal agreements and destructive-action
authority remain necessary. Do not bypass denial or reuse consumed demo grants.
Scope is single-owner; no shared hosting, unrelated projects or provider cleanup.

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
   Before committing product changes, run the implementation gate with `--staged`
   against the exact staged slice. Every changed path must be declared by that
   reviewed spec; verify its actual applicability and decision separately.
3. Use test-first development: one failing behavioral test at a confirmed public
   interface, then the smallest passing change. Run relevant checks; report
   passed, failed and unrun checks without conflating fixtures with real operation.
4. Before UI/release completion, inspect exact installed builds through accessibility,
   screenshots and authorized real-provider behavior. TestFlight is required for
   user delivery; a local build or upload receipt does not establish acceptance.
5. Before committing or completing a slice, perform the required
   [Ponytail review](specs/README.md#required-ponytail-review) and run its evidence
   gate on the exact staged files. A metadata pass is not a semantic review.

## Working discipline

- Inspect instructions, branch/worktree, dirty changes and applicable evidence first.
  Name exact files before edits. Preserve unrelated work and history; keep changes
  on the isolated branch. Commit only authorized work; do not push/merge by inference.
- Use small Fort-owned contracts. Keep exact identities and explicit capability
  differences. Never silently change provider, model, host, account or billing path.
- Treat the agent host and computer-tool sandbox as different security boundaries.
  A tool sandbox does not confine the host process. Do not weaken permission checks.
- Keep credentials and message bodies out of evidence. Do not replay historical
  demos or reset consumed authority. New credentials, persistent grants and startup
  activation retain their required per-action approval; continue independent work
  and report the exact blocked action without claiming it completed.
- Prefer simple behavior and removable dependencies over speculative infrastructure.
  Retain useful tests/contracts by behavior; do not wholesale copy old packages.
- Source, build, fixture, provider, installed-product and release evidence must be
  labeled separately. Explain uncertainty and unresolved decisions plainly.

## Where to continue

- [Current conflicts and keep/adapt/replace assessment](docs/architecture/reassessment-20261017.md).
- [Successor instructions](successor/AGENTS.md) before reading/changing its prototype.
- [TestFlight evidence and remaining gates](successor/TESTFLIGHT-DELIVERY-20261017.md)
  before delivery work; use current decision 059 authority, never expired sessions.
- [Architecture gate](specs/README.md) before any new or changed specification.
