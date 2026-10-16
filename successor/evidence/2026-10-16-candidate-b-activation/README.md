# Candidate B — approved activation preparation

Source: **`24010dd0a1bb9024f6b00abc8cb0a1c3f95317c7`**, branch
`codex/fort-openai-rebuild-20261016`, worktree
`/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.
[Exact source/app pins](pins.json) and [verification results](verification.json)
cover this activation revision. All previous source, evidence and recovery bundles
remain intact.

The owner approved the replacement test at 12:55:58 UTC on 2026-10-16. The
coordinator explicitly confirmed that approval includes this minimal activation
revision, refreshed pins and affected nonlive checks. [approval.json](approval.json)
records the supplied decision identifiers, original candidate source/evidence,
scope and disclosed same-user startup authority. No historical message retrieval
was needed. The coordinator reported the preceding candidate's final focused
review complete; this preparation does not claim a new independent review.

## Behavior changed

`prototype/activation.ts` now accepts only the exact `prototype/manual.ts` process
entrypoint instead of refusing every call. Direct provider-launcher use, imports
from another entrypoint and another file merely named `manual.ts` still refuse
before path inspection or process launch. This checks ordinary application
routing; it is not an OS security boundary and cannot prevent owner code from
mutating its own process arguments.

`prototype/manual.ts` changes only the path to the refreshed pins. It still
verifies source/app bytes before setup and waits for an explicit `start` command.
The official launcher, session controller, native cleanup, service, runtime
preflight and Apple code are byte-identical to the approved candidate. The new
activation test and adjusted entry tests exercise the changed routing; instructions
and packet metadata now reflect approval and the pending readiness gate.

The maximum remains two supervised five-minute sessions, work stopping at 292
seconds, five exact samples, existing ChatGPT sign-in, exact `gpt-6-astra`, and a
ten-minute local credential deadline including the gap. Same-process preflight
and every failed-check stop remain enforced. No fallback, extra start/message,
new sign-in, deployment or security-setting change is authorized by this revision.

## Nonlive evidence

- The new approval-route regression failed before the change as expected:
  [red result](activation-red.log).
- All six activation/manual/launcher checks passed:
  [activation output](activation-tests.log).
- All **27 affected tests passed, zero failed**, including bounded sessions,
  restart gates, cancellation, native cleanup fixtures and same-connection
  preflight: [combined output](affected-tests.log).
- Strict TypeScript passed, exit 0, with no diagnostics in [typecheck.log](typecheck.log).
- All **453 source files**, **five app files**, both canonical manifest hashes
  and the official runtime executable hash were verified. The app has exactly
  those five files and no symlinks. `git diff --check` passed.
- Main checkout status matches its retained baseline. The real candidate session
  directory remains absent. No provider process, real enrollment/credential,
  native app launch or live session timer was started.

The new activation test evaluates only the guard in an inert VM. The manual tests
use inert I/O and a synthetic packet; they assert that waiting at the prompt starts
no session and writes no enrollment. Other checks use temporary loopback/SQLite
fixtures and synthetic runtime transcripts. They do not inspect real provider
storage or use real native credentials.

The preceding 162-test run and five-turn Swift HTTP result remain historical
evidence for `e3305265`. They were not rerun here; the affected production paths
and Apple app bytes are unchanged. No Mac rebuild, rendered UI, real provider or
actual credential cleanup acceptance is claimed.

## Hold until attended readiness

The coordinator must relay that the owner is at the Mac ready to operate the app
before the controller, runtime, enrollment or either live timer is started.
No second approval for the unchanged test scope is needed. Once readiness is
relayed and these pins are checked, use the existing manual entry and attended
instructions. Stop on a preflight failure without retry or relaxed checks.

Retained HOME and same-user startup authority remain exactly as disclosed in the
approval. The process can act before `config/read` returns; the later protocol
check cannot prove absence of startup effects. No new isolation or remote
cancellation guarantee is introduced.
