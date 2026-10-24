# 062: Claude Code subscription runner

**Decision:** Toby approved building on 2026-10-24 ("Go ahead and build the claude runner too", Fort project thread).

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
