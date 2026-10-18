# Owned executor worker

October 18, 2026. Governing [Spec 059](../../../specs/059-modular-node-v1.md)
and [decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
This implements the receiving half of the existing private executor protocol.
It is a production library verified with synthetic runtime implementations. No
official provider adapter was launched or wired into the ordinary app.

## Ownership and validation

`serveExecutorWorker` owns one input/output stream pair and one runtime attempt.
Local composition pins the provider, runtime version and host before reading the
dispatch. It validates the complete input shape, exact binding/attempt identity,
input/configuration/history/policy digests, plain-text policy with no grants,
limits and deadline before calling an explicit synchronous runtime factory.
The transport cannot choose executables, environment variables, accounts or
billing paths. The factory must retain ownership if creation fails after starting
external work. No provider defaults or public listener were added.

The wire validator preserves original key order for the existing sealed JSON
digests. It preserves exact text, including valid NUL and Unicode characters,
instructions, reviewed bundles and prepared optional context. Bundles must include
their complete recorded sizes and hashes; import-time optional manifest fields
are not treated as complete runtime wire values. Digests verify consistency, not
authority. The trusted inherited pipe and selected executable remain prerequisites.

One correlated cancellation aborts the active attempt. EOF, channel closure,
duplicate dispatch, malformed/trailing data, a foreign cancellation or a work
deadline aborts it and suppresses terminal success. Runtime execution and disposal
must both resolve, the channel must remain valid, and the deadline must still be
live before one terminal frame is flushed. Failures expose a generic error without
provider details. Parent-side exit and process-group absence checks still apply.

Initial input is bounded to five seconds; dispatch work to 120 seconds; disposal
to 1500 ms. Dispatch frames are limited to the 1 MiB input allowance plus 16384
bytes, cancellations to 2048 bytes, and the channel to two frames. There is no
idle periodic polling or new production dependency.

## Verification and failures

Machine: M1 Max, 10 CPU cores, 64 GiB, macOS 27.2/26B5112a. Node 26.5.0 and
TypeScript 6.0.3. Tests use synthetic data and explicit empty child environments.
No credentials, actual runtime, persistent service, account or provider was used.

- Initial missing-module red: `/tmp/fort059-executor-worker-red.log`.
- First public stream slice passed: disposal had to resolve before any terminal
  bytes appeared, `/tmp/fort059-executor-worker-first-green.log`.
- Expanded boundary red caught rejected NUL text and a trailing partial-frame
  acceptance. Both were fixed; `/tmp/fort059-executor-worker-boundary-red.log`
  and `-boundary-green.log` retain the failure and correction.
- A separate close-only regression failed because a destroyed input did not emit
  EOF. The worker now observes input closure and premature output closure;
  `/tmp/fort059-executor-worker-close-red.log` records the failure.
- Final focused run: 57 passed, zero failed, one isolated benchmark skipped,
  `/tmp/fort059-executor-worker-behavior-final.log`. Includes recomputed invalid
  hashes/shapes, foreign identities, real bundle manifests, fragmented UTF-8,
  exact cancellation, expiry, EOF/closure, runtime/cleanup failure and bounded
  cleanup timeout. Recomputed hashes cannot bypass schema or policy checks.
- Full portable suite: 959 passed, zero failed, one isolated benchmark skipped,
  `/tmp/fort059-executor-worker-portable.log`. Strict TypeScript passed,
  `/tmp/fort059-executor-worker-typecheck-final.log`.
- The real `ExecutorProcess` ran the worker with a synthetic echo runtime, then
  with deliberate disposal failure. Echo exited 0, PID/group 87200 absent;
  cleanup failure exited 1, PID/group 87201 absent and outcome remained unknown.
  Both absence checks used independent signal-zero queries. Neither required
  forced termination. These are synthetic process checks, not provider evidence.
- Structural, implementation, staged-path and whitespace gates passed.

An isolated run then executed 20 serial fresh Node worker journeys through the
real parent process protocol. Each included dispatch validation, synthetic echo,
runtime disposal, terminal parsing, process exit and independent group absence.
p50 88.855 ms, p95 92.022 ms, maximum 96.686 ms; target p95 <=2000 ms.
No compiler/signing/provider/network work was included. Command used
`FORT_EXECUTOR_WORKER_BENCHMARK` with
`node --test --test-name-pattern='isolated budget' runner/executor-worker.test.ts`.
Raw samples/log: `/tmp/fort059-executor-worker-budget.json` and `.log`.

## Remaining gates

The next composition must select and supervise an official runtime using the
guarded child launcher. Cancellation timing across the outer executor and official
runtime still needs verification. The worker does not provide an OS sandbox,
prove the configured host identity or authorize provider access. Signed packaging,
real authentication/provider behavior and installed UI acceptance remain separate.

No Swift product files changed. Native builds, signed packaging and Apple delivery
checks were not rerun. The existing 30-file/190826-byte core artifact is unchanged,
manifest `c1a19bd8e8f0def68efa45f2b913dc5b2123466a3da6a5c14ce0eb8fded7f527`.
This slice does not complete v1 or establish a working provider application.
