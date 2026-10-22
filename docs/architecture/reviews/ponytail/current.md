What this repo does: Fort is a single-owner native messenger with a Node core and
optional official runtime adapters. This review covers the ChatGPT plan
`ChatRuntime` (runner/openai-plan-chat.ts and its test) and the matching Spec 059
path refinement, assuming one owner.

Reviewed the new runtime against the ChatRuntime contract, CodexChat's attempt,
context and account checks, the executor wire's provider handling and the merged
openai-responses, openai-plan-credential and openai-plan-account modules. Searched
the tree: nothing composes the runtime yet; executor-wire.ts and executor-worker.ts
already treat the provider as a string, so no contract change is needed.

Must fix (resolved): the first test fixture used a default parameter, so the
"missing credential" case silently ran with a saved credential. The fixture now
takes an explicit null and the case is exercised.

Should fix (open): sign-in is not composed; it waits on the owner's choice between
a loopback listener entitlement and a paste-back flow. Multi-agent metadata
(agent and recipient names) is not forwarded as Codex does; history keeps roles
and text only. Native sign-in, Mac removal of the credential file and the JWKS
source remain open from the previous slice.

Verdict: ready to commit as an uncomposed, fixture-tested runtime.
Not checked: real OpenAI requests, the executor worker composition, packaging,
native UI, macOS suites and TestFlight.

```fort-review
{
  "format": 1,
  "reviewedCommit": "39425163e4e216efdd7d1c536ebaa2302720329d",
  "reviewer": "Claude Code (orchestration architecture thread)",
  "scope": "successor/runner/openai-plan-chat.ts and its test, plus the Spec 059 October 22 path refinement and refreshed digest. Uncomposed runtime; no wire, service, native or packaging change. Single owner; no provider contact.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Red first: openai-plan-chat.test.ts failed on the missing module, then 7 of 7 passed.",
    "Strict typecheck passes under Node 26.5.0.",
    "Spec structural and implementation gates pass with the refreshed digest.",
    "Not run here: macOS native suites, real provider requests or sign-in."
  ],
  "exceptions": [],
  "findings": [
    { "id": "PT-OAI-7", "detail": "Test fixture default parameter hid the missing-credential case; fixed with an explicit null.", "disposition": "fixed" },
    { "id": "PT-OAI-8", "detail": "Sign-in composition waits on the loopback entitlement versus paste-back decision.", "disposition": "open" },
    { "id": "PT-OAI-9", "detail": "Agent and recipient names are not forwarded in history items; roles and text only.", "disposition": "open" },
    { "id": "PT-OAI-3", "detail": "Mac removal must delete openai-plan.json before any real credential exists.", "disposition": "open" }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "c6997fdacf49c58c1e518bea37e2fedad4e77d420d36420c916385f8142c9591",
    "successor/runner/openai-plan-chat.ts": "e5a841171e250747c6069e4fa493fb18a5ab3f0872117aede2d8b550d886dcee",
    "successor/runner/openai-plan-chat.test.ts": "31334c987ab9c2b925a693dd889ba6ca0f940c7b663c905d88a92fe79a61408e"
  }
}
```
