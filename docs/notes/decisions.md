# Fort — recorded decisions

Decisions the backlog routes to a human (`decision → you`). Recorded here so the
implementation has a single source of truth.

## AO-090 · Language + module boundaries — **DECIDED: Go, greenfield**

- **Language:** Go (single toolchain for `core`, `exec`, and `ui`).
- **Rationale:** strong concurrency for the PTY executor; eases borrowing
  Multica's Go daemon internals (AO-003); one toolchain end-to-end.
- **Module layout (one repo, hard seams):**
  - `core/`   — deterministic orchestration (rules, router, graph, store, server). Pure Go.
  - `exec/`   — native execution (`NativeRuntime` PTY executor, `FakeRuntime`). Implements `core/runtime.Runtime`.
  - `ui/`     — interface module (event/command contract, SSE, board, chat, gate inbox). HTTP/web.
  - `rules/`  — the routing ruleset(s) (YAML).
  - `flows/`  — flow definitions (YAML).
  - `cmd/fort/` — the `fort` CLI binary.
  - `docs/notes/` — spikes + decisions.
- **Seam rules (enforced by `core/arch_test.go`):**
  - `core` may **not** import `ui` or `exec` concrete packages.
  - `core` → `exec` only through the `core/runtime.Runtime` interface; `cmd/fort` wires the concrete `exec/native` runtime in.
  - `ui` may import `core`; `core` never imports `ui`.
- **Note:** the existing TypeScript Fort (`packages/`) is left as legacy/parallel; the
  Go build under `core/ exec/ ui/ …` is the rev. 2 native system.

## AO-091 · Open-core / licensing — **DECIDED: MIT (open-core posture)**

- **License:** MIT (already present in `LICENSE`, © 2026 Tobias Gunn). Fits the
  existing OSS + Homebrew distribution and the freemium posture in the market doc.
- **Attribution:** if/when Multica (Apache-2.0) code is vendored (AO-003), add the
  Apache-2.0 `NOTICE`/license headers for the vendored packages. As of this build no
  Multica source is vendored (see `multica-daemon.md`), so no attribution is required yet.

## AO-006 · Runtime topology (local vs VPS) — **DECIDED: local-first**

- **Decision:** run agents **locally** (developer machine) for v1. The PTY executor
  spawns the installed CLIs (`claude`, `codex`, `hermes`; `openclaw` pending install)
  against scoped workdirs.
- **Rationale:** simplest path to a working native system; "assign and walk away"
  beyond laptop uptime is a Phase 4+ concern. A persistent box can later run the same
  `fort` binary unchanged.
- **Provisioning note:** keys via env / `.env` (never committed) — see `.env.example`.
