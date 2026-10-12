# Runner boundaries

Only the deterministic fixture worker is connected to the service. The Codex
module is an **unwired, in-memory protocol adapter**. It cannot launch a process,
open a connection, sign in, read configuration or credentials, or write the ledger.
Stage 2's official-provider exit remains open.

## Codex turn seam

`CodexTurn` accepts one immutable attempt, an existing exact provider thread ID,
a request ID, a normalized absolute workspace path and exact UTF-8 text. Callers
invoke `start`, feed bytes to `receive`, request `interrupt`, and report a lost
transport with `disconnected`. Each result contains ordered wire writes,
normalized evidence and a request to close the transport. No transport is owned
by this class. Repeated `start` calls never produce another start request.

The private [runtime evidence types](../contracts/runtime.ts) preserve run,
attempt, generation, binding revision, runtime version and selected model, plus
an adapter-local sequence. These fields are correlation metadata, **not runner
authentication, authorization, attestation or durable fencing**. The supplied
runtime version is a binding assertion; the adapter cannot verify a binary.
Nothing changes the native/public v1 contract.

The adapter bounds input to 65,536 UTF-8 bytes, individual incoming frames to
262,144 bytes, total wire input to 1,000,000 bytes, deltas and final text each to
65,536 bytes, pending pre-response notifications to 32 and assistant items to
128. Split UTF-8 and coalesced frames are supported. Foreign thread/turn events
cannot provide evidence for the selected attempt. Early notifications wait for
the correlated start response; duplicate final item evidence must agree.

Only completed assistant items supply authoritative text, with reported
commentary/final-answer phase retained or explicitly unreported. Deltas remain
progress. Reasoning content and raw provider diagnostics are not emitted.
Completion requires a matching terminal notification and complete final items;
interrupted/failed terminals emit no successful reply. An interrupt acknowledgement
is not cancellation proof. Disconnect, malformed/oversized output, start rejection,
unsupported requests/tools and model rerouting seal the attempt as unknown; they
never authorize a resend. Interruption before `start` supplies never-started
evidence without producing wire output. Effective model remains unreported;
neither the selected model nor a model's textual claim is runtime attestation.

Every server request, including permission, tool and external-token requests, is
rejected. Unsupported tool activity produces unknown outcome and a best-effort
interrupt. This does **not** prevent an already started tool or prove termination.
The `unlessTrusted` and restricted read-only fields sent with `turn/start` are
requested provider policy, **not evidence of OS isolation**. This adapter must
not be wired to an ambient host-user Codex process.

## Required enclosing boundary

Before using this seam with an official runtime, the enclosing implementation must:

1. Authenticate an enrolled runner and fence installation/recovery epoch, host,
   attempt, generation, binding and event sequence outside provider output. Commit
   accepted evidence through the service's ledger transaction. The codec does
   not accept owner credentials or answer owner approval APIs.
2. Prove the chosen OS boundary against host home, ledger, owner credentials,
   Keychain, sibling runs, IPC/network, environment, symlink and descriptor canaries.
   Isolate the credential-bearing agent host from its computer/tool sandbox.
   Node permissions or an Ask/read-only label do not establish this boundary.
3. Pin and verify the official runtime executable and version, build an explicit
   environment allowlist, and use an exact fresh provider-owned config scope.
   Disable ambient credentials, config/skill/plugin discovery and API-key billing
   fallback. Do not copy an existing credential store or install/update a runtime
   automatically.
4. Implement initialization, selected-model availability, exact thread creation
   or safe resume, and bounded handshake/turn/stop deadlines. `CodexTurn` assumes
   the connection is already initialized and the exact thread is ready. It does
   not recreate history, reconnect, poll for completion, resume or retry.
5. Give one adapter exclusive ordered input for its turn. Write returned messages
   in order before bounded transport teardown; a close request or timeout does
   not imply the process stopped. Preserve unknown outcomes when reconciliation
   cannot prove what happened. Authenticate and deduplicate evidence before any
   durable state transition; never treat text or IDs alone as authority.
6. Validate the protocol against the exact runtime, then complete owner-authorized
   provider-native authentication and the real subscription/model/tool/interrupt/
   restart matrix. Initial tool/approval requests remain unsupported until an
   exact Fort owner-approval mapping is implemented and tested.

These are open implementation and acceptance requirements, not a claim that an
isolation arrangement or runner enrollment exists. No owner action is needed to
continue deterministic boundary tests. A real provider turn must wait for proven
confinement and any necessary concrete OS setup authorization, followed by the
owner's provider-native sign-in in that exact environment. Login alone cannot
close the isolation gate.

## Protocol provenance

The adapter and fixtures are newly authored; no donor implementation, provider
source, generated schema, credential or recorded provider conversation was copied.
The extraction list remains empty. Official [app-server documentation](https://learn.chatgpt.com/docs/app-server)
and [authentication documentation](https://learn.chatgpt.com/docs/auth), consulted
2026-10-12, informed the turn framing and provider-owned authentication boundary.
These pages describe the current protocol, not a version-pinned compatibility proof.

Read-only symlink metadata locates a candidate at
`/opt/homebrew/Caskroom/codex/0.157.1/bin/codex`. No Codex binary was executed, even
for version or schema generation, and no provider configuration was inspected.
The candidate version is inferred from the path; compatibility remains unverified.
Once a safe execution environment is proven, use that exact runtime's generated
schema and synthetic conformance tests before enabling a real turn. There is no
silent selection of a newer runtime or fallback provider.

See [test evidence and remaining gates](../evidence/2026-10-12-codex-protocol.md).
