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
