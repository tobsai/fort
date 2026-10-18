# Sandbox timeout cleanup correction

On October 18, the coordinator relayed an independent review of `8ae9179`.
It reproduced a Node child surviving termination of the Swift parent's process
group. Foundation `Process` had placed the child in another group. A child holding
its inherited output pipes could also prevent the supervisor's close event.
The earlier normal-exit proof did not cover this failure path.

The Swift probe now uses `posix_spawn` with an explicit process group inherited
from its owned parent. Its control record identifies the child PID and observed
group. The supervisor bounds startup at fifteen seconds, starts the selected
execution timeout once the child is reported, and has a two-second completion
fallback plus at most 1.5 seconds for absence verification. It verifies parent
PID, child PID and process-group absence. Container removal requires all checks;
an unconfirmed cleanup retains the container and reports failure.

The new test deliberately keeps the signed Node child's output pipe open. It
requires proof that the child ran, a timeout result, execution/cleanup within
four seconds, full run completion within twenty seconds, and independent ESRCH
checks for both PIDs. Its configured execution timeout is one second. These
timings exclude compilation and signing. The initial new test failed because
the scenario option was absent. A later run exposed a test setup error: timing
from app launch killed Swift before Node started. Separating the bounded startup
and execution phases made the intended hung-child path observable.

Final command: explicitly supplied existing Apple Development identity with
`node --test successor/packaging/sandbox-feasibility.test.ts`. Both the hung-child
and normal SQLite/denial tests passed, zero failures/skips. Strict TypeScript
checking passed. A narrow follow-up scan found no test UUID container or private
temporary-directory prefix. No provider, credential creation, startup registration
or security exception was involved. Normal and hung scenarios retained the same
parent/child sandbox entitlements and strict bundle-signature verification.

This corrects the synthetic packaging supervisor. It does not establish complete
containment of arbitrary descendants or replace the remaining production runtime,
lifecycle, TestFlight and physical-device acceptance gates.
