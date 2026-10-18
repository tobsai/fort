What this repo does: Fort is a single-owner native messenger with a durable core
and optional runtimes. This slice makes shared native controls respect one primary
Mac and one companion phone. Review assumes one owner and bounded local state.

## Semantic review

Codex self-review, October 18, 2026, base 0d75bf1. Used the pinned Ponytail method and
preserved baseline. Reviewed final role/receipt contracts, client requests, saved
intent changes, model restoration/disconnect/retry, settings and runtime controls,
saved-history preconditions and every new synthetic test. Traced existing primary
setup/removal and exact private Mac recovery callers. Compared service leave rules
and native validation; examined every awaited removal response and its later local
mutation. Reviewed initial failures, full native results and both unsigned builds.
Fresh core/host payloads and native app inventories were independently verified.
The owner decision covers implementation; it does not turn fixtures into grants.

## Must fix

1. **Phone disconnect used the host shutdown operation — fixed.**
   - What this is: Native settings previously treated every owner pairing as the Mac.
   - Problem: The client dropped companion role and sent whole-host disconnect, then
     claimed shutdown. A phone could also present host stop/resume/restart controls.
   - Fix: Retain typed role, require validated status for capable installations, hide
     host actions and validate the distinct exact companion leave receipt.
   - If skipped: Native controls would misrepresent authority and removal effects.

2. **Lost phone replies lacked a persistent recovery scope — fixed.**
   - What this is: A leave operation can commit before its response arrives.
   - Problem: Relaunch had only the old host-removal interpretation.
   - Fix: Save companion scope before sending, preserve it through confirmation or
     local forgetting, and retry only exact phone leave. Legacy records stay compatible.
   - If skipped: Recovery could invoke unrelated host shutdown or claim unverified revocation.

3. **A late reply overwrote a changed removal intent — fixed.**
   - What this is: Local records may change while a transport request is pending.
   - Problem: The observed intent-change test removed the credential after a stale reply.
   - Fix: Recheck the captured intent and exact connection before local mutation;
     changed-connection and changed-intent regressions both pass.
   - If skipped: Another local decision could be silently replaced by the earlier request.

4. **Confirmed phone cleanup could not delete its saved copy — fixed.**
   - What this is: Saved-history deletion requires verified disconnection and absent credentials.
   - Problem: Its old whole-record equality check rejected the new companion scope.
   - Fix: Require exact connection, confirmed state and absence while accepting either
     valid scope. The observed failure now passes and recovery records remain intact.
   - If skipped: A disconnected phone could not perform its separately confirmed local cleanup.

5. **Initial native pairing and physical acceptance remain open.**
   - What this is: These controls operate on an already saved pairing; preparation is separate.
   - Problem: Native prepared invitation/token setup, expired-phone restoration and
     explicit secure-listener/trust lifecycle are not composed. Apple access awaits sign-in.
   - Fix: Continue those native seams and exact installed UI/provider/TestFlight checks.
   - If skipped: Source and synthetic execution would be mistaken for a usable delivered product.

Verdict: Source, synthetic lifecycle, regression and unsigned-build checks pass.
The current settings have no installed accessibility/screenshot signoff. V1 remains
unfinished. No dependency, real credential, network listener or startup grant was added.
Not checked: actual phone/Keychain, HTTPS/LAN trust, installed changed controls,
App Group/startup, real provider, Apple records, TestFlight or remote CI. The new
model fixtures execute on Mac using an injected transport; iOS proof is compilation.

