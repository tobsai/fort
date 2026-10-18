# Executor parent-exit ownership

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md). Source base
`05d28c5`. This extends the [native core guard](parent-exit-guard.md) to the
optional process executor; it does not activate an official provider.

The regression submits a synthetic plain-text turn through the actual service
and `ExecutorProcess`, waits for its noncooperative worker and descendant, then
kills only the Node service PID. Before the change, the executor group survived
the three-second limit. The test finalizer removed the exact group and confirmed
absence. After the change, owner 19575, guardian/group 19583, executor 19604 and
descendant 19607 were all absent, including the group, about 11.25 ms after the
owner was killed. Absence was checked before final cleanup.

Executor composition now requires either cooperative process-group supervision
or an exact Mac guardian. The latter is refused on other platforms and rejects
relative/NUL-containing guardian paths. Configuration is copied at construction;
the fixture mutates its original guardian selection afterward and still launches
the selected executable. Missing guardians fail without an unprotected fallback.
The guardian watches the Node owner and launches the worker in its own group
with the exact selected environment and standard pipes. Dispatch/evidence,
deadlines and cancellation remain unchanged; success still requires correlated
terminal evidence and clean exit with group absence.

Passed:

- Eight macOS checks covering owner death plus guarded exact replies, cancellation,
  permission denials, missing guardian, stale evidence and terminal-then-hang.
  The fixture's Node permissions still deny ledger reads, network and child
  launch; the deliberate orphan regression separately permits its synthetic
  descendant so group cleanup is exercised.
- All 849 portable tests passed, including the existing executor behavior suite,
  new configuration rejection and compiled-core removal checks.
- Strict TypeScript, full/implementation spec gates and whitespace checks passed.

Logs: `/tmp/fort059-executor-parent-{red,green,controls,controls-final,controls-complete,typecheck,portable}.log`.
The first guarded-control run failed because its deliberately hung fixture had a
30-second service deadline but a ten-second harness limit. That original failure
is retained. Its fixture now selects a 200 ms service deadline for that case;
the unknown outcome and process-group absence pass within the unchanged limits.

The reusable native guard's short launch/idle measurements are in the preceding
evidence; no new whole-host or provider performance claim is made. The guard
exists only during an active executor. Native app builds and signed packaging
were not repeated because this slice changes the optional Node executor and its
synthetic fixtures, not Swift or packaging code.

Official Codex/Claude process adapters, signed executor containment, installed
XPC crash recovery, private socket recovery and ordinary setup/removal remain
open. Process groups do not confine hostile descendants that escape them. Agent
host privileges and computer-tool permissions remain distinct. No real credential,
provider, persistent access, startup registration or distribution action occurred.
