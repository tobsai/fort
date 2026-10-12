# Synthetic runner bridge — 2026-10-12

**The bounded synthetic bridge passes; official-provider Stage 2 remains open.**
This is backend fixture integration with actual HTTP/SQLite and synthetic process
crashes, not a real Codex run, production runner enrollment, or OS isolation proof.

| Provenance | Exact value |
| --- | --- |
| Branch | `codex/fort-core-openmaus-20261011` |
| Worktree | `/tmp/fort-core-openmaus-20261011` |
| Accepted protocol base | `43d0ca89798086fa1235217e6a0751271ddc748f` |
| Implementation commit | `18043450e26c3c73bb259fc8ab00ff8fbd89d733` |
| Final test/source tip | `611521e74a9ec28e3c0fc2a60725c462b8c9ef2b` |
| Donor upstream, unchanged | `8b53c0323dba2099a953ee2d77ab9330cd5acaa7` |

The parent reported independent acceptance of the protocol slice and authorized
the next synthetic runner-to-ledger slice recommended in
`reviews/codex-protocol-43d0ca8/REVIEW.md`. Its explicit limits were backend-only
work, deterministic transport/session fixtures, no real runtime or credentials,
no UI, sign-in, persistent permissions, deployment or push. Those limits were
preserved. This record reports execution by the implementing agent; it does not
claim a new independent review of this increment.

## Implemented behavior

Eight exact code files comprise the change: `contracts/runner.ts`,
`service/service.ts`, `service/synthetic-runner.ts`, its test and crash harness,
`runner/synthetic-transport.ts`, its tests, and `package.json`'s test list.
The accepted Codex codec, private runtime evidence types, native/public v1 schemas,
native sources, historical specs, donor and parent-owned `reviews/` are unchanged.

The service now records an immutable binding beside each accepted run. A fresh
ledger may explicitly select synthetic runner mode; it cannot later switch to the
ordinary fixture process path. Local trusted harness enrollment issues synthetic
runner sessions, scoped independently of owner-device credentials. Hashes, not
raw session/dispatch credentials, persist. Actual private HTTP commands validate
installation, recovery epoch, enrolled host/session, expiry/revocation and dispatch
capability inside the transaction after parsing the body. Copied IDs and reply
text cannot grant authority. The fixture remains single-owner and loopback-only.

Claims persist the exact binding, text/digest, attempt/generation, session, thread
and lease before returning work. Evidence must match the dispatch's selected model
and runtime version, the first correlated turn and the next sequence. Canonical
identical replay returns the original committed cursor without new effects;
changed replay, gaps, stale attempts and late terminals are rejected. An actual
synthetic recovery-epoch fixture also proves that copying the new IDs does not
restore an old runner session's authority.

Terminal messages, run/outbox state, ordered events and evidence receipt commit
atomically. Progress and commentary stay in private evidence; final/unreported-
phase text is preserved exactly in the transcript. Effective model remains
unreported. The public binding still says `fixture`, version `1`, model
`fixture-echo-v1`: private Codex-format evidence is not a real-provider claim.

The synthetic lifecycle emits ordered write/evidence/close actions and enforces
5-second handshake, 30-second turn, 2-second interruption and 2-second teardown
deadlines using an injected clock. Tests feed its actions through the real bridge.
An ACK, a force-close instruction, transport loss or process exit-zero never
establishes success or cancellation. Late cancellation cannot extend a deadline.
This is an action state machine; actual blocked stdio writes, process ownership,
OS cleanup and production scheduling remain an enclosing implementation's work.

A 40-second recorded dispatch lease supplies a separate service fence. Expiry
becomes visible and durable at the next authorized owner request, runner claim or
exact status check. Service startup immediately marks still-claimed work unknown.
There is no background daemon or retry loop. Unknown work holds its Conversation
barrier; explicit owner closure releases only later work and never rewrites or
repeats the original turn. Failed runtime terminals retain their private evidence
and conservatively project as unknown in the existing fixture v1 state model;
full production failure projection remains open.

## Executed checks

| Check | Result |
| --- | --- |
| Complete backend suite on the final code | **116 passed, 0 failed, 0 skipped**: previous 67 plus 41 bridge and 8 synthetic transport tests |
| Final focused synthetic transport suite | **8 passed, 0 failed/skipped** |
| TypeScript 6.0.3, strict/noEmit/skipLibCheck=false | **Exit 0**, no diagnostics |
| Staged and fixed-base `git diff --check` | **Exit 0** |
| Unchanged-scope check against `43d0ca8` | **Exit 0** for donor, specs, native source, public schemas, accepted Codex codec and runtime contract |

