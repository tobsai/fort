# Validation — 2026-10-11

Passed: dependency-free Node test run, 4/4 tests, Node v22.22.3:
`node --experimental-strip-types --test foundation.test.mjs`.
Tests exercise launch refusal, route policy and actual request-auth.ts serviceAllowed
public seam: guarded messages/tasks/requests/interrupt/queue/respond and internal
routes cannot use service-loopback trust. Only GET health remains allowed.
Red observed before policy existed (ERR_MODULE_NOT_FOUND); green after implementation.

Passed: byte comparisons of upstream LICENSE/NOTICE/LICENSING.md/SECURITY.md and
all third_party files; excluded enterprise/cloudflare/electron/deploy/scripts/
companion/apps directories absent; active package scripts contain only foundation
validation; no PostHog dependency or implementation remains active.

Unrun: install, full dependency resolution/audit, lock generation, typecheck,
upstream suites, build, HTTP authorization fixtures, Claude/Codex live runtime,
UI accessibility/screenshots, packaging and deployment. UI source is incomplete
because trademarked mascot modules were removed; packaging scripts are excluded.
Runtime cannot start. This is source import evidence, not application signoff.

Dependency declarations: React ^19.1.0, TypeScript ^5.8.3, Vite ^7.1.0,
Vitest ^4.1.11; pinned upstream snapshot/lock retained as provenance only.
No installed dependency versions were verified. Node >=24 / pnpm 10.33.0 required
by upstream; host has Node 22.22.3 / pnpm 10.28.0. No tools were installed.

Confirmed against the pristine upstream public seam: serviceAllowed POST
/api/bots/b1/messages/guarded returns true; the Fort regression requires false.
Server gate import itself is also tested to reject with the source-only error.
Passed committed change scope: all 2,435 added files reside under foundation;
no pre-existing Fort file changed. Worktree clean after commit.

## Quarantine correction validation

Observed red: two targeted tests reproduced route admission and owner-loopback
admin/client authority under the earlier implementation. Both now pass.
Current suite: `node --experimental-strip-types --test foundation.test.mjs quarantine.test.mjs`:
7/7 tests pass, including 324 authorization cases (3 trust variants × 9 paths ×
3 methods × 4 credential variants), without touching a credential store.
All sampled route and service decisions deny, including retained connector,
TTS, computer, approval/respond, guarded-message, pairing and internal paths.

72 inventoried entry sources exactly match refusal-only stubs and their archived
contents match recorded SHA-256. Node-supported entries were launched in separate
processes with an empty environment, exited 1, emitted only the refusal error and
no stdout. Six TSX/browser entry sources were evaluated as identical JavaScript
through data-URL imports; this does not claim a browser/TSX build or UI test.
No runtime dependency graph or archived implementation was loaded.

Previous health exception and four-test scope descriptions above are historical;
current policy denies every route and every authorization attempt. HTTP handlers,
provider environment canaries, real providers, builds, installation and deployment
remain unrun. Review artifacts and unrelated work were preserved unchanged.
