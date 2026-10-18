# Core parent-exit guard

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md). Source base
`90d901a`; exact source hashes, signed artifact, measurements and independent
checks are in [parent-exit-guard.json](parent-exit-guard.json). This is synthetic
lifecycle and packaging evidence, not installed-product or provider acceptance.

The regression killed the real Swift supervisor while its Node fixture and
descendant ignored EOF and SIGTERM. Before the guard, the owned group survived
the three-second limit. The test's finalizer removed that exact group. With the
guard, every reported PID and the group disappeared in about 12 ms; the test
checked absence before its cleanup finalizer.

The small Swift executable verifies the exact parent PID and its own group
leadership, registers a kqueue parent-exit event before spawning, and blocks a
native thread on that event. Parent exit or monitor failure kills the owned
group. The selected executable, arguments, working directory and environment
stay explicit. Node inherits only standard descriptors and the guardian's group;
the guardian closes its own standard descriptors after spawning and reaps Node.
Normal completion preserves its exit status. The monitor descriptor remains
owned until process exit, avoiding a close/wakeup race during normal return.

`MacCoreProcessTransport.launch` now requires an explicit lifecycle choice.
Existing unit fixtures select cooperative group supervision. The signed XPC core
selects its own bundled guardian. This is an owned-process lifecycle boundary,
not a sandbox for hostile children that deliberately escape their group. The
agent-host and computer-tool permission boundaries remain separate.

Passed:

- Six macOS guardian tests: abrupt supervisor death; exact request/clean close;
  wrong parent/shared-group rejection before launch; exact child exit status and
  missing executable; ten alternating warm launches; twelve-second idle sample.
- Added warm startup p95 7.50 ms against the 100 ms target. Guardian-only RSS
  5.70 MiB against 25 MiB. CPU time did not change; including the 10 ms reporting
  resolution gives an upper bound of 0.0833% against 0.1%. These short unsigned
  fixture measurements do not establish installed-host, energy or provider costs.
- All 84 Swift tests passed against the retained compiled core. Strict TypeScript
  and test-discovery checks passed. The first typecheck caught an unsupported
  `spawnSync` test option; the helper now uses typed asynchronous `spawn`.
- The signed XPC suite and final retained signed artifact passed. Evidence
  distinguishes guardian and actual Node PIDs, confirms group membership across
  restart, and requires their absence. Identity mismatch, owner recovery,
  durable messages, outer-executable denial and sandbox controls remain intact.
- Independent deep/strict signature, all seven exact entitlement targets, both
  copies of all 22 core file hashes, known PID/group absence, exact bundle
  executable absence and generated-container removal passed. No forced cleanup.
  Core manifest remains
  `4bdc218107386e8592dbaf0e3b2fb59e66ff9d01d73f24a485a83098e9cc249b`.

Retained artifact: `successor/.build/guardian-059-20261018/FortSandboxProbe.app`.
Logs: `/tmp/fort059-guardian-{red,green,controls,final,typecheck,typecheck-final,swift,swift-final,signed,artifact,discovery}.log`.
The original failure and first typecheck failure are retained. Portable core tests
(848 passing in the preceding core slice), the other three signed scenarios and
ordinary app builds were not repeated for this native lifecycle change.

The signed fixture proves guarded normal execution; the abrupt-death regression
uses the actual Swift transport in an unsigned synthetic supervisor. Abrupt death
inside installed XPC, ledger/socket crash recovery, executor cleanup when Node
dies, real owned setup/removal and provider/distribution acceptance remain open.
No credential, provider account, startup registration or ordinary app was changed.
