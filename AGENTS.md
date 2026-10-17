# Fort — agent entrypoint

Read [CLAUDE.md](CLAUDE.md) before acting; it applies equally to every coding agent.
Read [ARCHITECTURE.md](ARCHITECTURE.md) before proposing or changing requirements,
contracts, dependencies, implementation language, modules, hosting or delivery.
It is the authoritative four-principle design grid.

- **Current work:** implement [Spec 059](specs/059-modular-node-v1.md) through v1.
  Tobias adopted the modular design and Node/TypeScript on October 17. Swift owns
  native UI and Apple lifecycle. Do not rewrite to Go or stop at an intermediate build.
  Track the [v1 acceptance ledger](successor/V1-ACCEPTANCE.md); Spec 058 is historical.
- **New/materially changed requirements:** create/update a spec using
  [the template and review gate](specs/README.md). Check all four principles and
  record actual owner decisions. Structural validation is not architecture approval.
- **Before implementation:** run the spec's `--implementation` gate, then use a
  red → green behavioral slice at the public seam. Fixture, provider and installed
  product evidence are different claims. Before committing product changes, add
  `--staged` to check every changed path against the spec's reviewed declaration.
- **Surgical work:** name exact files, preserve unrelated changes, historical
  approvals and source pins. Do not revive legacy services or reset consumed demos.
- **Working in `successor/`:** read [its handoff](successor/AGENTS.md). It contains
  current implementation progress and the adopted Node foundation handoff.
- **Authority:** ordinary coding, tests, routine fixes, CI and requested delivery
  work are authorized by [decision 059](docs/architecture/decisions/059-node-foundation.md).
  Reuse it without routine confirmation. New security/credential/persistent-access
  grants, legal agreements and destructive actions retain per-action gates.
  Expired demos are consumed; provider cleanup/unrelated work remains out of scope.

Do not infer the current architecture from historical Go-only or provider-specific
specs. The [assessment](docs/architecture/reassessment-20261017.md) names conflicts,
open decisions, CI/build gaps and the preserved checkpoint.
