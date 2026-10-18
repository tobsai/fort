# Signed helper crash recovery

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md). Source base
`234edd2`; exact inputs, final result, independent checks and preserved failures
are in [xpc-crash-recovery.json](xpc-crash-recovery.json). This is a synthetic
signed bundle, not ordinary installed-product, provider or TestFlight acceptance.

The fixture keeps its ledger in one private directory inside its unique XPC
container. After a completed turn and cooperative core restart, a fixture-only
route kills the helper with SIGKILL. The pinned client confirms the old helper,
guardian, Node PID and owned group absent before requesting a replacement.
The replacement helper reclaims the recorded stale socket, retains installation
and pairing, and returns the original receipt and exact history without executing
the turn again. Normal owner recovery then disposes runtime, revokes pairing and
closes the replacement core. The external runner checks all three core groups,
all three Node PIDs and both helper PIDs.

Two failures exposed separate assumptions:

- Artifact `successor/.build/xpc-crash-059-20261018-a` timed out because its
  three-second bootstrap deadline expired during a ten-second launchd restart
  throttle. The scoped system log explicitly recorded “service spawn deferred
  by 10 seconds due to throttle”. Only recovery bootstrap now allows twelve
  seconds; the fifteen-second parent-work and three-second cleanup bounds stay.
- Artifact `successor/.build/xpc-crash-059-20261018-b` reached the new helper but
  core startup failed. Stored socket ownership exactly matched the filesystem.
  A separate public service test reproduced failure in a long private directory:
  recovery had used an absolute AF_UNIX address. It now uses the same short
  relative name as binding and forwarding inside the validated working directory.
  Filesystem identity checks remain absolute. The test-only client alias does
  not relax the service's directory/symlink checks.

Both original artifacts/results remain unchanged. After known PID/parent-group
and exact executable-path absence checks, only their four generated containers
were removed. The raw failure flags were not rewritten into passes. Independent
cleanup records and scoped log paths accompany them in the JSON evidence.

Passed:

- All four signed scenarios passed serially, including the hung-child timeout
  negative control. The retained successful artifact is
  `successor/.build/xpc-crash-059-20261018-c/FortSandboxProbe.app`.
- In that artifact, abrupt cleanup took 14.88 ms. Recovery bootstrap took
  10,157.82 ms, including launchd's deferral; parent work took 11,450.47 ms.
  These are one-run lifecycle diagnostics, not normal core startup budgets.
- Exact peer identity checks, owner recovery, all sandbox denials and group
  ownership passed across every core generation. No entitlements were expanded.
- Fifteen socket tests, all 862 portable tests, strict TypeScript and all 84 Swift
  tests passed. Swift used the exact compiled core from the retained artifact.
- Independent deep/strict signature, seven exact entitlement targets, both copies
  of all 24 core hashes, known PID/group and executable-path absence, and generated
  container removal passed. No forced XPC cleanup. Core bytes: 173,076; manifest:
  `934fe8a8ad112c585fbc4d547a7c2b024bab4a4ca5ba5145f5546ab7037d3275`.
- Full/implementation spec gates and whitespace checks passed.

Logs: `/tmp/fort059-xpc-crash-{red,first,second,third,long-path-red,long-path-green,signed,portable,typecheck-final,swift}.log`.
Failure cleanup: `/tmp/fort059-xpc-crash-{cleanup,second-cleanup}.json`.

Ordinary app setup/removal, official runtime execution/containment, secure iPhone
reachability and installed accessibility/screenshots remain open. A socket created
before its durable ownership record remains unknown and is preserved. No real
credential, account, startup registration, provider cleanup or distribution action
was performed.
