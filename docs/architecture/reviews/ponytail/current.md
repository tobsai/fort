What this repo does: Fort is a single-owner native messenger with a durable Node
core and optional runtimes. This slice adds explicit owner-only account actions,
assuming one primary Mac and one optional companion.

Codex self-review, October 18, 2026, base c5364af. Read every new module/test,
changed callers and neutral parsers. Traced authorization before and after awaits,
exact retries, persistence, interruption recovery, cancellation, challenge exposure,
shared runtime exclusion and host cleanup. Checked the private channel route rules,
compiled resources, final logs and isolated measurements. The pinned Ponytail
baseline is retained; no fresh whole-tree/dead-code claim is made.

## Must fix

1. **A catalog request could overlap sign-in — fixed.**
   - What this is: Both operations use the optional runtime.
   - Problem: The first service implementation admitted sign-in while an existing catalog awaited completion.
   - Fix: Track service-owned runtime operations and reject sign-in during one; the public HTTP regression passes.
   - If skipped: Runtime exclusivity would depend on an adapter's optional readiness implementation.

2. **Sign-in could still advertise restart — fixed.**
   - What this is: The runtime may report ready while account work is pending.
   - Problem: A public regression observed ready and restart eligibility during sign-in.
   - Fix: Service ownership overrides readiness and disables restart while active.
   - If skipped: Native controls could request conflicting runtime operations.

3. **The native account flow remains incomplete — open.**
   - What this is: Authenticated service actions now support one durable operation.
   - Problem: Native controls do not yet prepare/restore the exact operation or display/cancel its challenge.
   - Fix: Add native durable intent and tested controls against this contract, with no automatic login.
   - If skipped: The service would be packaged but inaccessible through normal Fort setup.

4. **Provider and delivery prerequisites remain unverified — open.**
   - What this is: Actual operation needs exact Apple/provisioning and provider authority.
   - Problem: Account access, profiles, helper network authority, installed UI and TestFlight are still unverified.
   - Fix: Continue source work and obtain only the concrete external grants once reviewable.
   - If skipped: Synthetic evidence would be mistaken for user delivery.

One metadata row bounds recovery; challenge/token data is excluded from persistence
and replay. Stop/disconnect/close await account settlement and runtime disposal.
Neutral parsers are shared by core and executor without importing provider modules.
No dependency or new process launcher was introduced.

Verdict: Primary-owner service behavior is verified synthetically; complete native
controls and external acceptance before claiming usable provider sign-in.
Not checked: Actual accounts, credentials, browser, network/startup grants, signing,
installed native UI, remote CI, physical devices or TestFlight. Review app unchanged.

```fort-review
{
  "format": 1,
  "reviewedCommit": "c5364af283231e7fc41481a7140ab4bd514c0b44",
  "reviewer": "Codex",
  "scope": "Primary-owner account HTTP lifecycle, durable retry and shared runtime exclusion, neutral parser relocation, spec and evidence.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "1275 portable passed, zero failed, 12 documented skips.",
    "16 focused service passed, one budget skip; 35 guarded Mac passed, three opt-in skips. Strict TypeScript passed.",
    "20 serial synthetic HTTP journeys, five warmups: p95 4.682709 ms under 100 ms.",
    "108 resources independently hash/size/membership verified.",
    "Public missing-route, active-catalog overlap and false-ready/restart cases observed red and fixed."
  ],
  "exceptions": [
    "Proportionate review; baseline retained. No dead-code claim.",
    "Synthetic HTTP/protocol only; actual provider, credentials, network/startup grants, installed native and TestFlight unrun.",
    "Neutral parsing moved to contracts; historical service/database names retained. Review app unchanged."
  ],
  "findings": [
    {
      "id": "OWNER-ACCOUNT-CATALOG",
      "detail": "Account start initially overlapped a pending catalog request. Service operation count now excludes the start; public regression passes.",
      "disposition": "fixed"
    },
    {
      "id": "OWNER-ACCOUNT-RESTART",
      "detail": "Runtime readiness could expose restart while sign-in held the service. Service reports busy and refuses restart; public regression passes.",
      "disposition": "fixed"
    },
    {
      "id": "ACCOUNT-NATIVE",
      "detail": "Durable native intent, exact challenge/status/cancel and restored-flow behavior remain to implement.",
      "disposition": "open"
    },
    {
      "id": "ACCOUNT-ACTIVATION",
      "detail": "Exact Apple records/profiles, network and provider/startup authority, installed behavior and TestFlight remain unverified.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "285294a12c0ceb76f0274953020288cacdd8ae5cc70ea342f4c7d213e9a0ea05",
    "successor/contracts/v1.ts": "4e5b69cb27488e5cd305af31fc58ba4a6c5ff57fa6b68791def7fc18e3e8f101",
    "successor/contracts/account.ts": "4e38f4605359c2e785f43ebb691d4f18e2c354357f95075eeb01912d4220afa2",
    "successor/runner/executor-account-wire.ts": "7f72a6b16daf443715820f8ce0681fceb2e45a17c6a41a84b10435d97a3f7c4b",
    "successor/service/openai.ts": "dc5e40e8f728595b2ac392743f19404de5a45853c5a3d1384b6eb7ef0c2fea5c",
    "successor/service/account-sign-in.ts": "a379d4a99dd2b4efb46dd3e57da91a4ecb6836f892fb3145901b2274512dd405",
    "successor/service/account-sign-in.test.ts": "116ea1a12a12a838af9ac11d0b9a708e32b45868680e3e787e8127cfbd14cff7",
    "successor/evidence/2026-10-17-node-foundation/owner-account-sign-in.md": "25035c67d15daf73a1e3737c993ddcd23f78a13626e5798038af892ed76e3f09",
    "successor/V1-ACCEPTANCE.md": "3277ffcfa129c1ece46ae455208ea2b9b583e6f4964da80abd98316c2f875536"
  }
}
```
