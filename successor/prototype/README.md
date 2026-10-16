# Candidate B — fourth bounded packet

The owner approved one further native-chat test at 21:29 UTC on October 16.
Only UI task `01a1451b-81d7-76cf-8e5c-a85d7164201c` executes it. See the
[approval and exact pins](../evidence/2026-10-16-candidate-b-fourth/README.md)
and [short operator instructions](../ops/candidate-b-fourth-20261016/MANUAL.md).
Preparation does not start a live runtime, enroll a device or consume the attempt.

`manual.ts` verifies the new source/app pins and selects only
`/Users/tobiasgunn/dev/fort/.local/prototype-candidate-b-fourth-20261016` with identity
`fourth-20261016`. Any persisted state prevents another start, including a new
controller after success or failure. Native freshness is checked after recording
its consumed start. No reset, directory substitution or retry is offered.
The first three state/ledger/approval/evidence packets remain byte-identical and
blocked, with one consumed start and zero receipts each.

Only one exact submission is admitted: Writer's Home, no skill, model
`gpt-6-astra`, instructions `Reply briefly using supplied context.`, and text:

> Remember the sample word cedar. Reply with that word only.

Wrong text, Agent, conversation or skill selections and any second submission are
refused before durable acceptance. Replaying an already accepted request returns
its original receipt without another run. The UI operator expects `cedar`; an
unexpected reply is reported without a corrective prompt. The controller's finished
state proves completed execution/cleanup, not independent semantic reply acceptance.
Historical five-sample/two-session behavior remains for regression coverage only;
the live entry cannot select it.

Work stops at 292 seconds, leaving eight seconds within the **five-minute total**
for owned-runtime disposal, listener closure and scoped credential cleanup. The
credential expires no later than 300 seconds from start. Success requires one
completed run/receipt, the exact app fully exited, zero actual native pending items,
and confirmed cleanup by that deadline. Late or failed cleanup blocks the packet.
These are supervised timers and bounded waits, not a hard-real-time guarantee.
SIGINT, SIGTERM, terminal closure and `abort` also cancel preflight; cancellation
consumes the sole start.

The complete runner and launch plan remain byte-identical to offline-canary source
`fcc3685dcf1828553b27fb2d27c4e75ca76b383c`. The named no-files profile, disabled
computer tools, empty environments/roots, exact provider/model, explicit user review,
strict config/catalog/feature/MCP checks and redacted failure diagnostics are retained.
No failed check may be relaxed during execution. The existing ChatGPT sign-in is
used without provider sign-in, API-key fallback or credential copying.

Automatic local pairing is performed through the existing native Connection UI by
the designated operator. Enrollment values stay in non-emitting CUA variables;
they must not enter chat, screenshots or logs. Cleanup revokes exact service
enrollment/device rows and deletes only issued installation/device items under
`Fort.Spec058.Device`, without reading their secret values. Matching connection
metadata and the temporary enrollment file are removed; ledger/history are preserved.
Unconfirmed deletion blocks the packet. No global credential sweep is authorized.

The pinned official Codex host still runs with the Mac user's HOME/access before
Fort can inspect effective config. The computer-tool policy does not confine that
host, Keychain, ambient startup effects or provider networking. Offline canaries
accepted the profile in the actual parser and demonstrated pathname denials, but
pre-opened descriptors remain usable and macOS refused nested full-policy testing.
No complete OS isolation, escaped-descendant containment, remote cancellation or
effective-model attestation is claimed. Native/provider UI acceptance remains the
designated operator's job. No new OS settings, permanent services, deployment,
public ingress, provider cleanup or fifth attempt is authorized.
