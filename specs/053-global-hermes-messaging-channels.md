# 053 — Global Hermes Messaging Channels

**Status:** Toby approved the recommended official Hermes platform-adapter
path and authorized implementation plus TestFlight delivery on 2026-08-24.
Implementation is in progress; no release claim exists until the exact build
is processed, installed, visually inspected, and completes a real Hermes turn.
The first TestFlight slice is an explicitly process-local concept proof, not a
durable relay-readiness claim. TestFlight `1.0.8 (2608241)` is consumed by the
superseded fixed-peer UI and `1.0.9 (2608242)` by the initial dynamic-channel
build. The visible release-identity slice consumed `1.0.10 (2608243)`.
Review corrections for multiplexed profile-secret isolation, RFC 8785 Unicode
separators, and contiguous event cursors use `1.0.11 (2608244)`.

**Decision owner:** Toby

**Depends on:** Spec 051 for Fort's durable channel-turn seam and Spec 052 for
the bounded one-peer live proof. Spec 050 remains separate execution-source
inventory work and is not a readiness prerequisite for messaging.

**Pinned Hermes implementation evidence:** Hermes Agent `0.20.5`, local
first-party checkout `981101239a064c020a9d18fc3b1060ae306934ed`; official
`BasePlatformAdapter` and third-party `kind: platform` plugin guidance.

## Decision

Fort is an external messaging platform for Hermes, analogous to Telegram.
Fort itself is not a Hermes plugin. A thin, profile-scoped Fort platform
adapter is installed inside Hermes because that is Hermes' official extension
path for a third-party messaging service:

```text
person -> Fort -> Fort platform adapter -> Hermes gateway -> exact Hermes Bot
```

The adapter uses Hermes' `BasePlatformAdapter`, `MessageEvent`, `SendResult`,
profile scoping, connection lifecycle, and user-authorization hooks. Fort owns
only its platform interface and authenticated channel projection. It does not
invent a replacement Hermes runtime, poll private Desktop RPC, parse Hermes
state from Fort, or extend the experimental generic relay as the product path.

This is messaging, not native execution. A Messaging Channel never becomes an
Agent Channel, Agent Option, execution target, Runtime invocation, A2A peer, or
Remote Run. No transport may fall back to any of those paths.

## Product model

One connected Hermes profile produces one Fort **Messaging Channel**. The
same canonical profile on two machines produces two distinct channels.

```text
Messaging Source = authenticated Fort adapter installation on one stable Fort machine
Messaging Channel identity = (Messaging Source, canonical Hermes profile ID)
display name = Hermes-owned presentation metadata
machine name = Fort-owned presentation metadata
Conversation = Fort-owned transcript for that exact Messaging Channel
```

Fort derives the channel ID as:

```text
messaging-channel:hermes:v1:<lower-hex-sha256(
  "fort-hermes-messaging-channel:v1\n" +
  decimal_utf8_byte_length(messaging_source_id) + ":" + messaging_source_id +
  decimal_utf8_byte_length(canonical_profile_id) + ":" + canonical_profile_id
)>
```

Field boundaries are length-prefixed. The source and profile tuple remains
authoritative; the hash never licenses merging. The canonical profile follows
Hermes' profile-name rules and is immutable for one channel. A profile rename
creates a new channel; Fort never guesses continuity.

The display name is mutable presentation only. It comes from the Hermes-side
`PluginContext.profile_identity` surface with Bot Mode title precedence
defined by Hermes, then profile display name, then canonical profile ID. The
Hermes host resolves its own private representation; the Fort adapter receives
only `{profile_id, display_name}`. Fort never hard-codes, configures, or infers
`Lewis`. A display-name change updates the existing channel. Until that
additive accessor ships in a Hermes release, the adapter is pinned to the
reviewed local first-party helper patch and fails closed when it is absent.

Machine identity is never accepted from an adapter frame. The authenticated
Fort endpoint binds the Messaging Source to the exact Fort machine; its stable
machine ID is identity and its current machine name is presentation. Hostname,
Bot title, profile label, token name, and phone-selected machine are not
machine identity.

