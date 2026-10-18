What this repo does: Fort is a single-owner native messenger with a durable Node
core and optional runtimes. This checkpoint evaluates the shortest Mac test path
without treating fixture execution as provider or TestFlight acceptance.

## Semantic review

Codex self-review, October 18, 2026, source 1b08c224. Applied the pinned Ponytail
method and retained its whole-repository baseline. Reviewed all three documentation
diffs against the actual app/resource inputs, assembly report and independent
inventory. Read the packager and exact native refusal test, traced ordinary app
construction through MacHostControl, and checked helper network configuration,
private runtime state and account/read prerequisites. Searched current tracked
host/service/native/runner code for login composition. Compared the fresh Apple
label-only recheck with the earlier UI evidence. No dead-code or fresh whole-tree
security claim is made. No source, permission or activation behavior changed.

## Must fix

1. **A fresh installation cannot sign into its provider yet — open.**
   - What this is: The runtime intentionally uses its own private state directory.
   - Problem: Model discovery requires ChatGPT authentication, but no ordinary
     native path implements the official login operation. An ambient Mac login is
     therefore insufficient; an approval alone cannot fill this code gap.
   - Fix: Implement and test an explicit official account flow before requesting
     its concrete activation. Keep credentials inside the selected runtime.
   - If skipped: A signed app would still fail to reach real-provider chat.

2. **Signed activation and account access are unverified — open.**
   - What this is: Apple identifies and provisions the client and nested helper.
   - Problem: This exact package has no profiles/final signature, Apple remains at
     sign-in, and the helper has no provider networking authority.
   - Fix: Verify existing records after owner sign-in, prepare exact signing inputs,
     and obtain only the missing grants when the concrete action is reviewable.
   - If skipped: Host activation correctly fails; relaxing inspection would hide
     the missing authority rather than establish usable delivery.

## Should fix

3. **Earlier status notes obscured the current test path — fixed.**
   - What this is: Older evidence describes an incomplete runtime payload.
   - Problem: Readers could miss the complete current unsigned assembly, or assume
     that final signing alone was the remaining task.
   - Fix: Record the 190-file artifact and each remaining implementation, Apple,
     provider and TestFlight gate. Preserve earlier checkpoints as historical.
   - If skipped: The user would receive an inaccurate estimate of testing readiness.

Verdict: The exact unsigned package is ready for review, not real-provider testing
or user delivery. Its remaining implementation and external gates are explicit.

The fresh helper build, 21 packaging controls, one exact native inspection and full
input/output hash verification pass. The prior cold-restart UI fix is proven only
in its named QA fixture. Not checked: current package launch, sign-in, service registration,
real-provider behavior, upload or physical/TestFlight acceptance. No new
benchmark is claimed, and no unrelated dirty work or parked phone code is included.

```fort-review
{
  "format": 1,
  "reviewedCommit": "1b08c224a497974f09c39b3af9c1f99c4be39cc0",
  "reviewer": "Codex",
  "scope": "Current Mac package and ordered testing checkpoint; documentation only, no product source change.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Fresh unsigned helper build passed; existing exact-source client/resources reused.",
    "21 packaging checks passed, zero skipped.",
    "One exact native inspection test passed: public unsigned refusal and synthetic signature callback reaches missing-profile guard.",
    "Independent inventory matched 190 output files and 188 input hashes; zero profiles.",
    "Read-only existing App Store Connect window still shows sign-in; no account records verified.",
    "Prior QA cold-restart/accessibility/screenshots/send evidence retained; no current package launch, provider, archive, upload or TestFlight check."
  ],
  "exceptions": [
    "Proportionate review of package/evidence and the traced activation/account path, not a fresh whole-repository audit. Baseline retained.",
    "Unsigned engineering identities remain; no fabricated signatures, profiles or account grants.",
    "Parked phone setup is excluded and retains two known red isolation tests.",
    "No source changes; previous full regression runs are labeled as previous, not rerun."
  ],
  "findings": [
    {
      "id": "MAC-ACCOUNT-FLOW",
      "detail": "Fresh private runtime has no ambient credential adoption, while model discovery requires an existing ChatGPT account and native login is not composed. Implement official explicit account flow before provider demonstration.",
      "disposition": "open"
    },
    {
      "id": "MAC-ACTIVATION",
      "detail": "Unsigned app is correctly refused; exact Apple records/profiles and helper network authority remain unresolved. Existing browser access is at sign-in.",
      "disposition": "open"
    },
    {
      "id": "MAC-EVIDENCE",
      "detail": "Older activation notes omitted the now assembled runtime and described UI inspection as unavailable. Current source/package, UI evidence and actual sign-in blocker are now stated separately.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "successor/APPLE-HOST-ACTIVATION.md": "b8aea42bf3ff769e67e4d42e7f9b3fd0c58a8b52a956a459fd57b5ff9d341be0",
    "successor/V1-ACCEPTANCE.md": "1586c94acdc3ae77186181466e9242732298cdce30ee43aebcd50759d6191b25",
    "successor/evidence/2026-10-17-node-foundation/mac-review-package.md": "b3d0d34d97342c7541ba9ad473030a5e9989bfffbda21719a51f00427c8965d1"
  }
}
```
