What this repo does: Fort is a single-owner native messenger with a modular Node
core and optional runtime integrations. This review assumes one Mac and one phone,
with explicit local listener activation and no shared hosting.

Reviewer: Codex primary implementation agent, October 19, 2026. Base:
`fc32c964fd817eccbdf8e0537b744f8af59e178b`. This is a scoped semantic review,
not an independent-reviewer claim. The whole-repository baseline is preserved.
Applied the pinned Ponytail skill as text only.

Read the controller, contract, complete low-level transport and affected tests;
reviewed emitted export/declaration requirements, the private channel and owned-core
callers, service owner-control transitions, spec, ledger and evidence. Traced
session/generation replay, asynchronous bind/stop/close races, changed ownership,
authority loss during bind, mutation isolation, malformed input, private-socket
identity and missing cleanup acknowledgment. No code was declared unused. The
controller is exported but ordinary host composition remains absent in this slice.

## Must fix before v1 acceptance

1. **Actual owner and Mac controls still need composition.**
   - What this is: An optional listener controller with injected authorization.
   - Problem: Ordinary Mac operation has no private control transport or UI, and
     the controller alone does not close on later service-owner revocation.
   - Fix: Bind control to the current primary owner through the signed private
     host, close on authority loss/host shutdown, then compose Mac controls.
   - If we skip it: Two-device setup is unavailable and an integration could
     incorrectly retain a listener after its owner grant ends.
   - Disposition: Open in the v1 ledger; no production control is enabled here.

2. **Installed and Apple delivery proof remains incomplete.**
   - What this is: Source, compiled artifact and disposable transport evidence.
   - Problem: It does not prove production TLS trust, permissions, provider
     operation, physical clients or TestFlight. Apple currently authenticates an
     Admin, with Account Holder selection and an updated agreement unresolved.
   - Fix: Continue code and exact package preparation; complete each authorized
     installed/Apple gate without inventing access or accepting the agreement.
   - If we skip it: Engineering evidence could be mistaken for delivered v1.
   - Disposition: Open; no key/certificate, credential, trust or startup mutation.

## Fixed during this slice

3. **Failed opening did not distinguish confirmed cleanup.**
   - What this is: A rejected optional listener opening.
   - Problem: A plain error could not tell the controller whether replacement was
     safe; treating every failure identically either blocked recovery or hid risk.
   - Fix: Return a typed, redacted cleanup outcome. Preserve unconfirmed shutdown
     as a blocked state and test both actual bind failure and missing acknowledgment.
   - If we skip it: Explicit listener recovery cannot safely choose a new attempt.
   - Disposition: Fixed; confirmed opening failure/cancellation, different-owner
     adoption and authorization lost during opening have public regression tests.

Verdict: The optional controller is bounded and inert until explicit use. Continue
with actual owner/private-host composition; this is not installed-product signoff.

Not checked: Swift/native UI/builds, guarded Mac/signed suites, actual provider,
production LAN/TLS/trust, startup activation, physical acceptance, archive, upload,
Apple processing, TestFlight, remote CI and provider cleanup. No whole service or
provider re-audit is claimed. Full portable regressions, strict TS, isolated budget
and exact compiled inventory passed. See [evidence](../../../successor/evidence/2026-10-17-node-foundation/network-control.md).

```fort-review
{
  "format": 1,
  "reviewedCommit": "fc32c964fd817eccbdf8e0537b744f8af59e178b",
  "reviewer": "Codex primary implementation agent",
  "scope": "Optional explicit listener control, cleanup classification, authority and replay boundaries, compiled artifact and evidence; no ordinary host activation",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Observed public controller and compiled export reds; 12 controller behaviors and three compiled/cleanup cases passed.",
    "Full portable: 1292 passed, 14 explicit skips, zero failed; strict TypeScript passed.",
    "Twenty isolated journeys after five warmups: p95 3.841 ms under 100 ms.",
    "All 46 payload hashes/sizes in fresh 47-file core verified; inert import requires no network permission."
  ],
  "exceptions": [
    "No native, guarded Mac, signed, production provider/LAN/trust, physical, Apple or TestFlight acceptance was run."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Actual primary owner, authority-loss cleanup, private host and Mac UI composition remain.",
      "disposition": "open"
    },
    {
      "id": "2",
      "detail": "Installed/provider/Apple/TestFlight acceptance remains open.",
      "disposition": "open"
    },
    {
      "id": "3",
      "detail": "Opening errors now distinguish confirmed from unconfirmed cleanup; authority and lifecycle regressions fixed.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "7e1c68234575c9170aeaf6b1593a9fdba98a1496a777cfd1074c3f46da325188",
    "successor/contracts/secure-network.ts": "76cb7c0c9c573dc254321decbcc2beb88124ce0d758ff1a4049b7dd8b61c29a4",
    "successor/host/network-control.ts": "84bbe28847c95884a11b555474d022f548e8f5dc1446365bec235214e127dec7",
    "successor/host/network-control.test.ts": "fec8572ab5e64ce5533a1efec36b97f38a278703647f12f63914ce6596f11045",
    "successor/host/secure-network.ts": "90dede0180288deba3f11b137bb497beed83f72163823e2063cee9ed067578ed",
    "successor/host/secure-network.test.ts": "fcafc7d653d688b594034c667901c9772d847676bce21d225805f36ad8754bdc",
    "successor/packaging/core-artifact.ts": "5186c7c4a2a66c6dbaa96b2b2fba45f63ba640fb3bacfd3428d05ec0bab7647c",
    "successor/packaging/core-artifact.test.ts": "186ab9baafb949d8fe54bd296c1a1b44cb2b56639c9b7dadd59a78261bf77fbd",
    "successor/tsconfig.core.json": "be24095c77591b1d69152ddaeef1c0cddfba38616a7e6a975051c5526c994e9f",
    "successor/V1-ACCEPTANCE.md": "669884b6bef35ba21589f36596b744cf528a3c2afecf97d0b83c54403cc95b64",
    "successor/evidence/2026-10-17-node-foundation/network-control.md": "fd5b84e4accbb583074390fe5a04b482f1af49839d684a5eeb0a3400496c677a"
  }
}
```