## Ownership and authorization

Hermes owns:

- which profiles run the Fort platform adapter;
- the Bot/profile display name;
- platform connection lifecycle and messaging behavior;
- its user allowlist, pairing, and allow-all policy; and
- whether a given Fort principal may talk to a given Bot.

Fort owns:

- authentication of the adapter connection;
- source-qualified channel identity and exact-machine routing;
- the account-visible list and Conversation transcript;
- acceptance before dispatch, with crash durability explicitly deferred; and
- one reversible `hidden` presentation preference.

The adapter registers Hermes' platform authorization hooks using
`FORT_ALLOWED_USERS` and `FORT_ALLOW_ALL_USERS`. Fort sends its stable
authenticated human principal as the message author. Fort never reads,
receives, mirrors, logs, or edits Hermes' allowlist. A connected adapter means
only that the exact Bot can currently communicate using the Fort platform
contract; it is not proof that any particular recipient is allowed.

There is no Fort create, enable, disable, enroll-bot, remove-bot, or delete-bot
operation. To add or remove a Bot from the active roster, the person changes
Hermes configuration. Fort offers only Hide and Unhide. Hide changes Fort
presentation, not transport or authorization; reconnection and metadata
refresh never unhide a channel.

Temporary transport loss leaves a reported channel visible as `offline`.
Fort never interprets silence or an unreachable machine as explicit
deregistration. An explicit Hermes deregistration operation remains required
before Fort can remove an active server-side row; ordinary plugin disconnect
cannot safely stand in for that operation and this beta does not invent it.

## Deep interfaces

The existing durable channel-turn interface remains the intended persistence
seam, but the first dynamic-platform beta does not yet connect its transcript
to that store:

```text
submit(bindingId, clientAttemptId, text) -> durable acceptance receipt
events(attemptId, afterSequence) -> ordered replayable event stream
cancel(attemptId) -> acknowledged outcome     // still deferred until its behavior slice
```

The Messaging Channel module adds only the catalog/contact surface required by
clients and the platform adapter:

```text
register(authenticatedSource, profileIdentity, liveConnection) -> channel receipt
channels(visibility) -> allocated deterministic list
hide(channelId, hidden) -> updated channel
post(conversationId, clientMessageId, text) -> Fort acceptance receipt
events(conversationId, afterSequence) -> ordered process-local events
```

For this concept proof, the Messaging Channel directory and transcript are
process-local. Acceptance occurs before socket dispatch and is idempotent for
the life of that Fort process, but it is not crash-durable or replayable after
a daemon restart. Wiring the already-built `channelturn` SQLite seam is a
separate behavior-first slice. Code, tests, an acceptance receipt, or a live
round trip must not be described as durable until that slice is complete.

The post receipt always exposes the accepted message identity and sequence.
`delivery_state=pending` means only that Fort completed the write to the exact
pinned adapter connection; it does not claim Hermes processed the message.
`delivery_state=unknown` with
`delivery_code=hermes_relay_delivery_failed` means the write outcome was
ambiguous. First response and idempotent replay return the same acceptance
identity and unknown outcome, and Fort never dispatches that client identity
again. A process-lifetime accepted replay recovers its original receipt even
if the adapter has since disconnected; availability gates only a genuinely new
client identity. The Apple client preserves the accepted message, clears the composer to
avoid accidental resubmission, and shows the unknown/no-auto-resend warning.

Authentication supplies account, Messaging Source, and machine identity.
Adapter or client bodies cannot choose or override them. Registration supplies
only canonical profile ID and Hermes-owned display name. Unknown versions,
duplicate identities, invalid names, source/profile mismatch, stale sockets,
and untrusted principals fail closed.

An authenticated socket is reserved while registration is in progress so a
concurrent duplicate cannot replace it, but reservation is not readiness. Fort
writes the exact `registered` acknowledgement before activating the sender or
projecting the channel `connected`; pre-ack rows remain `offline` and cannot
accept a fresh post.

