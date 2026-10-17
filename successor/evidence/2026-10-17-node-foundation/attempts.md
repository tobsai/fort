# Service-owned dispatch identity

Under [Spec 059](../../../specs/059-modular-node-v1.md), dispatch now carries the
service's durable attempt ID and generation through `ChatInput`. Codex uses that
same attempt in its protocol state machine. It no longer allocates a competing
attempt. It rejects mismatched run, binding, model, runtime version or invalid
generation before provider I/O. Existing stored runs without a generation remain
readable and acquire generation one on their next dispatch.

The authenticated service test first failed because dispatch omitted the attempt.
It now observes the running attempt through the public snapshot API, confirms the
runtime received it, verifies duplicate submission cannot dispatch twice, and
checks identity survives restart. The adapter test first accepted a foreign run;
it now rejects that dispatch with zero protocol calls.

The affected service, Codex adapter and startup-contract suite passed 440 tests,
zero failures/skips. Strict TypeScript checking passed. The initial restricted
test invocation could not open its loopback listener; normal-context execution
was approved and used only synthetic data, temporary storage and fixture runtimes.
No live provider or credential was accessed. Three recovery fixtures needed new
attempt identities when they constructed a new run; the final suite passed.

This fixes competing IDs in the existing in-process adapter. It does not yet
prove the planned process-boundary evidence envelope, dispatch digests/limits,
outbox recovery, or a real provider turn under the new foundation. Those remain
explicit v1 work.
