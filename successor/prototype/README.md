# Candidate B execution gate

This candidate has no enabled provider launcher. `activation.ts` always refuses
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
