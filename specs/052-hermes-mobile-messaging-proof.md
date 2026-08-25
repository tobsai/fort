# 052 — Hermes Mobile Messaging Proof

**Status:** Live proof completed on 2026-08-23. Spec 053 supersedes this
proof's future product naming, dynamic roster, cross-machine listing, and
lifecycle. The staged but unreleased single-peer title/machine correction was
abandoned when the product model changed; it never reached TestFlight.

**Depends on:** Spec 051

**Transport ID:** `hermes_platform_relay_v1`

## Goal

Prove that Fort can act as a messaging platform for one exact Hermes bot from
the iPhone app. This is a product proof, not a native-execution integration and
not a relay-readiness claim.

The first TestFlight proof succeeds when one benign iPhone message travels
through Fort to the enrolled Hermes profile and a Hermes reply appears in the
same Fort Conversation. Hermes-initiated scheduled delivery is a follow-on
criterion: it must not be claimed until Hermes can authenticate Fort as its own
logical messaging platform rather than as the generic relay transport.

## Product seam

`Hermes & Lewis`, the manually configured `bot_display_name`, the fixed Hermes
tab, and `/api/messaging/peers` are historical proof artifacts. They are not
the successor Messaging Channel contract. The completed proof evidence below
remains exact and must not be rewritten as a claim that dynamic discovery or
global routing already shipped.

Hermes is an authenticated messaging peer. It is not projected as a Fort
native execution provider and does not create a Fort execution target, run,
retry, or cancellation contract.

The proof exposes a small messaging surface through the existing authenticated
iPhone gateway path:

```text
GET  /api/messaging/peers
GET  /api/messaging/conversations/{conversationId}/events?after={sequence}
POST /api/messaging/conversations/{conversationId}/messages
```

The Hermes WebSocket remains a local daemon route and is never exposed through
the iPhone gateway relay.

Fort presents the peer with separate transport, bot, and placement truth:

```text
tab/transport: Hermes
title:         <bot display name>
small print:   <canonical Fort relay-host machine name> · <connection state>
```

The title must not concatenate the transport class with the bot name. The
machine name comes from Fort's canonical local node identity at the daemon
composition root; it is not inferred from the phone, parsed from a display
string, or duplicated in the Hermes secret configuration. This field proves
only which Fort node hosts the messaging relay. It does not claim native
execution, provider, model, memory, tool, or policy readiness.

## Proof implementation

The first implementation may keep one peer, one endpoint, one Home
Conversation, its transcript, and its delivery state in process memory. It
must not add a production ledger, generic provider hierarchy, offline queue,
or migration machinery before the round trip is proven.

The public proof seam is message submission plus ordered event reading. Tests
exercise that seam and the real local WebSocket protocol boundary; Fort
modules are not mocked.

Required ordering:

1. Fort accepts and records a human message before writing it to Hermes.
2. Hermes authenticates with the exact gateway and bot identity.
3. A Hermes outbound message is accepted only for the configured Conversation.
4. Fort records the Hermes message before returning a success acknowledgement.
5. A failed or ambiguous relay write never invokes native execution and is not
   automatically resent through another transport.
6. While this proof process remains alive, replaying the exact client message
   identity after an ambiguous relay write returns the same unknown-delivery
   outcome and does not dispatch again.

The production daemon does not subscribe to the connector's optional
accepted-message observer. That observer exists only for the standalone
terminal proof; it cannot delay Hermes acknowledgements in the daemon. The
real daemon wiring test sends and acknowledges more messages than the terminal
observer buffer can hold, without consuming that observer, and reads every
message back through Fort's public event surface.

## Fixed proof identity

Configuration fixes one canonical Hermes profile, relay gateway ID, bot ID,
bot display name, and Fort Home Conversation ID. Fort separately binds the
peer projection to the daemon's exact canonical local machine name. Secrets
are loaded from local configuration and are never returned by an API or
written to logs.

The relay bot ID authenticates the messaging endpoint. The canonical Hermes
profile is an enrollment mapping because relay v1 does not attest the profile
on the wire. Fort does not claim provider, model, memory, tool, or policy
readiness.

## Deliberate limits

The proof is:

- one Hermes bot and one allowed Conversation;
- text only, up to the relay descriptor limit;
- online only;
- process-local and disposable;
- separate from Agent Channel execution, A2A, Remote Runs, and native Hermes;
- without attachments, edits, deletion, reactions, secondary recipients,
  offline replay, crash recovery, exactly-once delivery, retry, or cancel.

