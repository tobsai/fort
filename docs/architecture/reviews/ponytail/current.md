What this repo does: Fort is a single-owner native messenger with a Node core,
Swift Mac/iPhone clients and optional official runtime adapters. This review
covers the Spec 059 October 23 revision: letting the owner's iPhone reach the Mac
host over Tailscale, plus the iPhone's TestFlight signing identity.

Reviewed every changed file and its callers: the secure listener's private-range
check and Host/peer-subnet enforcement, owned-core composition and the private
channel's networkControl frame, network-control owner authorization, the Swift
core transport, MacCoreHost, the XPC listener/channel and client, the Mac agent
service composition, the messenger model's new owner-only actions, the settings
and phone setup views, and the minimal Swift compile lists in tests. Searched the
tracked tree for other FortHostChannel conformers (none) and for explicit Swift
file lists (two, both updated). Persistence: two owner-only files in the app
container; no ledger, contract or HTTP route change. Auth: listener start still
requires active local owner access; invitation, join and revocation contracts are
unchanged, and the pasted invitation only fills fields.

Must fix (resolved): PT-PHONE-1, the shared-helper dependency that broke the
guardian and sandbox compile lists; PT-PHONE-5, choosing a carrier-grade NAT
address as if it were Tailscale. The Mac now binds only a utun address.

Should fix (open): PT-PHONE-2 listener restart after host restart, PT-PHONE-3
certificate expiry visibility and renewal, PT-PHONE-4 reproducible signing of the
patched Mac package.

Verdict: ready to commit and open a PR for this slice; installed pairing still
needs the owner's actions on the Mac and iPhone.
Not checked: installed listener start, a real phone join over Tailscale, TestFlight
installation, signed packaging suite, native HTTP journeys and UI tests.

