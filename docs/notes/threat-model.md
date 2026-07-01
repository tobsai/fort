# AO-041 · Fort threat model

Fort spawns agent CLIs that **execute code on your machine**. This note covers
the three core risks — code execution, secrets, and spend — and how the native
build mitigates each.

## 1. Code execution

**Risk:** a task node runs an agent CLI that can read/write files and run
commands with the user's privileges.

**Mitigations (implemented):**
- **Scoped workdir.** Every run executes in `WorkRoot/<run-id>` (`exec/native`
  sets `cmd.Dir`; proven by `TestRunExecutesInScopedWorkdir`). Agents start
  rooted in their own directory, not the repo root or `$HOME`.
- **Deterministic routing path.** Routing reads only task signals — no model
  decides what runs (proven by `TestRoutingIsDeterministic`). Inference happens
  only at `task` nodes (proven by `TestLinearFlowOnlyTaskInvokesRuntime`).
- **Cancelable.** Runs are spawned under a context; `Cancel()` kills the process
  tree (`exec.CommandContext`).

**Hardening backlog:** per-run OS sandbox (macOS `sandbox-exec` profile or a
container) restricting filesystem + network to the workdir; codex already
supports `--sandbox workspace-write` (wired in `DefaultProviders`).

## 2. Secrets

**Risk:** provider API keys leak into the repo or into spawned subprocesses that
don't need them.

**Mitigations (implemented):**
- **No keys in the repo.** `.env` is git-ignored; `.env.example` documents the
  required vars by name only.
- **Env allowlist (least privilege).** `native.Runtime.EnvAllow` restricts which
  host env vars reach a spawned CLI; secrets not on the list are withheld
  (proven by `TestEnvAllowlistWithholdsSecrets`). Set it to each provider's
  minimal key set.
- **Gateway-held keys.** With the optional gateway (AO-042), provider keys can
  live only in the gateway, never in the per-run env.

## 3. Spend

**Risk:** an unattended flow (or a retry loop) runs up provider cost.

**Mitigations (implemented):**
- **Per-flow spend cap.** The `exec/gateway` decorator enforces a budget and
  rejects dispatches past the cap (`ErrBudgetExceeded`, proven by
  `TestPerFlowBudgetCapEnforced`). `FORT_BUDGET` wires it on.
- **Bounded retries.** `task` retry is capped (`Retry.Max`) then escalates to a
  human gate (`TestRetryThenEscalateToGate`) — no infinite retry spend.
- **Traced calls.** Every admitted model call goes through the gateway `Tracer`
  (OTel plug point) for auditing.

## Trust boundaries
| Boundary | Control |
|---|---|
| inbound task -> router | deterministic matchers only; no code/model exec |
| router -> runtime | only `task` nodes dispatch; scoped workdir + env allowlist |
| runtime -> provider | gateway budget + tracing; provider-native sandbox flags |
| fort-core -> fort-ui | read-only event log + explicit command API (no shell) |
