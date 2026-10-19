What this repo does: Fort is a single-owner native messenger with a modular Node
core and optional official runtimes. This review traces one owner's account
selection through native controls, durable acceptance, replay and an owned executor.

Reviewer: Codex primary implementation agent, October 19, 2026. Base
`28dfa3b6111ac8a1656e064ac5ce93f75d0bf94a`. Scoped semantic review using the pinned
Ponytail skill and whole-repository baseline; not an independent-reviewer claim.

Read the changed contracts, account service, admission/dequeue callers, protocol
adapter, bounded worker channels, native parsing/actions, fixtures and evidence.
Checked owner/epoch/revision authorization, copy/validation boundaries, durable
selection, prior-work pinning, cancellation, restart and re-pairing. Traced model
and turn checks through the actual guarded synthetic process chain and compiled
host journey. Checked unchanged sandbox/entitlement and package gates. No code is
declared unused and no unrelated abstraction/removal is proposed.

## Must fix before installed acceptance

1. **The exact live account and signed installation remain unverified.**
   - What this is: The implementation is tested with authored account responses,
     disposable storage/processes, fresh unsigned native builds and an inert package.
   - Problem: It does not establish that the owner's real account returns the
     required optional/experimental identity fields, or that the final installed
     app/provider/cleanup journey works. No profiles or final signature exist here.
   - Fix: Resolve the pending secure Apple handoff, exact profiles/signature and
     owner account/workspace/model; review the completed artifact/grants, then run
     authorized installed launch/chat/restart/reconnect and removal acceptance.
   - If we skip it: Fixture and build evidence could be misreported as delivery.
   - Disposition: Open external acceptance gate. No credentials, account lookup,
     agreement acceptance, startup or outbound grant occurred in this slice.

## Fixed findings

2. **A ChatGPT account type could stand in for the selected identity.**
   - What this is: Sign-in previously returned only a kind, and preflight checked
     account type before model/thread creation.
   - Problem: The public protocol tests completed turns with missing or different
     selected account/workspace; the service could not persist or compare a choice.
   - Fix: Carry bounded nonsecret identity through the existing adapter, require
     explicit primary selection, persist it, pin accepted work and compare it before
     dequeue, discovery and provider dispatch/replacement. Native status says what
     is reported versus selected. Missing identity refuses work without fallback.
   - If we skip it: Work and provider acceptance could use a different account.
   - Disposition: Fixed in source and public synthetic tests, including restart,
     stale/foreign/companion refusal, changed workspace, independent provider shape,
     exact executor transport and re-pairing invalidation. Live proof stays finding 1.

3. **Historical fixtures assumed implicit selection.**
   - What this is: Prior runtime and bundled-host journeys proceeded from signed-in
     type directly to model discovery or dispatch.
   - Problem: New refusals exposed a waiting fixture and a native journey with
     three issues. A cancellation race expected the obsolete kind-only result.
   - Fix: Current service/native fixtures select through public controls; retired
     MVP preflight now has an explicit refusal/cleanup assertion. Update the race
     expectation to the exact nonsecret identity. No production bypass was added.
   - If we skip it: Regression suites hang or mistake an intended refusal for a bug.
   - Disposition: Fixed; final portable/native/guarded suites pass. Also fixed the
     intermediate nested selection transaction and the test's held-work assumption.

Verdict: The selected-account source gap is closed using the existing adapter and
operation record. Exact real-account, installed UI and TestFlight acceptance remain
open. There is no live-sign-in prerequisite for this completed source slice.

Not checked: Real credentials/accounts, final Apple profiles/signatures, installed
accessibility/screenshots/provider behavior, remote grant revocation, phone UI,
archive/upload/TestFlight or provider teardown. No global code/dependency removal
is claimed. CoreSimulator remains incompatible; no component upgrade was attempted.
The prior provider-file cleanup and separate host/computer sandbox boundaries remain
unchanged. No pairing expiry or product turn/time limit was introduced.

