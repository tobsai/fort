What this repo does: Fort is a single-owner native messenger with a modular Node
core and optional runtime integrations. This review assumes one Mac, one optional
phone and bounded private commands; it does not cover shared hosting.

Reviewer: Codex primary implementation agent, October 19, 2026. Base:
`f012d7ffac745b5026924717eb83f9b569a07786`. Scoped semantic review, not an
independent-reviewer claim; whole-repository baseline preserved. Applied the pinned
Ponytail skill as read-only text.

Read the changed controller/contracts, complete owned-core/private-channel lifetimes,
fixture and tests, service transaction/primary pairing/expiry/stop/recovery/close
paths, and artifact import/removal behavior. Reviewed command admission, authority
loss, asynchronous cleanup, secret retention, source-option mutation and compiled
output. No unused-code claim. No new dependency or production default was added.

## Must fix before v1 acceptance

1. **Installed Mac and delivery proof is incomplete.**
   - What this is: Source, fixture and compiled core behavior, with earlier unsigned
     native engineering packages and synthetic UI evidence.
   - Problem: Actual signed/profiled host activation, provider chat, restart and
     reconnect have not passed on the installed product; TestFlight remains open.
   - Fix: Prioritize that exact Mac package and authorized real-provider journey,
     followed by TestFlight and physical acceptance. Keep phone UI after the demo.
   - If we skip it: Passing fixtures would be mistaken for a usable delivered app.
   - Disposition: Open in the v1 ledger. Apple sign-in/G2, provisioning, permissions
     and startup/provider grants retain separate exact authority.

2. **Phone management still lacks native listener UI.**
   - What this is: Optional private Node listener control and existing companion contracts.
   - Problem: The ordinary native client has no wired listener command/invitation UI.
   - Fix: Compose signed native controls after the Mac demo gate, then verify trust,
     explicit activation and real phone behavior.
   - If we skip it: Ordinary two-device setup remains unavailable.
   - Disposition: Open; ordinary bootstrap does not opt into networking.

## Fixed during review

3. **EOF could wait forever for a listener opening.**
   - What this is: Private-channel shutdown originally waited for command workers.
   - Problem: A missing bind acknowledgment left the start worker pending before
     the outer owner could close its listener; the fixture required forced cleanup.
   - Fix: Start controller close before awaiting workers, retaining cleanup errors.
   - If we skip it: Host exit can remain blocked behind its own listener worker.
   - Disposition: Fixed through a public withheld-acknowledgment regression.

4. **Disabled networking depended on its implementation and mutable selection.**
   - What this is: Optional owned-host network composition.
   - Problem: Static imports broke an ordinary host with transport files removed;
     rereading a mutable options object could enable networking during startup.
   - Fix: Capture selection before awaiting and import only on explicit composition.
   - If we skip it: Disabling/removing the module is ineffective or unstable.
   - Disposition: Both observed failures fixed; compiled removal and mutation tests pass.

Verdict: Private owner composition is verified with synthetic processes and sockets.
Continue installed Mac/provider readiness; no production networking is enabled.

Not checked: Native UI/builds, guarded Mac/signed suites, real provider, production
TLS/trust/network/startup, physical clients, archive/upload/Apple processing,
TestFlight, remote CI and provider cleanup. No full provider/service re-audit is
claimed. Full portable regressions, strict TS, isolated budget and exact core
inventory passed. [Evidence](../../../successor/evidence/2026-10-17-node-foundation/owner-network.md).

```fort-review
{
  "format": 1,
  "reviewedCommit": "f012d7ffac745b5026924717eb83f9b569a07786",
  "reviewer": "Codex primary implementation agent",
  "scope": "Opt-in private owning-host listener composition, primary authority/expiry/recovery, framing/cleanup and compiled removal; synthetic single-owner scope",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Public private-host control, expiry notification and controller reconciliation reds observed and fixed.",
    "1308 portable passed, 15 explicit skips, zero failures; strict TypeScript passed.",
    "EOF stalled opening, optional implementation removal and caller option mutation reproduced then fixed; compiled-only authentication and shutdown passed.",
    "20 isolated cycles after five warmups: p95 7.133 ms under 100 ms.",
    "Fresh core: 46 payload hashes verified, 47 exact files including manifest."
  ],
  "exceptions": [
    "Native/guarded/signed/installed/provider/Apple/TestFlight checks were not repeated or performed for this Node slice."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Installed Mac/provider and TestFlight gates remain open and are the next priority.",
      "disposition": "open"
    },
    {
      "id": "2",
      "detail": "Native phone listener/invitation UI remains after Mac demo priority.",
      "disposition": "open"
    },
    {
      "id": "3",
      "detail": "Private EOF now initiates listener cleanup before waiting for workers.",
      "disposition": "fixed"
    },
    {
      "id": "4",
      "detail": "Captured selection and lazy import preserve inert and removable optional networking.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "ed0775347cbfce2dc8ac7aa9ded32e23d35bf4f09583c632e2dab1d709a7b528",
    "successor/contracts/secure-network.ts": "d5ba6db5dc585fc0036f9c3e79445909744fc6233b486ed6b4072f026259371a",
    "successor/host/network-control.ts": "1c22616b4a9f3345be181fd484389f42ee9088f4b94fda298b9462782bc194ea",
    "successor/host/network-control.test.ts": "5da1e337957eb77ae90e8b0678eadde6db8085083a752e4978adae8ec3322a4b",
    "successor/host/owned-core.ts": "304ccf610e3a0861afac450357666e9b98861b6c302903e4d979eaa32d6897cb",
    "successor/host/owned-core.test.ts": "97649dc15360a04386e8082075507fe4ae08040229028ea33ebb45c055aa4264",
    "successor/host/private-channel.ts": "4a8075cb962fab83b19013de36aa1f5372e56d901309464b1938d6a0e77ce047",
    "successor/host/fixtures/owned-core.mjs": "6ed490c58cfd90ed3e722082aae2e805e1e9dabad4e2783b2138dbad6714b45a",
    "successor/service/openai.ts": "13b7c831ef0ee0369df69e68d420869e411fd148bad412c2567ce47224e6a93d",
    "successor/service/owner-access.test.ts": "f237b99b129b9d9d599d099fba8d1938574832c5f7ea12804bb976600e190e93",
    "successor/V1-ACCEPTANCE.md": "26e5b04630f825460d8e5c1a7888b2c88eed918719f69eadb30961820289d09d",
    "successor/evidence/2026-10-17-node-foundation/owner-network.md": "8ddfa20983efbe029fd3de1e5401814199c3a9a3a3129eb0b0004b3351813265"
  }
}
```