```fort-review
{
  "format": 1,
  "reviewedCommit": "e93419bdeca9a2dc495e140c187395b8f9822149",
  "reviewer": "Claude Code (Mac and iPhone build thread)",
  "scope": "Spec 059 October 23 revision: phone access over Tailscale (secure listener private-range rule, explicit network composition in the bundled host, Swift network-control types and XPC operation, Mac Phone access settings, invitation link and phone paste) plus the TestFlight iPhone signing identity. Whole branch range from 3942516. Single owner; no provider contact.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Red first: Tailscale-range bind test failed (0 of 2 binds reached the listener), then passed; secure-network suite 18 pass, 3 optional budget skips.",
    "Red first: CompanionInvitationLink tests and the paste-invitation model test failed to compile before implementation, then passed.",
    "After PT-PHONE-5: phone access tests 3 pass, including tunnel-only address selection; rebuilt FortMac re-signed, codesign --verify --deep --strict passes and the app relaunches.",
    "Strict typecheck passes; portable suite 1442 tests, 1426 pass, 16 skipped, 0 fail (Node 26.5.0, macOS).",
    "macOS suite with synthetic guardian and fresh host resources: 82 pass, 0 fail.",
    "Native FortKit suite --no-parallel with CI fixture variables: 375 pass, 16 optional skips.",
    "Release builds of FortiOS (archived and uploaded), FortMac and FortHost succeed; patched signed Mac app passes codesign --verify --deep --strict and launches.",
    "Not run: signed packaging suite, native HTTP journey checks, UI tests, installed listener start, real pairing over Tailscale, TestFlight installation."
  ],
  "exceptions": [],
  "findings": [
    {
      "id": "PT-PHONE-1",
      "detail": "MacNetworkControl.swift first reused validCompanionIdentifier from CompanionSetup.swift, which broke the guardian and sandbox-feasibility minimal compile lists; it now has its own identifier check and both lists include it.",
      "disposition": "fixed"
    },
    {
      "id": "PT-PHONE-5",
      "detail": "Automated security review: any 100.64.0.0/10 address was treated as Tailscale, but carrier-grade NAT uses the same range on Wi-Fi, Ethernet or cellular, which would admit other subscribers as peers. The Mac now picks such an address only on a utun tunnel interface (test covers en0 and pdp_ip0 refusals). The host still accepts the range only for a bind the owner app selects; TLS and bearer authentication remain required.",
      "disposition": "fixed"
    },
    {
      "id": "PT-PHONE-2",
      "detail": "The listener does not survive a host restart (fresh controller session); the owner restarts it from Settings. Automatic restart needs a persisted owner intent and its own review.",
      "disposition": "open"
    },
    {
      "id": "PT-PHONE-3",
      "detail": "Tailscale certificates expire after about 90 days and are re-imported manually; the expiry is not shown in Settings yet.",
      "disposition": "open"
    },
    {
      "id": "PT-PHONE-4",
      "detail": "The signed Mac app was produced by patching the installed 310b06a9 bundle rather than through mac-application.ts plus a scripted signing step; a reproducible signing script remains a release gap.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "98082547aa43b65fbc8192f86d45cf2ab590a7e7c486b396b40f4006e66da852",
    "successor/V1-ACCEPTANCE.md": "19c601c3e4d87f83a0ff8c94ea899f95454e84ddb25bab5f5cd8ff5f54f52d6e",
    "successor/apple/FortKit/Sources/FortKit/CompanionInvitationLink.swift": "b97648776ed4eb0d7598a0b1f55dabfe60492fa71bdb9c8aa47f26ba6c3abe02",
    "successor/apple/FortKit/Sources/FortKit/CompanionSetupView.swift": "3938f924d476793d6349637d623c7296f2f5e401dfb008f3e543ff8eed4614f1",
    "successor/apple/FortKit/Sources/FortKit/MacAgentService.swift": "1c73901f98ec07fbab1c901b9ec861b5a7cdb15cceb11697fd70df91369dd740",
    "successor/apple/FortKit/Sources/FortKit/MacCoreHost.swift": "4b612ad9315619956f52dd12574d15dfe1448327150947ccea031143365d88ca",
    "successor/apple/FortKit/Sources/FortKit/MacCoreProcessTransport.swift": "f3e241c97b86bb38ef0b359c69bf5a5b1517c601c830c66eb08605922febaaef",
    "successor/apple/FortKit/Sources/FortKit/MacHostChannel.swift": "8c816ac3ba08d9b3cea7930904b5d9d936f34dec1ef6e1c44ce54b4f9334d485",
    "successor/apple/FortKit/Sources/FortKit/MacNetworkControl.swift": "bf7e12db17050792ff78885d38612ac29765666acd977ec17533f2532cd7f247",
    "successor/apple/FortKit/Sources/FortKit/MacPhoneAccess.swift": "5c90ae8f6059d15f205e935749b7670bc1f4541cd028c564e786830778ce0e3b",
    "successor/apple/FortKit/Sources/FortKit/MacXPCTransport.swift": "bf084d9652fd3bb620ab3a4698ca3021a4c7a0ef770423d3534e53b71900658f",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "c913ae817df89cfc230d7937021593425bd4e4f9bc7348c794079964902c7435",
    "successor/apple/FortKit/Sources/FortKit/PairingSettings.swift": "e28dc52ccfffbdc86f124432940eae1ac3cc6e2127abca1938d5fed918269a7b",
    "successor/apple/FortKit/Sources/FortKit/PhoneAccessView.swift": "78c2e59cfccd5b14293be9f2a7e98b4356b8a82210731b56fb78d666a349da14",
    "successor/apple/FortKit/Tests/FortKitTests/CompanionInvitationLinkTests.swift": "03522f09d525b78671ec8fb520789ca3671d066e837a83a5af29cb111d2c6305",
    "successor/apple/FortKit/Tests/FortKitTests/CompanionSetupModelTests.swift": "8c1ceefd666f5ffc61bddbe7a4b0fb030e5644eba443d839516697f12063332c",
    "successor/apple/FortKit/Tests/FortKitTests/MacNetworkControlTests.swift": "4541a61dbb2a225d7a24b6e317237526b2b056a12765da0a45d9d7a41bca112e",
    "successor/apple/FortKit/Tests/FortKitTests/MacPhoneAccessTests.swift": "402b0dff1e4987c0136fea6c119a372b6fab93df44c53159513a00ee4617b86d",
    "successor/apple/FortSpec058.xcodeproj/project.pbxproj": "fac579a4115acd7537a1651c361471a41fc04182334218e1d61f9bea8f2e39aa",
    "successor/apple/project.yml": "eef8ad95859d0e311050308321b37e2a401e782003eba7323359c4fba0c5e086",
    "successor/evidence/2026-10-17-node-foundation/phone-access-tailscale.md": "4d11209d3a59b118da8a7514d947a875b6ae909d14e7569996b4d32c41d4e63e",
    "successor/host/guardian.test.ts": "b573893e589d8fae5ad105849282bf81981d9cda05b5a76e09a5eefb0e428630",
    "successor/host/main.mjs": "61d12fef60e0a6882c9bfc0c7a4d4fe670d9dc46622dfc6aa28f303719134565",
    "successor/host/secure-network.test.ts": "8289f09d20e5a3ba038fe07ad887077b51ee6ad21a9da44448b47e81c8165b7b",
    "successor/host/secure-network.ts": "b5b321f9d31ba348bd54aeb18217e05de50436c4a2b2f87369fd933e27e8943e",
    "successor/packaging/sandbox-feasibility.ts": "6d1fb890734f6767f7cb978bb1a76e9ef24020e3a34edce230f14eb97927fecb"
  }
}
```
