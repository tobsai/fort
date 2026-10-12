# Codex protocol preparation — 2026-10-12

**Bounded preparation passed; Stage 2 is not complete.** The service still enables
only fixtures. This slice adds a standalone byte-to-evidence turn adapter, not a
working official-provider integration or a hostile-code sandbox.

Source commit: `f6a968efc62886cce6cc0bc7cfb37ce98f5b2eb6`, based on Stage 1 evidence
tip `57582939d043f405fc16ef4cc30ca8f8bf7f0409`. Branch:
`codex/fort-core-openmaus-20261011`; worktree: `/tmp/fort-core-openmaus-20261011`.
The parent requested safe Stage 2 boundary plumbing with deterministic fixtures
after independent Stage 1 acceptance. This introduces no new external authority.

## Change and trust boundary

The exact code files are `contracts/runtime.ts`, `runner/codex.ts`,
`runner/codex.test.ts` and the test script in `package.json`. The public v1 API,
service, ledger, fixture runner and native client sources are unchanged.

The adapter preserves exact input and immutable attempt metadata, frames bounded
UTF-8, correlates the provider turn, keeps progress separate from final item text,
preserves reported commentary/final-answer phase, and requires terminal evidence.
Tests cover early/foreign/stale frames, interrupted versus merely acknowledged
stop, loss of transport, malformed/oversized output, contradictory terminal/items,
model rerouting and unsupported authority requests. Effective model stays
unreported. There is no process/network/configuration/credential I/O in the adapter.

All server requests fail closed, including tools, permissions and token refresh.
This is not permission enforcement on a running provider: a tool could already
have begun before its event arrives. An interrupt write or close request is not
proof of termination. The provider policy fields do not sandbox the agent host.
The [runner boundary note](../runner/README.md) records the missing OS isolation,
authenticated/fenced transport, handshake, runtime compatibility and lifecycle work.
No owner or runner HTTP route was added. The known donor Slack/service-loopback
authority path remains outside the successor graph.

## Exact checks

Node 26.5.0 and the already pinned TypeScript 6.0.3/@types/node 26.6.5 were used.
No package was installed, no lifecycle hook ran, and no official runtime launched.
The full Node suite uses temporary local loopback fixtures and deterministic child
workers, for which the ordinary host-execution permission was granted.

| Check | Result |
| --- | --- |
| `node --test successor/runner/codex.test.ts` | **21 passed, 0 failed/skipped** |
| `node --test service/service.test.ts runner/fixture.test.ts runner/codex.test.ts` from `successor/` | **67 passed, 0 failed/skipped**, including all prior 46 fixture/service tests |
| `node successor/node_modules/typescript/bin/tsc -p successor/tsconfig.json` | **Passed**, strict/noEmit/skipLibCheck=false; exit 0, no diagnostics |
| `git diff --check` and final path/import inspection | **Passed**; no provider import in service/native/v1; only the named code files changed before the documentation commit |
| Swift, native builds, installed UI | **Unrun in this slice**; sources unchanged, Stage 1 evidence remains separate |
| Official runtime/schema generation, provider sign-in or real turn | **Unrun** |
| OS sandbox/Keychain/IPC/provider credential canaries | **Unrun; blocking live activation** |
| Runner enrollment/auth, durable provider-event fencing and runtime startup/stop/timeout | **Not implemented by this slice** |

All `node` commands above used `/opt/homebrew/opt/node@26/bin/node`. Full commands,
exit codes, source hashes, log hashes and retained outputs are in
[validation.json](codex-protocol-2026-10-12/validation.json).
Red-run logs are stored as deterministic gzip files to retain the raw bytes,
including test-reporter whitespace; the manifest hashes both artifact and output.

Test-first history is retained separately from final checks. The initial red run
failed to import the then-missing adapter (scaffold failure, not behavior proof).
The next red run had 15 failures on the corrected terminal notification shape and
the missing boundary behaviors. After those passed, two of 19 edge tests reproduced
long interrupt-ID rejection and acceptance of contradictory terminal evidence.
The last red run had four of 21 failures because message phase was discarded or
unchecked. Each was followed by passing targeted tests. An intermediate strict
typecheck failed on an overly narrow test-fixture literal; the fixture type was
corrected, and final strict checking passes. No final check remains failed.

## Provenance and open gates

This is new Fort code and hand-authored synthetic fixtures, informed by official
[app-server](https://learn.chatgpt.com/docs/app-server) and
[authentication](https://learn.chatgpt.com/docs/auth) references read on 2026-10-12.
No upstream source or schema was copied. No license/attribution file changed.
The donor remains quarantined at upstream
`8b53c0323dba2099a953ee2d77ab9330cd5acaa7`; its Apache-2.0 attribution remains intact.
Historical Fort source/specs and parent-owned `reviews/` were untouched.

Only symlink metadata was inspected for a potential local runtime:
`/opt/homebrew/bin/codex` points to `/opt/homebrew/Caskroom/codex/0.157.1/bin/codex`.
This does not establish the executable's reported version or compatibility with
the current docs. No runtime was selected for live use, upgraded or substituted.

Live activation still requires a proven enclosing OS/credential boundary, an
authenticated and durably fenced runner channel, exact-version schema conformance,
provider-native subscription/account/model evidence, and the complete native real-
turn/interrupt/disconnect/restart/replay matrix. A Mac-host result would not prove
self-hosted or Claude feasibility. No native, physical-device or release gate is
closed by synthetic protocol tests.

Further deterministic boundary work needs no owner intervention. Before the first
real turn, finish and review the concrete confinement implementation; request only
the precise OS/account/VM grant if that implementation needs one. After canaries
pass, the owner must authorize and complete provider-native sign-in in that exact
execution environment. Do not request sign-in as a substitute for confinement.

No push, remote fork/branch, visibility change, deployment, startup registration,
provider connection, paid call, credential access/copy, persistent permission or
provider cleanup occurred in this slice.
