# Process readiness and bounded cancellation

The full replay-suite run exposed a test race: its 100 ms cancellation timer
could fire before the synthetic child handshake, producing zero attempted start
writes. The test meant to exercise backpressure after readiness.

The synthetic supervisor now returns a readiness promise. It resolves true only
after the exact thread handshake has been validated and dispatch writes queued,
or false if the supervised process finishes first. It never rejects unobserved.
The backpressure cancellation test starts its bounded cancellation interval from
that readiness result. Startup remains subject to its own handshake deadline;
cleanup and group-absence assertions are unchanged.

- Red: the public readiness result was absent after a cleanly completed fixture.
- Green: all 23 real inert-process tests passed, including pre-ready failure,
  blocked stdin, cancellation, descendant/open-pipe cleanup and evidence ordering.
- Full registered Node suite: 759 passed, zero failures/skips, resolving the
  758/759 checkpoint in replay.md. Strict TypeScript passed.
- Source contains no provider activation or credential access.

Logs: `/tmp/fort059-process-ready-red.log`,
`/tmp/fort059-process-ready-green.log`, `/tmp/fort059-process-ready-node-all.log`,
`/tmp/fort059-process-ready-typecheck.log`. The native 60-test/HTTP passes belong
to the preceding replay slice; no native code changed in this correction.
