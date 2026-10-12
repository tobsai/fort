# Runner boundaries

The service defaults to the deterministic fixture worker. An explicitly selected
fresh synthetic ledger now supports authenticated fixture runner dispatch and
evidence ingestion. The Codex module remains an **in-memory protocol adapter**;
the new synthetic state machine exercises it without an official runtime. Neither
class launches a process, opens a connection, signs in, reads provider configuration
or credentials, or writes the ledger. Stage 2's official-provider exit remains open.

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
Nothing changes the native/public v1 contract. The synthetic HTTP bridge adds
authentication and durable fencing outside these adapter fields; it does not
make the adapter or provider output an authority source.

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

The synthetic slice tests the event/authentication portion of these requirements.
Production host enrollment, transport and isolation remain open; trusted fixture
enrollment is not a deployed runner identity system. No owner action is needed to
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

## Synthetic lifecycle and ledger bridge

`SyntheticTurn` consumes a recorded `SyntheticDispatch`, verifies its exact fixture
binding and text digest, and returns ordered actions. Handshake is a synthetic
thread-ID acknowledgement, not Codex initialization or model availability proof.
The injected clock drives 5-second handshake, 30-second turn and 2-second interrupt
deadlines. Closing requests get a 2-second teardown deadline and then a force-close
action. Late cancellation cannot extend an expired deadline. Deadlines, missing
terminal output and connection loss retain uncertainty. Never-started cancellation
requires a committed owner cancellation; after dispatch, a correlated interrupted
terminal can establish cancellation. An acknowledgement alone cannot.

Wire writes precede evidence and close actions in each returned batch. The tests
execute these actions in order against a fake transport and the real authenticated
loopback fixture API. This is a deterministic action engine, not a production
process/stdio loop: an enclosing implementation must drive its clock and actions,
bound actual blocked writes, own process termination, and report connection loss.
Force-close is an instruction, never proof that a process or external effect stopped.

The bridge's 40-second dispatch lease provides a separate service fence. Expiry
becomes durable unknown state on the next authenticated owner observation, runner
claim or exact runner status check. It never grants another attempt. This fixture
uses no always-running scheduler or host service. Runtime `failed` evidence is
retained privately and maps conservatively to the existing fixture v1 unknown
state; distinct production failure projection is still a later contract decision.

The synthetic binding stays visibly `provider: fixture`, version `1`, model
`fixture-echo-v1`. The private evidence's `provider: codex` names the exercised
wire codec; it is not evidence that an official runtime or selected model ran.
See [service seam and scope](../service/README.md#synthetic-runner-bridge) and
[crash/replay evidence](../evidence/2026-10-12-synthetic-runner-bridge.md).

## Bounded synthetic process experiment

`startSyntheticProcess` is an opt-in test harness, not a provider launcher or an
active service dispatch path. It launches only the fixed, newly authored
`process-fixture.mjs` with pinned Node 26.5.0, a fresh private empty directory,
explicit LANG/TZ, and pipes. No command, executable, shell, inherited environment,
provider configuration or credential can be selected through this API. The trusted
parent holds the dispatch/session credentials and delivers normalized evidence;
the worker receives only the synthetic protocol, exact text and thread identity.
The worker runs without Node permission flags so these tests do not mistake that
API policy for an OS boundary. It is trusted fixture code with ambient host-user
OS authority, and must never be replaced with an official or untrusted runtime.

Limits are enforced on real pipes: 5-second handshake, 30-second turn, 2-second
interrupt, 2-second teardown and 2-second cleanup observation. A complete outgoing
frame gets one 2-second write deadline, independent of chunk count; queued bytes
are capped at 524,288. Reads are fed incrementally with backpressure. Stdout is
capped at 1,000,000 bytes and stderr is drained/discarded up to 65,536; diagnostics
retain only bounded counters and categorical trace entries. One evidence upload
is active, at most two nonterminal events wait, and one terminal result is held
until cleanup. Each upload permits two bounded 2-second attempts of the identical
event, never another process start. These are separate asynchronous deadlines,
not a hard real-time scheduling guarantee. Tests may shorten but never enlarge
these limits. No timeout or acknowledgement proves an external effect succeeded.

A released pre-start evidence burst exceeding the queue cap fails closed, stops
the worker and marks evidence unconfirmed. It does not drop sequence numbers and
pretend a later event committed. The service's existing claim lease/restart fence
must resolve that ledger attempt to unknown; no replacement is dispatched.
Sustained real-provider progress and production upload recovery remain future work.

Cancellation, upload and write waits are independent. Closing first tries EOF and
TERM, then KILL for the one POSIX group created by this harness. Success requires
root exit, pipe closure and observed absence of that group before terminal upload.
Unverified cleanup downgrades a held terminal to uncertainty. Group absence proves
only that group absent; descendants able to change session/group could escape.
No broad process-name kill is used. Supervisor SIGKILL can leave a worker alive;
the crash test's outer owner explicitly kills and observes the trusted reported
group. There is no production outer supervisor or crash-proof process container.

## Temporary native canaries: incomplete confinement

`confinement-probe.c` is a test-only native syscall probe, compiled into a temporary
root with the Mac's existing clang. No C source is linked into the product. It runs
outside Node permission checks against synthetic files, an owned sentinel, and
owned Unix/127.0.0.1 sockets. The installed `sandbox-exec` manual marks that tool
**deprecated**. This experiment cannot establish a supported production boundary.

The restrictive deny-default profile aborts with SIGABRT before producing a probe
report. Adding metadata, sysctl and standard null/random device permissions did
not make it runnable. The separately labeled **diagnostic targeted-denial policy**
starts from allow-default and denies the synthetic targets and selected operations.
It demonstrates those denials only; unrelated host paths are not confined.
It allows workspace read/write, denies the owner/ledger/sibling/home/symlink canaries,
and denies the owned-process signal check, fork/exec, Unix IPC and loopback network.
A separate synthetic host actor may read the provider canary while its tool actor
cannot. This is a trust-boundary experiment, not provider credential integration.

The deliberately inherited file descriptor **bypasses the path denial**. Passing
only explicit stdin/stdout/stderr and closing other descriptors prevents that tested
leak. Clearing the synthetic environment secret is launcher hygiene, not sandbox
proof. Workspace success and unsandboxed positive controls verify each probe ran.
Global Keychain/TCC, Mach IPC, arbitrary host files, sockets to external hosts,
escaped process groups, real provider runtime/tool behavior and Linux confinement
are untested. No existing secret, account, permission or provider was accessed.

T07 and Stage 2 remain open. A supported enclosing process/credential arrangement
must be designed, approved for any concrete host changes, and independently tested
before exact official-runtime conformance or provider-native sign-in. The current
experiments need no new installation or persistent grant and request none.
See the [retained I/O and canary evidence](../evidence/2026-10-12-process-confinement.md).
