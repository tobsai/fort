# Guarded model discovery

October 18, 2026. [Spec 059](../../../specs/059-modular-node-v1.md) and
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md)
govern this extension of the existing model picker capability. All runtime and
account responses below are synthetic. No actual Codex executable, credential,
provider, sign-in, startup registration or signed app was invoked.

## Behavior and boundaries

The optional `ExecutorProcess.models` capability requires an explicit immutable
provider/runtime-version/host binding. A bounded private request carries only
that binding, UUID, deadline and consistency digest. The worker rejects a foreign
binding, unsupported operation, extra executable/history/grant fields, malformed
request or second operation before constructing an unauthorized runtime.
Digests detect mismatch; they do not authenticate the channel.

Discovery and execution share the existing process lifecycle and active slot.
The worker disposes its runtime before output. The parent validates correlation,
exact catalog union, model IDs, availability, labels, timestamp and size, then
requires a clean zero exit and confirmed owned-group absence. Cancellation and
disposal are checked again after cleanup, so an earlier ready frame cannot escape
after either action. Unconfirmed cleanup blocks further operations. Discovery is
absent from executors that did not configure it; no model is selected implicitly.

The Codex composition explicitly pins host identity and version 0.157.1. It uses
the same guarded empty workspace, configuration checks and account-class checks
as execution, followed by `model/list`. Synthetic evidence confirms
`account/read` uses `refreshToken:false`, no thread/turn/login is created, and
unsupported text models remain visible as unavailable. Runtime executable,
arguments, environment and grants remain local composition inputs, never request
fields. Agent-host supervision is distinct from the runtime's tool sandbox;
process-group cleanup does not prove containment of deliberate group escape.

## Verification and preserved failures

Environment matches the previous local composition evidence: Node 26.5.0,
TypeScript 6.0.3, Swift 6.4, Xcode 27.1/27A9275, macOS 27.2/26B5112a. Hardware
uses the earlier recorded M1 Max, 10 cores, 64 GiB inventory; it was not re-read.
Temporary Swift guardians and Node fixtures use explicit environments. Actual
account inspection and provider latency are excluded.

- Initial worker/parent/Codex public-seam tests failed because discovery was not
  supported, then passed. Logs share `/tmp/fort059-executor-models-` with suffixes
  `red.log`, `parent-red.log`, `codex-red.log`, and their green counterparts.
- Two behavioral regressions initially returned ready catalogs after cancellation
  or disposal between output and exit. `cancel-red.log` and `dispose-red.log`
  preserve those failures. Parent completion now rechecks both conditions.
- Worker negatives cover identity, digest, deadlines, input limits, absent
  discovery, wrong result, disposal failure, EOF, close, malformed cancel, partial
  input and second operation. Parent negatives cover foreign/malformed/duplicate/
  oversized/partial evidence, invalid UTF-8, nonzero exit, busy/stopped/aborted
  calls, immutable binding, deadline enforcement and unconfirmed group absence.
- The first combined parent regression command encountered sandbox `listen EPERM`
  for existing localhost HTTP fixtures. The approved rerun passed 85 tests.
  Its 22 exact temporary SQLite-only directories were inventoried and removed by
  exact-file unlink/directory removal; no existing data was targeted. Records:
  `denied-run-inventory.json`, `denied-run-cleanup.json`.
- A test-only TypeScript narrowing error was corrected by capturing the initial
  getter value before asserting it was undefined. No cast or compiler weakening.
- The expanded catalog fixture first failed because it supplied only the original
  one-row catalog (`catalog-fixture-red.log`). It now supplies distinct text/audio
  rows, account/configuration negatives and a cancellable pending model request.
- The first full portable run passed 1011 and failed one new uncertainty test.
  That fixture denied signal-zero queries but allowed stop signals to report
  `ESRCH`, which correctly proves absence. Correcting the fixture to deny all
  group access produced the intended cleanup-unknown state (`absence-green.log`).
- Final portable suite: **1012 passed, zero failed, one unrelated opt-in worker
  benchmark skipped** (`portable-confirmed.log`). Command:
  `PATH=/opt/homebrew/opt/node@26/bin:$PATH npm --prefix successor test`.
- Guarded Codex/process Mac suites: **21 passed, zero failed, three opt-in budgets
  skipped** (`mac-final.log`). Exact models, unavailable text support, configuration
  and account refusal, cancellation, shared execution slot, prior execution
  interruption, crash and group cleanup all passed. One command wrapper used
  zsh's reserved `status` variable after the tests finished, so the wrapper failed;
  the complete test log reports all results. No test failure is hidden by that
  wrapper. The separate catalog benchmark below exited successfully.
- Strict TypeScript and the structural gate passed with a corrected exit-status
  wrapper (`final-types-confirmed.log`). Implementation and staged-path gates and
  `git diff --check` passed before committing.

## Isolated measurement

Twenty serial fresh catalog journeys through outer guardian, Node worker, inner
guardian and Node protocol runtime. Timer includes model checks, fixture metadata
writes, disposal, exit and independent PID/group absence checks. Compilation,
signing, real account, provider and network time are excluded. No other tests or
builds ran concurrently. The first sample is retained.

p50 **196.345 ms**, p95 **199.145 ms**, maximum **396.085 ms**; target p95 <=2000 ms.
Command: `FORT_CODEX_CATALOG_BENCHMARK=/tmp/fort059-executor-models-budget.json
node --test --test-name-pattern='full synthetic Codex catalog journey'
successor/runner/codex-executor.test.ts`.
Raw samples/log: `/tmp/fort059-executor-models-budget.json` and `.log`.
Samples in milliseconds, rounded only for display:

```text
396.085 198.341 197.833 197.413 196.345 196.685 195.207 194.087 197.319 195.576
196.299 195.691 199.145 195.965 195.055 196.851 195.114 194.203 197.200 196.623
```

## Remaining

This is tested source composition, not a working installed provider application.
Production helper/bootstrap, exact executable/package provenance, actual sandbox
and account access, provider acceptance, installed accessibility/screenshots and
TestFlight gates remain. No Swift/client source changed in this slice, so native
builds and signed/installed checks were not repeated. No donor code, legal
attribution, private remote visibility or provider cleanup was changed.
