# Architecture gate verification — October 17, 2026

Scope: documentation and a dependency-free local spec validator on branch
`codex/fort-openai-rebuild-20261016`, based on paused checkpoint
`e02eb571e66620e407a360584817616074730b55`. Product source remains unchanged at
`09788c132647458cbdc683506c8bd5622f0be1af`. No services, accounts or providers were
started; no dependencies were installed.

## Results

| Check | Observed result |
| --- | --- |
| Test-first red run, Node 26.5.0 CLI suite before validator existed | 10 tests: 9 failed, 1 passed. Missing CLI produced MODULE_NOT_FOUND; the invalid-input test already expected failure. |
| `npm --prefix successor run test:specs` | Passed: 10 tests, 0 failures. CLI exercises missing/duplicate fields, progress states, accepted-content drift, decision metadata, blocked judgments, historical preservation, discovery, fenced examples and invalid invocations. |
| `node ops/architecture/validate-specs.mjs` | Passed structurally: 1 needs-reassessment review record and 55 unchanged historical files, preservation only. |
| `node ops/architecture/validate-specs.mjs --implementation specs/058-native-first-owned-agent-mvp.md` | Expected rejection, exit 1: implementation requires accepted review; found needs-reassessment. |
| `npm --prefix successor run pretest` | Passed: test lifecycle invokes structural check. The full product test suite was not run. |
| `npm --prefix successor run typecheck` | Passed, including the new pretypecheck structural gate. |
| `git diff --check` | Passed. |

Commands used the existing `/opt/homebrew/opt/node@26/bin` toolchain (Node
26.5.0); no install hooks ran. Historical hashes were generated from Git object
content at the paused checkpoint, excluding Spec 058. The template and README
are deliberately excluded from spec discovery; new spec copies must complete
their placeholders. No historical evidence file was rewritten.

## Limits

No architecture acceptance is claimed. The checker validates structure/status
and accepted-content integrity; a person must verify the decision record and
substance of evidence. Spec 058 remains blocked under all four principles.
No current provider-removal, optional-module-removal, benchmark, real host
uninstall or TestFlight proof was performed. Product/native tests and app builds
were not rerun for this documentation/tooling slice; earlier results remain in
[the existing QA record](../../successor/evidence/2026-10-17-interactive-qa/README.md).

No workflows/runners/branch protections, credentials, signing settings, archive,
upload, Apple processing or tester access were configured. CI and background-host
choices remain proposals for the broader research/design review. No tests/builds
are left running at this handoff.

## Follow-up: changed-path coverage

Independent review requested a small local changed-path-to-spec check. The CLI
now accepts `--implementation <spec> --staged [--base <full-commit-SHA>]`. It
checks all indexed changes against the selected spec's exact `Implementation
paths`, including additions, deletions and both sides of renames. The spec must
match its indexed contents; its declaration remains covered by the review digest.

The test-first extension produced 7 expected failures (`--staged` was not yet
supported), with the original 10 tests still passing. After implementation all
17 tests pass, including a path containing spaces, an undeclared path, unstaged
spec changes, a rename, deletion, explicit committed-range base, excluded
unstaged/untracked files, invalid options and wildcard rejection. The real Spec
058 with `--staged` still rejects implementation with exit 1. The full structural
check still reports 1 needs-reassessment spec and 55 unchanged historical files.

This does not authenticate an approval, prove semantic applicability or enforce
remote merge policy. It covers the staged index only, and defaults to changes
since HEAD; earlier committed changes require an explicit trusted base. The
[assessment](../../docs/architecture/reassessment-20261017.md#proposed-ci-enforcement-of-change-scope)
records the proposed trusted CI procedure and independent review findings on
attempt IDs, full-history persistence and scheduler writes. No product rewrites,
remote settings, credentials or approval integration were added. Full product
and native tests/builds were not rerun for this tooling-only extension.