The Fort daemon derives its Messaging Source ID from the stable `machine_id`
in its existing owner-only `relay.yaml`:

```text
messaging-source:fort-machine:v1:<stable Fort machine ID>
```

Neither `hermes-platform.json` nor an adapter frame may supply or override that
ID. The platform config holds one owner-only `profile_token_key`; that root key
never leaves Fort. Fort derives one bearer per canonical Hermes profile as
base64url-without-padding of:

```text
HMAC-SHA256(
  profile_token_key,
  "fort-hermes-profile-token:v1\n" +
  decimal_utf8_byte_length(canonical_profile_id) + ":" +
  canonical_profile_id
)
```

The adapter sends its canonical profile in `X-Fort-Hermes-Profile` and the
matching derived bearer. Fort validates both before the WebSocket upgrade and
then requires the registration frame to repeat that exact profile. This binds
machine/source through Fort configuration and isolates profiles without
inventing a Fort-owned Bot roster.

Every accepted post snapshots and remains pinned to one exact source, machine,
profile, channel, Conversation, and platform-adapter connection across its
connection check and dispatch. A reconnect cannot redirect that post to the
replacement socket. An unavailable channel fails;
Fort never substitutes a different profile, machine, source, model, provider,
transport, Agent, or native execution path. An ambiguous write is not resent.

## Hermes platform adapter

The adapter is a standalone Fort-owned package under
`integrations/hermes/fort-platform/`, installable as a normal Hermes
`kind: platform` plugin without modifying Hermes core packaging. The reviewed
Hermes host needs the additive public `PluginContext.profile_identity`
accessor described above; the plugin itself still makes no Hermes-core or
private-metadata import.

It:

1. captures the canonical `ctx.profile_name` in the profile-scoped plugin
   context;
2. obtains the Hermes-defined Bot display name through the pinned profile
   identity helper;
3. reads only a Fort URL and derived profile credential from the active profile's
   secret scope, plus Hermes-owned
   `FORT_ALLOWED_USERS` / `FORT_ALLOW_ALL_USERS`;
4. opens one outbound authenticated WebSocket to the Fort platform endpoint;
5. waits for exact registration acknowledgement before reporting connected;
6. converts Fort inbound text into `MessageEvent` using Fort's stable human
   principal and exact Conversation as the chat identity;
7. returns Hermes replies through `send` and reports success only after Fort's
   acceptance receipt; and
8. disconnects and lets Hermes' gateway lifecycle supervise reconnection.

It does not register tools, invoke a model directly, start a second gateway,
scan other profiles, read an allowlist, hold provider credentials, choose a
machine, create a Fort Agent, or fall back to the old relay proof.

Migration from the Spec 052 proof must also retire that proof's
`GATEWAY_RELAY_*` values from the Hermes profile secret scope. In particular,
Hermes treats `GATEWAY_RELAY_URL` as a process-level connector deployment and,
by default, disables directly connected platform adapters while it is present.
Spec 053 does not opt into `GATEWAY_RELAY_ALLOW_DIRECT_PLATFORMS` coexistence:
the old values are backed up and removed before the Fort platform adapter is
started. A failed migration must leave the new channel unavailable rather than
dial `/relay` or restore the hard-coded peer as a fallback.

## Apple projection

The first iPhone beta replaces the fixed Hermes transcript with a dynamic
`Channels` directory containing one row per Hermes Messaging Channel. Fort's
existing Agent Channels remain in the adjacent `Fort` product tab for this
concept slice; a single discriminated Agent-plus-Messaging directory is a later
navigation slice, not a prerequisite for proving the platform transport. A
Hermes channel row and transcript header render:

```text
<Hermes Bot name>
Hermes · <owning machine name> · <Connected|Offline>
```

`Hermes` is transport/kind copy, not the title. Two Bots with the same name on
different machines remain separate rows. The title is never `Hermes & Lewis`
and never defaults to `Lewis`.

