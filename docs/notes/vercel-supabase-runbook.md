# Vercel + Supabase control-plane runbook

This runbook implements the operational boundary in Specs 047 and 048. It is
not permission to create paid resources, migrate private data, change the write
authority, or retire the v1 relay.

## Deployment units

| Project | Root Directory | Runtime | Purpose |
| --- | --- | --- | --- |
| `fort-gateway` | `gateway` | supported Node.js | Auth.js/native-owner authentication, web, bounded SSE |
| `fort-control` | repository root | Beta Go Functions | bounded JSON control handlers only |

`fort-control` must never run `fort serve`, a provider, scheduler loop, relay,
filesystem watcher, or permanent listener. Both projects deploy one exact Git
commit. The gateway points to an immutable control deployment URL during
preview and promotion.

## Required server-only configuration

No values belong in Git, client bundles, logs, or this document.

`fort-control`:

- `DATABASE_URL`: Supavisor transaction-pooler URL with SSL on port 6543;
- `FORT_SCHEMA_VERSION`, `FORT_API_MIN_VERSION`, `FORT_API_MAX_VERSION`;
- `FORT_AUTHORITY_EPOCH` and `FORT_AUTHORITY_MODE`; only the exact
  `cloud_v2_write` mode may tick cloud Routines, while `legacy_v1_write`
  returns an authenticated disabled response before opening Postgres;
- `FORT_CONTROL_ASSERTION_KEYS_JSON`: JSON object mapping every currently accepted key ID to a canonical base64url HMAC key (32 bytes or longer);
- `FORT_BODY_KEYS_JSON`: JSON object mapping every retained application-AEAD key ID to a canonical base64url-encoded 32-byte key;
- `FORT_BODY_ACTIVE_KID`: exact key ID used for new body encryption; it must be present in `FORT_BODY_KEYS_JSON`, while old keys remain until no durable ciphertext references them;
- `FORT_REBIND_ACCEPTANCE_KEY_B64URL`: a separate canonical base64url HMAC key of at least 32 bytes used only for the short-lived, server-issued Agent Rebind acceptance token;
- optional `FORT_REBIND_ACCEPTANCE_TTL_SECONDS`, from 1 through 300 seconds (default 120); this token binds the account, Agent, opaque eligible-option ID, exact old/new revision evidence, preview digest, route, audience, issue time, and expiry;
- machine-token hashing/verification material; and
- `FORT_CRON_ACCOUNT_ID`: the canonical server-configured Fort account UUID
  for the single-account first release; the Cron request cannot select an
  account;
- `CRON_SECRET`: the exact secret Vercel sends in the `Authorization` header as
  `Bearer <CRON_SECRET>` to `/api/v2/cron/tick`; missing, repeated, or inexact
  authorization is rejected before the database opener runs.

`fort-gateway`:

- existing Auth.js, Google, allowlist, and native-session configuration;
- `FORT_CONTROL_ORIGIN`: immutable preview URL or compatible production alias;
- `FORT_CONTROL_ACCOUNT_MAP`: server-only normalized-owner-email to Fort-account-UUID map;
- `FORT_CONTROL_ASSERTION_KID` and `FORT_CONTROL_ASSERTION_KEY_B64URL`: the active key ID and its base64url 256-bit HMAC key;
- optional `FORT_CONTROL_ASSERTION_TTL_SECONDS` between 1 and 60 seconds;
- `FORT_V2_SSE_CUTOFF_MS`, shorter than the Node function duration; and
- a preview-only Deployment Protection bypass credential when required.

Production and preview use different assertion and AEAD keys. A key remains in
the decryption ring while any durable envelope references it.

Agent creation and Rebind accept only an opaque eligible-option ID resolved by
an injected server-side inventory. The checked-in production composition
contains no approved Agent options: both commands fail closed until an exact
adapter/source option is separately approved and injected. Clients never send
provider, model, machine, adapter, or authority components, and a Rebind must
preview first and return the resulting signed acceptance token unchanged.

## Worker execution and confidentiality

An enrolled machine starts the outbound worker explicitly:

```sh
FORT_MACHINE_TOKEN='…' fort worker --config /absolute/path/worker.json
```

The strict JSON config names the HTTPS control endpoint, account/worker/machine
IDs, the environment-variable name containing the scoped machine token, the
accepted capability-revision ID and revision, an absolute readiness command,
and optional bounded poll/heartbeat intervals. The token itself never belongs
in the file. `--once` performs at most one claim for diagnostics.