The TestFlight proof client snapshots peer readiness when its Hermes screen
loads and keeps its transcript in memory. After a daemon restart, relay
reconnect, or selected-machine/transport change, the app must be closed and
reopened before its state is judged. The proof also has no durable per-message
delivery-status UI: although the daemon remembers an ambiguous delivery for
the exact client message identity while that process lives, a later successful
event poll can clear the screen-level error. Continuous readiness
reconciliation and durable delivery-state presentation belong to the relay
readiness follow-on, not this concept proof.

These limits are acceptable only for this live concept proof. A successful
proof informs the later durable cloud design; it does not silently promote the
proof module to production infrastructure.

## Verification and release

Before upload:

1. focused RED/GREEN contract tests pass;
2. the local daemon and exact profile-scoped Hermes process complete the real
   benign send/reply proof;
3. the iPhone client reaches the same daemon through the existing encrypted
   gateway path;
4. no native execution or fallback is observed.

Then archive and upload one uniquely numbered TestFlight build with the
messaging proof surface enabled. Completion requires independent confirmation
that the exact build is processed in TestFlight plus a live installed-app
smoke. Build success, upload output, HTTP 200, or socket connection alone is
not proof completion.

Release checkpoint:

- TestFlight marketing version: `1.0.7`
- Apple proof build: `2608232` (uploaded, processed, installed, and live-smoke
  verified)
- `1.0.7 (2608231)` was uploaded and processed, but its installed-app smoke
  exposed two proof defects: process restart reused externally visible message
  IDs, and an older overlapping event poll could surface a false projection
  error after a successful send receipt. It is superseded and must not be
  treated as the completed proof build.
- Prior independently observed TestFlight build: `1.0.5 (2608221)`
- `1.0.6 (2608222)` is deliberately skipped because it existed only as an
  unverified source checkpoint and must not be confused with this proof.

### Live relay constraint discovered during the proof

The released Hermes relay contract successfully completed an authenticated
Fort send/reply turn on 2026-08-23. Hermes then rejected `/sethome` with
`Relay does not authenticate this logical home target`. Its generic `relay`
identity is intentionally excluded from Hermes home-channel persistence; the
relay is designed to front an authenticated logical platform such as Telegram
or Discord. Fort must not masquerade as one of those platforms merely to make
scheduled delivery pass.

Build `1.0.7 (2608231)` proved that a TestFlight-originated message could reach
Hermes and that Hermes could return the exact requested reply through Fort. It
did not complete installed-app signoff because that reply exposed the client
poll-reconciliation defect above. Build `1.0.7 (2608232)` supersedes it with
identity-derived external message/delivery IDs and exact-sequence overlap
reconciliation.

TestFlight independently displayed `1.0.7 (2608232)` after Apple processing.
The installed build then displayed `Hermes & Lewis` as connected, accepted one
fresh benign message through the authenticated gateway (`POST` status `202`),
and displayed Hermes' exact reply in the same Conversation while event reads
remained successful (`GET` status `200`). No visible error, native execution,
transport fallback, or identity substitution was observed. This completes the
bounded send/reply proof; it does not remove any deliberate limit above or
constitute durable relay readiness.

After closing two server-only proof defects (the optional terminal observer
stalling daemon acknowledgements after its eighth unconsumed item, and an
unknown delivery replay being projected as success), daemon
`0.14.2-hermes-proof.4` was installed and the same processed TestFlight build
was smoke-tested again. The app displayed the exact Hermes reply
`FORT HERMES SERVER READY PROOF 4`; Fort's event projection contained the
ordered human and linked peer messages, and the gateway POST/GET requests
remained `202`/`200`.

Hermes-initiated scheduled messages remain deferred until Hermes exposes a real
`fort` logical platform identity (or an equivalent first-party authenticated
home-target contract). The Fort relay protocol test still covers unsolicited
authenticated outbound frames, but that test is not a claim that Hermes can
originate them through its current scheduler configuration.

## Authorization

Toby authorized implementation, exact-profile configuration, required local
service restarts, benign live messaging, and TestFlight delivery with: “I want
to test the Hermes functionality. Do whatever is needed to make that happen.”

This authorization does not permit silent identity substitution, native
fallback, unrelated dependency installation, git commit, or git push.
