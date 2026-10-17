# Fort — agent entrypoint

Read [CLAUDE.md](CLAUDE.md) before acting; it applies equally to every coding agent.
Read [ARCHITECTURE.md](ARCHITECTURE.md) before proposing or changing requirements,
contracts, dependencies, implementation language, modules, hosting or delivery.
It is the authoritative four-principle design grid.

- **Current work:** architecture reassessment; feature/TestFlight expansion paused.
  [Spec 058](specs/058-native-first-owned-agent-mvp.md) is not implementation authority.
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
  the current prototype and engineering evidence, not an accepted replacement design.
- **External actions:** no credentials, sign-in, installation, background startup,
  provider cleanup, deployment or publication is authorized by architecture work.

Do not infer the current architecture from historical Go-only or provider-specific
specs. The [assessment](docs/architecture/reassessment-20261017.md) names conflicts,
open decisions, CI/build gaps and the preserved checkpoint.
