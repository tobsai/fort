# 024 — Mesh enrollment (`fort mesh`: self-managing pairing)

**Status:** approved (Toby, 2026-07-04) · revised after adversarial review, pending Toby's re-read.
**Governed by:** [021-fort-native](021-fort-native.md) · **Extends:** [022-multi-machine-orchestration](022-multi-machine-orchestration.md)

## Goal
Remove the manual steps spec 022 left to the operator: generating the shared
`FORT_NODE_TOKEN`, distributing it to every machine, and hand-editing
`machines.yaml`. Adding a machine to the mesh becomes: run **`fort mesh invite`**
on the hub, paste the printed one-liner on the new machine. Fort mints the
durable mesh token, delivers it over one code-authenticated exchange, registers
the machine (name, url, offered agents), and persists everything on both sides.
Determinism is preserved: enrollment is plain CRUD — **zero model calls**.

## Non-goals (v1 — YAGNI)
- No token rotation or per-machine tokens (one shared mesh token, as in 022).
  See D6: `mesh remove` is roster-only, **not** credential revocation.
- No gate-approval join flow (invite-code possession *is* the approval).
- No auto-discovery (mDNS/Bonjour) and no worker-side deregistration.
- No TLS between nodes (unchanged from 022: trusted LAN / tailnet).
- No cross-machine flows (unchanged 022 non-goal).
- No invite listing/revocation: codes are single-use with a hard-capped TTL;
  a leaked code is outlived, a lost one is re-minted for free.

## Approach

### Daemon-mediated administration (D8)
`mesh invite` and `mesh remove` are thin CLI clients of the **running
`fort serve` daemon** on the same machine (clear error if it isn't running).
All minting, registry writes, and store writes happen **in-process in the
daemon**. This is what makes the rest coherent: the daemon holds the freshly
minted token (no reload race), performs hot add/remove on its own runtime, and
is the single writer of SQLite and the managed registry. The admin endpoints
(`POST /api/mesh/invite`, `DELETE /api/mesh/machines/{name}`) accept
**loopback connections only** (checked against the connection's remote
address): local shell access on the hub is the admin credential (D7).
`mesh join` talks to the hub's `/api/mesh/join` like any remote client.

### CLI (`cmd/fort/mesh.go`)
- `fort mesh invite [--ttl 15m] [--advertise URL]` — hub side. Via the daemon:
  1. On first use, mints the durable mesh token (32 random bytes, hex),
     persists it to the hub's `node.yaml`, and prints a notice that the hub
     now also accepts inbound mesh exec (posture change; see threat-model
     update).
  2. Registers the hub itself in the managed registry if absent:
     name = its `NodeName`, agents = the same `$PATH` provider probe workers
     use, url = the advertised hub URL (below).
  3. Mints a single-use invite code (8 chars Crockford base32, shown
     `XXXX-XXXX`, 40 bits), stores only SHA-256 + expiry in SQLite, and prints
     the paste-ready line: `fort mesh join <hub-url> --code XXXX-XXXX`.
  The hub URL is `--advertise` if given, else `http://<first non-loopback
  interface, preferring the CGNAT/Tailscale 100.64.0.0/10 range>:<bind port>`.
  `--ttl` is capped at **1h** (reject above); default 15m.
- `fort mesh join <hub-url> --code <code> [--name N] [--port 4087]
  [--advertise URL] [--agents a,b]` — worker side. Probes `$PATH` for provider
  CLIs (`claude`, `codex`, `hermes`, `openclaw`); `--agents` overrides the
  probe, and join **refuses to proceed** if the final list is empty. POSTs
  `/api/mesh/join`; on success writes `node.yaml` and prints the next step
  (`fort serve`, or restart the service).
- `fort mesh remove <name>` — hub side, via the daemon: rewrites the registry,
  hot-removes the peer, appends `machine_removed`, and **prints a warning that
  the removed machine still holds the mesh token** and can call `/api/exec` on
  every node until the token is rotated (manual procedure documented in
  `docs/notes/threat-model.md`: stop nodes, delete/edit `node.yaml`
  everywhere, re-invite).

### Wire protocol (`POST /api/mesh/join`, hub side)
Request: `{"code": "...", "port": 4087, "name": "taloss-mac-mini",
"agents": ["claude"], "advertise_url": ""}` (name defaults to the worker's
hostname).
- **Every join requires a valid, unexpired, unused code** — including re-joins
  of an existing machine. Idempotency is a registry-side-effect property only,
  never a code-validation bypass: re-join with a fresh code and an existing
  `name` **updates** that entry's url + agents (recovers DHCP/IP changes; the
  code is the trust anchor, not the url). Bad/used code → `401`; expired →
  `410`. No request without a valid code ever returns the token.
