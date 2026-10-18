What this repo does: Fort is a single-owner native messenger with a durable local
ledger and replaceable runtime. This slice adds an optional secure local-network
boundary and corrects an observed native transcript viewport defect. Review assumes
one owner, eight bounded network connections and no shared hosting.

## Semantic review

Codex self-review, October 18, 2026, base 09a7d6c. Read the pinned Ponytail method
and preserved baseline. Read the final transport, test identity helper, public TLS
regressions, compiled artifact changes, native view/assertion and evidence. Traced
ordinary HTTP authentication/enrollment/revocation, private setup/recovery callbacks,
owned-core startup, native URLSession trust and lifecycle callers. Searched tracked
and new source references: the transport has no ordinary startup caller. Checked
request/response bounds, certificate/name/key validation, CIDR limits, socket identity,
shutdown races, production/test dependency separation and observed UI bounds.
Compared the final diff and independently verified all emitted core files. The
owner's implementation decision covers this preparation; fixtures do not authorize
network activation or establish provider/installed-product acceptance.

## Must fix

1. **Trickling input defeated the request deadline — fixed.**
   - What this is: A pending body must stop within the request's total lifetime.
   - Problem: The controller aborted but the body iterator kept accepting bytes.
   - Fix: Destroy incomplete input on abort; the observed 33-second failure now
     terminates near 30 seconds in the same public TLS test.
   - If skipped: Eight slow clients could retain every request slot.

2. **Canceling bind could leave startup pending forever — fixed.**
   - What this is: Closing during asynchronous socket bind must also finish opening.
   - Problem: The opening promise never settled after an early abort.
   - Fix: Share a lifecycle signal across bind/close and reject pending readiness.
   - If skipped: Native setup or shutdown could hang after cancellation.

3. **A private address could select a partly public subnet — fixed.**
   - What this is: Address and selected prefix together define allowed peers.
   - Problem: 192.168.1.2/15 and 172.16.1.2/11 reached bind.
   - Fix: Require the complete CIDR to remain within the corresponding private block;
     the observed pre-bind regression now refuses both, with a valid positive control.
   - If skipped: A misconfigured private listener could admit a wider peer range.

4. **Saved replies reopened outside the viewport — fixed.**
   - What this is: The native transcript restores a saved conversation on launch.
   - Problem: The reply existed but had zero visible height after cold restart.
   - Fix: Use one initial native anchor, preserving earlier-history mode. Exact AX
     bounds now show the entire same reply; screenshots and another send agree.
   - If skipped: The application appears to have lost a saved reply.

5. **The compiled transport was absent — fixed.**
   - What this is: Consumers need a removable explicit compiled module entry.
   - Problem: The first packaging regression found neither its JS nor declaration.
   - Fix: Add the named export/build entry; verify inert import without network
     permission and both independent fixture runtime contracts.
   - If skipped: Source tests would pass while the distributed entry was unavailable.

6. **Phone integration and external acceptance remain open.**
   - What this is: The secure boundary is prepared but ordinary startup does not compose it.
   - Problem: Current owner pairing permits one device, and Apple access is at sign-in.
   - Fix: Continue explicit companion pairing/lifecycle and verify exact existing
     Apple records after owner sign-in; obtain required per-action authority for
     actual TLS trust/network/startup and provider activation.
   - If skipped: Synthetic source/build/UI results could be mistaken for delivered v1.

Verdict: The bounded source slice and its observed regressions pass. The module
remains disabled by default; a usable two-device and TestFlight product is unfinished.
No new dependency or runtime fallback was added. No existing approval was replayed.
Not checked: physical LAN/subnet enforcement, production TLS trust/provisioning,
real helper/App Group/Keychain/startup, Apple records, real provider, physical iPhone,
TestFlight, remote CI or branch protection. The outside-subnet source-address test
is explicitly skipped on this Mac. See the linked acceptance ledger for all v1 gates.