The shipped CLI has an empty executable adapter registry. It rejects
operator-supplied adapter authorization and cannot start any real provider; a
claimed target fails explicitly as `adapter_not_authorized`. Enabling execution
requires a separately approved Fort-owned adapter contract and composition
that proves every immutable Binding selector and provider eligibility. Merely
provisioning a machine token or readiness command is not adapter approval.

Workers receive bounded plaintext prompts and context only in authenticated
HTTPS responses and submit bounded plaintext output chunks and terminal data
only over that channel. The Vercel control process opens the application key
ring, decrypts worker input at the response boundary, and encrypts submitted
output before the repository writes it. A worker never receives an application
AEAD key, `DATABASE_URL`, Supabase service credential, or direct database
write authority.

The command and protocol are not permission to provision a production worker,
approve an adapter, switch write authority, or cut over traffic. Those actions
remain subject to the promotion and write-authority gates below.

## Worker output artifact transport

The machine-authenticated `/api/v2/worker` command accepts four closed output
artifact operations: `artifact_create`, `artifact_status`, `artifact_chunk`,
and `artifact_finalize`. Every payload repeats the exact target, execution
attempt, lease, and fence; the authenticated account, worker, and machine are
supplied by the control handler, not selected by a Supabase credential on the
worker. Status responses contain the immutable manifest and ordered chunk
receipts only—never ciphertext or database credentials—so a worker can resume
missing indexes after interruption.

One request remains at most 4 MiB after JSON/base64 encoding. A manifest is
limited to 64 chunks, 2 MiB plaintext per chunk, and 128 MiB plaintext total.
Workers submit plaintext chunks and may append them out of order; an exact
duplicate is an idempotent success and any changed duplicate conflicts. The
control repository encrypts every chunk with the server key ring. Finalization
decrypts chunks one at a time in manifest order and rejects missing indexes,
count/length/key-ID drift, or a logical digest mismatch without aggregating the
128 MiB body in memory. Manifests and inserted ciphertext chunks are immutable.

The root `vercel.json` invokes one per-minute Cron tick. The handler performs
one transaction and returns: it holds an account-and-scheduler advisory lock,
row-locks the durable monotonic watermark and active Routine rows (their
selected revisions are immutable),
reconciles at most 128 Routines and 512 exact occurrences over a five-minute
catch-up and one-minute look-ahead window, and never starts a provider. Future
occurrences remain `scheduled`; workers can see `queued` only at or after the
persisted due time. More than 90 seconds late becomes
`missed_needs_attention`. A bound failure rolls back occurrences, events, and
the watermark together. The per-minute production plan and fresh price quote
remain explicit provisioning gates.

## Local database verification

The repository pins Supabase CLI `2.115.0` in these commands:

```sh
npx --yes supabase@2.115.0 start
npx --yes supabase@2.115.0 db reset
npx --yes supabase@2.115.0 test db
npx --yes supabase@2.115.0 db lint --level error
```

The migration must leave all operational tables in `fort_private`, enable and
force account RLS, keep API roles unprivileged, and give the `fort_gateway`
`NOBYPASSRLS` role only its explicit runtime grants. Every runtime transaction
sets a validated account UUID with transaction-local `set_config` before any
query. A-to-B-to-anonymous reuse is part of acceptance.

## Promotion order

1. Record a fresh Supabase cost quote and receive exact project approval.
2. Apply the additive schema migration from an approved migration connection.
3. Verify pgTAP, security/performance advisors, backup availability, and the
   exact migration version.
4. Deploy `fort-control`; its health response must report the expected commit,
   schema version, API range, and still-current authority epoch.
5. Deploy `fort-gateway` against that immutable compatible control URL.
6. Run authenticated web/native, forced reconnect, worker claim, cancel,
   Handoff, Group, Routine, and rollback acceptance.
7. Promote the gateway alias only after the paired deployment passes.

Schema contraction happens in a later deployment after the compatibility
window. A failed second deployment leaves the previous compatible alias pair
live.

## Write-authority cutover

Before migration, the signed mode remains `legacy_v1_write`; cloud previews
cannot mutate production truth. Final cutover requires a frozen SQLite backup,
zero queued/Working targets, scheduler fencing, encrypted deterministic export,
staging rehearsal, full count/digest verification, and explicit approval.

Only then may a new monotonically increasing epoch declare
`cloud_v2_write`. There is no dual write. Rollback freezes cloud writes,
verifies a Postgres-to-new-SQLite import, and advances to another signed
`legacy_v1_write` epoch before local mutations resume.

## Retirement gate

Railway and the Cloudflare v1 relay remain recoverable and read-only through
the acceptance window. Remove them only after independently approved real
runtime families complete cross-framework Group/Handoff acceptance, the
structured `HandoffEmitter` passes, backup/restore succeeds, and the rollback
path has been rehearsed.
