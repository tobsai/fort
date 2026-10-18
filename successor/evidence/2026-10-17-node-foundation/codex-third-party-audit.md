# Codex third-party attribution audit — October 18, 2026

Governing [Spec 059](../../../specs/059-modular-node-v1.md), decision 059. This is
read-only provenance research for the already selected Codex 0.157.1 artifact,
not an additional dependency, runtime replacement or distribution clearance.
The exact binary remains SHA256
`27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d`, source
`36650394c5b38c2990ccf2a3457165ca3e9d9726`.

## Result and deliverables

The [audit manifest](codex-third-party-audit.json) records all source URLs, published
archive checksums, collected notice hashes, exact upstream revisions, gaps and the
local review archive's complete membership. The archive is retained at
`successor/.build/codex-third-party-audit-20261018.zip` (ignored local build evidence):
**3,257 files, 6,681,695 bytes**, SHA256
`b7add7bd05ea3324273a82f595f4a34d100df0055596c35463a060f356a0f073`.
All archive entries were independently read back and verified against their hashes.
The archive includes collector sources, public metadata/notice texts and selected
published source-header evidence. Nothing in it was installed or executed.

The exact [Cargo lockfile](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/Cargo.lock)
contains 1,468 packages. Traversing the CLI's lock graph conservatively reaches
1,337: 1,177 registry packages, 16 Git packages and 144 workspace packages. This
includes development/build and other-platform dependencies. It is **not a binary
SBOM**, nor proof every listed dependency is present in the selected arm64 binary.
The [release workflow](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/.github/workflows/rust-release.yml)
identifies target-specific release builds but supplies no complete notice bundle
in the inspected tree. Root LICENSE/NOTICE alone cannot establish closure.

- Downloaded **1,177 exact registry archives**, 205,858,353 bytes on the initial
  collection pass. Every archive matched its Cargo checksum; zero download/hash failures.
- Found **1,902 notice files in 1,062 packages**. Kept original bytes and paths,
  including multi-license declarations; no copyright text was fabricated.
- **115 archives lacked a selected standalone notice**. Published Cargo VCS metadata
  and immutable upstream trees located additional notice candidates for **93**.
  Twelve lacked VCS metadata; others required nonstandard paths or source headers.
- Conventional paths left **22** cases. A second read found published copyright/license
  source headers for 15 of them, including the full MIT notice for `bech32 0.9.1`.
  Those observations are not automatically classified as complete legal closure.
- All 16 Git packages map to seven exact upstream revisions with root notice texts.
  The graph includes Windows-only packages; inclusion in the audit does not enable them.
- `difflib 0.4.0`: all six published Rust files match the later immutable upstream
  [tree and MIT notice](https://github.com/DimaKudosh/difflib/blob/f035fb8e656f27119e23eca9d5b996df14f3885e/LICENSE).
  Keep that original copyright wording; crate authorship metadata is not a substitute.
- Later `fax` source differs: one matching and eleven changed Rust files for `fax`,
  and one changed file for `fax_derive`. Its later notice is retained as a candidate,
  not asserted to match the old source. `derive/LICENSE` is a link target string;
  the actual parent notice must be used, not that string as purported license text.

## Concrete remaining judgments

Resolve the exact applicable attribution for `debugserver-types 0.5.0`, `fax 0.2.6`
and `fax_derive 0.2.0`. Their published metadata declares MIT and names authors;
no standalone notice or matching source header was found in the inspected original
archives/trees. Do not invent a copyright year or silently copy a different version's
license. Review the remaining metadata/header cases and dependency membership, then
produce the application notice payload and verify it against the pinned runtime.
This is implementation/research work, not a generic request for owner legal approval.

The graph also contains MPL components, including `option-ext`, Symphonia crates
and the Git-pinned nucleo packages. A final distributed payload needs appropriate
source availability and recipient information for any such included covered code.
MPL permits larger works under other terms while retaining covered-source obligations;
this does not mean Fort's unrelated source must be relicensed. See
[Mozilla's license, sections 3.1–3.4](https://www.mozilla.org/en-US/MPL/2.0/).
Do not confuse the separate omitted hosted-voice payload with audio code that may be
statically included in the official executable. The final runtime scope still needs
verification; this audit grants no voice/account permission.

## Method, failures and limits

Read exact public source metadata and crate archives only. Collectors used a 1-GiB
initial aggregate download budget, 50-MiB archive bound, 512-MiB expanded-member
bound, and 2-MiB selected-text bound. They read archive members without extraction
or execution; output filenames are content hashes. Follow-up passes reread selected
public archives. HTTP 404s for candidate paths were absence observations, not ignored
successes. Metadata/header omissions are listed in the manifest and raw reports.

System Python lacked `tomllib`; metadata parsing then used installed Python 3.14.
No package install or global configuration change was made. The upstream tree query
through web browsing initially failed; the public GitHub read-only API succeeded.
Search results unrelated to the pinned repository were discarded.

Runtime/build tests were not rerun for these evidence-only files. Their preceding
[source/configuration results](node-openssl-configuration.md) remain separate.
No credentials, account login, official Codex execution, external message, feature
activation, signature/provisioning, startup registration, real-data deletion, provider
cleanup, repository publication or visibility change occurred. The current review
Mac app still contains only its earlier root Codex LICENSE/NOTICE and remains
unsigned, unlaunched and unsuitable for distribution acceptance.