The shared Settings surfaces render the running bundle's release identity as
`Version <marketing version> (<build>)`, including the iPhone connection sheet
that remains reachable before a machine is connected. The value comes from
`CFBundleShortVersionString` and `CFBundleVersion`; Fort never hard-codes or
guesses a missing release identity.

The iPhone account view may aggregate only already-enrolled, independently
trusted Fort machines. Each row retains its exact machine transport; opening
or sending through one row does not replace the app's selected Agent transport
and never reroutes to another machine. Untrusted machines are not queried or
silently trusted.

Hide is reversible device-local presentation in this beta. Hidden channels
leave the normal list but remain inspectable and retain history. This slice
adds no Remove or Delete action and makes no cross-device Hide synchronization
claim. Notification policy is deferred.

The public Fort contract is `GET /api/messaging/channels`, followed by the
existing exact-Conversation events and post routes. The Hermes adapter connects
to `/platforms/hermes` with its exact derived profile bearer and profile header.
Its registration wire contains only contract version, the same canonical
profile ID, and display name; Fort returns the allocated channel and
Conversation IDs. The completed Spec 052 `/relay` and
`/api/messaging/peers` paths remain historical compatibility only and never
project the dynamic roster.

## Vertical slices and tests

Tests observe behavior through public module, HTTP, WebSocket, and Swift client
interfaces. They use real local Fort modules and scripted external endpoints,
not internal SQL assertions or mocks of Fort modules.

### Slice 1 — authenticated dynamic registration

- RED: two exact profile registrations produce two allocated channels whose
  names come from registration, not Fort config.
- Same profile ID on different authenticated sources produces distinct IDs.
- A name change preserves identity; duplicate/invalid/conflicting registration
  fails closed.
- Disconnect changes only contact state; reconnect restores the same channel.

### Slice 2 — exact-channel message round trip

- An accepted human message is recorded in the process-local transcript before
  adapter dispatch.
- Only the connection registered for that channel receives it.
- A Hermes reply is recorded before acknowledgement and appears in ordered
  process-local events with exact attribution.
- Ambiguous delivery is not resent and no fallback path runs.

### Slice 3 — Apple global list and Hide

- Swift contract tests combine channels from two trusted machine transports,
  preserve the exact transport per row, and distinguish duplicate names.
- Selection opens the exact channel transcript; title and small print use the
  Hermes name and owning Fort machine.
- Hide removes only that channel from the visible list and never mutates
  Hermes or another machine.
- The exact bundle version and build are visible in Settings with a stable
  accessibility identity.

### Slice 4 — live release proof

- Install the adapter and compatible Hermes identity helper on each machine in
  the live proof, with owner-only secrets and no provider-secret movement.
- Restart only the exact affected Fort/Hermes services after configuration is
  validated.
- Complete a benign real message/reply on every advertised channel used for
  signoff; a socket or HTTP success alone is insufficient.
- Inspect the exact installed iOS build through accessibility plus screenshot;
  any wrong bot/machine, duplicate app, raw error, disabled valid action, or
  failed turn blocks release.
- Archive and upload one unique build. Completion requires independent App
  Store Connect confirmation that that exact build processed successfully and
  an installed TestFlight smoke of the processed build.

## Deliberate deferrals

This spec does not claim production relay readiness, crash-durable transcripts,
restart replay, a unified navigation list for Agent and Messaging Channels,
offline queueing, explicit deregistration, exactly-once
delivery, attachments, edits, reactions, cancellation, push notification
policy, cross-account sharing, native Hermes execution, A2A, Remote Runs,
provider/model/tool/memory readiness, or migration from Source Agents. Catalog
presence, compilation, tests, a live socket, HTTP success, or TestFlight
processing alone cannot establish those claims.

## Rollback

The completed Spec 052 build `1.0.7 (2608232)` remains the historical proof,
not a dynamic-channel fallback. Rollback disables the Fort platform adapter
and removes the new Apple projection; it must not silently re-enable the
hard-coded `Lewis` peer, generic relay, native execution, or any alternate
transport.
