# Runtime Recon — how `fort-exec` drives each agent CLI headless

**Spike:** AO-002 (Phase 0). **Feeds:** AO-014 (`Runtime` interface + `NativeRuntime` PTY executor).
**Probed on:** macOS (Darwin 25.6.0), 2026-06-30; OpenClaw refreshed
2026-07-23 on the enrolled execution host. **Method:** token-free help probes
for every CLI plus one explicit OpenClaw local one-shot contract check.
Reproduce the original probes with
`scripts/recon/recon-{claude,codex,hermes}.sh`.

## Target CLIs

| CLI | Path | Status | Version |
| --- | --- | --- | --- |
| `claude` | `/Users/tobiasgunn/.local/bin/claude` | installed | 2.1.158 (Claude Code) |
| `codex` | `/opt/homebrew/bin/codex` | installed | codex-cli 0.128.0 |
| `hermes` | `/Users/tobiasgunn/.local/bin/hermes` | installed | Hermes Agent v0.15.1 (2026.5.29) |
| `openclaw` | `/opt/homebrew/bin/openclaw` | installed on enrolled host | 2026.7.1-2 |

Each installed CLI defaults to an interactive surface when run without its
headless entry point. The explicit non-interactive paths below are what
`NativeRuntime` uses.

---

## 1. `claude` (Claude Code)

> Default `claude` with no flags launches an interactive session. `-p/--print` is the headless switch.

**1. Headless invocation**
```
claude -p "<task prompt>"
```
Recommended full form for `fort-exec`:
```
claude -p "<task prompt>" \
  --output-format stream-json \
  --include-partial-messages \
  --model sonnet \
  --permission-mode bypassPermissions \
  --no-session-persistence
```
The prompt may also be the trailing positional arg, or piped on stdin.

**2. Streaming.** `--output-format` (only valid with `-p`) takes `text` (default), `json` (one final result object), or `stream-json` (realtime JSONL — one JSON object per line). `--include-partial-messages` adds token-level partial chunks. `--include-hook-events` adds hook lifecycle events. This is the richest, most structured stream of the four — map JSONL objects directly to `RunEvent`s. Text mode is line-buffered to a pipe and is fine for a dumb passthrough.

**3. stdin / signal path (human-in-the-loop).** `--input-format stream-json` (with `--print` + `--output-format stream-json`) lets you **feed realtime user messages as JSON on stdin** to a running turn-based session — this is the native fit for Fort's `signal()`. `--replay-user-messages` echoes injected user messages back on stdout for acknowledgment. Session continuity: `--resume <session-id>` / `-c/--continue`, and `--session-id <uuid>` pins a known id so Fort can resume deterministically. `--fork-session` branches on resume.

**4. Exit codes.** `0` on success; non-zero on error. In `--print` mode the workspace-trust dialog is skipped and invalid settings are silently ignored, so a clean non-zero exit is the reliable failure signal. `--max-budget-usd` can hard-cap a print run (will exit when exceeded).

**5. Auth / env.** Config dir `~/.claude` (OAuth/keychain by default). Env vars Fort should be aware of (names only): `ANTHROPIC_API_KEY`, `ANTHROPIC_BASE_URL`, `CLAUDE_CODE_OAUTH_*`. `--bare` forces strictly `ANTHROPIC_API_KEY` / `apiKeyHelper` (no OAuth, no keychain) — useful for hermetic headless runs. 3P providers (Bedrock/Vertex/Foundry) use their own credentials.

**6. PTY note.** **No PTY required** for `-p/--print` — it explicitly detects a non-TTY stdout (pipe/redirect) and runs non-interactively. A plain pipe is sufficient and preferred. Reserve a PTY only if Fort ever drives the interactive TUI directly (not the plan).

Event map (spec 030): complete assistant lines -> message (text blocks, joined)
+ tool per tool_use block (subagent when the tool is Task, data
{"description","agent"}; tool data {"name","summary"}). user tool_result lines
-> tool {"name":"tool_result","summary":<preview>}. The terminal result line,
stream_event partials, and system lines stay raw stdout (the planner reads the
result line from stdout — spec 026).

---

## 2. `codex` (Codex CLI)

> Bare `codex` forwards to the interactive TUI. `codex exec` is the documented non-interactive entry point.

**1. Headless invocation**
```
codex exec "<task prompt>"
```
Recommended full form for `fort-exec`:
```
codex exec "<task prompt>" \
  --json \
  --sandbox workspace-write \
  --ask-for-approval never \
  --skip-git-repo-check \
  --output-last-message /tmp/codex-last.txt
```
Prompt can be the positional arg, `-` (read from stdin), or piped (piped stdin is appended as a `<stdin>` block when a prompt is also given).