```fort-review
{
  "format": 1,
  "reviewedCommit": "09a7d6c51be50d4a9a51afe0871ecb581aba393b",
  "reviewer": "Codex",
  "scope": "Optional private-subnet HTTPS boundary, compiled export, exact synthetic native transcript correction and Apple read-only access evidence; one owner, no enabled LAN or provider.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Final portable suite: 1170 passed, 8 explicit skips, zero failures/cancellations, 35.607 seconds.",
    "Strict TypeScript passed; 36-file core manifest independently verified.",
    "Observed red-to-green public TLS enrollment, deadline, bind cancellation, subnet and packaging regressions.",
    "Twenty isolated warm 1 KiB TLS journeys: p95 2.739 ms; idle close 0.388 ms; active close below one second.",
    "Exact ad-hoc sandboxed QA app: AX visible fraction 0 before and 1 after cold restart; scoped screenshots and new synthetic send/reply/settings inspected; final process count zero.",
    "Fresh QA and unsigned ordinary Mac/iPhone Simulator Debug builds passed."
  ],
  "exceptions": [
    "Outside-subnet peer test skipped: macOS has no 127.0.0.2 source address; interface settings were not changed.",
    "Six pre-existing isolated budgets were not rerun; TLS budget excluded from the full suite and passed separately.",
    "Full native unit suite and physical iOS execution were not rerun for this presentation-only change; exact running QA and both platform builds were checked.",
    "No production TLS identity, trust installation, network/startup activation, credential, Apple agreement, provider, TestFlight or remote CI action."
  ],
  "findings": [
    {
      "id": "tls-deadline",
      "detail": "Trickling input exceeded the total request deadline; abort now destroys incomplete input and the public timed regression passes.",
      "disposition": "fixed"
    },
    {
      "id": "tls-bind-cancel",
      "detail": "Abort during bind left opening unresolved; one lifecycle signal now settles bind and leaves its exact port reusable.",
      "disposition": "fixed"
    },
    {
      "id": "tls-private-subnet",
      "detail": "Overly broad private CIDRs reached bind and could include public peers; validation now confines the selected subnet to RFC 1918 before bind.",
      "disposition": "fixed"
    },
    {
      "id": "tls-artifact",
      "detail": "Compiled output omitted the optional transport; explicit export/build entry now passes inert import and independent runtime contracts.",
      "disposition": "fixed"
    },
    {
      "id": "native-initial-viewport",
      "detail": "Cold restart restored the reply outside the visible transcript; one native initial scroll anchor passes exact AX bounds and scoped screenshots.",
      "disposition": "fixed"
    },
    {
      "id": "phone-lifecycle",
      "detail": "Transport remains uncomposed and the owner policy permits one device; explicit phone pairing, production identity/trust and native lifecycle are required before a usable two-device prototype.",
      "disposition": "open"
    },
    {
      "id": "apple-installed-acceptance",
      "detail": "Existing Apple access stopped at sign-in; actual team/client/helper/group records, physical provider/UI and TestFlight acceptance remain unverified.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "e7abd06400fa809b04bd4e2885191c645f6e6db08ce9d796fe444dff6b9c3145",
    "successor/V1-ACCEPTANCE.md": "f94c92d48385a9bdb8d4e29a9d1376f750246990b64d9af89c9c6471d8d26e10",
    "successor/apple/FortKit/Sources/FortKit/MessengerView.swift": "d1ce56dfcaae342a2bb9f66f2841cc97d14fc6de05ff60f4c81f2fed2f5c5947",
    "successor/apple/check-fixture-viewport.swift": "c38dc1c8dc31ee18a010d8882a9b313e98b6afa301aa9a6147ec7d0a2c4696f7",
    "successor/evidence/2026-10-17-node-foundation/apple-access-ui.md": "9119d8ab574f51fd6d520f6eb410f89a15fbedd9996c75ea5d3577df0ac41de8",
    "successor/evidence/2026-10-17-node-foundation/qa-settings.png": "bd6bca8150a501e6d2889185f19667d6802d019ca2a6c2c79ec2ae9dc6b05319",
    "successor/evidence/2026-10-17-node-foundation/qa-transcript-after.png": "27c3a4c3260e1087d2123ecd23b674b8b2fbff490204cf9c6c9c13881e623ed5",
    "successor/evidence/2026-10-17-node-foundation/qa-transcript-before.png": "b2deb6e5d599a3a279e1b7da51fba64629858c4045403e9fcd73ff969547c8be",
    "successor/evidence/2026-10-17-node-foundation/secure-network.md": "649773fca6cece9982a2093b259e43a950acd84c1b851d963d7342b278d7ffa9",
    "successor/host/network-test-identity.ts": "5d1082a44320c841c5849ad2272bcc61cff366ba8e9b953e172c86899007db6c",
    "successor/host/secure-network.test.ts": "317033958a0461d6a6d3c29d00512df3f6000650bc2393ea0c461f17f6dbb75c",
    "successor/host/secure-network.ts": "898f2713d5d1dc06eda3b14c3e5fc0c4c9a82ad971d3ff59a67b3bb4b5aec59e",
    "successor/packaging/core-artifact.test.ts": "d2b1a4f077af296bf4ee23a706880f6f5ef105d62d108ab01b957262b63bc19c",
    "successor/packaging/core-artifact.ts": "30679ad3c7200cd7a6440d838f5ff17cac8de622957a4e7a7b0c488a5b36978b",
    "successor/tsconfig.core.json": "00a33f1f68446fe1f8b1d252ca6f67372c8d1a3754f9675be89fbe179f46dc91"
  }
}
```
