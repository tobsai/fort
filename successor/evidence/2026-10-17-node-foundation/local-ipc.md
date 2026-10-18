# Private socket feasibility under App Sandbox

The signed synthetic Node child completed a Unix-socket round trip inside its
private container while outside-file reads/writes and TCP loopback listening
remained denied. This establishes one local IPC option under the unchanged
sandbox entitlements. It does not implement the Mac host or its XPC lifecycle.
[Spec 059](../../../specs/059-modular-node-v1.md) governs this bounded proof.

The [observed result](local-ipc.json) records the exact artifact, process IDs,
source input hashes and entitlements extracted from the signed binaries. Source
base is `26bc27b`; the input hashes identify this subsequent probe revision.
The retained bundle is under
`successor/.build/local-ipc-059-20261018-a/FortSandboxProbe.app`.

The Swift parent has only `com.apple.security.app-sandbox`. The Node child has
only that entitlement and `com.apple.security.inherit`. Both retain hardened
runtime signing; no JIT, network or filesystem exception was added. Node runs
with JIT disabled. The private socket uses a short name relative to the fresh
private working directory, avoiding long container paths at the Unix socket
API. Its server and client both execute in the same synthetic Node process.
The exchange has a two-second bound and a 64-character response bound.

Verification:

- Red: the signed probe lacked the private-socket result.
- Green: both sandbox tests passed, zero skips, using the previously selected
  existing development identity. These cover the normal private SQLite/socket
  journey and termination of a child that retains its output pipe.
- A fresh retained probe completed, reported the private round trip and all
  negative controls true, and confirmed parent/child/process-group absence and
  synthetic container removal. It used no provider and registered no startup job.
- Independent strict/deep signature verification passed. Signed entitlements
  were extracted and matched the unchanged set. An initial restricted-shell
  inspection could not evaluate local trust; verification passed with normal
  authorized access to the existing local signing configuration.
- Strict TypeScript and the structural/accepted spec gates passed. The unrelated
  portable suite and runtime relocation test were not repeated; the most recent
  portable result remains 829 passing at the preceding read-replay slice.

The agent host and executor remain separate security boundaries. Inheriting the
parent sandbox does not isolate a child from the parent's privileges. Apple's
[sandbox guidance](https://developer.apple.com/library/archive/documentation/Miscellaneous/Reference/EntitlementKeyReference/Chapters/EnablingAppSandbox.html)
describes inheritance and recommends XPC for privilege separation. This probe
does not establish cross-process peer authorization, service HTTP over a socket,
XPC, login/background registration, update/removal recovery, workspace grants,
official-provider authentication or reachable iPhone transport. No shipping
transport decision or entitlement expansion is inferred from this result.

Logs: `/tmp/fort059-local-ipc-red.log`, `/tmp/fort059-local-ipc-green.log`,
`/tmp/fort059-local-ipc-artifact.log`, `/tmp/fort059-local-ipc-typecheck.log`.
