# Swift-owned private core process

`MacCoreProcessTransport` now implements the native request contract over the
portable private-pipe relay. Explicit composition supplies the executable,
arguments, private directory and complete environment. It launches without a
shell or inherited environment, creates its own process group, and connects only
stdin/stdout; diagnostics are discarded. It neither enrolls a device nor chooses
a provider. [Spec 059](../../../specs/059-modular-node-v1.md) governs this slice;
source base `49867c4` plus these files.

The transport uses nonblocking descriptors and dispatch sources for I/O and exit.
Readiness and write drain each have a five-second bound. At most eight calls may
remain outstanding, including canceled calls awaiting acknowledgment. Requests
have monotonic IDs and at most 30 seconds; accepted input/output/body limits are
2/12/8 MiB. Corrupt, duplicate, foreign-ID and unsupported-version responses fail
the channel. Cancel/deadline settles the caller once, sends a correlated cancel
and allows five seconds for acknowledgment. It never retries a domain request.

Close sends EOF, then TERM at two seconds and KILL at four if needed. It returns
only after reaping the child and observing its group absent, or reports failure
at six seconds. Normal close, forced shutdown and protocol failure are distinct.
The wrapper requests close on destruction. This is process ownership, not an OS
containment claim for hostile descendants that escape their group. The agent
host and computer-tool sandbox remain separate.

Verification:

- The new public Swift journey initially failed because the transport type did
  not exist. A subsequent startup failure was traced to Node's TypeScript parser
  needing WebAssembly when JIT is disabled. Source fixtures use normal parsing;
  the compiled artifact uses `--jitless --no-experimental-strip-types`.
- All 74 Swift package tests passed with Swift 6 checking, including parameterized
  controls. The actual native client and core prove unauthorized/foreign identity
  rejection, explicit synthetic enrollment, exact Unicode/whitespace messages,
  replay, cancellation, timeout recovery, close/socket absence and restart receipt
  recovery with zero duplicate execution.
- Eight out-of-order responses preserve their request bodies; a ninth active
  call is refused locally. Cancel acknowledgments release admission and the same
  channel accepts another request. Malformed JSON, oversized frame/body, foreign
  ID/version, invalid status, duplicate response and early child exit fail closed.
- Absent readiness fails and reaps its process. An ignored cancel, blocked input,
  and a TERM-resistant child plus descendant complete within their bounds. Forced
  shutdown is reported for the latter two. Each owned PID/group is checked absent.
- Early-exit descriptor ownership was corrected to avoid closing the same handle
  through both the exit path and dispatch cancellation. Frame scanning now starts
  after previously searched bytes; the seven malformed-output cases dropped from
  about 10 seconds to 0.27 seconds in the local debug test. This is a regression
  observation, not a controlled benchmark.
- The finalized native journey also passed against retained artifact
  `successor/.build/core-channel-059-20261018` with JIT and type stripping disabled.
  Its manifest/hash evidence remains in [the relay record](private-channel.md).
- Structural, accepted/staged spec and whitespace checks passed. No TypeScript
  production code changed; the preceding 836 portable tests remain separate.

This slice has no signed sandbox, XPC-to-core, provider, installed UI or physical
device claim. The ordinary app still uses its existing composition. XPC service
ownership, bundle resource selection, real setup/recovery/removal, runtime
containment and TestFlight remain open. Synthetic bootstrap files are generated
only in disposable test directories and never supplied by the transport.

Logs: `/tmp/fort059-native-process-red.log`, `-first.log`, `-controls.log`,
`-swift.log` and `-compiled.log` under the same prefix.
