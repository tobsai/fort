# Runtime Recon — how `fort-exec` drives each agent CLI headless

**Spike:** AO-002 (Phase 0). **Feeds:** AO-014 (`Runtime` interface + `NativeRuntime` PTY executor).
**Probed on:** macOS (Darwin 25.6.0), 2026-06-30. **Method:** safe probes only — `--version`, `--help`, and subcommand help. No real task was executed; no tokens burned; no provider network calls. Reproduce with `scripts/recon/recon-{claude,codex,hermes}.sh`.

## Target CLIs

| CLI | Path | Status | Version |
| --- | --- | --- | --- |
| `claude` | `/Users/tobiasgunn/.local/bin/claude` | installed | 2.1.158 (Claude Code) |
| `codex` | `/opt/homebrew/bin/codex` | installed | codex-cli 0.128.0 |
| `hermes` | `/Users/tobiasgunn/.local/bin/hermes` | installed | Hermes Agent v0.15.1 (2026.5.29) |
| `openclaw` | — | **NOT INSTALLED** | — (contract inferred — see below, marked TODO) |

All three installed CLIs default to an **interactive TUI** when run with no flags/subcommand, and all three expose an explicit **non-interactive / headless** path. The headless paths below are what `NativeRuntime` should use.

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

## 4. `openclaw` — NOT INSTALLED (contract inferred — **TODO: verify when installed**)

`openclaw` is **not present on this machine** (`which openclaw` → not found), so the following is the **expected contract inferred** from its peers and from in-repo signals, not verified:

- Both `hermes` (`hermes claw — OpenClaw migration tools`) and `codex`-adjacent tooling reference OpenClaw, and Fort itself "replaces OpenClaw" — so OpenClaw is an agent CLI in the same family as the three above.
- **Expected headless invocation (TODO verify):** a one-shot/print flag analogous to `claude -p` / `hermes --oneshot` / `codex exec` — likely something like `openclaw run "<prompt>"` or `openclaw --print/-p "<prompt>"`, or a `headless`/`exec` subcommand. **Discover the real flag from `openclaw --help` once installed.**
- **Streaming (TODO):** assume an opt-in JSON/JSONL stream flag (`--json` / `--output-format`) like its peers; fall back to line-buffered text otherwise.
- **stdin / signal (TODO):** assume `--resume`/session id for turn continuity; check for a stream-json stdin or ACP/JSON-RPC mode for live injection.
- **Exit codes (TODO):** assume `0` success / non-zero failure.
- **Auth / env (TODO):** assume a `~/.openclaw` config dir and provider keys via env (`ANTHROPIC_API_KEY` / `OPENAI_API_KEY`) — confirm names from its docs.
- **PTY (TODO):** assume same shape — headless path on a plain pipe; PTY only for its TUI.

**Action:** install `openclaw`, run `scripts/recon/recon-openclaw.sh` (to be written, mirroring the others), and replace this section with probed facts before AO-014 is closed. Phase 0's exit gate ("run all four CLIs headless") is **not met** until then.

---

## Implications for AO-014 (`NativeRuntime`)

**PTY vs pipe.** None of the three installed CLIs *require* a PTY for their headless path — `claude -p`, `codex exec`, and `hermes --oneshot` all run correctly on a **plain pipe**, which is the default `NativeRuntime` transport. A **PTY is only needed for the interactive variants**: live approval prompts (`codex --ask-for-approval on-request`, hermes hook/`--yolo` prompts without `--accept-hooks`) or driving a bare TUI. Recommendation: implement `NativeRuntime` with **pipe transport as the default** and a **PTY mode as an opt-in per provider** (the README already frames this as a PTY executor, so keep the `pty.StartWithAttrs` path, but prefer pipe + non-interactive flags so most runs never need it). `openclaw` PTY-ness is TODO. Note: AO-014's acceptance ("`signal` injects input into a running *interactive* task") implies at least one provider must be exercised in PTY/interactive mode — Claude's `--input-format stream-json` (pipe, not PTY) is the cleanest way to satisfy "inject input into a running task" without a PTY at all.

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
| `openclaw` | **TODO** — assume resume/session id; check for stream-json stdin or ACP. | Verify on install. |

**Net:** standardize on **pipe + non-interactive flags + JSONL parsing** for the common path; keep an opt-in **PTY** lane for interactive approvals; model `signal` per-CLI as (a) stream-json stdin (Claude), (b) resume/approval (Codex), (c) ACP/resume (Hermes), (d) TODO (OpenClaw). Re-estimate AO-014 noting Claude/Codex give structured streams cheaply while Hermes needs ACP for parity and OpenClaw is still unprobed.
