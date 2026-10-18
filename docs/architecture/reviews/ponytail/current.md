What this repo does: Fort is a single-owner native AI messenger with a local Node
core and optional runtime adapters. This change adds required review evidence and
connects native removal to the existing verified host controller.

## Semantic review

Reviewer: Codex, October 18, 2026. Reviewed base is
`48954abd3a9b632f2368c734d8ce42c995b547d5`. The exact 25 changed paths and their
content/mode digests are recorded below. The baseline is
[baseline-20261018.md](baseline-20261018.md), with its whole-tree inventory and
explicit depth limits. This is a self-review, not an independent reviewer attestation.

I reviewed the staged implementation, tests and callers at the package-builder,
native controller/model/lifecycle, Git snapshot and CI boundaries. The ordinary
load remains one owner. The gate uses local Git blobs and Node's standard library;
it invokes no provider, service, hook or upstream script. It rejects stale contents
and mode changes, missing paths, malformed provenance, unresolved index entries,
symbolic links and empty comparisons. Its result establishes evidence coverage,
not the truth or quality of this semantic review.

## Must fix

1. **Ordinary native resume remains absent.** The stop path closes its service;
   the fixed bootstrap restores persisted pause and has no explicit native resume
   operation. The core test's explicit resume option is not an installed user flow.
   Add a tested exact-host resume operation with cleanup and identity checks.
   Until then, v1 acceptance stays blocked. Owner: this implementation task.

## Resolved baseline findings

2. **Native CI fixture inputs.** The existing native job now builds its synthetic
   guardian and resources and passes all four required paths. The complete local
   native journey passed with these inputs. The fixture copies the selected Node's
   Homebrew library only when present. Remote CI and standalone-Node execution
   remain unverified; local preparation is the claimed fix.
3. **Executor destination exactness.** A public regression reproduced the accepted
   unpaired-surrogate path. The builder now rejects it before creating an entry.
   All four executor-artifact behavior tests pass, with the opt-in timing test skipped.
4. **Unused removal constructor.** Deleted the eight-line convenience constructor
   after tracked-tree and successor caller searches. Production uses the controller's
   exact app/helper/endpoint/client identity and rechecks it before effects.
5. **Stale README.** Current instructions now point to Spec 059, the acceptance
   ledger, full quality suites, required review and native CI fixture preparation.
   Old Stage 1 instructions are explicitly historical; their evidence was preserved.

The additional removal changes passed tests for signature rejection before any
recovery/progress write, foreign-pairing refusal, disconnect recovery without
unregistration, saved-progress control availability and credential-cleanup retry.
Unavailable deletion is rejected before effects. Durable progress, startup proof
and independent helper-exit checks remain intact. Actual app removal is a system
handoff after confirmed preparation, not a direct recursive app deletion.

## Tests and limits

- 27 gate functions passed: 18 spec and 9 review functions. Initial missing-CLI
  and executable-mode failures were observed before their implementations.
- 1,088 portable tests passed; 5 opt-in budgets skipped. A restricted attempt failed
  local socket creation; the direct probe returned EPERM. Its exact process group
  was stopped, then the supported local-socket run passed.
- 234 native functions passed with isolated timing, signature and complete-host
  checks excluded from that parallel run. The complete synthetic host journey then
  passed independently in 1,327.049083 ms, with owned process groups absent.
- Twenty isolated removal projections had p95 0.268042 ms against 50 ms. This excludes
  real signatures, Keychain, SMAppService callbacks and provider time.
- Strict TypeScript, workflow YAML parsing and fresh unsigned Mac/iPhone Simulator
  builds passed. See [removal evidence](../../../../successor/evidence/2026-10-17-node-foundation/native-removal-composition.md)
  for logs, all timing samples, corrected fixture compilation failures and limits.
- The final staged spec gate covered all 26 staged paths. The review gate covered
  25 content/mode digests, excluding this report itself, in 0.52 seconds against
  the 5-second budget. Whitespace validation passed.

Verdict: This local slice has behavior and build evidence. The missing native
resume action, unresolved deletion scope and external delivery gates still block v1.

Lean: -8 lines, -0 dependencies from the removed constructor. The required review
gate adds 79 implementation lines and uses the existing CI workflow.

Not checked: Real startup activation, real Keychain cleanup, provider accounts,
official runtime invocation, signing profiles, installed screenshots/accessibility,
physical iPhone and TestFlight. The full separate macOS adapter suite was not rerun
because those implementation files did not change. Historical/quarantined code was
not re-audited. No remote CI or branch protection was configured or verified.

## Snapshot evidence

The following metadata binds this report to reviewed Git content and file modes.
It is not a generated substitute for the semantic review above.