**2. Streaming.** `--json` prints **events to stdout as JSONL** — one event per line, the structured stream to normalize into `RunEvent`s. Without `--json`, output is human-formatted text. `--color always|never|auto` controls ANSI. `-o/--output-last-message <FILE>` writes just the agent's final message to a file (clean way to grab the result without parsing the stream).

**3. stdin / signal path.** No first-class "inject into a running turn" flag like Claude's stream-json input. For human-in-the-loop, Codex uses an **approval gate** rather than free-form stdin injection: `-a/--ask-for-approval untrusted|on-request|never` decides when the model pauses for human approval; under a PTY those approvals are answered interactively. For Fort's turn-based `signal`, the clean path is **session resume**: `codex exec resume <session-id> "<follow-up>"` (or `--last`), follow-up via positional or `-` (stdin). For a live blocking approval, drive the approval prompt via the PTY. There is also an `mcp-server` / `app-server` mode if Fort later wants a JSON-RPC channel instead of a PTY.

**4. Exit codes.** `0` success, non-zero failure. With `--ask-for-approval never`, command-execution failures are returned to the model rather than escalating, so the process still completes normally — rely on the exit code plus the final-message file.

**5. Auth / env.** Config home `~/.codex` (overridable via `CODEX_HOME`); `config.toml` + `auth.json`. Auth keys present (names only): `OPENAI_API_KEY`, plus OAuth `tokens`. So env var of interest: **`OPENAI_API_KEY`** (and `CODEX_HOME` to point at an isolated config). `--ignore-user-config` skips `config.toml` (auth still via `CODEX_HOME`). Sandbox is built in: `-s read-only|workspace-write|danger-full-access`; `--dangerously-bypass-approvals-and-sandbox` only when Fort sandboxes externally.

**6. PTY note.** `codex exec --json` runs fine on a **plain pipe — no PTY needed** for the non-interactive path. A **PTY is needed only** if Fort wants live interactive approvals (`--ask-for-approval on-request`) answered mid-run, or if it ever drives the bare-TUI. Default Fort path = pipe + `--ask-for-approval never` + external sandbox.

---

## 3. `hermes` (Hermes Agent)

> Bare `hermes` / `hermes chat` is the interactive TUI. `-z/--oneshot` is the headless switch.

**1. Headless invocation**
```
hermes --oneshot "<task prompt>"
```
Recommended full form for `fort-exec`:
```
hermes --oneshot "<task prompt>" \
  --model anthropic/claude-sonnet-4.6 \
  --accept-hooks \
  --yolo
```
`-z/--oneshot` sends a single prompt and prints **ONLY the final response text** to stdout (no banner, spinner, tool previews, or session-id line). Tools, memory, rules, and `AGENTS.md` in the CWD still load; approvals are auto-bypassed.

**2. Streaming.** `--oneshot` is **not** a structured event stream — it emits the final response text only (clean for piping, poor for progress). For a structured / streaming channel, Hermes exposes **`hermes acp`** — an ACP (Agent Client Protocol) **JSON-RPC server over stdio** intended for editor integration; this is the route to get incremental events/tool-calls if `RunEvent` granularity matters. `--oneshot` stdout is line-buffered text suitable for a passthrough `RunEvent` (start → final-text → exit).

**3. stdin / signal path.** `--oneshot` reads its prompt from the positional arg, `--file`, or stdin (`-`), but it is one-shot — no mid-turn injection. Human-in-the-loop options: session continuity via `--resume <session|id>` / `-r` and `--continue [name]` / `-c` (resume by name or most recent); or run **`hermes acp`** and inject follow-ups over the JSON-RPC stdio channel (the proper interactive `signal` path). For unattended runs that can't show a TTY approval prompt, `--accept-hooks` (= `HERMES_ACCEPT_HOOKS=1`) auto-approves declared shell hooks and `--yolo` bypasses dangerous-command approval.

**4. Exit codes.** Standard `0` ok / non-zero error for the agent loop. (The sibling `hermes send` documents `0 ok, 1 delivery/backend error, 2 usage error` — a good convention to assume for the family; treat non-zero as failure.)

**5. Auth / env.** Config home `~/.hermes` — `config.yaml` (model/provider/toolsets) + `auth.json` (multi-provider: `providers`, `credential_pool`, `active_provider`; here `openai-codex`) + `.env`. Provider/model are overridable per-invocation via `--provider` / `--model` or env **`HERMES_INFERENCE_MODEL`**; hook auto-accept via **`HERMES_ACCEPT_HOOKS`**. `--ignore-user-config` ignores `~/.hermes/config.yaml`. Underlying provider keys (e.g. `ANTHROPIC_API_KEY` / `OPENAI_API_KEY`) come from `~/.hermes/.env` + the provider pool. `--profile` gives fully isolated Hermes instances — useful for per-agent sandboxing.

