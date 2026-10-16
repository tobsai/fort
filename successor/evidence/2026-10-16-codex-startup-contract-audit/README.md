# Complete startup-contract audit — no fourth attempt

Source commit: `d98d9e9881ec38be99171b1683068e151170b64a` on
`codex/fort-openai-rebuild-20261016`, based on `4a1fec29445e4f28b6211d911a082b46543d4f14`.
The worktree is `/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.
This evidence is a review record, not execution approval or a new activation packet.

The [gate-by-gate audit](../../runner/STARTUP-CONTRACT.md) covers config, canonical
features and aliases, removed/forced flags, pagination, global/thread MCP, auth,
exact model selection, thread policy and turn policy together. The
[source-grounded fixture](../../runner/fixtures/codex-0.157.1-startup.json) includes
all 150 canonical registry entries and schema-shaped synthetic responses.

## Result

The third consumed attempt reported `features/unified_exec`. Pinned source explains
the expected mismatch: Codex 0.157.1 forces that backend selector on unless a managed
requirement pins it off. Its ordinary CLI opt-out cannot establish the expected false
row. Shell authority is instead gated by `shell_tool:false` and no environment.
The exact live row was not retained, so this is a source-backed diagnosis plus a
nonlive reproduction, not a reconstructed provider payload.

The audit also reproduces two predicted compatibility failures before any further
live execution: disabled MCP entries are legitimately listed; and model invocation
identity is `model`, distinct from catalog `id`. Three minimal production corrections
preserve explicit capability checks. Missing observations do not become false.
No launcher arguments, real settings, model selection, sample, manual entry, ledger,
native source or installed app were changed.

**Still blocked:** Codex 0.157.1's custom deserializer rejects Fort's
`turn/start.sandboxPolicy.access.type:restricted`. Removing that restriction would
discard the requested boundary. It is preserved. The comprehensive nonlive fixture
models the rejection and never claims a completed conversation. A named permissions
profile with equivalent restrictions and confirmed provenance is the smallest next
design/implementation task; its exact syntax, requirements interaction and enforcement
must be proved before proposing another live attempt. Stable JSON Schema alone does
not expose this custom-deserializer rejection.

## Validation

- Full nonlive Node suite: **477 passed, 0 failed**, exit 0 ([log](full-suite.log)).
  Includes temporary synthetic processes and local fixture listeners; no official
  Codex process or provider connection was started.
- Focused startup/handshake suites: **288 passed, 0 failed** ([log](handshake-tests.log)).
  Covers every current required config/feature check, pagination faults, MCP state,
  account/model identity, missing/changed thread policies and the unresolved turn rejection.
- Strict TypeScript: exit 0 ([empty-success log](typecheck.log)). Whitespace check passed.
- Registry names, stages and macOS defaults: all 150 matched immutable source.
  All 63 public-source files matched their recorded SHA-256 values
  ([provenance manifest](upstream-sources.json)). No upstream implementation was executed.
- Expected red evidence: one failure each for the forced backend, disabled MCP rows
  and model identity, then each passed after its targeted correction. See
  [backend red](red-unified.log) / [green](green-unified.log),
  [MCP red](red-mcp.log) / [green](green-mcp.log),
  [model red](red-model.log) / [green](green-model.log).
  Those isolated early regressions used a synthetic accepted turn response; the final
  suite models the later-discovered restricted-read refusal. Neither is live evidence.
- All three attempt file sets, ledger bytes and state bytes matched the before-audit
  fingerprints ([preservation](preservation.log), [attempt metadata](attempts-preserved.json)).
  Each remains `starts:1`, `phase:blocked`, zero receipts. Databases were hashed as
  files and never opened through SQLite. Existing `credentialCleanup:deleted` is a
  recorded state, not a fresh Keychain inspection.
- Main checkout status matched its preserved baseline. Historical packets/evidence,
  native app file set/hash and executable hash stayed unchanged.

Not run: official runtime/provider preflight or turns; real account/config/credential
reads; an independent JSON Schema validator or compiled Rust deserializer; native
launch/pairing/Keychain/UI signoff; Swift/native rebuilds; independent review; named
profile implementation/enforcement tests. The operator's clean-shutdown and empty
native-storage report is retained as reported context, not re-observed here.

## Pins and limitations

[pins.json](pins.json) seals 462 source files. Canonical source-list SHA-256:
`d4f9f6696151356d0c224cb8488ec1610331b152d744254f9e2016a06d5aeb4e`.

Upstream tag `rust-v0.157.1`, source
`36650394c5b38c2990ccf2a3457165ca3e9d9726`; public-source manifest canonical hash
`0fecef13a2c27fd3749473a03a9078e7f64af6f18b6c951e2e2e4114c19d9b67`.
Official runtime executable SHA-256:
`27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d`.
Unchanged five-file native app list SHA-256:
`9c32247c3c77b1618515aec7faad3944826ec9e11d4ecc0d58483c8ecd23ad2b`;
app build source remains `de9e482bac99920d52a03b6904542a78226c5ba2`.

The same-user agent host remains outside the computer-tool sandbox proof. It starts
with host HOME/access before Fort can inspect effective controls. Authentication
metadata and uncached model discovery can perform network work. No successful
provider acceptance, hostile-host isolation, effective-model attestation, remote
cancellation, installed-product signoff, deployment or release is established.
All three consumed packets remain blocked; no new start or approval is created.
