# Invalidation replay and native refresh

Spec 059 slice. The service now exposes authenticated `/v1/events` with explicit
`invalidation` semantics, monotonic cursors, 256-event pages, 10,000-event
retention, waits up to 25 seconds and at most eight simultaneous waiters.
Expired/future cursors and changed recovery epochs require a durable refresh.
Waiters release on change, timeout, client close or service shutdown; authorization
is checked again before a delayed response. Browser-origin restrictions remain.

Installation reads include a cursor captured before native collection reads.
Workspace changes and scheduler fault transitions produce invalidations alongside
message/run changes. Swift waits on that cursor while active, refreshes only on
change, and recovers expired cursors through a full durable refresh. Backgrounding
cancels the task. Idle event pages do not rewrite cache files. An unchanged
snapshot also avoids a redundant cache write. Older services without the advertised
capability retain a 30-second compatibility refresh. Scheduler faults appear in
the native view; installed accessibility/visual acceptance is still unrun.

Verification used temporary synthetic data only:

- Red: service lacked the replay capability; native journey failed to compile
  because the wait method was absent.
- 76 affected service/migration/settings/pairing/shutdown tests passed. Coverage
  includes paging, exact cursor resume, idle timeout, retention expiry, eight-wait
  backpressure and capacity recovery. Existing transaction and recovery tests pass.
- 60 Swift tests passed, including no idle full refresh/cache modification and
  expired-cursor refresh followed by idle resume.
- Compiled `FortNativeCheck` passed against the actual local HTTP service, including
  invalidation wakeup and idle replay, ten synthetic executions, six restarts,
  discarded responses and no duplicate/canceled execution.
- Strict TypeScript and structural/implementation gates passed.
- The registered full Node run passed 758/759, with zero skips. The existing
  blocked-stdin cancellation test canceled at its fixed 100 ms timer before
  `turn/start` was attempted under concurrent load. This is an unresolved test
  synchronization failure at this checkpoint, not an all-suite pass. A separate
  focused correction and full rerun follow.

Logs: `/tmp/fort059-replay-final-service.log`,
`/tmp/fort059-replay-native-final.log`, `/tmp/fort059-replay-native-http.log`,
`/tmp/fort059-replay-typecheck-final.log`, `/tmp/fort059-replay-node-all.log`.
No real provider, account, startup service, installed UI or distribution was used.
