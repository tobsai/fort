What this repo does: Fort is a single-owner native AI messenger with a local Node
core and optional runtime adapters. This slice adds an offline SQLite history-deletion
interface, exercised only on disposable fixtures. Expected load is one stopped owner
installation, bounded to 256 MiB across its ledger and sidecars.

## Semantic review

Codex self-review, October 18, 2026, base `ac1ecad`. Used the pinned
[Ponytail skill](https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md)
and preserved baseline. Read the complete new operation/tests; traced service schema,
owner setup/disconnect/re-pair, epoch admission, schedule progress/consent and private
socket ownership. Reviewed native removal/lifecycle/storage composition to identify
what this seam cannot prove. Searched the tracked source and new files for callers:
there is no production caller yet. Reviewed final diff, refusal cases, rollback,
post-commit cleanup reporting, preservation rules, measurements and evidence limits.

## Must fix

1. **Native host deletion is still unavailable — open.**
   - What this is: The new operation is an offline adapter interface. Ordinary native
     removal still reports history deletion unavailable.
   - Problem: A caller-supplied identity and SQLite lease do not establish user consent,
     startup removal, executor exit, credential cleanup or App Group ownership.
   - Fix: Wire the exact native lifecycle and retained confirmation through the fixed
     verified maintenance program, then exercise installed-product acceptance.
   - If skipped: This remains source preparation rather than the requested full removal
     feature. Owner: implementation agent; no real-data deletion is authorized here.

2. **Erasing occurrence records can repeat an enabled schedule — fixed.**
   - What this is: Shared occurrence IDs prevent duplicate scheduled work, while each
     schedule keeps its own checked-through time.
   - Problem: Removing IDs while one enabled schedule lags can make old work eligible.
   - Fix: Require paused schedules; preserve definitions/progress. The existing fresh
     preview/enable flow advances progress. Public reopen/re-pair tests verify no replay.
   - If skipped: An apparently successful history deletion could cause unexpected work.

3. **Future lifecycle records were accepted — fixed.**
   - What this is: Setup and deletion receipts have versioned persisted formats.
   - Problem: Version 2 records could pass the first implementation's state checks.
   - Fix: Two failing refusal tests now pass with explicit supported-version checks.
   - If skipped: A later format could be erased without understanding its semantics.

## Should fix

4. **Directory link-count pin rejected normal SQLite activity — fixed.**
   - What this is: Opening this database may create WAL and shared-memory files.
   - Problem: Treating the directory's link count as immutable rejected that valid work.
   - Fix: Pin directory inode/owner/mode, while separately enforcing single-link regular
     database/sidecar files. The complete public deletion journey now passes.
   - If skipped: The operation refuses a normal stopped installation.

Verdict: This bounded offline seam passes its final focused tests and strict types;
continue native integration. No native capability, provider action or release signoff
is inferred. No new dependencies, generic cleanup framework or public route was added.
Not checked: real history, hostile same-UID mutations, native ACL/Keychain/container
inspection, actual startup/process absence, installed UI, provider operation or TestFlight.
The full portable suite and benchmark preceded the last two format guards; final focused
checks include them. The detailed evidence states the exact sequence and unrun checks.

```fort-review
{
  "format": 1,
  "reviewedCommit": "ac1ecad5cb10e807a89b97684e74d311c17d90f8",
  "reviewer": "Codex",
  "scope": "Offline exact-owner SQLite history deletion, schema/lifecycle/filesystem refusal, atomic epoch fence and retry, retained configuration/schedule consent, synthetic behavior and measurement evidence. Native wiring remains open.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "NATIVE-HOST-DELETION",
      "disposition": "open",
      "detail": "Production caller remains disabled until exact confirmation, startup/process absence, credential and App Group checks are wired and verified."
    },
    {
      "id": "SCHEDULE-REPLAY",
      "disposition": "fixed",
      "detail": "Deletion requires paused schedules; public re-pair/fresh-preview tests verify no erased occurrence replay."
    },
    {
      "id": "FUTURE-REMOVAL-FORMATS",
      "disposition": "fixed",
      "detail": "Reproduced future setup/deletion version acceptance and added fail-closed checks."
    },
    {
      "id": "DIRECTORY-PIN",
      "disposition": "fixed",
      "detail": "Directory inode/owner/mode pin permits SQLite sidecar creation; regular file link-count checks remain strict."
    }
  ],
  "tests": [
    "Final focused suite: 38 passed, one optional benchmark skipped; final strict TypeScript passed.",
    "Earlier full portable suite: 1140 passed, six optional measurements skipped; before the two future-format guards.",
    "20 isolated 10000-message deletion/compaction samples: p50 132.961167 ms, p95 139.239458 ms, max 140.539292 ms; budget 2000 ms; before final two version guards.",
    "Public setup/send/disconnect/delete/re-pair, stale epoch, schedule consent, unsafe schema/files, transaction rollback and post-commit cleanup retry tested on disposable fixtures only."
  ],
  "exceptions": [],
  "files": {
    "specs/059-modular-node-v1.md": "7bc7c2b9dfd0e830d3019e39eba2e4c1c7481efc67a9a9db7c2229f5b9300b43",
    "successor/V1-ACCEPTANCE.md": "021bc1f6ace97cea799b5e2828909905ff5e3d9833f49f1da05b044fad75071d",
    "successor/evidence/2026-10-17-node-foundation/host-history-deletion.md": "50f65bb198ad502d3dba80ca7850832d642c4d19a9587286b0fb06dfe6686d00",
    "successor/service/delete-history.test.ts": "a7adc028cfb90d966d3754312999d28b2f676e1314586f76593c0720e571063a",
    "successor/service/delete-history.ts": "8d5da3c64be71c5e1898e6b3695c257dbb78617c4eb9e2d2c87883ae7a9a2ae3"
  }
}
```
