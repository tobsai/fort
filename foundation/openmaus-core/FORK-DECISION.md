# Fort core fork decision — 2026-10-11

The owner authorized a restricted OpenMausBot core fork under Fort, retaining
TypeScript initially while personal preferences and backend/client architecture
remain undecided. This overrides conflicting Hermes/OpenClaw-only, no-native-CLI,
and feature exclusions in Specs 054/056 for this isolated exploration only.
Those specs, Fort source, history and unrelated work remain intact. Local teardown
completion was reported by the owner; provider closure and clean-baseline acceptance
are not asserted or changed by this work. No provider cleanup is authorized here.

## Exact provenance and isolation

Public upstream: https://github.com/milind-soni/OpenMausBot
Base: `8b53c0323dba2099a953ee2d77ab9330cd5acaa7` (reviewed October 11 snapshot).
Fort branch: `codex/fort-core-openmaus-20261011`, based on Fort `5616faa`.
Worktree: `/tmp/fort-core-openmaus-20261011` (macOS resolves `/tmp` to `/private/tmp`).
Source: `foundation/openmaus-core/`. No remote fork, push, publication, merge,
visibility change, resource creation, credentials or provider login occurred.
Sparse checkout displays foundation and root files; existing Fort tracked files
remain in branch history and the original dirty checkout is untouched.

## License and independence

Only Apache-2.0 core source was imported. `enterprise/`, `cloudflare/`, `apps/`,
`deploy/`, `companion/`, Electron packaging/updater, upstream automation and
installation scripts were excluded. LICENSE, NOTICE, LICENSING.md, third_party,
and embedded third-party notices remain. Upstream legal attribution and historic
technical identifiers are not Fort branding. UI display strings use Fort; mascot
artwork, CursorAvatar and preview source were removed. Clients are deliberately
incomplete until a neutral Fort client is extracted; do not restore mascot assets.

PostHog implementation is replaced by inert exports; no identity or events are
sent even if callers opt in. PostHog, electron-updater and Wrangler are removed
from the active manifest. Active scripts expose only dependency-free foundation
tests; no prepare/install/package/download/deploy hook exists. Original manifest
and lock are non-active provenance snapshots, not a verified Fort lockfile.

Cloud identity, tunnels, control plane, Boat, Composio and hosted voice are not
available: every application launch is blocked unconditionally. Mixed upstream
core contains dormant integration references for extraction review; it is not
independent runnable core yet. Do not mistake disabled execution for completed
removal of every vendor feature. No paid service is auto-enabled or required to
run the foundation checks. No application has been run.

## Security gates

Read upstream SECURITY.md and request-auth.ts. Upstream explicitly documents a
sessionless service-loopback Slack worker Full-access escalation. Fort removes
all SERVICE_ALLOW exemptions except GET health, and denies guarded worker task,
message, request, queue and response routes regardless of Ask/Full mode. This is
integration disablement, not a claim that Ask fixes the escalation. HTTP request
handling also checks the route policy, and server/CLI/frontend entry points fail
closed. Worker/tunnel deployment source was not imported. Before runtime unlock,
exercise real HTTP authorization fixtures and replace stale upstream security
policy prose with Fort-specific reporting and scope.

Agents using official Claude/Codex runtimes execute with host-user privileges.
A computer-use sandbox confines computer tools; it does not sandbox agent shell,
filesystem, credentials or network. Single-owner local execution only is the
intended later path. No shared/multi-user hosting is approved. No permission
broker or runtime permissions were weakened.

## Review map and decisions

- `server/drivers/claude.ts`, `codex.ts`, `driver.ts`: official CLI runtime seam;
  retain for an isolated local path, inspect account/install/update dependencies.
- `server/index.ts`: very large HTTP composition root; extract owner-only API,
  remove dormant cloud/paid/pairing paths before any launch.
- `server/request-auth.ts`, permission broker modules: authorization boundary.
- `server/store*`, `shared/`: persistence and transport types; establish an API
  that native/alternate clients can consume before choosing backend language.
- `src/`: React client reference source, incomplete after mascot removal.
- `foundation-policy.mjs`, `server/fort-foundation-gate.ts`: deny/launch gates.
- `package.json`: dependency declarations inherited, not installed/verified.
- `IMPORT-MANIFEST.json`: exact redacted hashes of imported/modified source.

Astra review should decide minimal personal workflow, host permissions, persistence
ownership, API/events boundary, local/self-hosted topology, neutral/native client,
and TypeScript versus an alternative backend. No broad rewrite is implied.
Resolve paths still using `~/.openmausbot` to a fresh Fort data root before launch;
never read existing upstream or Fort runtime data. Replace stale branding/links,
provider setup, optional-service settings and auto-update paths before enabling UI.

## Validation

Observed red: foundation tests failed because policy module did not exist.
Observed green: `node --experimental-strip-types --test foundation.test.mjs` passes all three policy/authorization behaviors.
This proves only restricted route decisions and source-only launch behavior.
Static integrity/security checks are recorded in VALIDATION.md.
No dependency install, upstream suite, typecheck, build, HTTP fixture, provider
smoke, UI/accessibility, packaging, deployment or installed-product signoff ran.
Installed Node 22.22.3 is below upstream >=24; pnpm 10.28.0 differs from upstream
10.33.0. Active dependency graph needs a new lock and audited script-free install
in an isolated environment after extraction; current source is not a working app.
