# Codex feature inventory compatibility — nonlive correction

Source **`6e0ebb69f559695d89446cc53b5c566db2fb84c8`**, branch `codex/fort-openai-rebuild-20261016`, in
`/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.
[Exact pins](pins.json), [verification](verification.json), [diagnosis](diagnosis.json)
and [preserved attempts](attempt-preservation.json). Both attempts are blocked;
no additional start, reset or attempt is authorized. The manual entry retains
the consumed replacement's pins. These new pins are for review only.

## Finding and evidence limit

The replacement ran source `f7d7c9f1f8943f6a49708a30ef1208748a25b8d9` with evidence `8d79b4c89e52c9953b274d3914c2b13932807bc7`,
consumed one start and stopped at `features/memory_tool`. Its state records zero
receipts and credential cleanup `deleted`, with no port or enrollment file.
The coordinator reported runtime/app absence and an empty native directory;
this correction did not rerun those live/UI probes.

The raw feature response was not retained, so the saved checkpoint alone does
not tell whether the alias was absent or true. However, the executed Fort check
examined `memories` before `memory_tool`. Reaching the latter proves that its
canonical `memories` observation was exactly false, with well-formed unique rows
and completed pagination. There is no evidence that canonical memory was enabled.

Pinned Codex source establishes a definite client compatibility defect:

- [Legacy inputs](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/features/src/legacy.rs#L33)
  map `memory_tool` to MemoryTool and `imagegenext` to ImageGeneration.
- [Canonical registry](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/features/src/lib.rs#L1146)
  names those features `memories` and `image_generation`. They are present stable
  features, not unavailable capabilities that can safely be ignored.
- [Inventory implementation](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/request_processors/catalog_processor.rs#L346)
  returns canonical registry names and their effective booleans, including refreshed
  thread scope. It does not produce independent alias rows. Its upstream test
  expects that same canonical list.

The public tag `rust-v0.157.1` resolves through tag object
`ac0e23e5232692b95268583c8278c50b8c436d2b` to commit `36650394c5b38c2990ccf2a3457165ca3e9d9726`.
[Source fingerprints and facts](upstream-sources.json) include the schema,
registry, alias map, endpoint and tests. All 149 canonical names were
compared with Fort's required inputs; only these two aliases were absent.
Schema acceptance of an input alias does not imply its presence in a response.
No upstream implementation was copied into Fort or executed.

The source-supported explanation is an omitted alias row, not enabled canonical
memory. Because the raw response is absent, omission is not claimed as a directly
captured observation. The deterministic reproduction demonstrates the failure
with canonical memory explicitly disabled; no live retry was needed to establish
or correct this defect. The later `imagegenext` check had the same proven flaw and
is corrected in the same narrow change.

## Correction and safety properties

Fort keeps every original launch override, including both aliases, unchanged.
Only response observation maps `memory_tool` to `memories` and `imagegenext` to
`image_generation` for this already version-pinned adapter. Each canonical entry
must exist and be exactly false. If an alias is unexpectedly also present, it
must be false as well. Missing/renamed canonical keys, alias-only reports, true
values, malformed booleans, duplicate rows and incomplete pagination still fail.
No inferred disabled default, generic rename fallback or unavailable-feature
exception is introduced. Both global and pre-turn checks use this rule.

The old synthetic fixture included nonexistent alias rows and therefore concealed
the defect. It now uses canonical names. The public no-turn preflight test was
red before the fix ([regression](regression-red.log)); the minimal reproduction
also produced the exact retained checkpoint ([red](repro-red.log)). Both now pass
([green](repro-green.log)) without creating any real thread or turn. Negative
fixtures retain enabled/missing/malformed refusals; per-thread changes block
synthetic turn dispatch. No runtime settings, model, prompt, deadline, credential,
service, native app or cleanup implementation changed.

## Verification and remaining work

- **53 affected nonlive tests passed; zero failed**, including **26 Codex tests**:
  [affected](affected-tests.log), [focused](focused-tests.log).
- Strict TypeScript exit 0 ([output](typecheck.log)); whitespace checks passed.
- **457 source files and 5 unchanged app files** verified.
  Source-list SHA-256: `b47e5f6290b1b07574c201a46645166be53388f4e140a5e229b6e9d20afd49ba`.
  App-list SHA-256: `9c32247c3c77b1618515aec7faad3944826ec9e11d4ecc0d58483c8ecd23ad2b`.
- The official executable hash was reverified by reading its bytes, without
  execution. Both attempt state/ledger file sets remain byte-identical; original
  packet and evidence records are preserved. Main status matches its baseline.
- No live runtime/provider start, account call, enrollment, credential/config
  access, OS settings change, new attempt, reset, deployment or service install.
  Native UI/Keychain, Swift/build checks and independent review are unrun here.

**No further live attempt is necessary to diagnose this feature-name mismatch.**
A separately authorized bounded run would still be needed to validate the
corrected runtime path and eventual native/provider behavior. No later preflight
or turn success is inferred from this fix, and no such run is authorized now.
Both consumed attempts stay blocked. The same-user agent-host versus computer
sandbox boundary and ambient startup limitations remain unchanged.
