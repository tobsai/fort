# AO-003 · Study Multica's Apache-2.0 daemon — borrow/skip decision

**Question:** Multica's daemon (Go, Apache-2.0) solved "spawn an agent CLI, manage its
PTY, parse its stdout, manage lifecycle." Do we vendor/fork specific packages, or just
study the approach and write our own thin executor?

**Constraint in this build:** the Multica source is **not present in this session** (no
repo added). This note therefore records the *architectural* borrow/skip decision and
the attribution plan; if the source is later added, the per-file vendoring can follow it.

## What `fort-exec` actually needs (from AO-002 recon)

The recon (`runtime-recon.md`) showed every installed CLI runs headless over a **plain
pipe** with non-interactive flags and a **JSONL** event stream:

| CLI | headless command | stream | PTY needed? |
|-----|------------------|--------|-------------|
| claude | `claude -p <prompt> --output-format stream-json` | JSONL | no |
| codex | `codex exec <prompt> --json --ask-for-approval never` | JSONL | no |
| hermes | `hermes --oneshot <prompt>` (or `hermes acp`) | text / JSON-RPC | no |
| openclaw | *(not installed — TODO)* | ? | ? |

This is the single most important finding: **a PTY is optional**, not load-bearing. The
hard parts Multica's daemon handles (raw TTY mode, terminal resize, ANSI de-mux) are
only needed for the *interactive TUI* lane (live approvals), not the default headless run.

## Borrow / skip decision per component

| Multica component | Decision | Why |
|---|---|---|
| CLI detection / which-binary | **skip** | Trivial: `exec.LookPath` + a per-provider config entry. |
| PTY handling | **skip vendoring; borrow the pattern** | Use `github.com/creack/pty` (MIT) directly for the opt-in interactive lane. No need to fork Multica's wrapper. |
| stdout parsing / event normalization | **skip; build our own** | Our `RunEvent` schema is Fort-specific; each CLI's JSONL is small and provider-specific. A thin per-provider adapter is clearer than adapting Multica's. |
| process lifecycle (spawn/wait/cancel/signal) | **build our own on `os/exec`** | Go stdlib (`exec.CommandContext`, `cmd.Process.Signal`, stdin pipe) covers spawn/cancel/stdin injection; ~100 lines. |
| session/daemon supervision | **skip for v1** | Fort's own store + scheduler own run lifecycle; a long-lived daemon supervisor is Phase 4. |

**Net decision: thin study, no vendoring.** Build `exec/native.NativeRuntime` on Go
stdlib `os/exec` (default pipe lane) + `creack/pty` (opt-in interactive lane). This is
less code than adapting Multica's daemon and avoids carrying Apache-2.0 vendored source.

## AO-014 re-estimate

Original estimate **XL**. With recon showing pipe+JSONL works and no PTY/daemon fork
required, the realistic core is **L** (interface + pipe executor + 3 provider adapters +
FakeRuntime), with the interactive-PTY `signal` lane as a follow-on **M**.

## Attribution plan (AO-091)

No Multica source is vendored, so **no Apache-2.0 attribution is required** for this
build. If a specific Multica package is later vendored, add its `LICENSE`/`NOTICE` under
`third_party/<pkg>/` and a header pointer, and note it in `decisions.md` (AO-091).
