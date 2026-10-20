# Codex 0.163.0 offline candidate — October 20, 2026

Prepared on `codex/fort-openai-rebuild-20261016`, after
`3aba02fd3a694c36ee052639cd0a0275cfbb0799`, under the offline preparation scope in
[Spec 059](../../../specs/059-modular-node-v1.md#offline-preparation-for-codex-01630).
This is a tested adapter/package candidate, not an installed runtime update or a
working authenticated GPT-6.1 Sol application. No provider login, inference,
credential extraction, host activation, pairing, upload, push or merge occurred.

The preceding [Quit repair](model-selection-recovery.md#pairing-navigation-quit-repair-october-20)
is committed and installed. At 04:06 UTC its 206-file manifest still matched report
`5d24d0c8e3595334e2bb0b0becb10b84d698b7516eb25c22f4802de0017b98f8`.
Client, host, offline runtime, startup job, credential path and active connection
were absent. Only file presence was checked for credentials. Retained history was
not changed. The installed runtime remains 0.157.1.

## Exact upstream and candidate

The [official 0.163.0 release](https://github.com/openai/codex/releases/tag/rust-v0.163.0)
was published October 15 at 21:30:38 UTC. Reviewed source:
[`83086188ae0614bd882818ff1aa23f3194962012`](https://github.com/openai/codex/tree/83086188ae0614bd882818ff1aa23f3194962012).
The annotated tag object is `129b8ec7ee6f4511557a4856279f529025c25af4`.

| Item | Bytes | SHA256 |
| --- | ---: | --- |
| Official arm64 macOS archive | 99,551,880 | `61c4d9a92521bdb42b0c1d9f87c673f6da628a06de5945cbdd9a0010632f6d10` |
| Unmodified official executable | 249,952,688 | `8813367285524ac798dd96d1b071aea30d58708c7226fd236cd275cf1cfdee31` |
| Review package report | — | `3ecd55ce2c16a6a3b0f23893ceb9ebad454283834984613599c51df631ecfae6` |
| Canonical sorted payload inventory | — | `9dc3f53cf4f85652c78dc56c6fa2fa7dd4ebf49dae4264cb95b5422e04daea3c` |

Local review app:
`/tmp/fort059-codex-0163-20261020/review-package/Fort.app`.
Its `package-report.json` records every input and all 202 output files
(419,567,457 bytes). Independent readback verified exact paths, modes, sizes,
hashes, legal payload and source revision. The app is unsigned, unprovisioned,
uninstalled and unlaunched. Its LaunchAgent definition is inert package content.

Inputs are the Quit-fixed Mac client, a fresh successful FortHost Release build,
112 compiled host resource files, reviewed Node 26.5.0 and its libraries/notices,
the existing process guardian, and the exact Codex binary. Temporary copies of the
unsigned Xcode outputs change only their Info.plist bundle identifier from the
fixture identifier to the existing installed identities `io.mtree.fort.mac` and
`io.mtree.fort.mac.host`. The copies, original plist hashes and exact changes are
recorded in `unsigned-inputs.json`; original build outputs are preserved.

## Runtime behavior and boundary

Before execution, inspected upstream protocol, features, configuration loading,
managed preferences, model catalog and legal/source metadata as text. No downloaded
source, install hook, package hook or build script was executed.

The real binary ran behind the reviewed diagnostic guard with a fresh empty private
HOME, CODEX_HOME and workspace, bounded output and cleanup, and denied network.
The first guard run stopped before execution under the outer tool sandbox. The
next run initialized but refused `config/read` because managed preferences could
not synchronize. This refusal was preserved; no permissive fallback was accepted.

Source review identified the macOS forced-preference keys
`com.openai.codex` / `config_toml_base64` and `requirements_toml_base64`. A
presence-only CFPreferences check found neither forced; no values were read.
The diagnostic guard then allowed only cfprefsd lookup, its shared-memory reads and
preference reads in addition to the original system-library/private-directory
permissions. This exception is explicit: preferences IPC can access state beyond
ordinary file rules. It is not complete host isolation. Production permission and
entitlement policy did not change, and managed requirements were not bypassed.

The final capture confirms:

- All 175 feature rows, four permission profiles and all inventory pages were read.
  Global and thread MCP inventories are empty.
- Account is null and authentication is required. The bundled catalog has eight
  entries including exact `gpt-6.1-sol`; this is not an account entitlement result.
- An empty ephemeral thread accepts exact `gpt-6.1-sol`, provider `openai`, no
  environments, roots, instruction sources or dynamic tools, and the Fort no-files,
  read-only/no-network profile. No `turn/start` or login was sent.
- `cli_daybreak`, `api_key_model_discovery`, `api_key_cyber_access_programs` and
  `ultrafast_mode` are explicitly disabled and verified in both scopes. Missing,
  malformed or enabled controls refuse before a turn.
- Upstream still forces `unified_exec` true despite its opt-out; it selects a
  backend. `shell_tool=false` and empty environments remain the shell authority
  controls. Existing alias checks are retained. In-app voice/browser annotation
  flags are desktop capability permissions; Fort exposes no corresponding operation.
- Analytics, auto-update, cloud skills, ambient instructions, remote control,
  integrations and computer capabilities retain their existing opt-outs/checks.
- Exit was zero, the owned process group was absent, and no credential file existed.

The current public control verifier separately accepts the captured global/thread
responses through ten replayed RPC calls. This tests the adapter against observed
data; it is not a second live runtime or inference result. The checked-in fixture
explicitly labels its invented account, synthetic identifiers/workspace and adapted
default model. The original unauthenticated observations remain separate.

The agent host and computer-tool sandbox remain different boundaries. No inference
about complete host confinement is drawn from a no-files thread.

## Attribution

The old 0.157.1 legal payload and startup capture remain intact. The new six-file
legal payload is 9,290,451 bytes and contains the original Apache-2.0 LICENSE,
NOTICE, dependency notices and covered-source copies.

The conservative CLI closure has 1,343 packages: 1,177 registry records, 18 Git
packages and 148 workspace packages. It includes build, development and other-target
dependencies and is not a binary SBOM. Exact version/checksum equality permits
1,158 existing registry records to be reused; 19 new or changed crate archives were
downloaded and checksum-verified. Missing age/age-core notices were recovered at
their exact crate VCS revisions. Updated crossterm and rmcp notices use exact Git
commits. Prior supplemental notices are retained and labeled conservatively.

The rmcp/rmcp-macros transition has mixed MIT/Apache-2.0 terms. The full original
transition notice and both license texts are preserved; earlier contributions are
not represented as wholly relicensed or as an unrestricted OR choice. Its
CC-BY-4.0 documentation notice is retained; no upstream documentation is newly
copied in this update. Existing MPL covered-source obligations retain 15 source
archives. Readback verified all 3,248 notice archive entries, 19 source archive
entries, every declared digest and all copied legal bytes. Details and source URLs
are in `packaging/vendor/codex-0.163.0/THIRD-PARTY.json`.

## Validation

All Node checks use the reviewed `/opt/homebrew/bin/node` 26.5.0. Logs below are
under `/tmp/fort059-codex-0163-20261020/`, unless another directory is named.

| Check | Result | Evidence |
| --- | --- | --- |
| Exact 6.1 model/version and new opt-out regressions | Red, then green | Adapter/control logs and new public regressions |
| Complete portable suite | 1,363 pass, 16 optional skips, zero failures | `portable.log` |
| TypeScript typecheck | Pass | `typecheck-final.log` |
| Compiled executor packaging | 4 pass, 1 benchmark skip | `package-green.log` |
| Compiled Mac executor/guard checks | 29 pass, 2 benchmark skips | `executor-macos-final.log` |
| Mac assembly including source-provenance regression | 21 pass, zero skips | `provenance-green.log` |
| Wrong/missing binary refusal and signed offline version/provenance | 2 pass, zero skips | `signed-provenance-green.log` |
| Fresh FortHost Release build | Pass | `helper-build-final.log` |
| Exact review package and legal readback | Pass, 202 files | `review-package-verification.json` |
| Installed unchanged/disconnected check | Pass at 04:06 UTC | `installed-still-disconnected.json` |
| Spec, staged implementation, exact review and whitespace gates | Required before commit; terminal results record execution | Architecture tools |

The previous Quit slice independently passed 342 native tests with 16 optional
skips and Mac/iPhone Simulator Release builds. Those results do not establish a
new runtime's installed/native or provider behavior. The signed probe executes
only `codex --version`, denies outside read/write and loopback listening, confirms
process cleanup, and preserves the original binary.

Failures retained during preparation: outer-sandbox guard refusal; managed-
preferences refusal; an aborted malformed-MCP fixture run after its baseline
became empty; compiler cache access failures; missing synthetic selected-account
fields in compiled fixtures; initial old-version/hash expectations; and two stale
source-revision report labels caught during final review. Each source issue was
corrected and its relevant check passed. The provenance assertions failed first
against the old SHA, then passed with the exact new SHA. An incidental PATH-selected
Node 22 rerun refused before the intended check; rerunning with the explicit reviewed
Node produced the meaningful red/green results. Initial assembly refused the Xcode
fixture bundle identities before output; normalized temporary input copies passed.
The first staged scope check rejected path declarations placed after the implementation
section; moving the same declarations into that section corrected the scope record.
No long-running test/build remains stuck. Earlier failed artifacts/logs remain
diagnostic evidence and are not accepted packages.

## Remaining gates and next scope

**Do not install this candidate over the retained ledger.** Existing installation
metadata binds the runtime to 0.157.1. `createApplication` deliberately refuses a
different version with “explicit binding migration is required.” A public regression
now proves refusal and successful reopening under the original version, with the
exact agent binding, receipt, messages and completed run unchanged. No production
migration or data rewrite was added.

Before an installed runtime update, design and test an explicit history-preserving
binding migration, including pending work, recovery and rollback. Preserve historical
versions and binding revisions; deleting history or silently changing queued work
is not an upgrade strategy. Then prepare an exact signed/provisioned package and
state the bounded installed-update/activation/account-verification scope.

Live managed ChatGPT availability, effective GPT-6.1 Sol inference, the exact paired
installed Settings confirmation/Quit journey, physical iPhone, TestFlight and G2
upload remain unrun or held. The completed earlier attended grant is not reusable.
Any new live activation/sign-in must use a fresh explicit scope and the official
managed runtime integration; no new authentication route or extracted credential.
