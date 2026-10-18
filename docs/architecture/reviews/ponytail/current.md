What this repo does: Fort is a single-owner native messenger with a replaceable
runtime adapter. This slice packages attribution and source copies for the pinned
optional Codex executable. It assumes one review bundle and no provider execution.

## Semantic review

Codex self-review, October 18, 2026, base e4bc021; pinned Ponytail method and preserved
baseline. Reviewed the assembly path end to end, exact source inventory checks,
copy/recheck/rollback, fixture overrides and failure controls. Checked the original
license/license-file distinction and MPL source-delivery requirements against primary
sources. Reviewed the prior audit's unresolved cases, source/header correspondence,
license expressions, alternative choices and the generator's preserved bytes/paths.
Independently reopened all ZIP members and source archive files, compared hashes,
reviewed the recipient notice, and verified all files in the fresh unsigned app.
The original root legal files and runtime binary are unchanged. This is preservation
and engineering packaging review; it does not authenticate upstream ownership or
claim every conservative dependency exists in this target binary.

## Must fix

1. **Review app could omit required third-party/source resources — fixed.**
   - What this is: Earlier assembly required only Codex's root LICENSE and NOTICE.
   - Problem: Missing, modified or unlisted new legal resources still allowed output;
     the three observed failing tests reproduce this.
   - Fix: Pin and inventory the complete six-file payload before output; preserve
     original notices, source declarations and covered sources with retrieval text.
   - If skipped: A later review app could silently lose the prepared attribution.

2. **Native full history removal remains incomplete — open.**
   - What this is: The private compiled deletion seam exists and cache deletion is shared.
   - Problem: Owner-facing host deletion still lacks durable confirmation/lifecycle composition.
   - Fix: Complete that native coordinator before enabling the action.
   - If skipped: Source deletion would be mistaken for verified product removal.

3. **Apple/provider acceptance is still missing — open.**
   - What this is: The resulting app is an unsigned inert engineering artifact.
   - Problem: Existing account records/profiles, installed behavior and TestFlight have not been verified.
   - Fix: Verify exact prerequisites and perform separately authorized installed checks.
   - If skipped: Review assembly could be reported as delivered v1.

Verdict: Attribution/source payload and assembly controls pass; continue native
composition. No dependency or runtime initialization was added. The 9.23 MB legal
addition stays below its 16 MiB bound and can be removed with the optional runtime.
Not checked: target-exact binary membership, independent upstream rights ownership,
Apple eligibility/account agreements, signing, installed UI, provider execution,
physical iPhone or remote CI. Three missing-notice cases are documented choices,
not invented copyright statements or a blanket legal clearance.

```fort-review
{
  "format": 1,
  "reviewedCommit": "e4bc02141a35fd5e1ff0f4125561f1d71fc0822d",
  "reviewer": "Codex",
  "scope": "Pinned optional Codex attribution and covered-source resources, immutable binary/source correspondence, exact legal payload assembly guards and inert fresh app verification.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "INCOMPLETE-LEGAL-PAYLOAD",
      "disposition": "fixed",
      "detail": "Assembler now requires exact original root notices plus conservative attribution and covered-source archives/manifest/readable retrieval notice; missing, changed and unexpected legal files refuse before output."
    },
    {
      "id": "NATIVE-HOST-DELETION",
      "disposition": "open",
      "detail": "Durable exact native confirmation and lifecycle/credential/cache composition remain before enabling host-ledger deletion."
    },
    {
      "id": "APPLE-PROVIDER-ACCEPTANCE",
      "disposition": "open",
      "detail": "Unsigned inert assembly is not provisioned, installed, TestFlight-available or provider-verified; iOS runtime remains unavailable with mismatched CoreSimulator."
    }
  ],
  "tests": [
    "Initial missing-copy test failed; three missing/modified/unlisted resource controls failed before enforcement.",
    "Final Mac assembly: 20 passed, zero skips; strict TypeScript passed.",
    "Full portable: 1158 passed, six optional skips, zero failures.",
    "Independent validation: 3252 ZIP members, 333 original source files in 15 archives, original binary/root notices unchanged.",
    "Fresh unsigned Debug helper built; 186-file inert app independently verified (414399113 bytes); six legal resources pinned and copied.",
    "Swift source unchanged by this slice; preceding final 253 native functions passed/ten optional skips remains separate."
  ],
  "exceptions": [
    "Conservative CLI lock graph includes dev/build/other-target dependencies; not a target binary SBOM or rights authentication.",
    "Three old MIT crates have no standalone copyright notice in inspected originals; preserve exact declared license, source, labelled author metadata and standard terms without fabricating copyright.",
    "No downloaded code or actual Codex executed; no credential, account, signature, startup, network activation, user-data deletion or publication.",
    "No new legal agreement accepted; Apple eligibility and actual account agreements remain separate verification."
  ],
  "files": {
    "specs/059-modular-node-v1.md": "870d9213f2f27506d4440c5d673c93cb5b56faeb12c43dece27e8b9548abdcef",
    "successor/V1-ACCEPTANCE.md": "af05311efd4cf3cc4024534b90e680792f2be4eab2533745c309b4fc1ebadac9",
    "successor/evidence/2026-10-17-node-foundation/codex-attribution-package.md": "7e715a8ddc40726857f7074b97b3a373ebebc5337e945063508553f12b9d9687",
    "successor/packaging/mac-application.test.ts": "5035bba3213d6293674a80189f5959588edcc0af5be13d5079d94251d4d70a70",
    "successor/packaging/mac-application.ts": "f74245b928ab2c1995224a53aab4adb63fd7091e30c3bf4a7018b31ec2d960a7",
    "successor/packaging/vendor/codex-0.157.1/SOURCE-AVAILABILITY.txt": "74b3bb5f6a65cf41da7d88342df969e69598094292e83b6f4a44bca41aace78a",
    "successor/packaging/vendor/codex-0.157.1/THIRD-PARTY-NOTICES.zip": "a8052a3acfe55a59d3e2156f7008cb147a9fb38df0bbc14ec20fba9edbaffb7a",
    "successor/packaging/vendor/codex-0.157.1/THIRD-PARTY-SOURCES.zip": "720e728d378bc4e9d42f59e44de06d593539b9f7945ba18fdc1efe6be9d93354",
    "successor/packaging/vendor/codex-0.157.1/THIRD-PARTY.json": "bd10f9057e7924a4350d4fe2fed95cccc75220ece241d8ee8fbfc78560c191c6"
  }
}
```
