What this repo does: Fort is a single-owner native messenger with a Node core and
optional official runtime adapters. This review covers the first fixture-only
slice of the OpenAI plan-usage route from the Spec 059 October 22 revision: three
new runner modules and their tests, assuming one owner.

Reviewed the six new files against that spec section, the provider documentation
it cites (sign-in, models and inference, token reference, errors and recovery,
preview limitations) and the existing runner conventions in codex.ts, codex-chat.ts
and chat-runtime.ts. Searched the tracked tree: nothing imports the new modules
yet, so no persistence, service, HTTP or native boundary changes in this slice.

Must fix (resolved): the stream codec first folded
`subscription_sharing_usage_unavailable` into the usage-limit failure. The provider
documents it as temporary and retryable, so it now has its own reason and copy.
Refresh error handling was simplified to one explicit terminal-code condition.

Should fix (open): the modules are not composed. The provider literal, sign-in
challenge contract, executor wiring, native sign-in view and credential removal
on Mac uninstall remain for later slices. The JWKS location is injected because
the pages read do not name it; composition must source OpenAI's published keys.
The file store checks mode, owner, link count and symlinks but cannot read macOS
extended ACLs from Node; native removal keeps that check.

Verdict: ready to commit as an uncomposed, fixture-tested slice.
Not checked: any real OpenAI sign-in or inference, the real token endpoint's
refresh request shape (assumed standard OAuth form fields), native UI, signed Mac
packaging, TestFlight and macOS-only suites. Five existing portable tests fail only
in this root-user container, matching the record in PR #20; they pass in remote CI.

```fort-review
{
  "format": 1,
  "reviewedCommit": "e88e65b898f4128ddd00519cc5f7fb9fb15d9441",
  "reviewer": "Claude Code (orchestration architecture thread)",
  "scope": "Spec 059 October 22 revision, fixture-only slice: successor/runner/openai-responses.ts, openai-plan-credential.ts, openai-plan-account.ts and their tests. Uncomposed; no contract, service, native or packaging change. Single owner; no provider contact.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Red first: each new test file failed on the missing module, then passed: openai-responses 9, openai-plan-credential 4, openai-plan-account 5.",
    "Strict typecheck (tsc -p successor/tsconfig.json) passes under Node 26.5.0.",
    "Portable suite under Node 26.5.0 in a Linux root container: 1441 tests, 1421 pass, 15 skipped, 5 fail in owner-demo-startup and synthetic-process timing/evidence tests that this slice does not touch and that pass in remote CI (recorded in PR #20).",
    "Not run: macOS native suites, Swift builds, signed packaging, any real provider sign-in or inference."
  ],
  "exceptions": [],
  "findings": [
    { "id": "PT-OAI-4", "detail": "Usage-unavailable (503, retryable) was conflated with usage-limit; now distinct.", "disposition": "fixed" },
    { "id": "PT-OAI-2", "detail": "CLAUDE.md, successor/AGENTS.md and team notes still describe subscription use as CLI-only; update with the composition slice.", "disposition": "open" },
    { "id": "PT-OAI-3", "detail": "Mac removal must delete openai-plan.json before any real credential exists; native paths need a later revision.", "disposition": "open" },
    { "id": "PT-OAI-5", "detail": "OpenAI JWKS location and refresh form fields are not named in the documentation read; composition must confirm both at the first attended sign-in.", "disposition": "open" },
    { "id": "PT-OAI-6", "detail": "Node cannot inspect macOS extended ACLs; the file store relies on mode/owner/link/no-follow checks and native removal keeps the ACL check.", "disposition": "open" }
  ],
  "files": {
    "successor/runner/openai-responses.ts": "a2f177e1e0b0e060566c96d8190fdf13c361dbdf524acd78a4292322ca073401",
    "successor/runner/openai-responses.test.ts": "57136bafe49654cb52bd3038dd61f8b487457dc900554adb1112ad95ffbc6fb0",
    "successor/runner/openai-plan-credential.ts": "9de228ea946562bc35f98378fef9f1db4faa515d7bf7edd53f7d75e925e61ec0",
    "successor/runner/openai-plan-credential.test.ts": "ae4b4be91a609d05d4b297c5138b23f4f27ba8fa1279e44ae0bb51e3b997c29a",
    "successor/runner/openai-plan-account.ts": "e7ba46d0d4ba3936151ccf8ba512e029e466fadb8b10a6869eef973a3dbcc532",
    "successor/runner/openai-plan-account.test.ts": "8099886b829466923da9165efc51980a0751e978af99bcf3393be7615510294e"
  }
}
```
