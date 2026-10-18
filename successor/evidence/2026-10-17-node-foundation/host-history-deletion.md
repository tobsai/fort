# Offline host history deletion — source and synthetic evidence

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md), existing
owner implementation decision. Base: `ac1ecad`. October 18, 2026.

`deleteOwnerHistory` is a provider-neutral offline maintenance interface. It has no
HTTP route, CLI, startup service, provider construction or production native caller.
The ordinary Mac removal capability still refuses full host-history deletion. This
slice implements the ledger operation; it does not claim installed removal or v1.
Only disposable synthetic data was deleted. No owner history, credentials, App Group,
Keychain, account, provider process, startup registration or remote resource was touched.

## Scope and boundary

The exact request pins installation, current recovery epoch, disconnected device and
explicit deletion ID. The caller must separately verify action-time confirmation,
startup removal, owned process absence and native credential cleanup. An exclusive
SQLite lease proves this ledger has no competing SQLite writer; it does not prove
that an executor process has exited or authorize deletion. The future native caller
must supply those proofs before invoking this operation.

One transaction removes messages, runs, request receipts, execution contexts, read
positions, approvals, cancellation records, occurrence history, events and dispatch
outbox. It changes the recovery epoch and writes a body-free retry receipt. Old pending
requests fail after reopening and re-pairing. A matching retry finishes cleanup without
rotating again; another request cannot reuse the old epoch.

Retained bytes include agent identities/bindings, conversation definitions and owner
notes, skills and revisions, routine/schedule definitions and revisions, and schedule
progress. Provider/runtime files and unrelated files are never traversed. Schedules
must already be paused: erased shared occurrence deduplication could otherwise let a
lagging schedule repeat old work. The existing re-enable flow requires a fresh preview
and advances its watermark. Deletion neither pauses a schedule silently nor starts work.

Canonical absolute directory, owner/private modes, regular single-link files, 256 MiB
combined ledger/sidecar bound, no-follow file opening and pinned inode checks protect
this cooperative local workflow. Unknown tables/columns/views/triggers/entities/metadata,
future lifecycle formats, active or expired device rows, enrollment, pending/canceling
setup, stale socket ownership and foreign identities refuse. Native ACL/container and
hostile same-UID race protection are not established by this Node seam.

SQLite secure-delete, checkpoint and vacuum run under the lease. The synthetic marker
was absent from current database/sidecar bytes after completion. A post-commit cleanup
failure reports that records were deleted but cleanup remains unconfirmed; exact retry
succeeds. This is not forensic erasure of SSDs, filesystem snapshots, backups or copies
on another device. Owner notes and saved settings may intentionally contain owner text.

## Verification

Pinned Node 26.5.0; TypeScript 6.0.3. All commands ran from `successor/` with
`/opt/homebrew/opt/node@26/bin/node`, using only local fixture listeners and files.

| Check | Result |
| --- | --- |
| Initial public setup/send/disconnect/delete/re-pair test | Red: unimplemented operation; then green |
| First implementation | Directory pin incorrectly treated SQLite sidecar creation as replacement; fixed by pinning directory inode/owner/mode while checking file link counts separately |
| Schedule refusal regression | Red for enabled schedule; fixed with explicit paused-state requirement |
| Initial focused deletion/pairing/re-pair/migration run | 50 passed, one optional measurement skipped |
| Public paused-schedule/owner-note retention and fresh-consent journey | Passed; erased occurrences and old preview cannot replay |
| Full portable suite | 1,140 passed, six optional measurements skipped, no failures; before the final two future-format guards |
| Semantic-review future setup/deletion format tests | Both red; both fixed |
| Final focused deletion suite | 38 passed, one optional measurement skipped; includes all preceding behavior and the two version guards |
| Final strict TypeScript check | Passed |
| Isolated deletion benchmark | 20 samples passed; 37 tests, no skips at that revision; before the two version guards |

The final focused suite includes 21 unsafe stored-state variants; ten unsafe file variants;
wrong installation/device/epoch/ID; active-host lock and merely-stopped refusal; all scoped
record families; exact retry and stale-request denial; transaction rollback and post-commit
vacuum failure injection at SQLite's API. Future format checks were added during semantic
review. The public journeys exercise service creation, actual local HTTP behavior, shutdown,
offline deletion and reopened service behavior. Raw fixture mutation supplies corruption
and retained-family cases; those mutations are not production entry points.

No new native test/build, package assembly, actual host installation, real history deletion,
real-provider run, screenshot/accessibility inspection, archive/upload or TestFlight check
was run for this unused TypeScript interface. Prior package evidence remains unchanged.
Full native lifecycle wiring and iPhone saved-history deletion remain implementation work.

## Timing

MacBookPro18,2, 10 CPU, 64 GiB, macOS 27.2, Node 26.5.0. Twenty independent warm
fixtures, each with 10,000 synthetic 1-KiB message records plus the ordinary two-message
setup journey. Time only the synchronous deletion/compaction call; fixture construction
and HTTP work are excluded. No other build/test workload ran concurrently. This is an
engineering measurement, with no provider or installed-product latency claim.
Nearest-rank p50 **132.961167 ms**, p95 **139.239458 ms**, maximum **140.539292 ms**;
target p95 <=2,000 ms. The final two format guards were added afterward and pass focused
behavior/type checks; this benchmark is not represented as a rerun of that final revision.

Samples in milliseconds:

```json
[
  134.44441599999993,
  133.36487499999998,
  134.1422500000001,
  131.954833,
  137.09404200000017,
  133.30925000000025,
  132.96116699999993,
  132.18120799999997,
  136.18404199999986,
  131.65687499999967,
  132.44045899999992,
  132.92483300000004,
  133.95049999999992,
  140.53929199999948,
  133.73462499999914,
  132.63504199999988,
  132.8467500000006,
  132.0477919999994,
  139.239458,
  132.24779200000012
]
```

Reproduce: `FORT_HISTORY_SAMPLES=/tmp/fresh-history-samples.json /opt/homebrew/opt/node@26/bin/node --test service/delete-history.test.ts`.
Use fresh disposable fixtures only. The environment variable selects the measurement
report destination, not a ledger or user-data target.

Local logs retained, with SHA256:

- `/tmp/fort059-history-red.log` — `d886c99c8dc26ef5abe7c5bcbbd570403c80cf679fca9f4386c88ce74d488d5b`
- `/tmp/fort059-history-green.log` — `f64655af68c37c484a7b81d06f710da52fb9351992c1f692f9655a460756b87d`
- `/tmp/fort059-history-controls-red.log` — `e62e67d845d47dec19c7f05ed7778e6f480781c2a879a0ba2339ed87b7067e3c`
- `/tmp/fort059-history-focused.log` — `4a9f67cb24e8a04edc0f101c23a842ee05aa9427a346d2e3f719aeb0761464d0`
- `/tmp/fort059-history-budget.log` — `44d88a349dd53cb2a68c7827c4341b4903155fbccc9891692e0e321e9bf35edb`
- `/tmp/fort059-history-portable.log` — `8cc025d13ceec869238726a2119b16469c44f11b5d02f06c818d3a55491e8d13`
- `/tmp/fort059-history-version-red.log` — `03dae5007c70d6473d5112171a1954df09bfb0d79730b1b05ff807bd18a00e69`
- `/tmp/fort059-history-final.log` — `9ad8c4281822269a9316fcb6fa482f850ece57ce595a425ed865e8b2792b04b9`
- `/tmp/fort059-history-typecheck-final.log` — `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
