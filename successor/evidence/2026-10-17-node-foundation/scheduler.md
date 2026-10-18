# Scheduler wakeups and idle writes

Spec 059 slice, Node 26.5.0. Only temporary synthetic installations were used.
The scheduler uses one timeout for the nearest occurrence, capped at one minute
for clock/sleep reconciliation. With no remaining enabled occurrence it has no
scheduler timer. Schedule mutation recomputes the timeout after commit. Failed
evaluation is visible at authenticated `GET /v1/scheduler`, with a one-minute
retry. Shutdown clears the timer. No persistent service was activated.

No-due evaluations return without a transaction or checkpoint write. Missed-work
policy now uses actual occurrence lateness and startup recovery; removing idle
writes must not make an on-time occurrence appear missed. Existing DST gap/fold,
one-time, overlap, multi-year recovery and exact consent behavior stays covered.

- Red: idle evaluations changed the WAL digest; scheduler status returned 404.
- Green: 72 affected service/settings/pairing/shutdown tests passed, zero skips.
  Sixty one-second synthetic clock advances left the WAL byte digest unchanged.
  The next on-time occurrence was accepted once despite an hour without writes.
  An actual two-second timeout claimed one occurrence and returned to idle.
  Corrupt persisted schedule data exposed a fault and a 60-second retry deadline.
- Strict TypeScript and structure/implementation gates passed.

Commands: `node --test successor/service/openai.test.ts
successor/service/agent-settings.test.ts successor/service/owner-pairing.test.ts
successor/service/openai-shutdown.test.ts`, and `npm --prefix successor run
typecheck`. Logs: `/tmp/fort059-scheduler-final.log` and
`/tmp/fort059-scheduler-typecheck.log`.

An initial diagnostic assertion attempted a large byte-array diff. Its identified
synthetic test processes were stopped and the assertion was replaced by a SHA256
comparison; the final red/green runs completed. The service's exclusive ledger
lock was retained. These checks are not a 60-second CPU/energy/wakeup measurement.
Native fault presentation and event-driven client refresh remain separate work.
