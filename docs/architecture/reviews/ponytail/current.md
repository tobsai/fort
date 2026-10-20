What this repo does: Fort is a single-owner native messenger with a Node ledger
and optional runtime adapters. This review covers the requested account and Mac
composer changes on the existing branch, not a broader architecture rewrite.

Read changed source/tests, account parsers and persistence, service admission and
queued dispatch identity checks, native model/catalog callers, image storage/cards,
package inputs and the final synthetic UI evidence. No provider adapter, auth
permission, dependency, history schema or runtime binary changes are introduced.
The recorded findings below are fixed; no unresolved in-scope defect remains.

The first default is saved with account completion, with exact reuse thereafter.
Malformed choices, stale scopes, canceled/expired results and orphaned run history
cannot acquire an implicit new identity. Explicit replacement remains visible only
under Advanced and does not retarget accepted queued work. Native measurement no
longer mutates a text document while SwiftUI probes sizes. The same final QA code
passes actual visible typing, Shift-Return, Return, empty-send and image removal.

Verdict: Ready for the separately verified signed disconnected installation.
Not checked: real provider account changes or inference, human IME use, physical
iPhone/TestFlight, remote grants, or every unrelated historical file. The existing
baseline and previous reviews remain in Git history; this is primary-agent review.

```fort-review
{
  "format": 1,
  "reviewedCommit": "3f84c1547777cb65f9906bfcdb53725b44ee3e7d",
  "reviewer": "Codex primary implementation agent",
  "scope": "Single-account default, exact identity safeguards, native advanced account flow, Mac composer keyboard/layout and unsupported image-picker removal; callers and queued identity pins reviewed.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Public service first-account red then green: 43 pass, 2 optional benchmark skips.",
    "Native immediate-completion/retry discovery red then green; exact account and history regressions retained.",
    "Full portable 1423 pass/16 skips; final full native 350 pass/16 skips, zero failures.",
    "Strict TypeScript check and final Mac Release/QA Debug builds pass.",
    "Rendered QA accessibility/screenshots: collapsed first account, explicit advanced mismatch replacement, visible multiline draft, one synthetic Enter submission, no duplicate on empty Return, local preview/removal and no inactive picker.",
    "AppKit marked-text delegation and hosted layout regressions pass; real human IME/provider acceptance unrun."
  ],
  "exceptions": [
    "Primary-agent semantic review, not independent audit.",
    "One active runtime account only; no simultaneous multi-account credential vault.",
    "Mac local image import is intentionally unavailable without expanding permissions; existing previews remain.",
    "Installed signed verification is the next separate stage; no source/build pass is presented as installation.",
    "No new real-provider access. Prior live prompt use remains 2/3. Physical iPhone, TestFlight, effective-model attestation and remote grant closure remain open."
  ],
  "findings": [
    {
      "id": "ACCOUNT-FIRST",
      "detail": "First verified identity required redundant selection and immediate completion skipped discovery. Default selection is atomic and catalog reconciliation covers immediate/retried completion.",
      "disposition": "fixed"
    },
    {
      "id": "ACCOUNT-LEGACY",
      "detail": "Retained runs with no identity must not be silently treated as a new installation. Indexed existence guard and scoped legacy retention prevent that; public regression passes.",
      "disposition": "fixed"
    },
    {
      "id": "ADVANCED-CONTROL",
      "detail": "Rendered native DisclosureGroup did not expose account actions. Explicit accessible expand/collapse button passes the actual QA interaction.",
      "disposition": "fixed"
    },
    {
      "id": "COMPOSER-LAYOUT",
      "detail": "Speculative size probes mutated the live document to zero width. Measurement is now pure and actual scroll-view layout owns width; screenshots and keyboard journey pass.",
      "disposition": "fixed"
    },
    {
      "id": "IMAGE-PICKER",
      "detail": "The Mac local-file picker was exposed without user-selected-file access. The inactive control is removed while existing previews and removal remain.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "docs/architecture/decisions/059-node-foundation.md": "2e303a9dbe3b34825d28d46c97801b9300830ca39f04b07a677ded74a1c24e4c",
    "specs/059-modular-node-v1.md": "65d8bd714fc6c6f6ab9eba61c1f98980480fe1aef95e9f6dcf830709b4c3c071",
    "successor/APPLE-HOST-ACTIVATION.md": "25f804fd54d9581baa0ba9a2b0ff55669fec6282b1f0215b65a57601f734c2b1",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "6e040f7444c80356efbf603b3ccf502a0a887cf77fcf9463f9dbe5e68965c89d",
    "successor/V1-ACCEPTANCE.md": "4cad04381fe5084c79a2bcf6f67b89005d997f8ecc15c85f1116ecf94c42769a",
    "successor/apple/Apps/FortApp.swift": "ab49407cf53215762bd960909dd343c75ef506f73f206cc72275de6630c1f98a",
    "successor/apple/FortKit/Sources/FortKit/AccountSignInFixture.swift": "ae900a995a4a05462af406f60882b5705b9aef7c11aef78a16e7574c00cf71a1",
    "successor/apple/FortKit/Sources/FortKit/AccountSignInView.swift": "dbc8df5dfaa0b66e2c7e92aae28a67b760c2ed1e68338fd37735777976b15cf3",
    "successor/apple/FortKit/Sources/FortKit/MessageComposer.swift": "f23a57d239d6875314dde448f999e47f33788d44b2607bbf62130bfb2a257b76",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "ee7c4523cf2fd0ec67cb63ce716089982cda6a8c9de51071acdc1ec729219621",
    "successor/apple/FortKit/Sources/FortKit/MessengerView.swift": "c15a13d9e12570b2a771d82a45b02f0be468f2fa1b953618a191da34b71cfffb",
    "successor/apple/FortKit/Tests/FortKitTests/AccountSignInFixtureTests.swift": "55f2d87c5e1b1fca72777486a12a0d42c12a5aa6d4d8f6f9b1f1abebf540dfe6",
    "successor/apple/FortKit/Tests/FortKitTests/AccountSignInTests.swift": "7bd6faafb3227e51f59b941ee032dbea7661e91a968eae27f030bbe5a2763129",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostJourneyTests.swift": "291f37f7ef055bc8f5512317d91b05a9e6532413ae3e587067cc84dc93d6df83",
    "successor/apple/FortKit/Tests/FortKitTests/MessageComposerTests.swift": "a42ecb4304ad272bc01cfb55af3e4332a3e14a84310542b2d4f34db4bbd7ffea",
    "successor/evidence/2026-10-17-node-foundation/single-account-composer-20261020.md": "ee0ae29013d7371c0b53d28a8f9b46bad47f9c1c3ea196ee27c6fc9523a31017",
    "successor/service/account-sign-in.test.ts": "7acaed2df4555a5c4305be42e74462169fc8d1b4eb02cca63594f893283b5bd2",
    "successor/service/account-sign-in.ts": "129e9c7b296aba88658ee0694f8395013013cc56d125c243ea895c43c701b8a2",
    "successor/service/openai.ts": "d84bfbaafd9b1fbdd7e9bdc5c9fdd69bd3d845cf9fb5ab9161e1160a359f8f12"
  }
}
```
