What this repo does: Fort is a single-owner native messenger with a local ledger
and optional providers. This slice gives iPhone the same explicit saved-copy deletion
as Mac. Expected load is one owner and the existing bounded local cache.

## Semantic review

Codex self-review, October 18, 2026, base bb2086d. Applied the pinned Ponytail skill
and retained baseline. Reviewed all changed sources and callers: captured connection,
disconnected/restored model state, credential absence, metadata lease, bounded
traversal, no-follow/owner/link/inode/ACL checks, refusal/retry and visible state reset.
Inspected actual iOS SDK ACL declarations and both compiled clients. No filesystem
check was removed; Mac startup/setup implementation remains guarded. Checked the
package test target graph after the first library-only scheme exposed no tests.
Read final diffs, test logs, timing samples and exact artifact inventory.

## Must fix

1. **iPhone runtime acceptance is still missing — open.**
   - What this is: Shared source and tests now compile for iOS.
   - Problem: Local CoreSimulator is incompatible with Xcode and cannot establish runtime behavior.
   - Fix: Run synthetic tests on compatible tools, then physical Data Protection and installed UI acceptance.
   - If skipped: Build success could be mistaken for a tested iPhone app.

2. **Full host-ledger deletion is still incomplete — open.**
   - What this is: This control removes only a device's saved copy.
   - Problem: Native host deletion still lacks durable confirmation and lifecycle composition.
   - Fix: Complete the separate host-maintenance coordinator and exact installed checks.
   - If skipped: The owner cannot request verified removal of host conversation history.

Verdict: Shared cache slice passes source/fixture/build checks. No new implementation
of filesystem deletion or dependency was added; platform guards were narrowed while
preserving the existing checks. This is not installed-product or release signoff.
Not checked: physical/iOS execution, actual Keychain/Data Protection, installed
accessibility/screenshots, provider behavior, signing/TestFlight or remote CI.

```fort-review
{
  "format": 1,
  "reviewedCommit": "bb2086d9ee3bc7df9c5e5f85605d57c42bf1ac20",
  "reviewer": "Codex",
  "scope": "Share exact-confirmation saved-cache deletion on iPhone and Mac without weakening filesystem/credential checks; network-endpoint synthetic model behavior, both client builds and iOS test compilation.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "IPHONE-CACHE-EXECUTION",
      "disposition": "open",
      "detail": "Shared code and tests compile for iOS, but incompatible CoreSimulator prevents iOS runtime/physical Data Protection/UI acceptance."
    },
    {
      "id": "NATIVE-HOST-DELETION",
      "disposition": "open",
      "detail": "Native host-ledger deletion still needs durable captured confirmation and completed lifecycle/credential/cache composition."
    }
  ],
  "tests": [
    "Initial network-endpoint model status expectation failed, then passed after shared device-neutral action.",
    "Four saved-cache behavior functions including 15 unsafe and three interruption/change cases passed; dedicated fifth timing function passed with 20 isolated measurements, p95 46.304375 ms within 500 ms.",
    "Full native: 253 executed functions passed, ten optional skips.",
    "Unsigned Debug Mac/iPhone Simulator clients passed; FortKit-Package test bundle compiled and linked for arm64/x86_64 iOS Simulator.",
    "Exact app inventory includes Debug dylibs; manifest digest recorded. Portable source unchanged; preceding 1158-pass/six-skip result not rerun."
  ],
  "exceptions": [
    "No iOS test execution or installed UI acceptance; CoreSimulator 1171.7.0 is older than required 1174.9.2.",
    "No actual history/Keychain/provider/network/startup/account action.",
    "Initial FortKit build-for-testing scheme had no test configurations; only later FortKit-Package build establishes test compilation."
  ],
  "files": {
    "specs/059-modular-node-v1.md": "e759ec8dfd2f2872500539149dfdc5c7fb07087e65765e081da14dced0156633",
    "successor/V1-ACCEPTANCE.md": "a4779efb0a031c4ae7d71b0bae4c1dfb99aaf76525c3051186bd22914b162616",
    "successor/apple/FortKit/Sources/FortKit/DeviceStorage.swift": "58d906d524cd17397eb3c358388c4a8d11e6f2ebda815db2d955b7fef56ff13a",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "bc77d52e03f84d7abfd079c51f14955f9c320663b5b785be53a5a2d8548026a0",
    "successor/apple/FortKit/Sources/FortKit/OwnerSetupFiles.swift": "b540aededba58c19a5fdce08b670c33d2b155bc0b81c5fd9d5c51f9d1fbd799b",
    "successor/apple/FortKit/Sources/FortKit/OwnerSetupPreparation.swift": "06bcf69cde51f1ffbfcc554f030937836d56d09a2b3bbac20f102a0e1edaa4b1",
    "successor/apple/FortKit/Sources/FortKit/PairingSettings.swift": "d7f8c4f769833bb7c6f7f7aa221fd31b7225feb1a42afbfa4569e573103c1844",
    "successor/apple/FortKit/Sources/FortKit/SavedHistoryRemoval.swift": "55b9b9b36d47f46b75ae12478e8f1b56da0e842d6687150d9d4648fa143fcee6",
    "successor/apple/FortKit/Tests/FortKitTests/SavedHistoryRemovalTests.swift": "78cc5e02c7f345a281b3be263826224693b8ab4ad80c41cd8c2dd30fe7115776",
    "successor/evidence/2026-10-17-node-foundation/iphone-saved-history.md": "1284b8b7909495b91720381b39ec10e16d5755874e6224489512ed2e6c40191c"
  }
}
```