```fort-review
{
  "format": 1,
  "reviewedCommit": "0d75bf19a0d0706e4f6b678b0ead5530ad7af6f4",
  "reviewer": "Codex",
  "scope": "Native companion authority, exact leave receipt, saved scope, relaunch and late-response recovery, separately confirmed saved-history cleanup and settings composition. Single owner, synthetic execution only.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Observed native role, wrong whole-host route, scoped saved-history and changed-intent failures now pass.",
    "Focused final: 11 functions passed with parameterized receipt, role and state-change cases plus existing host regressions.",
    "Full native: 269 functions passed and 11 optional skips, zero failures; 52.135 seconds.",
    "20 isolated synthetic lost-reply/relaunch/retry samples: p95 2.112 ms against 100 ms budget.",
    "Fresh unsigned Mac and iPhone Simulator Debug builds passed; exact 7-file inventories retained.",
    "Fresh 98-file host resource package independently verified; compiled synthetic host journeys ran.",
    "Current changed UI installation/accessibility/screenshots, physical iOS, real credentials/provider, LAN, Apple and TestFlight remain unrun."
  ],
  "exceptions": [
    "Model fixtures run on Mac through injected transport and memory-only credentials. Physical phone and HTTPS trust are not simulated acceptance.",
    "Unaffected optional timing and signed/review-package probes were excluded; the affected budget ran separately.",
    "Unchanged TypeScript/source core retains the service commit portable and strict-typecheck evidence; no redundant rerun claimed."
  ],
  "findings": [
    {
      "id": "phone-host-operation",
      "detail": "Companion role preserved; host operations excluded and distinct leave receipt validated.",
      "disposition": "fixed"
    },
    {
      "id": "phone-recovery-scope",
      "detail": "Companion intent survives lost replies, relaunch, partial cleanup and local forgetting without host recovery.",
      "disposition": "fixed"
    },
    {
      "id": "late-local-state",
      "detail": "Captured intent and exact connection rechecked after awaited replies.",
      "disposition": "fixed"
    },
    {
      "id": "phone-saved-history",
      "detail": "Exact confirmed removal and credential absence permit scoped saved-copy deletion.",
      "disposition": "fixed"
    },
    {
      "id": "native-setup-and-delivery",
      "detail": "Native initial pairing/expired restoration, secure lifecycle and installed/physical/provider/Apple/TestFlight remain open.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "97c6bb87c82a6350b7bed4439cb154173b2ce0a7dbff60e506200646521cb568",
    "successor/V1-ACCEPTANCE.md": "0ddd6a73bd36be262fb3d40255992b683eeaff7355eb758afea218c17ee39e7e",
    "successor/apple/FortKit/Sources/FortKit/DeviceStorage.swift": "2b0ae8b0002fa639537d74580a54856bcb65b3bad1c97435e62c922cf62618ba",
    "successor/apple/FortKit/Sources/FortKit/FortClient.swift": "468166cc2ef22aa1b2a0f73aedfd6578c72710779d2784a352fd0fc65fc30836",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "bc3c13fdc97a182d3ff1810caeb189f08a7d18d38839dd08e1a2bc662d5fbc5d",
    "successor/apple/FortKit/Sources/FortKit/MessengerView.swift": "846fc505a1e1eef0efeeecd7620e1e2843e891ad2c5869d610e67487e203b6e8",
    "successor/apple/FortKit/Sources/FortKit/OwnerLifecycle.swift": "78b4050783db4fee23689cbe9fa8a88f7282fa1bab369ce8d6619779d555ac0f",
    "successor/apple/FortKit/Sources/FortKit/PairingSettings.swift": "3b653d5d432d965cd56994de11731624a69da0262dd95d7360ed43ddefda4df6",
    "successor/apple/FortKit/Sources/FortKit/SavedHistoryRemoval.swift": "9f9ca40bbb3a4436822ae3f5d2e30a4ccd9d880b165a80ead0f503a805b087d7",
    "successor/apple/FortKit/Tests/FortKitTests/CompanionLifecycleTests.swift": "25c5d877000968161f22af430b740f9b05bcc43bf69a1fae626643ef1bbb8869",
    "successor/evidence/2026-10-17-node-foundation/native-companion-removal.md": "9f37d6e02831fd47843a663adbd5dc169874981230291b2bdbdd2930ceee8822"
  }
}
```