- Ordering & failure: write `machines.yaml` first, then mark the code used;
  if the registry write fails → `500` and the code stays valid (single-use is
  consumed only on success).
- Worker URL: derived from the **observed source IP** of the request plus the
  advertised port; a non-empty `advertise_url` wins. Validation (either
  source): scheme must be `http`, host must parse, **loopback is rejected**,
  and a host outside RFC1918/CGNAT/link-local-excluded ranges is rejected
  (keeps the cleartext token on the trusted network). The derived URL is
  echoed in the join response and the hub log so the operator sees what was
  registered.
- Response: `{"token": "...", "name": "<canonical>"}`.
- Mounted by `fort serve` only (never `fort control`). No rate limiter:
  single-use 40-bit codes with a ≤1h TTL close brute force on their own
  (~7.5k guesses/15min at any plausible request rate vs 2^40).

### State
**Hub** —
- Invites: new `invite` table in the existing SQLite store
  (`code_hash`, `created_at`, `expires_at`, `used_at`) — written only by the
  daemon (D8), so no cross-process SQLite contention.
- Registry: a **Fort-managed** `machines.yaml` in the data dir, written
  atomically (temp file + rename). `fort serve`/`fort control` auto-load it
  when present. Precedence: `FORT_MACHINES` env > managed file exists >
  single-machine mode. When `FORT_MACHINES` is explicitly set, enrollment
  refuses to write (error with guidance) rather than touching an
  operator-managed file.
- **Data dir** = the directory of the resolved `DBPath`
  (default `.fort-native/`); `node.yaml` and the managed `machines.yaml` live
  beside the DB.
- Events: `machine_joined` / `machine_removed` appended to the event log with
  `run_id = ""` and the registry entry as payload; the feed/board must
  tolerate run-less events (ui change listed below).
- Hot application (in-daemon, D8): `exec/cluster.Runtime` gains
  `Add(name, rt)` / `Remove(name)` behind a mutex; the registry is held in an
  `atomic.Pointer[machines.Registry]` **shared by placer, cluster, and
  `control.Roster`**, so placement and `/api/machines` reflect changes
  immediately. Node exec endpoints are always mounted and answer 403 until a
  token exists (`exec/node` already behaves this way for an empty token), so
  first-invite bootstrap needs no restart.

**Worker** —
- `node.yaml` in the data dir: `{name, token, addr}`
  (addr = listen address implied by `--port`, e.g. `0.0.0.0:4087`; operator
  may edit to pin an interface). Written `0600`; the temp file used for
  atomic writes is opened `O_CREATE|0600` before any bytes are written. The
  hub's `node.yaml` follows the same rules.
- Config precedence (in `core/config`): **env > node.yaml > defaults** for
  `NodeToken`, `NodeName`, `Addr`. Spec-022 setups (pure env) keep working
  byte-for-byte; `node.yaml` only fills gaps.

### Bootstrap symmetry
The hub's own durable token lives in its `node.yaml`, so after the first
`mesh invite` the hub **accepts inbound exec** like any worker. (Promoting a
worker to a full control plane remains manual — workers never receive the
registry in v1.)

## Decisions
- **D1 — invite code over gate-approval join.** Chosen by Toby (2026-07-04).
  No unauthenticated admission path; possession of a short-lived single-use
  code is the admission decision.
- **D2 — observed-source-IP for the worker URL.** The worker cannot reliably
  know its own reachable address; the hub can. `--advertise` is the escape
  hatch. Consequence: join must be performed over the network path the mesh
  will use (join over Tailscale ⇒ mesh over Tailscale).
- **D3 — registry stays YAML, but Fort-owned.** Keeps 022's inspectable static
  registry and single loader; enrollment becomes its writer. SQLite holds only
  invites (secrets) and events (history), not the roster source.
- **D4 — one shared mesh token, minted once.** Rotation/per-node tokens are
  deferred; the `node.yaml` schema leaves room for them.
