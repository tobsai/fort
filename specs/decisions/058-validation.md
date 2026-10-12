# Spec 058 evidence and validation

Written 2026-10-12. This record concerns documentation and preservation of the existing quarantine. It does not report implementation or product acceptance.

## Scope and provenance

Working tree: `/tmp/fort-core-openmaus-20261011`, branch `codex/fort-core-openmaus-20261011`.

Product-source checkpoint: `2dfe43743a35d0a5e2639870c58055f17f95a1bc`. The new commit adds only Spec 058, its executive page, diagram sources, this record, and an additive root README link. Existing untracked review artifacts are preserved and excluded from the spec commit.

The specification follows the latest instruction to make native first class in MVP. It explicitly interprets this as SwiftUI iPhone and Mac apps. It records that interpretation and the changes to historical scope rather than editing prior intent.

Root `AGENTS.md`, `CLAUDE.md`, `CONTEXT.md`, and the foundation's instructions and provenance notes were read. No project `.agents/skills` directory was present in this sparse worktree or tracked in its branch; the historical checkout's `.agents`, `.claude`, and `.codex` lookup exposed no additional skill guidance. The to-spec synthesis and technical-writing/unslop guidance were used within the user's local-document-only scope. Issue tracker publication and an additional preference interview were outside that instruction.

Numbering was checked with `git ls-tree -r --name-only HEAD -- specs` and the historical checkout's `specs/` listing. This branch retains historical numbered specs through 053; the separate checkout contains 054 through 057. No 058 was present in either. Spec 058 does not copy, rewrite, or renumber them.

The historical files were read at `/Users/tobiasgunn/dev/fort/specs/`. Their SHA-256 values at this review are:

| File | SHA-256 |
| --- | --- |
| `054-messaging-first-personal-ai-companion.md` | `e466e656eee2b97379b2d464c622a67d7430bdf586283fbc75bc6c7736d29af0` |
| `055-legacy-teardown-and-clean-baseline.md` | `97fd71a41e01dc427cd6740987a1ff9e77cc4f6cb06af3622189eb6ea4473cfd` |
| `056-messaging-first-personal-ai-companion-build.md` | `f271498a5407c42b4faf18f7535de66a4181caf3a58ab111ee550a33c74391b2` |
| `057-spec055-teardown-progress.md` | `88c84dfac6ac606706975618b7aea1a71588eaca81d9d607d42abb5b8a2fb120` |

The previously completed independent architecture review and quarantine addendum remain under `reviews/fort-foundation-20261011/`. They were supporting evidence, not approval of the new design. The source was also inspected directly: provider contracts, Claude/Codex environment construction, package declarations, request authorization, entrypoint inventory, and import manifest.

## Astra review

An independent reviewer was explicitly configured with `model: gpt-6-astra`, high reasoning, and fresh context for this spec. The tool accepted that configuration. Neither the reviewer nor coordinator had an independently observable runtime model attestation, so this is an Astra-configured review, not a claim of attested inference identity.

The reviewer confirmed native-first sequencing and identified concrete gaps. The draft was amended to specify exact UTF-8 text, transactional Agent/Home binding checks, persisted routine steps, bounded read acknowledgements, Conversation execution barriers, and permission language that avoids repeated approval for already-authorized work.

Restore received an additional pass. A new recovery epoch invalidates old cursors; missing client receipts, restored queued work, and possibly executed schedule occurrences cannot be automatically replayed. The final reviewer response found no other blocking contradiction and requested the queued-work/schedule restore clause, which is included in section 8 and T16. No provider or product suitability is established by this review.

## Checks executed

| Check | Result and limit |
| --- | --- |
| `node --experimental-strip-types --test foundation.test.mjs quarantine.test.mjs`, from the foundation directory | 7/7 pass; 324 auth combinations. The suite executes refusal stubs and pure fixtures, never the archived product |
| SHA-256 comparison against `IMPORT-MANIFEST.json` | 2,509/2,509 match; no donor source change |
| Exact entry-stub bytes and inventoried archive hashes | 72/72 match the refusal stub and recorded archive hash |
| `git diff --name-only -- foundation` | Empty before commit |
| Local Markdown links and heading structure | Checked for the newly written spec documents and README addition |
| Acceptance IDs and required coverage | T01 through T20 are unique; every required product area has a corresponding proposed behavior test |
| `git diff --check` and staged scope | Checked before the focused local commit; only the requested documentation belongs in that commit |

These are documentation/integrity checks and the existing quarantine regression suite. T01-T20 in Spec 058 are requirements for future implementation and have not been executed. No new product tests were written for this document-only change. The diagram files were inspected as Mermaid sources; no renderer was installed and no rendered-diagram validation is claimed.

## Primary documentation checked

The following pages were read on 2026-10-12 for the bounded claims and feasibility gates in the spec:

- [OpenAI authentication](https://learn.chatgpt.com/docs/auth): distinguishes ChatGPT subscription sign-in from API-key billing.
- [Codex app-server](https://learn.chatgpt.com/docs/app-server): describes the official integration and authentication protocol.
- [Claude Code legal and compliance](https://code.claude.com/docs/en/legal-and-compliance): supplies the credential-use restrictions requiring an exact-use feasibility decision.
- [Apple background strategies](https://developer.apple.com/documentation/backgroundtasks/choosing-background-strategies-for-your-app): recorded as a platform-validation reference; the fetched page exposed only a minimal shell, so it is not substantive evidence of Fort behavior.
- [Apple TestFlight](https://developer.apple.com/testflight/): distribution reference to recheck before an authorized release.

No subscription session, login, provider API, physical-device client, or self-hosted installation was exercised. Exact provider/topology suitability remains unproven.

## Remaining gates

Spec approval precedes implementation. The proposed first slice requires a bounded extraction manifest, supported toolchains, an enforceable runner isolation method, both native apps, and one official provider. Each required provider/topology cell and every installed-product gate must later pass before full MVP completion can be claimed.

This work did not install packages, unlock a runtime, read credentials, change provider accounts, deploy, push, upload a new deliverable, or publish anything. No PDF was generated and no new Library-sharing permission was assumed.
