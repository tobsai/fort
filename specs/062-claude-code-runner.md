# 062: Claude Code subscription runner

**Decision:** Toby approved building on 2026-10-24 ("Go ahead and build the claude runner too", Fort project thread).
On 2026-10-24 he also chose "sign in once inside Fort" and "Read-only access to Claude's folder" (decision cards in the same thread); see *Mac app composition*.

## What and why

Fort chats with Claude models using Toby's existing Claude subscription, the same
way the Codex runner uses his ChatGPT plan. This adds `ClaudeCodeChat`, a
`ChatRuntime` that drives the official `claude` CLI over its stream-json protocol:
account pinning, model catalog, one stateless turn per process, and cancel.

A terms check on 2026-10-21 found local, single-user use allowed on these conditions,
which the runtime enforces:

- spawn the official, unmodified `claude` binary (no Fort HTTP client for Anthropic);
- sign in only through Anthropic's own login (`claude auth login`); Fort never reads,
  copies or forwards the CLI's credential;
- keep `ANTHROPIC_API_KEY` and every other API or cloud credential out of the child
  environment;
- single owner, normal volume. Unattended self-hosted routines are out of scope.

Not in this change: composing the runtime into the executor and native app, an
in-app sign-in flow, and real-account verification on Toby's Mac.

## Risk

**Billing/provider path.** The main risk is a silent switch from the subscription to
API billing. The guards:

- The child environment is built from an allowlist, so no `ANTHROPIC_*`, OAuth-token
  or cloud-provider variable can reach the CLI.
- `--bare` is never used, because it ignores the subscription sign-in and requires an
  API key.
- Before the message is sent, `claude auth status --json` must report `authMethod:
  "claude.ai"` with no API key, and the stream-json `initialize` response must report
  `tokenSource: "claude.ai"`, the same email, `firstParty`, and no `apiKeySource`.
  Console, API-key, Bedrock/Vertex and environment-token sign-ins are refused, not
  converted.
- The session `init` message must show the selected model and `apiKeySource: "none"`.
  Each assistant message must come from the selected model, and fallback notices are
  refused. `CLAUDE_CODE_DISABLE_MODEL_ACCESS_FALLBACK` is set.

**Ambient capability.** The runtime starts with no tools, no MCP servers, no setting
sources, no slash commands or skills, no session persistence, no auto-memory, no
CLAUDE.md files and no telemetry. `init` must confirm the empty tool, MCP, command and
skill lists. Any permission prompt, tool use or other request from the CLI ends the
turn as outcome-unknown. The CLI's local messaging socket must sit inside Fort's
private runtime directory, so the path to that directory is length-limited (see
below).

**Credentials.** None are added. The CLI keeps its own sign-in. Fort only reads the
account ID (org UUID) and email from `claude auth status`.

**Undo.** The module is not yet composed into the app. Removing the two files removes
it.

## Plan

`successor/runner/claude-code-chat.ts` holds the launch plan (`claudeCodeLaunch`),
the account parser (`parseClaudeAuthStatus`) and `ClaudeCodeChat`. Each call spawns
one process. `models()` runs `initialize` without `--model` and lists the exact
resolved model IDs, dropping the `default` alias. `execute()` passes the system
prompt in `initialize` rather than on the command line, sends one user message
carrying the conversation context, and returns text only when the final `result` is
`success` with `is_error: false` and matches the last assistant text. The CLI reports
an auth failure as `subtype: "success"` with `is_error: true`, so `is_error` is
checked rather than the subtype. Cancel sends a stream-json interrupt, and only a
following `result` counts as confirmed cancellation.

Rejected options:

- Calling the Anthropic API directly with the subscription token. The terms forbid it.
- Holding the token in Fort, the way the OpenAI plan runtime does. The terms forbid
  it.
- Pinning one hard-coded CLI version, as the Codex runner does. Claude Code ships
  several releases a week, so the version is instead pinned per binding
  (`attempt.runtimeVersion`) and checked against `initialize`.

Tests (`runner/claude-code-chat.test.ts`, fixture process only) cover:

- the launch plan, the `--bare` exclusion and environment isolation;
- refusal of non-subscription sign-ins;
- the context and system-prompt placement;
- preflight refusals before any message is sent;
- refusal of tools, prompts, model changes and unconfirmed results;
- confirmed versus unconfirmed cancel;
- model catalog deduplication.

**Real-binary check.** Claude Code 2.1.310 was run in a cloud container with no Claude
subscription. The CLI reached a non-subscription credential there, and the runtime
refused it: `models()` returned `error`. The check also showed that a long
`XDG_RUNTIME_DIR` moves the messaging socket to the shared `/tmp/cc-socks-<uid>`
directory, which is why the private runtime directory is capped at 72 bytes.

## Mac app composition

Added after the first runs on Toby's Mac.

**Sign-in.** The App Sandbox can't read the Terminal sign-in in the login Keychain.
`ClaudeCodeChat.signIn` runs Claude Code's own claude.ai login inside Fort's sandbox,
over stream-json: `claude_authenticate {loginWithClaudeAi: true}`, the CLI's page shown
as a `browserAuthorization` challenge (only claude.com or claude.ai `…/oauth/authorize`),
then `claude_oauth_wait_for_completion`. The CLI keeps the resulting credential in its own
store inside the sandbox. Fort never sees it and reads the account back from
`claude auth status`. A probe on Toby's Mac confirmed login, a turn and persistence, with
no Keychain dialog.

**Persistent access.** The Fort Host helper gets one sandbox exception:
`com.apple.security.temporary-exception.files.home-relative-path.read-only` for
`/.local/share/claude/`, so it can run the owner's installed `claude` in place. Copying
was tried first and fails: the sandbox will not launch a binary in Fort's container,
whether the sandbox wrote it (quarantined), Terminal wrote it, or it is a hard link.
Apple accepts the exception for TestFlight to the owner's devices, but App Store review
generally rejects it. Undo: remove `Host/FortHost.entitlements`.

**Host mode.** While the multi-provider service is built, the helper is Claude-only when
the app has written `<host data>/ClaudeCode/selection.json`. On first start it pins the
newest installed version in `pinned.json`. Later updates don't change the pin, so stored
bindings keep matching. Choosing Claude again re-pins. If the pinned version is
uninstalled, the host refuses to start and says so. The mode shares the host's one ledger
and pairing (the owned core serves only its working directory). The ledger records one
runtime identity (provider, version, host id) when it is first set up, so any existing
ChatGPT host refuses to start in Claude mode, with or without agents, and a re-pin to a new
Claude version refuses the same way. Holding both providers needs the multi-provider service.
A failed start leaves its reason in `<host data>/host-start-failure.txt`, which the app shows
under Local host, because the helper discards the host's stderr. It has its own private home and empty workspace. Every session runs under `FortProcessGuardian`.
