# Pinned Codex attribution package

October 18, 2026; Spec 059, base e4bc021. This completes the current conservative
source-attribution payload and covered-source packaging. It is not an exact target
binary SBOM, an assertion of third-party ownership, an Apple eligibility decision,
or provider/installed acceptance. No downloaded source or actual Codex was executed.

## Exact inputs and disposition

Codex 0.157.1 remains the original 238,223,808-byte binary, SHA256
27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d; source revision
36650394c5b38c2990ccf2a3457165ca3e9d9726. Existing root LICENSE and NOTICE are
byte-for-byte unchanged. The prior audit and its original gaps remain historical.

The payload contains 1,177 exact registry packages and 160 Git/workspace entries
from the conservative CLI lock graph, including development and other targets.
Original collected notices, Cargo metadata, source headers and immutable upstream
references are retained. Alternative licenses use Apache-2.0 when available,
otherwise MIT or the applicable original permissive license; AND requirements and
MPL-only terms remain. No GPL/LGPL alternative is elected. All original collected
license texts remain even when another offered alternative is used.

Cargo's license field declares the package's release license; it is distinct from
an optional license-file path ([Cargo manifest documentation](https://doc.rust-lang.org/cargo/reference/manifest.html#the-license-and-license-file-fields)).
The three unresolved standalone-notice cases are handled without manufacturing a
copyright statement: debugserver-types 0.5.0, fax 0.2.6 and fax_derive 0.2.0 retain
their original checksum-verified source archives and MIT declarations, labelled
original author metadata, and unmodified standard MIT terms. Authorship is not
relabelled as copyright ownership; no year is invented. SPDX templates are labelled
as templates, including their placeholder line. The package declaration is the
license basis, not a claim that Cargo authenticates rights or supplies a missing
notice. A different later fax notice is excluded. Difflib's actual later notice is
included with the prior exact six-Rust-file correspondence evidence.

For the remaining no-standalone-file cases, preserve the collected original source
headers, immutable upstream notices/README declarations, published Cargo metadata
and relevant standard terms. THIRD-PARTY.json and archive provenance retain the
correspondence rather than silently attributing unrelated upstream notice text.
The exact original source/license metadata remains available to review every choice.
No new legal agreement is requested or accepted by this engineering step.

MPL section 3.2 requires covered-source availability and recipient information
([original license](https://www.mozilla.org/MPL/2.0/)). The source ZIP includes
unmodified exact registry archives for option-ext and all nine Symphonia crates,
termina conservatively despite its MIT alternative, and nucleo/nucleo-matcher at
4253de9faabb4e5c6d81d946a5e35a90f87347ee. The three MIT cases above are included too.
All 14 registry archives match their Cargo checksums. The fifteenth is the exact
nucleo revision archive, with its own SHA and every source member recorded.
Source and full MPL text ship with the runtime; SOURCE-AVAILABILITY.txt tells the
recipient how to obtain the bundled copies through Finder without a network account
or a new hosting service. Fort adds no restrictions to those original source rights.

## Verification and artifact

- 3,233 notice ZIP entries plus 19 source ZIP entries independently read and verified.
- All 333 source files inside the 15 original archives independently verified.
- Additional payload size: 9,228,672 bytes, below the 16 MiB spec bound; no runtime work.
- Initial copy assertion failed with absent attribution. Three refusal tests then
  failed because missing/modified/unlisted legal files still permitted assembly.
  The assembler now inventories and checks the exact pinned six-file legal set
  before creating output. All 20 final assembly checks passed with zero skips.
- Strict TypeScript passed. Full portable rerun: 1,158 passed, six optional skips,
  zero failures, 25.190 seconds. Mac-only assembly checks ran separately above.
- Fresh unsigned Debug helper built against current source. Swift source was
  unchanged by this slice; the preceding 253-pass/ten-skip native run remains
  applicable and was not repeated for legal resource changes.
- Fresh inert app at /tmp/fort059-attribution-review-20261018/Fort.app: 186 files,
  414,399,113 bytes. Every reported file hash/size independently verified. Its
  package-report.json SHA256 is e2ae99c1182878abf3c9c4f00e62bfe0fd7eb879c784513f66ded0cba4be324d.
  It includes the current compiled maintenance resource and six exact Codex legal
  files. It is unsigned, uninstalled, unlaunched and has no startup registration.

Pinned artifact resources (all beneath packaging/vendor/codex-0.157.1):

- `LICENSE`: 10,926 bytes; SHA256 `d17f227e4df5da1600391338865ce0f3055211760a36688f816941d58232d8dc`.
- `NOTICE`: 242 bytes; SHA256 `9d71575ecfd9a843fc1677b0efb08053c6ba9fd686a0de1a6f5382fd3c220915`.
- `SOURCE-AVAILABILITY.txt`: 1,869 bytes; SHA256 `072ecb9716ccdd0f5a0dc9f27bf5f22225431be5b206da801dcb7d282d914ca3`.
- `THIRD-PARTY-NOTICES.zip`: 6,007,446 bytes; SHA256 `82fe87e4ffdf2c6061514c10dfed75a2fe0f0537dc808d1ab98fd537bb120227`.
- `THIRD-PARTY-SOURCES.zip`: 726,266 bytes; SHA256 `37f1f6c01311c4a6d3bc30efe81527c563ce2a26f5b7d41ff4b2003658c9ab8f`.
- `THIRD-PARTY.json`: 2,493,091 bytes; SHA256 `e08eed4f0f065230e18dca896e015e6070efb6c892a9aad98a08478496c28734`.

## Commands and remaining gates

Assembly tests used pinned Node 26.5.0 with --test packaging/mac-application.test.ts,
FORT_TEST_CODEX=/opt/homebrew/Caskroom/codex/0.157.1/bin/codex and
FORT_TEST_NODE_BUNDLE=/tmp/fort059-assembly-node-input. The CLI assembled fresh Debug
FortMac/FortHost under /tmp/fort059-iphone-cache-mac/Build/Products/Debug using
/tmp/fort059-maintenance-host-resources, /tmp/fort059-assembly-node-input,
/tmp/fort059-assembly-guardian and /tmp/fort059-assembly-configuration.json.
Input identifiers remain synthetic engineering IDs; team T3JB5MYZ93 is not proof
of provisioning. No permission, account, signing identity or entitlement was changed.

The exact Apple records/profiles and any actual account agreements still need
read-only verification; none was accessed or accepted here. Installed/runtime terms
and real provider acceptance must use new applicable per-action authority. The
remaining native host-deletion coordinator, physical iPhone/installed UI, signing,
TestFlight upload/processing/tester availability and provider checks are separate.
Do not confuse bundled source availability with authorization to activate access.

Generation used the authored local /tmp/fort059-build-codex-attribution.py against
only the retained public audit and covered-source files. It executed no downloaded
code. ZIP timestamps/order are fixed; all entries and their original source URLs/
hashes are in THIRD-PARTY.json and the archived provenance. SPDX standard texts are
pinned to license-list-data v3.26.0; their exact URLs and hashes are recorded there.

## Raw local evidence hashes

- `/tmp/fort059-codex-attribution-red.log`: SHA256 `f4a48e56fece07a78e2edaa9ea03efa04c2ff441ad0b9615bc69a911b8079d9f`.
- `/tmp/fort059-codex-attribution-controls-red.log`: SHA256 `4b4c18d75363da98ef4224975e2ae18f065002e699f066dc99b38b1ca01404ff`.
- `/tmp/fort059-codex-attribution-assembly-tests.log`: SHA256 `071debec4f8ebf66c4e699b81e0d4d0e40f007ebcfdf0fc9967652c22c0a66a7`.
- `/tmp/fort059-codex-attribution-typecheck.log`: SHA256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- `/tmp/fort059-codex-attribution-helper-build.log`: SHA256 `63f02add032eab5ec66fe2a0c7a7dff1857555cd61e9231ac177fb8662c44ad2`.
- `/tmp/fort059-codex-attribution-app.log`: SHA256 `1a18f6cbae23c6e213864bc0394f222677121f4e28aeb0192a66bec31a302d99`.
- `/tmp/fort059-codex-attribution-portable.log`: SHA256 `b8aad5398b2e345f5d05a2f4a84ecad8a3a77e169ebf990b4857623fcb19c843`.
- `/tmp/fort059-build-codex-attribution.py`: SHA256 `1a0bcac84227e649e95f485d0af793194a5efcc66af5b874d1a1c84da735ae31`.
