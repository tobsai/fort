# Phase 4 — Hardening, gateway, distribution

**Goal:** make Fort safe to leave running, add cost controls, and package it for install — turning the working system into something durable (and shippable, if you take it to market).
**Exit gate:** a clean install runs the full system with spend caps enforced.

---

### AO-041 · Security baseline
- **Type:** task · **Pri:** P1 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-005, AO-014
- **Do:** Agents execute code on your machine — enforce least-privilege per runtime, sandboxed workdirs, no secrets in repos, keys via env/gateway. Write a short threat-model note.
- **Acceptance:**
  - [ ] Each runtime runs with a scoped workdir + minimal permissions.
  - [ ] `docs/notes/threat-model.md` covers code-exec, secrets, and spend.

### AO-042 · Optional gateway (keys / budgets / failover)
- **Type:** task · **Pri:** P2 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-014
- **Do:** Put **agentgateway** or **plano** in front of the providers: shared keys, per-flow budgets/spend caps, failover, OTel tracing. Wire budgets to `task` nodes.
- **Acceptance:**
  - [ ] A per-flow spend cap is enforced in a test; every model call is traced.

### AO-043 · Packaging & distribution (Homebrew tap)
- **Type:** task · **Pri:** P2 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-018
- **Do:** Package Fort for one-command install (Homebrew tap, install script), matching your existing OSS distribution. Versioned releases.
- **Acceptance:**
  - [ ] `brew install` (your tap) installs a working `fort`; a release is cut.
