# Named permissions profile — source and fixture validation only

Source: `81b64b50ffa3d7aff079f043282877bb2046c8cf`, based on
`3a364ebf63e846e214f72605ed3a62d34d2b8393`, branch
`codex/fort-openai-rebuild-20261016`.
Worktree: `/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.
This packet grants no execution authority and is not a new attempt packet.

## Outcome

Implemented the process-scoped `fort_candidate_b_no_files` profile and migrated
thread/turn requests to its supported `permissions` selector. The unsupported legacy
restricted-read field is gone. The declaration narrows the intended boundary from
workspace reads to no computer-tool file reads or writes, with tool network disabled.
Empty environments, disabled tools, exact model/provider selection, untrusted approval,
explicit user review and refusal of server permission/tool requests remain.

Fort verifies the exact declaration and default, complete catalog with explicit allowed
status, exact selected identity/null parent, empty roots, legacy readOnly/network false,
and all prior startup controls. Inheritance, extra grants, proxy/socket settings,
catalog denial/substitution and observed in-turn policy changes fail closed. The
launch plan is macOS-only. The profile was not applied; no settings file was created.

[PERMISSION-PROFILE.md](../../runner/PERMISSION-PROFILE.md) traces parsing, managed
requirements, selection/reload, internal helper grants, macOS denial lowering and
the reporting limits at the exact upstream source. [STARTUP-CONTRACT.md](../../runner/STARTUP-CONTRACT.md)
records the now-consistent startup/turn fixture contract.

The API reports profile identity and a lossy legacy sandbox summary, not the complete
effective filesystem ACL. Verification therefore combines declaration checks with
pinned-source semantics; it is not independent OS/runtime attestation. The macOS
source appends deny-glob rules after ordinary grants, including helper paths. Actual
Seatbelt behavior, unusual paths and bypass resistance have not been measured.
The same-user agent host remains outside this computer-tool boundary.

## Checks

| Check | Result |
| --- | --- |
| Full nonlive Node suite | **561 passed, 0 failed**, exit 0 ([log](full-suite.log)). Synthetic processes and temporary fixture listeners only. |
| Startup/profile contract suite | **343 passed, 0 failed** ([log](negative-tests.log)); included in the full suite. |
| Strict TypeScript | Exit 0 ([empty-success log](typecheck.log)). |
| Independent TOML syntax | All 15 process-scoped assignments parsed with Python 3.14 standard-library `tomllib`; exact profile/default matched literal expectations ([log](toml.log), [synthetic arguments](synthetic-launch-overrides.json)). No actual Codex parser invocation. |
| Red/green migration | Named handshake [red](red-handshake.log) / [green](green-handshake.log); launch profile [red](red-launch.log) / [green](green-launch.log); policy-change refusal [red](red-update.log) / [green](green-update.log). Expected initial failures were resolved. |
| Source provenance | 81 immutable public source files hash-verified ([manifest](upstream-sources.json)); no upstream source execution. |
| Preservation | All three attempt file sets/state/ledger hashes unchanged; each has one consumed start, blocked phase, zero receipts. Native app/executable hashes and main checkout status baseline preserved ([log](preservation.log), [attempt metadata](attempts-preserved.json)). |
| Whitespace | Passed. |

No real credentials were created/read, no provider/runtime was started, no persistent
settings or OS/network/security configuration was changed, and no attempt was consumed.
Existing nonlive service tests use temporary synthetic authentication data. Ledger
files were hashed without SQLite connections. Recorded cleanup statuses are not fresh
Keychain inspections. Historical evidence, manual entry, native source and packets
remain unchanged; the manual entry still references the consumed third-attempt pins.

## Pins and remaining real validation

[pins.json](pins.json) seals **464 source files**, with canonical source-list SHA-256
`29ee5e990fcfe63c533236a6370f7e264fea82811cfe5c83bdffe261cb5aa4ac`.

- Upstream `rust-v0.157.1`: `36650394c5b38c2990ccf2a3457165ca3e9d9726`.
- Public-source manifest SHA-256: `b97481c748954f7645e37975ff82772eaedbc1902be811dcfd457ca6bc5bc124`.
- Unchanged official runtime executable SHA-256: `27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d`.
- Unchanged native app list SHA-256: `9c32247c3c77b1618515aec7faad3944826ec9e11d4ecc0d58483c8ecd23ad2b`, built from `de9e482bac99920d52a03b6904542a78226c5ba2`.

Unrun: compiled Rust/official-runtime profile parsing and effective reports; real macOS
filesystem, network, IPC, descriptor and privileged-daemon canaries; Unicode/newline
filenames, symlinks/hardlinks and rename cases; independent JSON Schema validation;
real account/config/credential reads or provider turns; native builds, pairing,
Keychain/UI signoff; independent review of this delta.

The next evidence gate is separately authorized isolated non-provider runtime/OS
validation against these pins, before considering any provider attempt. No fourth
attempt is requested or enabled by this work. The result is a source implementation
with passing contract tests, not a working tested provider application or proven host
confinement boundary.