```fort-review
{
  "format": 1,
  "reviewedCommit": "28dfa3b6111ac8a1656e064ac5ce93f75d0bf94a",
  "reviewer": "Codex primary implementation agent",
  "scope": "Nonsecret selected provider identity through existing account operation, executor, admission/dequeue and native controls; synthetic restart and fresh inert package",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Observed failing public account selection and missing/wrong-account protocol tests before implementation.",
    "Final portable: 1326 passed / 16 skips; strict TypeScript passed; full native: 328 passed / 16 skips, zero issues; guarded executor: 19 passed / 2 skips.",
    "Exact unsigned package refusal: 1 passed. Fresh unsigned Mac/helper/iPhone builds and 200 output / 198 input inventories verified.",
    "Isolated five warmups/twenty selection transactions: p95 2.221708 ms below 5 ms; provider time excluded."
  ],
  "exceptions": [
    "Synthetic accounts only. No real credential file, account read, sign-in, Apple/legal mutation, entitlement change, startup registration or installed/release acceptance.",
    "Required optional/experimental provider identity fields must be available in an authorized real session; absence stays fail-closed."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Exact real account/workspace/model, signed installed UI/provider/removal and TestFlight acceptance remain unrun.",
      "disposition": "open"
    },
    {
      "id": "2",
      "detail": "Explicit owner selection, durable identity pin and fresh runtime comparison now reject missing or changed account evidence; public synthetic proof passes.",
      "disposition": "fixed"
    },
    {
      "id": "3",
      "detail": "Current fixtures now select through public controls; retired packet preflight refuses without a pin; final regression suites pass.",
      "disposition": "fixed"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "4d5ffbd2814dcfe8b58f3cbf249b9d4adbbdfd18e34271a5750db8d94a574102",
    "successor/APPLE-HOST-ACTIVATION.md": "00ddaeee3da64180e28c3f49aaa1f0288e2b36dff4fb83826c58dff8d089f9d0",
    "successor/V1-ACCEPTANCE.md": "772318007ff205c126591cc90640583d6ca835167dc40a99480740977768dc7b",
    "successor/apple/FortKit/Sources/FortKit/AccountSignIn.swift": "efed027806be07cd00c50d5063f8e9b8414637410da1ce14f4da0abe1bff9a9a",
    "successor/apple/FortKit/Sources/FortKit/AccountSignInView.swift": "0ed1c79a3083d36307e1a07a6d12fd6da9e7d1105e3b55a9a0406a4302deddce",
    "successor/apple/FortKit/Sources/FortKit/FortClient.swift": "0308cf5b02afa5bc2e8d642e65a304f302f6fa553e98fa18961e3067013cb829",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "81793604064b6a777fc6ecf2b534b0ed02f49ffbba2e849d704482783947bcdd",
    "successor/apple/FortKit/Tests/FortKitTests/AccountSignInTests.swift": "6d586b4b3c65f573ea72a266c84810b531e69afd95f7ccbad81e328d17ff9955",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostJourneyTests.swift": "5436755300fac7c3215a6e8a848514e83eff377233aab026f79a3d1fbdac3d92",
    "successor/contracts/account.ts": "87750b355e716be3b52d842d281e126516c9b6968f6738ac5ab40f05b3790c38",
    "successor/contracts/v1.ts": "dd1c40150a64f8c9fc09752cac6cc9d8affd4c5a04f0101fa03221140807d47f",
    "successor/evidence/2026-10-17-node-foundation/account-correlation.md": "957940baeccf04db0f3513df030926a47663b86a72e19b999491f5dd8761e42d",
    "successor/prototype/mvp-session.test.ts": "d0801fa80017cb8a21f422c711f860c7087a8531c8714264a62ab0c027c85763",
    "successor/runner/chat-runtime.ts": "c8a811e171fd03e305196de657829f0d5edf1da2526de6fe2eac51636cdefe31",
    "successor/runner/codex-account.test.ts": "0d4937c4fa35e28ebbf7d4980296cc00032c26c0269b0e382f98acbaab495697",
    "successor/runner/codex-account.ts": "a82bdbcbe09069d86bf206d1c7cd87eeb60ccd67373725c056c57b3e1bdf664f",
    "successor/runner/codex-chat.test.ts": "a6459cc4eef3ee4b473888c9330d6606260535671852042b688448bd2942484a",
    "successor/runner/codex-chat.ts": "8e89d437c9681b6fd179406d71fd702c1ea594e057f8296e954812fe956b92df",
    "successor/runner/codex-executor.test.ts": "d058669388e75fbc3ff138a38f684ea7817cd4c43c3efe7161301d2e1240d029",
    "successor/runner/codex-preflight-fixture.ts": "96d4338bb5b8a3d17e2cdacd968c5fcc457a52f292abae6fac1d96b8dee1e585",
    "successor/runner/codex-startup-contract.test.ts": "40c19abf3e869ddc87a693d6d5f2b42f753fad124df6032d4e51ab74683e013c",
    "successor/runner/executor-account.test.ts": "5b297c0539255a36e11b207cd009544d75de79ea944889b935a80d1e63b2af62",
    "successor/runner/executor-models.test.ts": "3bccbc4e38aff3b7059005bb5717f0a2327a1fe953d9ee16cccc1152b825dc91",
    "successor/runner/executor-process.ts": "623101aeae47c480348302465f29b2e2bd56e24631e6ff6fd2839feac2f60ee1",
    "successor/runner/executor-wire.ts": "6a4cc290d3eb36a001886f6e52e9b671b58854505d04a87d501dc2f03add61ae",
    "successor/runner/executor-worker.ts": "bf61b83bbe37e09425f4cf4925a616e3314981235775f18a01a9a154cf4ff33f",
    "successor/runner/fixtures/codex-0.157.1-startup.json": "0c6f9f5344670f46d43568453e3cc8ab3e4c12831c809e64107f1f294bea5848",
    "successor/runner/fixtures/codex-executor-runtime.mjs": "b05d12ed071da608f59e3fc0384ec2ec9bbd9816dd25978ed059189bdbef8b06",
    "successor/runner/fixtures/executor-account-worker.mjs": "0842ecdd2f9f0e1487621fdc54b8b662088899ae19932606e40602343ec568dd",
    "successor/runner/owned-process-fixture.mjs": "d3fca277264d5e6623d9c629bf50966a21e50848f3ed27a3dc78a57c70e7ab16",
    "successor/service/account-sign-in.test.ts": "ab7baff67c4dab921f4648daceccbac934532192e2be56e8e47232a1ddfbd83f",
    "successor/service/account-sign-in.ts": "04c6b7bc3c37b92e71cf369ef8e4cadeb800025fc88ff014b6fce6a7d5ab69bf",
    "successor/service/openai.test.ts": "74924c3bd9da508e97e9ebfd4535a0c12855ff8d5156d2bff71f007c2ae57559",
    "successor/service/openai.ts": "7f0dbbe850c62212d3935b9d12b05632169db4f6b47e192dfd68e5d84065e180"
  }
}
```
