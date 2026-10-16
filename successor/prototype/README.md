# Candidate B — approved owner trial prepared, not started

The owner approved this fresh trial at 21:52 UTC on October 16. Only UI task
`01a1451b-81d7-76cf-8e5c-a85d7164201c` may start it. Read the
[approval and pins](../evidence/2026-10-16-candidate-b-owner-trial/README.md) and
[short operator instructions](../ops/candidate-b-owner-trial-20261016/MANUAL.md).
The fourth candidate already completed one real native text turn; its
[acceptance evidence](../evidence/2026-10-16-fourth-native-acceptance/README.md)
and all four consumed packets remain preserved. This preparation starts nothing.

The fixed `owner-trial-20261016` packet admits up to five owner-entered plain-text
messages in the same Writer Home, selected model `gpt-6-astra`, instructions
`Reply briefly using supplied context.`, with no skills. Each preceding run must
complete before another message is accepted. Text bytes are preserved. Other Agent,
shared-conversation and skill selections, a changed binding, and a sixth request
are refused before acceptance. Replaying an accepted request returns its receipt
without another run. Only the human may choose/type/send these messages. The UI
operator must not send a sample, corrective prompt, or test message.

The clock begins with `start`: 540 seconds for preflight, pairing and conversation,
then 60 seconds reserved for cleanup within the 600-second total. Readiness prints
both exact deadlines without enrollment values. Immediately after pairing and
opening Writer Home, the operator must notify the parent that the owner can type,
with both deadlines and zero messages sent. Pairing delay consumes this same budget.

At the work cutoff the controller stops accepting/dispatching, cancels active work,
sends TERM only to the exact pinned native executable, confirms exit, closes the
owned runtime/listener, and revokes the exact service/native temporary credentials.
`done` ends early; `abort`, SIGINT/SIGTERM or terminal closure cancel the trial.
Normal expiry or early completion can finish with zero through five completed runs;
an unfinished run, nonempty pending queue, unknown native state, stop failure or
late/unconfirmed cleanup blocks the packet. The native PendingStore does not create
a queue file until the first message; only a zero-receipt owner trial permits that
specific absent-file case after confirming the paired installation and scope.
Malformed files still fail. These bounded waits are not hard-real-time guarantees.

Any persisted state consumes the sole start, including startup/freshness failure.
No restart, state reset, alternate packet root or adoption of a prior ledger is
allowed. Scoped cleanup deletes only issued installation/device Keychain items
under `Fort.Spec058.Device`, without reading their secrets; it removes matching
connection metadata and temporary enrollment. Ledger, snapshots and pending data
remain preserved. The completed fourth cache is archived with file hashes before
fresh pairing. No credentials are copied or created during preparation.

The native app and complete runner are unchanged from the working fourth candidate.
Only controller admission/lifetime, automatic native shutdown and the launch plan's
explicit owner-trial duration bound change. The named no-files profile, disabled
computer tools, empty environments/roots, exact provider/model, explicit user review,
strict config/catalog/feature/MCP checks and redacted diagnostics are retained.
The existing ChatGPT sign-in is used without sign-in or API-key fallback.

The official Codex agent host still has the Mac user's HOME/access before preflight.
The computer-tool policy does not confine that host, Keychain, ambient startup or
provider networking. Actual parser/pathname canaries passed; pre-opened descriptors
remain usable and full generated-policy OS isolation is unverified. Effective model
attestation, escaped-descendant containment and remote cancellation are unproven.
No OS/security changes, provider cleanup, startup services, deployment, public ingress
or additional attempt is authorized. A protected authentication prompt requires the
owner; never bypass it.