```fort-review
{
  "format": 1,
  "reviewedCommit": "48954abd3a9b632f2368c734d8ce42c995b547d5",
  "reviewer": "Codex",
  "scope": "All 25 staged paths: required review workflow/CLI/CI, native removal composition and bounded baseline fixes. Callers and affected boundaries reviewed as described above.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "PT1",
      "disposition": "open",
      "detail": "Ordinary native resume remains a v1 blocker owned by this implementation task; no release exception."
    },
    {
      "id": "PT2",
      "disposition": "fixed",
      "detail": "Native CI prepares explicit synthetic fixtures; matching local journey passed. Remote runner and standalone Node layout remain unverified."
    },
    {
      "id": "PT3",
      "disposition": "fixed",
      "detail": "Public executor builder now rejects ill-formed Unicode before writes; observed red then green."
    },
    {
      "id": "PT4",
      "disposition": "fixed",
      "detail": "Removed unused convenience constructor; controller retains exact package identity and action-time verification."
    },
    {
      "id": "PT5",
      "disposition": "fixed",
      "detail": "Current README now uses Spec 059 and complete quality commands; historical instructions labeled."
    }
  ],
  "tests": [
    "27 gate functions passed; 1088 portable passed and 5 optional budgets skipped; 234 native functions passed.",
    "Strict TypeScript, YAML parse and fresh unsigned Mac/iPhone Simulator builds passed.",
    "Isolated synthetic bundled-host journey passed in 1327.049083 ms with groups absent; 20 removal metadata samples p95 0.268042 ms.",
    "Restricted portable attempt failed socket creation with EPERM and was stopped; supported local-socket rerun passed."
  ],
  "exceptions": [
    "History deletion remains disabled pending exact scope; no v1 exception is granted.",
    "No real provider, startup, Keychain, signing/profile, installed UI, physical device, TestFlight or remote CI acceptance.",
    "Historical and quarantined source inventory is not a deep audit."
  ],
  "files": {
    ".github/workflows/fort-quality.yml": "21f1f398c568a5cb060f224359bfd28c760d34f3c95a12ab25b8a8fa321c7029",
    "AGENTS.md": "6bcd34cbf8ecf3cc24c8df866412ef20080b432a422b8b393e963675477bec49",
    "CLAUDE.md": "fac6e6ee3601912951a8a52ec9955542c040a4e1a360fd97cd009952066d5615",
    "docs/architecture/decisions/060-required-ponytail-review.md": "4863b3e0cae6f6b971af5505c49cdb18b702905218404998307db6a4288578f5",
    "docs/architecture/reviews/ponytail/baseline-20261018.md": "4b6ef76852a6775977dbb1f839512d947743eaf5c533adea19f3903e3500dc93",
    "ops/architecture/validate-reviews.mjs": "b750bf51abe46f47ed8a6cfa0bf386b1e6b290efa9f6fb31eb491ec0bd9d3f5d",
    "ops/architecture/validate-reviews.test.mjs": "97fa0e0ccf7db6d9c5d007e08447c922fa30e6aa32e992adda4516435683252e",
    "specs/059-modular-node-v1.md": "8b1e90548a163029289b907c8a76273788bbb12fa9519396495e123c5549dbbb",
    "specs/README.md": "4d52508a9f6a6c67377bdfc6b77f164c322c427342c6f0c3276b7cbe14e77708",
    "successor/AGENTS.md": "f3822a1c5455f3b5fe0bce143c4c3623b3fa5b816375fdae903c3662a95e70c3",
    "successor/README.md": "f2736b6643c2c43440fae64700ece0c07f4f0107fc4acfcf86df19a130b62437",
    "successor/V1-ACCEPTANCE.md": "7895229dfa691c9f4c550874013ca59f18d629ae1f99bd96240bf562203351b9",
    "successor/apple/FortKit/Sources/FortKit/MacHostControl.swift": "4486e4875537f6925bebab860511b83e98ee269a864877ff6b897abd31a4fc6d",
    "successor/apple/FortKit/Sources/FortKit/MacHostRemoval.swift": "6d09a91a6c0e55a73b73cb50f9f46ccda9ba5fb4e5f2446fdd43a9049213bf63",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "e4904cd4a1eb02d29b61348e5f106c6d9d8e008908cf433764b311023d3f5f47",
    "successor/apple/FortKit/Sources/FortKit/OwnerLifecycle.swift": "719c8b964a13d291480d2fb3536823c5c7336be5c1604dd012709e35e3f14fc6",
    "successor/apple/FortKit/Sources/FortKit/PairingSettings.swift": "6a0886a02bf61679d068d2026b6bb135bbfa5362a7b6fb567ad1db634273cef3",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostJourneyTests.swift": "e3f643c83d5ac8419c899860cec39a985b981411b6f5fac53da6d6a1e99a9ca8",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostRemovalTests.swift": "fad76e69bb322180eceefb23183497061a14d1d1f80f5459a477c7daf85c3316",
    "successor/apple/FortKit/Tests/FortKitTests/OwnerLifecycleTests.swift": "24d4b714a500944eb66613e3a5047901469fa83ae158dad0257786e8dea3d55d",
    "successor/apple/FortKit/Tests/FortKitTests/OwnerSetupModelTests.swift": "ec77fd4c9df70d73e5b5eab1e69ad9baac5aaeeb308cd1d3767898922b130d64",
    "successor/evidence/2026-10-17-node-foundation/native-removal-composition.md": "30761994d135b93366a0cc82a661b2ad0f2fec45fa3151182bb7e4ba213c3f1b",
    "successor/package.json": "4dd1b142a20722f79b224e87fcbc52386e8b74f37396fed95dc1bd4255f3101d",
    "successor/packaging/codex-executor-artifact.test.ts": "838fa34aa19dc6a7078fe27e044e6f0fa0544a765f23e3c4a46bf3230ab97134",
    "successor/packaging/codex-executor-artifact.ts": "1864582e8bf73fa959418a1bca89af8bfd189345cf9738a400150405d739026d"
  }
}
```
