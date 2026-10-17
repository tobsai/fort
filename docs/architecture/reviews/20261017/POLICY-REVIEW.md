# Review of the new architecture constitution and gate

This is a separate review of local commit **`1a864996e4e4f72bca99d22d745f891c3ff8ffe9`**, after the implementation review was pinned to `e02eb571e66620e407a360584817616074730b55`. All twelve originally inspected policy files match the committed Git blobs byte-for-byte; the committed validation report was then read and added as the thirteenth file. Product implementation remains unchanged at `09788c132647458cbdc683506c8bd5622f0be1af`. Exact captured bytes are under `policy-snapshot/`; hashes are in [PROVENANCE.json](PROVENANCE.json). The reviewer did not edit these source files. This commit is local documentation/tooling, not remote CI enforcement or architectural compliance.

**Assessment: suitable as a design-review framework, with its stated enforcement limits. It does not approve a replacement design or certify the current application.** The four principles have clear behavioral proof requirements. Spec 058 is correctly marked for reassessment, including explicit blockers rather than retroactive compliance. No blocking defect was found in the inspected policy changes.

| Area | Assessment | Evidence |
| --- | --- | --- |
| Authoritative instructions | Root AGENTS points to CLAUDE and the constitution. CLAUDE supersedes old Go/vendor mandates, while successor AGENTS makes historical handoffs subordinate to the current pause. | [ARCHITECTURE.md](policy-snapshot/ARCHITECTURE.md), [CLAUDE.md](policy-snapshot/CLAUDE.md), [successor/AGENTS.md](policy-snapshot/successor/AGENTS.md) |
| Independence | Correctly separates AI, hosting, storage, modules and Apple distribution; demands an independently shaped non-OpenAI fixture and absent-adapter builds. Real provider acceptance remains separate. | [constitution §1](policy-snapshot/ARCHITECTURE.md) |
| Removability | Requires package/artifact omission and core send/persist/replay, including removal/readdition and retained history; connector authority and host/tool boundaries are explicit. | [constitution §2](policy-snapshot/ARCHITECTURE.md) |
| Performance | Requires numeric budgets, conditions, repetition and separated provider cost; incidental build logs are rejected as benchmarks. The current budgets remain unassigned. | [constitution §3](policy-snapshot/ARCHITECTURE.md), [Spec058 Speed](policy-snapshot/specs/058-native-first-owned-agent-mvp.md) |
| Lifecycle/delivery | Preserves no default pairing expiry, optional expiry and keep-history default; distinguishes revoke/forget and each TestFlight stage. | [constitution §4](policy-snapshot/ARCHITECTURE.md) |
| Spec template | Captures boundary, removal impact, acceptance, evidence, tradeoffs, status and unresolved judgment for each principle. This is useful review structure without pretending text is proof. | [TEMPLATE.md](policy-snapshot/specs/TEMPLATE.md) |
| Structural gate | Discovers nested spec Markdown, rejects duplicate/missing review fields, validates status, hashes accepted content and refuses historical-only implementation authority. | [validator](policy-snapshot/ops/architecture/validate-specs.mjs) |
| Preservation | All 55 historical manifest hashes were independently compared with their Git blobs at the manifest's source commit and matched. | [manifest](policy-snapshot/ops/architecture/historical-specs.json), [provenance](PROVENANCE.json) |
| Current gap inventory | Accurately names Codex literals, memory coupling, missing performance proof, absent host adapter and delivery limits. The independent review adds storage amplification, idle checkpoint writes and attempt correlation. | [reassessment](policy-snapshot/docs/architecture/reassessment-20261017.md), [main review](REVIEW.md) |

## Executed checks

These checks exercise the documentation gate only. They do not execute Fort, providers, builds, installs or release actions.

| Command | Result |
| --- | --- |
| `node --test ops/architecture/validate-specs.test.mjs` | Exit 0; 10 passed, 0 failed |
| `node ops/architecture/validate-specs.mjs` | Exit 0; 1 review record, 55 preserved historical files |
| `node ops/architecture/validate-specs.mjs --implementation specs/058-native-first-owned-agent-mvp.md` | Expected exit 1: `implementation requires accepted review; found needs-reassessment` |

The exact outputs are preserved in [gate-results.json](gate-results.json). Toolchain identity is recorded in PROVENANCE.json.

The tests exercise the CLI with temporary spec trees, including historical drift, accepted-content drift, draft rejection, blocked judgment, nested files, fenced examples and invalid paths. They therefore prove useful gate behavior rather than merely searching source strings. They still do not judge the truth of evidence or validate application architecture.

## Limits to retain in future wording

- An accepted content hash detects drift. A writer can replace both the digest and a purported decision reference. Actual owner authorization is a separate human/trusted-record check, as the workflow already states.
- The gate cannot decide whether a performance budget is meaningful or a fixture resembles another provider. Required tests, measurements and review supply that proof.
- The npm pretest/pretypecheck hooks run the structural scan. They intentionally permit a `needs-reassessment` record to pass structure. They are not the implementation gate and do not map changed application files to an accepted spec.
- Requirements outside `specs/`, direct command invocation and remote branch settings remain outside local enforcement. CI and review should retain the explicit implementation-spec check, without presenting a local hook as universal enforcement.
- Preserve accepted design versus verified implementation versus release authority as separate states. Planned tests can be accepted as a plan; they cannot justify delivery claims.
- Move old operational handoffs out of standing instruction files when practical, retaining immutable links and evidence. The new precedence notice prevents immediate ambiguity, but the long historical instruction body still increases reader load.

The owner must still settle TestFlight topology, target language, connector contract, reachable transport and performance budgets in a reviewed proposal. This report supplies that proposal's architecture input; it does not manufacture those decisions.

## Exact enforcement gaps for the executive packet

1. **Decision authenticity is unchecked.** The validator accepts a substantive decision-reference string and a self-computed matching digest. The existing accepted-design fixture names `fixture-record.md` without creating that record and passes. This directly demonstrates the documented limitation: hashes protect content association, not authorization. Human/trusted decision review remains required.
2. **Spec applicability is unchecked.** There is no mapping from changed product files to their governing spec. An accepted unrelated spec can satisfy that spec's implementation check; the checker cannot determine whether it covers the proposed changes. The full structural scan and explicit change-to-spec review are both necessary.
3. **Ordinary npm checks do not enforce acceptance.** Their new hooks call the structural check, which correctly accepts the current needs-reassessment document. They do not run the implementation mode. No checked-in CI or branch protection enforces the stronger sequence. Do not label these hooks as an implementation lock.
4. **Historical baseline governance is manual.** The manifest is locally editable. Its 55 entries currently match the recorded Git revision; future additions or changes require independent review against Git. Rehashing an altered historical file is not authorized preservation.
5. **Substantive compliance is deliberately outside the parser.** Complete prose can pass while benchmarks are missing, adapters remain coupled or removal is broken. Four blocked Spec058 rows and the expected implementation rejection are honest negative controls, not product compliance.

These are explicit scope limits rather than newly discovered hidden bypasses contradicting the documentation. The gate is a guardrail for a cooperative review workflow, not a security boundary against an actor who can rewrite its inputs. Its ten tests prove parser/CLI behavior, not the architecture principles themselves.

The committed [VALIDATION.md](policy-snapshot/ops/architecture/VALIDATION.md) accurately separates its Node 26.5.0 typecheck/precheck results from unrun product tests and delivery. This review independently ran the CLI suite and controls using host Node v22.22.3; it did not rerun the reported typecheck. Both toolchains are recorded without treating the older host as validation of the product's Node 26 pin.
