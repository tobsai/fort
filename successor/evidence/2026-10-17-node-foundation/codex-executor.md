# Guarded Codex protocol composition

October 18, 2026. [Spec 059](../../../specs/059-modular-node-v1.md) and
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md)
govern this local integration. All runtime responses in this evidence came from
a synthetic Node protocol process. No actual Codex executable, provider account,
credential, startup registration or network service was used.

## Composition

`createCodexExecutorProcess` selects the Mac parent-exit guardian and a bounded
3500 ms cooperative cancellation allowance. `serveCodexExecutor` composes the
owned executor worker, guarded child launcher and existing Codex 0.157.1 protocol
adapter. It pins provider/version locally and requires the configured host identity
to match the dispatch. Executable, argv, guardian, workspace and environment come
from local configuration, cloned before reading input. Dispatch data cannot select
them. An empty exact workspace is required before launching the runtime.

The adapter retains existing account-class, exact-model, permission-profile,
configuration and feature checks. It uses the dispatch's selected model and work
deadline. The worker awaits runtime disposal before returning a terminal. No
historical activation packet, real-user HOME, implicit environment, replacement
session or fallback provider was introduced.

The outer executor's default cancellation allowance remains 250 ms. Its explicit
optional allowance accepts integers from 250 through 3500 ms. At cancellation it
is capped at the lesser of that allowance and remaining dispatch work plus the
existing 250 ms grace, so it cannot extend the deadline cleanup bound. TERM/KILL
bounds are unchanged. The Codex adapter already waits up to two seconds for an
interrupt confirmation and child cleanup may need another second. Its 3500 ms
allowance lets those operations finish without treating a requested cancellation
as proof that the runtime stopped.

## Verification

Machine: M1 Max, 10 CPU cores, 64 GiB, macOS 27.2/26B5112a. Node 26.5.0,
TypeScript 6.0.3, Swift 6.4 and Xcode 27.1/27A9275. The existing guardian source
was compiled to a temporary path. Synthetic processes received explicit empty
environments. Fixtures returned synthetic `account/read` data; no real account
was read. Compiler/process tests are classified in the macOS suite.

- Initial missing-composition red: `/tmp/fort059-codex-executor-red.log`;
  first complete protocol journey passed in `-first-green.log`.
- Delayed-cancellation red: the old parent allowance forced shutdown after
  256.489 ms, before a synthetic 450 ms interruption response. The result stayed
  unknown, `/tmp/fort059-codex-executor-cancel-red.log`.
- Combined executor/service checks then passed: 34 checks, zero failures and one
  isolated benchmark skipped, `/tmp/fort059-codex-executor-behavior.log`.
  A hung worker with the longer allowance still met the prior deadline cleanup
  bound; its measured entire service operation was 720.626 ms.
- Final composition checks: 9 passed, zero failed, one isolated benchmark skipped,
  `/tmp/fort059-codex-executor-final.log`. Exact echo, delayed cancellation,
  unconfirmed interruption, unsafe configuration, unavailable exact model,
  nonempty workspace, historical forced-unknown behavior and parent death passed.
- Final delayed cancellation was confirmed in 486.201 ms with clean outer exit
  and both groups absent. An unconfirmed interruption stayed unknown after
  2036.207 ms, also with clean outer exit and both groups absent.
- Abrupt launcher death removed PIDs 89509, 89510, 89511, 89512, 89513 and 89514,
  and groups 89509, 89510 and 89512, within 11.602 ms. Independent signal-zero
  checks confirmed every absence; no fallback fixture cleanup was required.
- Full portable regression suite: 960 passed, zero failed, one unrelated isolated
  worker benchmark skipped, `/tmp/fort059-codex-executor-portable.log`.
  Strict TypeScript passed, `/tmp/fort059-codex-executor-typecheck.log`.
- Full structural, implementation, staged-path and whitespace gates passed.

The separate benchmark ran 20 serial fresh full protocol journeys through the
outer guardian, Node worker, inner guardian and synthetic runtime. It includes
runtime checks, exact text dispatch, reply, disposal, flush, exit and independent
process/group absence checks. It also includes the fixture's temporary diagnostic
file writes. Compilation, signing, provider and network latency are excluded.
p50 207.458 ms, p95 216.690 ms, maximum 635.041 ms; target p95 <=2000 ms.
Command used `FORT_CODEX_EXECUTOR_BENCHMARK` with
`node --test --test-name-pattern='isolated budget' runner/codex-executor.test.ts`.
Raw samples/log: `/tmp/fort059-codex-executor-budget.json` and `.log`.

## Limits

This proves protocol and lifecycle composition with synthetic processes. It does
not verify the configured executable's provenance, the real app-server package
layout, authentication, effective provider permissions or signed App Sandbox
behavior. Agent-host supervision and computer-tool sandboxing remain different
boundaries; deliberately creating another session can escape process groups.
Production activation still needs the reviewed signed bundle and exact runtime.

Ordinary app/core bootstrap, runtime model discovery through this worker boundary,
native service activation/removal integration and actual provider acceptance remain
open. No Swift product or packaging files changed. Native builds, signed runtime
probes, installed UI and Apple/TestFlight checks were not rerun. The portable core
artifact remains unchanged at 30 files/190826 bytes, manifest SHA256
`c1a19bd8e8f0def68efa45f2b913dc5b2123466a3da6a5c14ce0eb8fded7f527`.
No v1 completion or working provider-application claim follows from this evidence.
