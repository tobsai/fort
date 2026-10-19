# Private owning-host listener composition

October 19, 2026. [Spec 059](../../../specs/059-modular-node-v1.md), base
`f012d7ffac745b5026924717eb83f9b569a07786`; unchanged-scope
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md).

An owned core can explicitly compose private `networkControl` commands. Omission
leaves that frame unsupported and loads no transport implementation. Even when
composed, the controller starts stopped. The ordinary bootstrap does not opt in.
The service supplies exact current-primary authority; a phone, foreign installation
or replaced primary cannot control it. An expired, stopped or removed latest primary
can inspect/stop but cannot start. Post-commit notifications and one unreferenced
expiry timer reconcile lost authority; there is no idle polling. Notifications are
wakeups, not authority. Current service state is rechecked on every command.

The existing private pipe carries bounded JSON and retains its eight-operation,
size, timeout and cancellation limits. Canceled/timed-out replies do not invent a
rollback: an admitted start is recovered by exact retry or inspection. EOF/abort
closes the controller before waiting for its workers, then the service/runtime.
Each cleanup is attempted even if the other fails. Unconfirmed listener shutdown
remains failed. No HTTP control route or bearer-token exception was introduced.
TLS keys are not written to the owner ledger, restored after restart or logged.
Agent-host permissions and computer-tool confinement remain separate boundaries.

## Verification and observed failures

- The first public owned-host TLS journey failed because its private frame was
  unsupported. It now pairs a synthetic primary, inspects stopped state, explicitly
  starts TLS, rejects an unauthenticated request (401), accepts the exact primary
  (200), rejects foreign control and a proposed HTTP control route (404), retries
  without another listener and confirms stop/connection refusal.
- Expiry notification and public controller reconciliation tests failed before
  implementation. Two service tests now cover active/expired/removed/replaced/closed
  authority and one deadline without polling. Controller reconciliation closes the
  endpoint without replaying an old start after authority returns.
- Focused host checks: 11 passed, one opt-in budget skipped, zero failures before
  the additional review regressions. They cover paired-phone denial, owner stop and
  disconnect, EOF/abort, failed runtime cleanup, canceled/timed-out starts, malformed
  commands/frames, restart and compiled-only operation.
- An initial lost-reply test incorrectly assumed a sub-millisecond deadline would
  beat binding. The fixture now deliberately delays the public HTTPS listen callback
  by 100 ms. Cancellation/timeout is deterministic; exact retry observes one listener.
- Review exposed three further defects, each reproduced first: static import kept
  disabled networking dependent on its files; EOF waited for an opening worker before
  initiating its cleanup; a mutable caller option could enable networking after
  startup began. Lazy import, cleanup-before-worker-wait and synchronous option
  capture fixed them. The EOF red required fixture SIGKILL at seven seconds; the
  corrected test exits cleanly with runtime disposal and no process group remaining.
- Missing TLS close acknowledgment fails after five seconds and still closes the
  private socket/disposes runtime. Both this case and stalled-opening EOF passed.
- Compiled-only networking passed, followed by physically removing both transport
  implementation files from that disposable artifact and successfully restarting
  the ordinary host with retained owner authentication. The test invokes compiled
  JS with source stripping disabled. No production input was removed.
- **Full portable suite: 1,308 passed, 15 skipped, zero failed**, 42.781 seconds.
  Skips are 14 opt-in budgets and unavailable alternate 127.0.0.2 source address.
  `/tmp/fort059-owner-network-portable.log`. Strict TypeScript passed:
  `/tmp/fort059-owner-network-typecheck.log`.
- Isolated private-pipe inspect/start/exact-retry/authenticated-TLS-request/stop:
  five warmups then 20 samples; p50 **4.991 ms**, p95 **7.133 ms**, maximum
  **7.356 ms**, target p95 below 100 ms. Process startup, disposable identity
  generation, provider, LAN and user time excluded. Two Node processes, one ledger,
  private socket and loopback TLS; no concurrent Fort build/test. MacBookPro18,2,
  10 CPUs, 64 GiB, macOS 27.2 arm64; Node 26.5.0, TS 6.0.3, @types/node 26.6.5.
  Samples `/tmp/fort059-owner-network-samples.json`, log
  `/tmp/fort059-owner-network-budget.log`.
- Fresh `/tmp/fort059-owner-network-core`: **46 payload files / 267,027 bytes**;
  **47 files / 274,370 bytes including manifest**. All hashes, lengths and exact
  membership verified independently. Manifest SHA256:
  `50206c005032b0b6c9de4900d5adbe2cec3538d82c8cfaa58ed214c38a54cc8b`.

Use pinned `/opt/homebrew/opt/node@26/bin/node` with
`successor/verification/test-suite.mjs --suite portable`, then strict
`successor/node_modules/typescript/bin/tsc -p successor/tsconfig.json`.
Benchmark: `--test --test-name-pattern='twenty isolated private owner listener'
successor/host/owned-core.test.ts`, with a fresh FORT_OWNER_NETWORK_BENCHMARK path.
Focus/red logs are `/tmp/fort059-owner-network-{red,green,focus,removal-red,
removal-green,eof-red,eof-green,mutation-red,mutation-green}.log`;
service/controller reds are `/tmp/fort059-owner-access-red.log` and
`/tmp/fort059-owner-reconcile-red.log`. Raw logs contain synthetic fixtures only.

## Limits and next priority

No native network control UI, signed XPC networking, production listener,
entitlement, TLS trust, provider sign-in, startup service or real phone was enabled.
Native/guarded/signed tests and installed UI were not rerun for this Node slice;
previous evidence remains revision-bound. Archive/upload/Apple processing/TestFlight
and physical acceptance remain open. Source, fixture and compiled-artifact proof is
not a usable installed app.

Latest owner steering prioritizes the installed native Mac launch/chat/restart/
reconnect journey with an actual provider, then TestFlight, ahead of phone UI.
Finish that exact package and its authorization prerequisites next. Existing
private-host source supports this without enabling the optional phone listener.
Apple sign-in is being switched from the verified Admin session to the explicitly
selected mtree login through existing secure UI. No password was revealed/copied,
new certificate/key generated, agreement accepted, profile provisioned or startup
service activated. The G2 action remains separate from TestFlight distribution.
