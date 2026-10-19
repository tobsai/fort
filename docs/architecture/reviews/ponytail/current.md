What this repo does: Fort is a single-owner native messenger with a modular Node
core and optional runtime integrations. This review assumes one Mac and one phone,
with explicitly selected local transport and no shared hosting.

Reviewer: Codex primary implementation agent, October 19, 2026. Base:
`ddccfeb58dfed0c13b2859c3cd5f3813b770bb70`. This is a scoped semantic review,
not an independent-reviewer claim. The whole-repository baseline is preserved.

Read the complete optional secure transport and its tests, core artifact builder
and declaration/export tests, owned-core/private-channel lifecycle, changed spec,
ledger and evidence. Traced abort during bind, successful bind, caller cancellation,
server error, concurrent explicit close, request teardown and cleanup timeout.
The new result shares the existing stop operation and preserves its error result.
There is no new dependency, controller, authority path, configuration store or restart.
No code was declared unused. Applied the pinned Ponytail skill as text only.

## Must fix before v1 acceptance

1. **Mac companion controls and listener ownership are still uncomposed.**
   - What this is: The native management contracts and optional TLS transport exist.
   - Problem: The ordinary Mac still cannot start and manage an installed phone's
     verified endpoint and invitation through those contracts.
   - Fix: Compose explicit host lifecycle/status and Mac controls, retaining exact
     identity, lost-response handling and required external grants.
   - If we skip it: The owner cannot complete ordinary two-device setup.
   - Disposition: Open in the v1 ledger. This slice provides shutdown observation only.

2. **Installed and Apple delivery evidence is incomplete.**
   - What this is: Local source, compiled artifacts and synthetic transport results.
   - Problem: They do not verify actual TLS/trust, Keychain, permissions, provider
     operation, physical devices or TestFlight. Apple certificate creation is approved
     separately but currently requires the owner's direct Apple sign-in.
   - Fix: Complete the exact authorized installed and Apple gates. Preserve existing
     certificates and keep the signing key local when the G2 flow can proceed.
   - If we skip it: Engineering evidence could be mistaken for a delivered v1.
   - Disposition: Open. Xcode's system-component install prompt remains untouched;
     no new certificate/key was created, installed, exported or revoked.

## Fixed during this slice

3. **Asynchronous transport shutdown was invisible to its caller.**
   - What this is: Abort and server errors already close the optional TLS listener.
   - Problem: The returned handle exposed only URL and close, so a caller that had
     not initiated shutdown could keep presenting the endpoint as available.
   - Fix: Expose one readonly completion promise shared by every close path. Preserve
     pending, confirmed and rejected outcomes; an expired cleanup deadline stays failed.
   - If we skip it: Future Mac status controls can continue to show a stopped endpoint.
   - Disposition: Fixed through public lifetime tests using real disposable sockets,
     exact-port reuse and Node server-boundary failure injection.

Verdict: The optional transport now exposes its bounded shutdown result. Continue
with owning-host and Mac composition; this commit is not installed-product signoff.

Not checked: Native UI/builds and guarded Mac/signed suites were not repeated;
previous native evidence remains revision-bound. No production listener, credential,
trust or startup activation, actual provider/physical acceptance, archive, upload,
Apple processing, TestFlight, remote CI or provider cleanup ran. Full portable tests
and strict TypeScript passed; two affected timing budgets and the fresh compiled
artifact were verified. The source review did not re-audit every service/provider.
See [evidence](../../../successor/evidence/2026-10-17-node-foundation/secure-network-lifecycle.md).

```fort-review
{
  "format": 1,
  "reviewedCommit": "ddccfeb58dfed0c13b2859c3cd5f3813b770bb70",
  "reviewer": "Codex primary implementation agent",
  "scope": "Observable lifetime of the optional secure transport, cancellation/server-error/close semantics, compiled output and evidence; one owner and explicit local fixture listeners",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Observed missing public lifetime failure before implementation; four focused lifecycle cases passed, including missing shutdown acknowledgment.",
    "Full portable suite: 1,278 passed, 13 skips, zero failures; strict TypeScript passed.",
    "20 isolated samples after five warmups: TLS request p95 2.774 ms and open/abort/closed/port-reuse p95 3.718 ms, each below 100 ms.",
    "Fresh compiled core: 42 files and 251,959 bytes independently verified against manifest; emitted public lifetime declaration inspected.",
    "Native/guarded Mac/signed/UI/provider/LAN/physical/TestFlight checks were not rerun for this transport-only slice."
  ],
  "exceptions": [
    "No new ordinary host composition is claimed; the optional entry remains disabled by default.",
    "12 opt-in budgets and one unavailable alternate-loopback-source check skipped in full portable run; both affected budgets ran separately."
  ],
  "findings": [
    {
      "id": "SECURE-TRANSPORT-LIFETIME",
      "detail": "A caller could not observe shutdown triggered by abort or server error. The shared public completion now reports confirmed or unconfirmed shutdown without starting/restarting a listener.",
      "disposition": "fixed"
    },
    {
      "id": "PHONE-MAC-COMPOSITION",
      "detail": "Mac invitation/revocation controls and owning secure listener lifecycle remain uncomposed.",
      "disposition": "open"
    },
    {
      "id": "INSTALLED-APPLE-ACCEPTANCE",
      "detail": "Actual TLS/trust, credential/network/startup/provider, physical and TestFlight gates remain open. Separately approved G2 certificate creation waits for direct owner Apple authentication.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "15d82fb3ee910cd7731f769a23dede211af6d6a3e715c3ca3f43ad8d57ca19f8",
    "successor/V1-ACCEPTANCE.md": "8a9222fa77130b0858b7c0e00486e42f05c7bbce80b585f4a13e6a984fde5bb3",
    "successor/evidence/2026-10-17-node-foundation/secure-network-lifecycle.md": "b93c893b5933b48896a8b48f54b65dcc9182987566c84b06f29f34863dab76ab",
    "successor/host/secure-network.test.ts": "49534431e6014a2dc58420d12cc7b8203ce97f32231d81864a96bd9269bf8173",
    "successor/host/secure-network.ts": "2aef8fea70fe20330629368ad9557a4a1218955e59427f02c204edfd51711bd9"
  }
}
```
