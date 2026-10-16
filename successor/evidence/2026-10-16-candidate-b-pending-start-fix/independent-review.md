# Candidate B session wiring — independent review

Source: `bbb415d15966013b3227d6eecb6bf85887d76de9`; evidence: `858ddcd`; worktree: `/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`. Scope: the bounded manual prototype and previously identified risks only.

**Decision: one small controller fix is required before the manual test. The admission, restart, bounded enrollment and fail-closed protocol gates otherwise look sound within this scope. Activation remains disabled.**

## Runtime / standards finding

**P2 — Stop cannot cancel a pending start.** In `successor/prototype/manual.ts:41`, SIGINT/SIGTERM aborts only `active`. Line 49 assigns `active` after awaiting `packet.start()`. During preflight the operator's stop therefore closes readline without canceling the pending runtime operation. Startup can continue, return, and print “Preflight passed” before the controller finally aborts it. `CandidatePacket.start()` has no external cancellation input.

Independent reproduction against the actual controller source, with inert filesystem/session/launcher dependencies, is `/tmp/fort-manual-pending-start-abort-repro.mjs`. Re-run on Node 26.5.0 produced:

```json
{"abortsWhilePreflightPending":0,"abortsAfterStartResolves":1,"cancelBetween":1,"printedReadyAfterSIGINT":true}
```

This demonstrates continued **no-turn startup after stop**, not extra model-turn dispatch. The fixture invokes no provider, credential operation or real session.

Smallest fix: own cancellation before awaiting startup; propagate its signal through `CandidatePacket.start()` and preflight; dispose promptly and prohibit listener/enrollment/readiness exposure after cancellation. Add the blocked-preflight SIGINT regression. Preserve the blocked packet/start count rather than allowing another attempt.

## Specification and remaining gates

- **Five samples:** admission runs inside the acceptance transaction and requires literal text, exact model/instructions, stable Agent/binding identity, the correct Home or shared Reviewer recipient, exact shared participants, and the designated skill only on sample three. All previous runs must be completed. Identical retries return the original receipt without another run.
- **Two sessions:** exclusive ownership plus saved state prevents parallel/additional starts. First session admits four samples; second admits only the fifth. The 292-second work cutoff reserves eight seconds for cleanup. Unknown/crashed state cannot authorize another start.
- **Restart:** actual app exit and empty installation-scoped pending state are checked before the second runtime launches; the same installation and four completed receipts are revalidated. Changed pending state permanently blocks the packet. Cleanup is checked before granting handoff.
- **Enrollment:** one code/device expires within ten minutes of the first start, including the gap. Reopening cannot extend it. Failure/final completion revokes server authorization; only verified handoff retains it. Cleanup targets the exact issued `Fort.Spec058.Device` account(s) without reading their values. Refused deletion remains unconfirmed. An uncatchable crash may leave a local item, but authorization expires and uncertain state blocks restart.
- **Preflight:** required effective controls, disabled features, complete bounded pagination and empty MCP inventory are checked before sample dispatch, then rechecked for the thread. Missing fields, unsupported methods, omitted feature names, disabled-server status entries, cyclic pagination or unexpected routing all conservatively stop the test. These are runtime compatibility outcomes, not reasons to weaken controls or invent a replacement provider. The [pinned MCP-status parameters](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/ListMcpServerStatusParams.ts) and [feature-list parameters](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/ExperimentalFeatureListParams.ts) both support thread scope. [Config](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/app-server-protocol/schema/typescript/v2/Config.ts) allows additional fields but does not guarantee every required field will be returned.

## Precise residual startup risk and next action

The official process must already be running before it can answer `config/read`. Under the retained HOME and owner UID it may discover configuration, read its provider store, initialize configured components, or perform startup network/file activity before Fort can reject the configuration. The protocol preflight gates thread/turn requests; it cannot retroactively prevent those startup effects. Process-group cleanup also cannot contain a descendant that escapes that group. Static flags, hashes and an empty eventual inventory do not prove otherwise.

After the pending-start cancellation fix and corresponding pin update, the smallest next action is a fresh decision for the exact bounded test that explicitly includes this same-user startup authority, existing provider-store use, and temporary Fort enrollment/cleanup. If that residual authority is unacceptable, enforce the required boundary before starting. No unrelated redesign is required. The first authorized start should run this same-connection no-turn preflight inside its existing budget; failure stops the packet with no sample, fallback, extra start or relaxed checks. Successful preflight allows only the prescribed attended samples. Real replies, UI behavior and actual cleanup are evidence collected by that test.

## Evidence

- Independently verified **452 source hashes, five app hashes and both canonical manifest hashes**. Apple source remains unchanged from the pinned build; source-to-evidence delta contains evidence only.
- Independently ran **34 focused tests on Node 26.5.0: all passed**. They used temporary loopback services, injected credentials/erasers, synthetic transports and owned Node actors.
- Independently reproduced the pending-start cancellation defect outside the candidate tree.
- Read the stored 154-test and five-turn Swift HTTP logs. TypeScript and Swift HTTP results are owner evidence; no rendered UI or provider acceptance is inferred.
- No official provider invocation, credential access, settings changes, deployment or publication. Owner source unchanged; git status remains `?? .build/`. Review artifacts are under `/tmp` only.