- **D5 — worker probes agents at join time only.** The offered-agents list is
  static after enrollment (re-join or `--agents` to change) — placement inputs
  stay deterministic config, not live probes.
- **D6 — `mesh remove` is roster-only, not credential revocation.** The
  removed machine keeps the shared token until manual rotation. The command
  says so out loud; the rotation runbook lives in the threat-model note. Toby
  is approving this consequence, not just the D4 non-goal.
- **D7 — loopback = admin.** Invite minting and removal are privileged;
  requiring the CLI to run on the hub box (loopback source check) reuses the
  operating system's own access control instead of inventing a second
  credential.
- **D8 — enrollment runs in the daemon.** CLI subcommands are HTTP clients of
  the running `fort serve`. Consequences: hot add/remove is real (same
  process), the minted token is live without restart, SQLite has a single
  writer, and `mesh invite`/`remove` require the hub to be up.

## Affected files
- `cmd/fort/mesh.go` (new) — `mesh invite|join|remove` subcommands (HTTP
  clients).
- `cmd/fort/main.go`, `cmd/fort/wire.go`, `cmd/fort/control.go` — mount
  join/admin endpoints (serve only); auto-load managed registry + `node.yaml`;
  always-mount node exec endpoints.
- `core/config/config.go` — `node.yaml` layer + precedence; data-dir
  derivation from `DBPath`.
- `core/machines/machines.go` — `Save` (atomic write), `Add`/`Remove`/update
  helpers.
- `core/store/` — `invite` table + queries; `machine_joined`/
  `machine_removed` event kinds.
- `exec/cluster/cluster.go` — mutex'd `Add`/`Remove`.
- `exec/meshjoin/` (new) — join + loopback-admin handlers; wired in `cmd/fort`
  so `ui` stays free of exec imports (arch seam intact).
- `control/roster.go` — read the shared registry pointer (live roster).
- `ui/` (feed) — tolerate events with empty `run_id`.
- `machines.example.yaml`, `README.md`, `.env.example`,
  `docs/notes/threat-model.md` — document the managed flow, the hub's posture
  change on first invite, and the token-rotation runbook.

## Test criteria (`go test ./...`, `-race` on join/hot-apply paths)
- Invite lifecycle: valid code admits exactly once; second use → 401; expired
  → 410; unknown → 401; `--ttl` above 1h rejected; comparisons constant-time.
- **No-code oracle check:** join with a correct existing name+url but
  missing/invalid code → 401, and the response never contains the token.
- Re-join: fresh code + existing name updates url/agents (simulated IP
  change); registry ends with one entry per name.
- URL validation: loopback and non-private `advertise_url`/observed hosts
  rejected; derived URL echoed in the response.
- Ordering: registry-write failure → 500 and the code remains usable.
- Registry write is atomic and load-back-identical; refuses to write when
  `FORT_MACHINES` is explicitly set.
- Permissions: `node.yaml` is `0600` on hub (after first invite) and worker
  (after join), including the temp file window (created `0600`).
- Config precedence: env > node.yaml > defaults, each var independently.
- Hub self-entry: after the first invite + one join, unpinned placement still
  prefers the hub for agents it offers; `--machine <hub-name>` resolves.
- Hot application: a run dispatched to a machine joined *after* boot executes
  without restart (two in-process instances, worker under `FORT_FAKE=1`,
  join with `--agents claude`); after `mesh remove`, placement to that name
  fails and `/api/machines` reflects both transitions without restart.
- Empty probe: join with no CLIs on `$PATH` and no `--agents` refuses locally
  (no request sent).
- Admin endpoints reject non-loopback sources (401/403).
- Determinism guard: no `Runtime` (model) calls anywhere in enrollment paths.
- Existing 022 env-only configuration passes the full 022 test suite
  unchanged.

## Rollback
Additive feature. Disable by simply not using `fort mesh`: with no
`node.yaml`/managed `machines.yaml` present, behavior is byte-for-byte spec
022 (the always-mounted node endpoint answers 403 with no token, same as
022's "disabled" semantics). Full rollback = revert the commits and delete
`<data-dir>/machines.yaml`, `<data-dir>/node.yaml`, and the `invite` table
(auto-created; dropping the DB file also suffices). No migration of existing
data is performed either way.
