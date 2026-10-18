# Signed Swift XPC feasibility

The temporary sandboxed Swift app exchanged an exact bounded value with its
embedded XPC helper. The client pinned the service identifier/signing team; the
service pinned the client identifier/signing team. Both were set before resuming
connections. Deliberately mismatched requirements prevented successful replies.
The helper, app, Node child and synthetic containers were absent at completion.
[Spec 059](../../../specs/059-modular-node-v1.md) governs this proof.

The [record](xpc.json) identifies source base `11739c8`, exact input hashes,
generated identifiers, process IDs, artifact and extracted entitlements.
Retained bundle:
`successor/.build/xpc-059-20261018-a/FortSandboxProbe.app`.
The parent and both XPC helpers have only App Sandbox; Node has App Sandbox and
inheritance. Strict/deep signature verification passed with the existing selected
development identity. There are no network, JIT or filesystem exceptions.

The positive helper echoes at most 4,096 UTF-8 bytes. Each client call has a
three-second deadline. A second helper uses the same code with a deliberately
wrong expected client identifier. Its call reports `NSXPCConnectionInterrupted`
(4097). A wrong service identifier requirement on the positive helper reports
`NSXPCConnectionCodeSigningRequirementFailure` (4102). A timeout does not count
as rejection. This checks the observed connection behavior, not arbitrary hostile
clients or every possible signature failure.

Helpers have an eight-second lifetime bound. The harness observes their exact
generated executable paths and independently verifies the positive helper PID
is absent. Forced cleanup would fail the proof; none was required. Only after
owned processes are absent are the generated synthetic containers removed.
Nothing is registered with ServiceManagement or configured to start at login.

Verification:

- Red: the public packaging test failed with `Unknown synthetic scenario`.
- The first implementation failed its negative controls. The diagnostic run
  showed a valid round trip, an interrupted rejected-client connection and a
  timeout while reconnecting to a helper that had exited on invalidation. Keeping
  the helper alive within its bounded lifetime produced the specific service
  code-signing error. The final assertions require the observed 4097/4102 errors.
  Signature requirements and entitlements were not loosened.
- All four signed sandbox scenarios passed, zero skips: XPC, compiled core,
  hung-child cleanup and the original SQLite/socket denial controls.
- The retained artifact passed the same checks and preserved the core's
  authenticated exact-message, restart/replay, duplicate and socket-cleanup proof.
- Strict TypeScript passed after capturing the narrowed test PID in a local
  constant. Full structural and accepted/staged spec gates passed. Unrelated
  portable/native suites were not rerun; their latest results remain 831 Node
  and 63 Swift tests, with fresh unsigned native builds at the removal slice.

The XPC exchange and compiled-core scenario are separate operations in this app.
The XPC helper does not yet own Node or forward service commands. This does not
prove a production bridge, persistent host, supervisor-death containment,
provider authentication, iPhone transport or TestFlight acceptance. A child that
inherits a sandbox shares its parent's privileges; agent-host isolation and the
computer-tool sandbox remain separate requirements.

Apple documents the embedded-service structure and on-demand lifetime in
[Creating XPC Services](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingXPCServices.html).
The actual Xcode 27.1 SDK `NSXPCConnection.h` exposes the macOS 13+ connection
requirement API; `FoundationErrors.h` defines the observed 4097/4102 errors.
Signing constraints follow Apple's
[requirement language](https://developer.apple.com/library/archive/documentation/Security/Conceptual/CodeSigningGuide/RequirementLang/RequirementLang.html).

Reproduce with the pinned Node 26.5.0 packaging CLI, the previously selected
existing signing identity, a new absolute destination and `--scenario xpc`.
Logs: `/tmp/fort059-xpc-{red,first,diagnostic,lifetime,green,artifact}.log` and
`/tmp/fort059-xpc-typecheck-final.log`. Failed diagnostic artifacts remain in
`/tmp/fort059-sandbox-xpc-test-5zu4bk` and
`/tmp/fort059-sandbox-xpc-test-dYJKSr`; their processes/containers were confirmed
absent. These fixtures contain no provider credentials or real conversations.
