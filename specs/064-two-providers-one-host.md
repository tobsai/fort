# 064: ChatGPT and Claude on one host

**Decision:** Toby approved "both" on 2026-10-24 ("build both").

## What and why

Today a Fort host serves exactly one runtime. Its ledger pins one runtime identity
(provider, version, host id) when it is first set up, so Toby's existing ChatGPT host
can't use Claude: Claude-only mode (Spec 062) crash-loops on it. After this change one
host serves both. Each Agent picks its provider when it is created, and its binding keeps
that provider for good. Existing ChatGPT Agents don't change.

## Risk

- **Provider path.** Each turn runs on the provider its Agent's binding names, never on
  another. If that runtime isn't available on the host (Claude not enabled, or the pinned
  Claude version is gone), the turn fails visibly and nothing is sent. There is no
  fallback between providers and no API billing.
- **Credentials.** No change in custody. ChatGPT sign-in stays where it is. Claude keeps
  its own login inside its private home, and Fort never reads it. The two sign-ins keep
  separate account choices in the ledger, and each turn pins the account of its own
  provider.
- **Ledger.** The existing `runtimeIdentity` row stays the host's primary runtime
  (ChatGPT on the Mac). Additional runtimes are not written to the ledger. A binding
  names its runtime, which is enough.
- **Undo.** Turning Claude off in the app removes the Claude runtime from the next host
  start. Claude Agents then fail visibly until it is turned back on. Nothing is deleted.

## Plan

- **Core** (`createApplication`, `serveOwnedCore`): takes an optional list of extra
  runtimes beside the primary one. A binding is matched to a runtime by provider,
  version and host id. Agent creation takes an optional `provider` (default: primary).
  `/v1/models` and `/v1/account/sign-in*` take `?provider=`. Each runtime has its own
  account sign-in and choice, stored under provider-suffixed metadata keys; the
  primary's keys are unchanged. `installation.runtimes` lists every runtime. Restart
  and stop stay primary-only for now.
- **Host** (`host/main.mjs`): always serves Codex as primary. When the owner has turned
  Claude on, it adds Claude Code as an extra runtime. If Claude can't start, the host
  still serves ChatGPT and records why in `host-start-failure.txt` (#34).
- **Mac app**: a provider choice when creating an Agent, plus sign-in and model lists
  per provider. "Use Claude Code on this Mac" becomes "Turn on Claude".
- **Rejected**: a separate ledger per provider, which would split pairing and history.
  Also rejected: migrating the ledger's identity to Claude, which would strand the
  ChatGPT Agents.
- **Tests**: a two-runtime core test where each Agent's turn reaches only its own
  runtime, with separate sign-ins and model lists. A test that a Claude Agent fails
  visibly when its runtime is absent. A test that a single-runtime ledger is unchanged.