Commands used `/opt/homebrew/opt/node@26/bin/node` **26.5.0** and existing pinned
development dependencies. No package install or lifecycle hook ran. The full
command from `successor/` was:

```sh
/opt/homebrew/opt/node@26/bin/node --test service/service.test.ts service/synthetic-runner.test.ts runner/fixture.test.ts runner/codex.test.ts runner/synthetic-transport.test.ts
```

The actual process matrix killed newly spawned fixture processes only:

| Service checkpoint | Verified result after restart |
| --- | --- |
| Before claim commit | Rollback leaves queued work; no dispatch existed |
| After claim commit, before acknowledgement | Unknown outcome; no automatic redispatch |
| After receipt, before persistence | Unknown outcome; no partial output |
| After started-evidence commit | Unknown outcome; successor remains blocked |
| After terminal message insertion | Transaction rolls back; no partial reply |
| Before terminal commit | No partial messages, terminal state or receipt survives |
| After terminal commit, before acknowledgement | One complete reply survives; authenticated identical replay is harmless |

Runner checkpoints were before start, after committed start, before terminal send,
after committed terminal, and deliberate **exit-zero without a terminal**. The first,
second, third and exit-zero cases become unknown on lease observation and remain
blocked across restart. Only the committed terminal case is complete. Additional
tests inject a fault after the second of two terminal messages and lose the terminal
HTTP acknowledgement; rollback/retry and committed replay preserve exactly one
complete result.

The final suite also verifies partial-upload session expiry/revocation, wrong
authority and binding fields, canonical key-order replay, malformed/oversized
evidence, cumulative delta limits, first-cancellation/completion races, competing contradictory terminals, pre-dispatch
cancellation, late old-attempt output, immutable input digest, and integrated
handshake/turn/stop/unsupported-request deadlines.

## Failed checks and corrections

The first bridge red run failed on the then-missing enrollment seam; the first
transport red run failed on a missing module. These are scaffolding failures,
not behavioral proof. Subsequent red runs reproduced missing canonical replay,
schema/progress/revocation handling, status/expiry handling, unsafe array-to-label
coercion, empty-input acceptance incompatible with the codec, a timeout mislabeled
as cancellation and late cancellation extending an expired deadline. Each was
corrected before final validation. `611521e` strengthens the initial-cancellation
race and adds concurrent contradictory terminals; exactly one event commits. A replay-conflict test initially supplied a
malformed terminal; it was corrected to exercise a valid conflicting terminal.
Intermediate strict checks also caught unfinished declarations and narrow test
types; final strict checking passes. One test-file append used the wrong working
directory and wrote nothing; it was corrected before the final 116-test run.

[validation.json](synthetic-bridge-2026-10-12/validation.json) records source hashes,
commands, counts and retained logs. Red outputs are stored as deterministic gzip
files, with both compressed and raw-output hashes; all content is synthetic test
data. No final check remains failed.

## Unrun checks and next gate

No Swift tests, native build, installed UI, Simulator or physical-device session
ran in this backend-only increment. The existing Stage 1 installed evidence is
separate. No official CLI, version/schema generation, provider authentication,
subscription/model/quota check, OS/Keychain/IPC isolation canary or self-hosted
provider test ran. No current product-readiness claim extends to those areas.

The safe next work is to implement and prove a concrete OS/credential boundary
with synthetic canaries and a bounded real process-I/O harness. Production host
enrollment/TLS and rejected-event diagnostic retention also remain open. Any new
OS/account/VM permission must be a concrete separately authorized action; none was
needed or requested here. After confinement is proven, validate the exact official
executable and generated schema, then obtain owner-authorized provider-native
sign-in in that environment. A successful login alone cannot close confinement.
The native real-turn/cancel/disconnect/restart/replay matrix and all provider/
topology feasibility cells remain required.

All code is newly authored; no donor/provider source or attribution was copied,
removed or relicensed. No existing or provider credentials were read or copied; only fresh synthetic
fixture credentials were used. No provider was
contacted, no external agent or desktop UI session was used, and no persistent
permission, account, startup service, deployment, push, visibility change or
provider cleanup occurred. The donor's Apache-2.0 notices remain intact.
