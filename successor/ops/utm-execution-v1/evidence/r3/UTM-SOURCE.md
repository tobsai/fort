# Retained UTM source evidence

Official repository: https://github.com/utmapp/UTM
Pinned commit: `048ca7498ea3a374439149d51739d94c5300bcda` (UTM 4.7.5 source review).

Only the three text blobs in `source-provenance.json` were retrieved during R3.
They total 32,755 decoded bytes; the three API responses total 45,984 bytes.
Each decoded file matches the previously pinned tree's Git blob SHA-1, exact
size and newly recorded SHA-256. Requests used no credentials, proxy or
redirect. The first sandboxed attempt failed DNS before receipt of any content;
the narrowly approved retry completed. No source was compiled or executed.

Eight previously cached Swift files are retained in `cached-utm-source/` with
`cached-source-provenance.json`. Their SHA-256 values match the prior cache index,
and their Git blob identities and sizes match the same pinned tree. They were
not downloaded again. The receipts establish content identity, not a signed
source-to-binary reproducibility claim.

These files retain their original Copyright © 2022 osy notices and Apache-2.0
headers. `LICENSE-UTM-Apache-2.0.txt` supplies the standard license text, copied
from the existing local Apache license; it is not a new network input. The
pinned tree contains no root NOTICE or NOTICE.txt. Source bytes are unmodified,
including trailing whitespace; the directory's exact evidence patterns exempt
only trailing-space checks for those bytes.

Schema conclusions implemented in `fort_probe/utm_schema.py` and `config.py`:

- The shared configuration extension defines oldest/current version 4.
- Information requires `Name`, `IconCustom` and `UUID`. The candidate now includes
  `IconCustom: false`; custom icons and notes are absent.
- Kernel, initrd and drive names resolve inside the package's `Data/` directory.
  The file plan binds `Data/kernel.Image` to the decoded kernel's digest/size.
- The registry's `Registry` dictionary uses uppercase UUID strings as keys.
  Each entry contains `Package`, `Suspended`, `ExternalDrives`,
  `SharedDirectories` and `WindowSettings`, in addition to name/UUID.
- File records contain `Path`, `Bookmark`, `ReadOnly` and optionally
  `BookmarkRemote`. Nonempty package bookmarks need independently supplied,
  digest-bound resolution without UI or mounting. A pathname alone is not proof.
- UTM filters invalid share/external-drive bookmarks while decoding. Fort checks
  the raw entries first and rejects any declared share or external drive.
- The source parser requires persisted/in-memory registry agreement. Capturing
  those live views and resolving bookmarks is a later missing adapter; the Mac
  path refuses an absent provider. No user preference domain was read here.

The Swift source schema is now known. A Python fixture that applies this schema
is not a Swift decoder run, an app readback, or proof that a VM will boot.
