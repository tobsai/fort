What this repo does: Fort is a single-owner native AI messenger with a local Node
core and optional runtime adapters. This slice supplies a fixed bundled OpenSSL
configuration to the two Node launches. Expected load is one owner and private host.

## Semantic review

Codex self-review, October 18, 2026, base `6c9895e`. Used the pinned
[Ponytail skill](https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md)
and preserved baseline. Read changed source/tests plus caller launches, bundle file
inspection, private runtime plan, artifact builders, assembler and sandbox evidence.
Traced file ownership, bounded reads, emitted argv/environment, process refusal and
cleanup. Reviewed the final delta and artifact manifests. This is not independent
attestation. No whole-tree unused-code claim or dependency reduction is made.

## Must fix

1. **Production launches ignored the proven local OpenSSL configuration — fixed.**
   - What this is: The sandbox probe selected a fixed file; ordinary host/worker did not.
   - Problem: The recorded probe hit forbidden global configuration; the new red worker
     test still launched after corrupting its supposedly selected configuration.
   - Fix: Both resource artifacts carry the fixed bytes; preparation validates them,
     and both launch arguments select them. Public behavior tests now refuse malformed
     startup before provider availability. Package composition preserves both files.
   - If skipped: Installed execution retains an unwanted machine configuration prerequisite.

## Should fix

2. **Complete third-party attribution is still missing for the Codex binary — open.**
   - What this is: Pinned root LICENSE/NOTICE are retained, but not the whole dependency set.
   - Problem: Distributing the current review artifact could omit required notices.
   - Fix: Finish the pinned source/license inventory and artifact inclusion before shipping.
   - If skipped: Distribution obligations remain unverified. Owner: implementation agent;
     no generic additional owner approval is inferred for read-only license research.

Verdict: Configuration omission is fixed in source, synthetic runtime tests and the
unsigned package. Continue notice/deletion work and exact external acceptance gates.
Not checked: hostile same-user races, final signatures/profiles/App Groups, live provider,
account access, startup registration, native UI/accessibility/screenshots, iPhone
transport, TestFlight, remote CI or branch protection. Existing permissions are unchanged.
The initial overlapping native run failed two timing assertions; an isolated serial
run passed without relaxing budgets. Source fixtures do not establish installed operation.

```fort-review
{
  "format": 1,
  "reviewedCommit": "6c9895e584754a486c68e645d0bff2af02a82e4c",
  "reviewer": "Codex",
  "scope": "Both production Node launch paths, bundled fixed configuration, safe file inspection, builders, assembler and behavior tests; self-review with baseline retained.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "OPENSSL-BOOTSTRAP",
      "disposition": "fixed",
      "detail": "Packaged fixed configuration reaches host and executor argv; unsafe/altered preparation and late malformed launch refuse before provider availability."
    },
    {
      "id": "CODEX-NOTICE-CLOSURE",
      "disposition": "open",
      "detail": "Full static dependency notice inventory remains; root attribution and exact runtime pin preserved."
    }
  ],
  "tests": [
    "1104 portable passed; five optional budget skips.",
    "250 native functions passed in isolated serial run; nine optional checks skipped. Earlier concurrent run failed two timing assertions; no threshold relaxed.",
    "10 compiled executor and 17 assembly checks passed; strict TypeScript passed.",
    "20 isolated samples: private executor p95 533.575 ms, native metadata p95 0.916 ms.",
    "Fresh unsigned Release Mac/helper/iPhone Simulator builds passed; 173 output hashes/modes and 171 input hashes independently verified.",
    "One read-only native package refusal/layout check passed. Installed UI/provider/signing/TestFlight and remote CI unrun."
  ],
  "exceptions": [],
  "files": {
    "specs/059-modular-node-v1.md": "5b2cb40af1c505bcc1f23202477110bade09753de0bd62b63028d1ddf4c4bb4f",
    "successor/V1-ACCEPTANCE.md": "d9dd42d984c23cd91134c35b47d819d6fc9dadd916fc2aa03731faf95f911bbd",
    "successor/apple/FortKit/Sources/FortKit/MacAgentService.swift": "abf81292845b0edc8b0db6af917982a03e031c8112ee25f565d1f043ba6eb1c3",
    "successor/apple/FortKit/Sources/FortKit/MacHostBundle.swift": "3ec5cd5c3120ae9db87b9dc8a81a8bb445063b7b690b3b52bededf4dfb440b80",
    "successor/apple/FortKit/Sources/FortKit/MacHostProgram.swift": "d2c9f270d43e99ca143ff8908428d645d7d595ee64c80dd21047f6c0d6334ffe",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostJourneyTests.swift": "a041300843a8fe2a74fd64180096a2458688e6b3e660ab1eced9075aa4dc87e7",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostProgramTests.swift": "f6b480a4af79b747abd418515b69ca24e29153061947f94b45f0bcbdb007397d",
    "successor/evidence/2026-10-17-node-foundation/node-openssl-configuration.md": "586d28d22ae7aa19cc18973367784b3c3992c31768203b388c883a59b3db5481",
    "successor/packaging/codex-executor-artifact.macos.test.ts": "ff77cfb144da553329aacf18b59d8696b5bf84b7ef67a46a445c17af996732a5",
    "successor/packaging/codex-executor-artifact.test.ts": "b78cedc3250bace723fc07d4e6b891ac68e9ec650b00beae037223c49d620d65",
    "successor/packaging/codex-executor-artifact.ts": "f87530b15e47407be67d6d39e2e86f6c01a1127b82c95e2b0367311ec7f7a42e",
    "successor/packaging/fixtures/executor-contract.mjs": "4f60287f824103357a31c6291274b40b5839e605bfd30a8cdbafbc38bf3e1c69",
    "successor/packaging/host-resources.test.ts": "9d44dff70672d1c48451b7338dc49b11720ee1fcd94e44353781b2451ccbbb3f",
    "successor/packaging/host-resources.ts": "f5dbc7677d53331d64efbe4aa8692c6a9b6c6281c54f89d205b190ceb5ff2a2c",
    "successor/packaging/mac-application.test.ts": "aea05531e8cc81d016b46819f8b4d8691a0c60e443c122205fbc22bcf6bb2004",
    "successor/packaging/mac-application.ts": "cfcae08df4f21f0cb00841d4a688efde04bef49a033fb48f9cf3cd0eef3fc42d",
    "successor/runner/codex-executor.ts": "910422b020875647aaa3060adc1acec58ef181391b3454fc570f030878270a77"
  }
}
```
