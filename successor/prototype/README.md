# Candidate B execution gate

This candidate has a prepared owned-process launcher, which remains disabled.
`activation.ts` always refuses
activation. There is no environment flag, old Packet A approval or elapsed timer
that enables it. The service requires an explicitly injected runtime; tests inject
synthetic transports. Do not wire an official process to it without fresh review
and an exact execution decision for the changed source and app.

The native workspace and SQLite ledger preserve Agent Homes, shared conversations,
explicit recipients, instruction skill revisions and accepted turn context. Each
Codex turn is prepared as a fresh ephemeral thread using that durable context.
Only final or unphased assistant output can become a saved reply. Model selection
is exact; effective-model attestation remains unavailable.

The prepared Codex adapter targets 0.157.1. The pinned upstream protocol describes
[`untrusted` approval](https://github.com/openai/codex/blob/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/AskForApproval.ts)
and [`readOnly` plus explicit network access](https://github.com/openai/codex/blob/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/SandboxPolicy.ts).
The adapter also requests empty environments, tools and capability roots. These
additional controls are not all represented in that tag's ordinary generated
schema; their acceptance and effect must be checked against the exact executable.
Unknown fields may be ignored. A passing synthetic test does not establish that
the installed runtime applies them.

The agent host still runs with its operating-system user authority. Requested
computer-tool sandboxing is not confinement of the host process, provider auth,
Keychain, configuration discovery or plugin loading. No OS isolation, provider
configuration changes, login, credential creation, startup service or deployment
has been performed for this candidate. No multi-user hosting is supported.

Before any proposed live test: independently review a committed revision, build
and pin its exact app, verify runtime/configuration controls without disclosing
secrets, and resolve the host boundary and execution scope. The previous lost
artifact's approval does not apply. Keep existing queued work held after restart;
unknown outcomes block later work in that conversation and are never retried.

The review follow-up adds an owned process group with an absolute five-minute
session deadline, two seconds for SIGTERM and two seconds for SIGKILL. Disposal
requires observed child close and absence of its owned group. Service shutdown
disposes even an idle runtime and awaits this confirmation; cleanup failure blocks
the restart sequence. These are bounded lifecycle controls, not OS confinement.
A process that escapes its group remains part of the unresolved host boundary.

`owned-codex.ts` declares the exact executable/hash, environment allowlist and
proposed control flags. It refuses activation before inspecting paths or launching
anything. Static checks found its feature/configuration keys in the versioned
public schema. They do not establish the executable's effective merged settings.
Existing HOME remains the provider's own store; it also permits ambient discovery.
The three named MCP overrides do not prove all possible servers are disabled.
No user configuration or credential store was inspected during this follow-up.

Any fresh execution packet must retain the original limit of two supervised starts,
at most five minutes each, for the five exact samples. The second start requires
four retained completed receipts, an empty actual native pending queue and confirmed
service/runtime/app cleanup. Otherwise stop; do not flush pending work, add prompts,
retry uncertain work or consume the conditional restart. The synthetic HTTP harness
checks its own four receipts and resource-free runtime cleanup; it is FortClient/
PendingStore conformance. Separate nonlive tests exercise MessengerModel response
loss and replay. Neither is SwiftUI or live-provider acceptance.
