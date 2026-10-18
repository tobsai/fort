# Local and CI verification

Use Node 26.5.0. From the repository root:

```sh
npm --prefix successor run test:inventory
npm --prefix successor run test:specs
npm --prefix successor run typecheck
npm --prefix successor test
npm --prefix successor run test:macos
```

The runner discovers every `.test.ts` and `.test.mjs` below `successor`, excluding
dependencies, generated output and evidence. Each file belongs to exactly one
suite. The portable suite runs by default. The macOS suite compiles an inert app,
checks its owned shutdown, and runs synthetic sandbox diagnostics. Neither suite
starts a provider or reads a provider account.

The signed suite is separate:

```sh
FORT_SIGNING_IDENTITY=<existing-development-identity> npm --prefix successor run test:packaging
```

It requires macOS and an explicitly selected existing signing identity. Missing
identity is a failure, not a skipped pass. It packages a synthetic Node/Swift
probe and checks relocation, restricted access and process cleanup. It does not
establish official runtime confinement, distribution or installed acceptance.

The prepared [workflow](../../.github/workflows/fort-quality.yml) pins actions and
Node, installs dependencies with hooks disabled, and has read-only repository
permissions. Its portable job targets Linux. Its native job targets Xcode 27.1
and also runs Swift tests and compiled HTTP journeys. It has no release secrets,
signing or uploads. No workflow has been pushed or run remotely for this slice.

The workflow's spec checks establish structure and accepted-content integrity.
They do not authenticate owner approval or establish changed-path coverage for a
mixed historical branch. Before each local implementation commit, stage its exact
paths and run the accepted spec gate with `--staged`. Review the actual decision
and applicability separately. A future PR needs a trusted base and applicable
scope review before a CI range check can be treated as enforcement.

See [the results and limits](../evidence/2026-10-17-node-foundation/quality-ci.md).
