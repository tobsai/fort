# Candidate B — implemented disabled session runner

Source checkpoint: **`bbb415d15966013b3227d6eecb6bf85887d76de9`** on
`codex/fort-openai-rebuild-20261016`, in
`/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.
The exact source/app lists are in [pins.json](pins.json). This is implemented,
nonlive-tested preparation, ready for independent review. The unchanged activation
guard still refuses both the manual entry and official launcher. No further scope
decision was needed to implement this authorized nonlive work.

The [previous independent review](../2026-10-16-candidate-b-independent-acceptance.md)
accepted the three fixes at `73f6444`. It does not independently review this new
delta. The earlier packet's unimplemented-wiring and 30-day-credential proposal
are superseded by the current [runner documentation](../../prototype/README.md),
[machine packet](../../ops/candidate-b-manual-packet-20261016/packet.json), and
[attended instructions](../../ops/candidate-b-manual-packet-20261016/MANUAL.md).

## What is now connected

`prototype/manual.ts` connects source/app verification, the session controller,
the owned official launcher, same-connection preflight, local enrollment and the
actual native-state inspector. Its public entry rejects before these operations
today. After an authorized guard revision and corresponding pin verification,
the intended command is:

```sh
/opt/homebrew/opt/node@26/bin/node successor/prototype/manual.ts
```

The controller accepts `start`, `status`, `done`, and `abort`; it accepts no prompt
text. It displays only the path of a short-lived local enrollment file. The user
opens/quits the pinned app and uses the five unchanged samples. No hidden setup
script, provider sign-in, environment activation flag or extra preflight process
is needed or provided.

The runtime verifies every configured control, selected provider routing,
telemetry exclusions, required disabled features and the complete paginated MCP
inventory. A late page, cyclic/truncated inventory, missing required field,
unapproved routing, ambient server or thread-only capability prevents dispatch.
Four explicit browser/computer feature disables supplement the earlier flag set;
the exact argv/environment is in [launch-plan.json](launch-plan.json). Only the
normal declared OpenAI/ChatGPT base URLs are accepted when those effective fields
are present. No alternative endpoint or API credential path is introduced.

The no-turn preflight initializes once, verifies existing ChatGPT account class
and exact `gpt-6-astra` availability, then retains that connection. Each turn
rechecks controls and the returned ephemeral thread policy before its cancellation
fence. Pre-dispatch failure is reported as `notStarted`, never as a completed turn.

The HTTP gate checks the literal next sample, Agent/instructions, binding/Home
identity, selected skill and shared recipient before acceptance. Earlier runs
must be completed. Durable start state plus an exclusive lock caps starts at two;
uncertain/crashed state never authorizes a new start. The outer work cutoff is now
**292 seconds**, retaining the same five-minute total and reserving eight seconds
for owned TERM/KILL, native process checks and credential cleanup. A timer closes
even an idle session without waiting for another user command.

The second start requires the four saved completed receipts, unchanged ledger and
installation, the actual native pending file decoding to an empty array, and
confirmed app/service/runtime cleanup. A queue that becomes nonempty between
sessions permanently blocks that packet; clearing it later does not authorize a
retry. The same port is reused so the native connection identity remains stable.

## Bounded local credential

The runner always selects the new session-bounded enrollment option. One code and
one device credential expire no later than ten minutes after the first start,
including the gap. Reopening cannot extend that deadline. Only a verified handoff
to session two retains the credential. Final failure/completion or controller exit
between sessions revokes its server authorization and closes enrollment.

Cleanup attempts deletion of only the exact newly issued `Fort.Spec058.Device`
Keychain account(s), without reading secret values. Issued device IDs allow cleanup
even if writing the native connection file failed. Successful deletion removes
matching connection metadata; sample history/pending evidence remains. A refused
Keychain deletion is explicitly reported as unconfirmed and blocks the packet;
the server token is already revoked. An uncatchable crash may leave an item or
metadata, but the persisted deadline bounds authorization and the stale lock
prevents automatic restart. No real credential was created or deleted here.

## Validation

| Check | Result |
| --- | --- |
| Combined relevant Node suites | **154 passed, 0 failed**, [raw output](session-wiring-tests.log) |
| Strict TypeScript | Passed on the final source; receipt `5043b3` |
| Swift FortClient/PendingStore HTTP conformance | Five synthetic turns passed against temporary SQLite; [output](session-wiring-native-http.log) |
| New source pins | 452 tracked non-evidence successor/Spec 058 files; canonical hash verified |
| Existing app | All five app hashes match; Apple source unchanged; no rebuild or signing change |
| Main checkout | Status byte-identical to the retained recovery baseline |
| Activation | Guard byte-identical to `73f6444`; public entry/launcher refusal tests pass |

The Node suite uses explicit synthetic wire transports, temporary loopback
services, actual temporary SQLite ledgers and owned Node test actors. Native file
checks use synthetic installation state and an injected credential eraser. They
do not inspect the user's Keychain, configuration or actual app queue. The HTTP
conformance executable is the unchanged previously built Swift client, not the
rendered MessengerModel/SwiftUI UI. The earlier 23 Swift model/unit tests and Mac
build were not rerun because Apple source is unchanged.

Red-before-green checks exposed missing configuration/MCP/preflight gates,
unbounded enrollment, acceptance without a session gate, missing controller/native
entrypoints, parallel-start ownership, insufficient cleanup reserve, restart after
a changed native queue, and custom routing/telemetry bypass. Their final
regressions pass. During validation, one intended red deadline assertion lacked
its cleanup hook, and a combined run reused an old Node HTTP connection across
the same-port restart (`ECONNRESET`). The test actors were stopped, cleanup hooks
were added, and the HTTP fixture now closes each connection to model quitting the
native app. The [interrupted combined-run output](session-wiring-interrupted-test.log)
is preserved. No provider was involved. The final bounded combined run passes.

## Remaining runtime evidence and actual limits

No installed provider process, real account/auth store, configuration secret,
native Keychain operation or app launch occurred. No system security setting,
startup service, deployment, remote publication or provider cleanup changed.

The exact binary must still demonstrate the required fields and controls during
an authorized no-turn preflight. The versioned schemas permit optional data;
omitted controls or even disabled-server status entries cause a conservative stop.
The source review cannot establish whether the installed binary will pass. The
schema for effective configuration and its routing/telemetry fields is the public
[0.157.1 config schema](https://raw.githubusercontent.com/openai/codex/rust-v0.157.1/codex-rs/core/config.schema.json).

It is impossible for this protocol preflight to prove there were no host startup
side effects before `config/read`: the host must already be running to answer it.
Retained HOME and same-user host authority remain; no direct secret/configuration
reader or new OS isolation was added. The code gates thread creation/discovery
requests and turns, not all host activity. Unknown extra sandbox fields, escaped
descendants, remote cancellation and effective-model attestation remain unproven.
Manual app/Keychain/signature behavior and true provider cleanup require the later
attended test. These limitations are explicit; no new unimplemented session
controller step is being deferred to the user.
