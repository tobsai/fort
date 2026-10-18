# Active-core executor process contract

Design: [Spec 059](../../../specs/059-modular-node-v1.md). The existing Node
implementation decision covers this local process and public-API slice.

The active application now seals a dispatch in the same transaction as its
service-owned attempt and compact context record. The digest covers installation,
recovery epoch, exact binding, attempt/generation, complete input/configuration/
history digests, policy, limits and deadline. Inputs and metadata are immutable
before an adapter receives them. The plain-text policy has no external grants.
Digests detect mismatches; they are not credentials or an approval mechanism.

The optional `ExecutorProcess` adapter uses an explicitly selected executable,
arguments, working directory and environment. It sends one dispatch over owned
pipes, validates bounded terminal evidence, and correlates cancellation to the
same attempt/generation/digest. It accepts identical terminal repeats once,
rejects conflicting or foreign results, and requires a clean observed child and
process-group exit before returning a confirmed result. It bounds stdin waiting
by the dispatch deadline and shutdown timers. Failure remains unknown and keeps
later work in that conversation behind the durable barrier.

The default deadline is 120 seconds. Input is limited to 1 MiB, reply text to
64 KiB, total stdout/stderr evidence to 256 KiB and terminal frames to eight.
Cancellation gets 250 ms for a correlated result, then TERM and another 250 ms
before KILL, with a bounded final cleanup observation. Missing cleanup blocks
new dispatch. Evidence is provisional until exit and cannot complete after the
dispatch deadline. No worker receives the device token or a ledger handle.

## Verification

The initial test failed because the executor implementation did not exist. The
new service journey then passed exact Unicode text, durable reply and duplicate
receipt behavior. Synthetic stale-generation and cancellation modes were added
through failing expectations and verified through the authenticated service.

A later failing test reproduced a real defect: a valid result arriving after
the deadline could complete during cancellation grace. The parser now refuses
expired evidence and the deadline marks the result unconfirmed. That test passes.

| Check | Result on October 18, 2026 |
| --- | --- |
| Executor public service/process suite | 24 passed, 0 failed/skipped |
| Full discovered portable suite | 816 passed, 0 failed/skipped |
| Strict TypeScript | Passed |
| Compiled FortNativeCheck against the changed service | Passed; ten synthetic executions, six service restarts, lost responses and cancellation reconciliation without duplicates |

Fault coverage includes stale generation, wrong attempt/digest, contradictory
terminals, oversized text, stdout/stderr flooding, invalid UTF-8, missing terminal,
nonzero exit, repeated terminal, late terminal, changed input/policy digest,
expired dispatch, ignored cancellation, unread stdin and a child that hangs after
reporting success. Hung cases verified both child PID and group absent with
ESRCH. Their completion stayed within the test's 2.5-second deadline plus cleanup
budget; this is a bounded behavior test, not a performance benchmark.

The trusted synthetic worker runs with Node permissions allowing only its source
file. Actual probes confirm its source is readable and synthetic ledger access,
child spawning and loopback networking are denied. Its environment contains only
macOS's injected `__CF_USER_TEXT_ENCODING`, or no entries on other platforms.
The permission test initially assumed networking failed synchronously; Node
reports that denial on the socket's error event. The corrected check observes
that event. No parent environment values or credential contents were inspected.

## Limits and remaining work

The core still supports the existing trusted in-process official adapter during
migration. That path does not yet implement this process protocol or its process
deadline/cleanup proof. The native HTTP check uses fixtures. No real provider,
new credential, startup registration or external connection was activated.

An owned process group and Node permissions do not confine hostile native code
or prove the agent host's OS sandbox. The adapter does not claim containment of
descendants that escape its group, survival cleanup after abrupt supervisor death,
or a complete installed lifecycle. Swift supervisor/OS containment and official
runtime packaging remain separate gates. The earlier signed App Sandbox probe
is separate evidence, not an attestation for this worker.

Protocol v1 is a single terminal result, with no reconnect/retry execution stream.
Restarted active work remains unknown; it is not automatically replayed. The
service's stored attempt identity and state checks reject a result that no longer
belongs to the active attempt. Optional connector grants, module lifecycle,
official runtime execution, signed app delivery and physical acceptance remain.