**6. PTY note.** `--oneshot` with `--accept-hooks --yolo` runs **non-interactively on a plain pipe — no PTY needed**. A **PTY is needed only** if Fort drives the interactive `chat` TUI or wants to answer hook/approval prompts live without `--accept-hooks`/`--yolo`. For structured streaming + interactive signal, prefer `hermes acp` (stdio JSON-RPC) over a PTY.

---

## 4. `openclaw`

> `openclaw agent` is the supported one-shot entry point. The old Fort
> assumption `openclaw run` was invalid.

**1. Headless invocation**

```sh
openclaw agent --local --agent main --message "<task prompt>" --json
```

`--local` selects the embedded agent runner, avoiding a dependency on the
separate OpenClaw gateway service. `--agent main` selects the configured
default agent. The enrolled host completed this exact shape successfully
against OpenClaw 2026.7.1-2.

**2. Output.** `--json` emits a final JSON result with response payloads and
run metadata. It is a one-shot result, not a documented JSONL token stream.
Fort's lenient JSON parser extracts recognized response text and retains
unrecognized lines as raw stdout.

**3. stdin / signal path.** The one-shot prompt is supplied by `--message` or
`--message-file`. `--session-id` and `--session-key` can target explicit
session continuity, but Fort does not yet map its signal channel onto an
OpenClaw session.

**4. Exit codes.** `0` on the verified successful local turn; treat any
non-zero result as failure.

**5. Auth / configuration.** Configuration and auth profiles live under
`~/.openclaw`. Local mode uses the configured agent's auth profile on the
enrolled host. Fort passes no provider secret and does not synthesize a model
identifier.

**6. PTY note.** No PTY is required. `agent --local --message ... --json`
runs correctly over ordinary pipes.

---

## Implications for AO-014 (`NativeRuntime`)

**PTY vs pipe.** None of the four installed CLI headless paths require a PTY:
`claude -p`, `codex exec`, `hermes --oneshot`, and
`openclaw agent --local` all run correctly on a plain pipe. A PTY is only
needed for interactive variants, such as live approval prompts. Keep pipe
transport as the default and PTY mode as an explicit provider option.

**Normalizing stdout → `RunEvent`s.** Tiered by how structured each CLI's stream is:
- **Structured JSONL (preferred):** `claude --output-format stream-json` and `codex exec --json` emit one JSON object/event per line. Parse line-delimited JSON → typed `RunEvent`s (turn-start, partial token, tool-call, tool-result, final, error). `claude --include-partial-messages` gives token-level granularity; `codex -o/--output-last-message` gives a clean final-result file alongside the stream.
- **Text-only:** `hermes --oneshot` emits final text only. Wrap as a minimal event sequence (`started` → `output(text)` → `completed`). For richer Hermes events, use `hermes acp` (stdio JSON-RPC) instead and map ACP notifications → `RunEvent`s.
- Treat each provider as a small **adapter** behind one `Runtime` interface (`dispatch`/`stream`/`status`/`signal`): the adapter owns the per-CLI flag set and the stdout→`RunEvent` decoder; `FakeRuntime` (per AO-014) replays canned JSONL for unit tests.

**`signal` (stdin injection) mapping, per CLI:**
| CLI | `signal` mechanism | Notes |
| --- | --- | --- |
| `claude` | Write a JSON user message to **stdin** with `--input-format stream-json` (+ `--output-format stream-json`). | First-class realtime injection; no PTY. `--replay-user-messages` to confirm receipt. Best fit for Fort's gate-inbox → `signal()`. |
| `codex` | **Resume** (`codex exec resume <id> "<follow-up>"`) for turn-based; for a *live* pause, answer the **approval prompt** over a PTY (`--ask-for-approval on-request`). | No stream-json stdin; injection is resume- or approval-shaped. |
| `hermes` | Run **`hermes acp`** and send follow-ups over stdio JSON-RPC; or turn-based via `--resume`/`--continue`. | `--oneshot` itself is one-shot — no mid-turn injection. |
| `openclaw` | Turn-based targeting via `--session-id` or `--session-key`; no live Fort signal mapping yet. | The verified `--local` command is one-shot. |

**Net:** standardize on **pipe + non-interactive flags** for the common path;
keep an opt-in **PTY** lane for interactive approvals; model `signal` per-CLI
as (a) stream-json stdin (Claude), (b) resume/approval (Codex), (c) ACP/resume
(Hermes), and (d) explicit session targeting for a future OpenClaw adapter.
