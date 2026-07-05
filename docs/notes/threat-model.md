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

## 4. Multi-machine mesh enrollment ([spec 024](../../specs/024-mesh-enrollment.md))

**Risk:** distributing the shared inter-Fort token (spec 022) across hosts, and
the posture change that comes with a host starting to accept inbound exec.

**Design:**
- The mesh token is **minted once**, by the first `fort mesh invite` a hub
  ever runs, and delivered to each joining machine over **one
  code-authenticated exchange** (`fort mesh join`) — never typed or copied by
  hand. It is one shared token for the whole mesh (spec 022 unchanged; no
  rotation or per-machine tokens in v1 — see spec 024 D4/D6).
- **Posture change:** before the first `mesh invite`, a hub's node-exec
  endpoint answers 403 with no token configured (inert). After the first
  invite, the hub holds a live token in its own `node.yaml` and **begins
  accepting inbound mesh exec** like any worker. `fort mesh invite` prints
  this out loud when it happens.
- **Invite codes** are single-use, 40-bit (Crockford base32, `XXXX-XXXX`),
  hashed (SHA-256) at rest in SQLite — never stored or logged in the clear —
  and expire at a hard-capped TTL (`--ttl`, default 15m, max 1h). A code that
  fails or has already been used never gets a second chance; a lost code costs
  nothing to re-mint.
- **Enrollment endpoints:** `POST /api/mesh/invite` and
  `DELETE /api/mesh/machines/{name}` accept **loopback connections only** —
  local shell access on the hub *is* the admin credential (D7), no separate
  admin secret exists. `POST /api/mesh/join` is reachable from the network
  (a new machine isn't on loopback yet) but is **code-authenticated**: no
  request without a valid, unexpired, unused code ever gets back the token.
- **`mesh remove` is roster-only** (D6): it drops a machine from the registry
  and hot-unregisters it, but the removed machine **keeps the shared mesh
  token** and can still call `/api/exec` on every remaining node until the
  token is actually rotated. `fort mesh remove` prints this warning every
  time.

### Token rotation runbook

`fort mesh remove <name>` does **not** revoke the shared token — treat it as
"take off the roster," not "cut off access." To actually revoke a machine's
access (compromised host, offboarded machine, leaked token), rotate the token
across the whole mesh:

1. Stop `fort serve` on every machine in the mesh (hub and workers).
2. On each machine, edit `.fort-native/node.yaml` and change (or blank) the
   `token` field — or simply delete `node.yaml` if you're going to re-run
   `fort mesh join` anyway.
3. Start the hub (`fort serve`) and run `fort mesh invite` again; this mints a
   **new** shared token into the hub's `node.yaml`.
4. Re-join every worker you still trust with the new printed
   `fort mesh join <hub-url> --code ...` line, then start `fort serve` there.
5. Do **not** re-join the machine you're revoking. It has no path to the new
   token unless someone runs `fort mesh join` on it with a fresh code.

There is no partial/per-machine rotation in v1 (spec 024 D4) — rotation always
replaces the one shared token for the whole mesh.
