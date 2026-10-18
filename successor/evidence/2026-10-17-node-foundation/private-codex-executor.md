# Compiled private executor — October 18, 2026

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), local implementation under
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md). This is
compiled composition with synthetic processes, not actual Codex/provider operation.

`createPrivateCodexExecutorProcess` accepts only the exact Codex executable, guardian,
private directory and host ID. It requires macOS, Node 26.5.0 and its compiled module;
the same module becomes the worker with fixed jitless/no-TypeScript/no-warning flags
and an empty environment. Construction checks the private plan but starts no process.
The worker reconstructs that plan before using the existing guarded runtime protocol.
No caller-supplied worker, argv or environment is accepted by this constructor.
Existing lower-level library exports remain for explicit composition and fixtures.

The two guardians, protocol identity, deadlines, disposal and absence checks are
reused. A startup-file change after construction is refused in the worker before
runtime launch. This recheck narrows the inspection interval; it does not make path
inspection and execution atomic. The direct worker entry rejects missing/unknown
arguments with one generic diagnostic. No provider/model/account fallback, credential
copy, sign-in or startup registration was introduced.

## Evidence

- Public compiled export test and full private catalog/text journey first failed
  because the constructor was absent, then passed after implementation.
- Full portable suite: **1,079 passed, 0 failed, 3 opt-in budgets skipped**.
- Affected Mac suites: **27 passed, 0 failed, 3 opt-in budgets skipped**; existing
  cancellation, parent-death, abrupt-exit and independent group-absence checks passed.
- Final strengthened fixture added an authored ambient `NODE_OPTIONS` preload that
  would write a marker and exit. It stayed inactive in all new private journeys.
  The isolated run passed **25 checks**, including the parent test, 20 full private
  journeys, prior generic/foreign-worker cases, changed configuration and foreign
  dispatch. No tests or builds were edited while running.
- Actual synthetic runtime observations confirmed fixed stdio arguments and private
  HOME/CODEX_HOME/TMPDIR, with only the previously observed macOS encoding addition.
  Catalog queries used `refreshToken:false`; only the later exact-text request created
  a thread/turn. Both outer and inner process groups were independently absent.
- A foreign-host dispatch was refused before a new runtime child: prior PID evidence
  remained unchanged and no thread/turn appeared. Its parent result is deliberately
  `outcomeUnknown`, because the rejecting worker supplies no valid terminal evidence.
  The first test incorrectly expected `notStarted`; the test was corrected without
  weakening production evidence requirements.
- Strict TypeScript, compiled exports, an external typed consumer, direct CLI
  missing/unknown-argument refusals and structural/accepted-spec checks passed.
  Core independence tests still pass with the optional adapter absent.

Twenty serial compiled catalog-and-text journeys, fresh private layouts, warm
toolchain/filesystem caches, no provider/network latency: p50 **497.288 ms**,
p95 **569.234 ms**, maximum **615.240 ms**; target p95 <=2 seconds. Each sample
includes a fresh test driver, two sequential worker/runtime operations, both
guardians per operation and independent absence checks. Compilation/layout creation
are outside the timed interval. Host: previously recorded M1 Max/10 physical cores/
64 GiB; current arm64 macOS 27.2, Node 26.5.0. Raw samples:
`/tmp/fort059-private-executor-budget.json`.

Logs: `/tmp/fort059-private-executor-{red,journey-red,journey,artifact,portable,macos,typecheck,budget}.log`.
No actual Codex binary or account was invoked; all executable runtime data was authored
synthetic protocol data. No new signing, profiles, App Group, named listener,
registration, provider, installed accessibility/screenshots or TestFlight invocation.
Swift/project files did not change, so native builds were not repeated.

## Retained artifact and next boundary

`successor/.build/codex-private-worker-059-20261018`: 42 files, 134024 bytes excluding
manifest; complete file-set, size and SHA256 verification passed. Manifest SHA256:
`44555daa2f6d8192fcae23dc875c532b46bd5ef7a14e9b978660d77122f2454c`.
Earlier artifacts remain intact. The compiled library has no runtime binary,
credential, installer or dependency/startup script; legal attribution is unchanged.

The helper still needs a concrete bootstrap/resource assembly, private directory and
host-identity preparation, verified executable/legal closure and exact Apple/account
authority. The constructor deliberately does not choose those on behalf of the
installation. The agent-host sandbox and computer-tool policy remain separate.
V1, real provider behavior and installed delivery remain unverified.
