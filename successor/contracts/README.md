# Fort fixture HTTP v1.0

This is the Spec 058 Stage 1 fixture subset. It is not the full MVP API.
`v1.ts` defines the wire data; `openapi.json` describes the implemented routes.
The JSON fixtures are shared with FortKit tests. Identifiers are opaque strings.
All timestamps are UTC ISO 8601. No client clock establishes message order.

Every route except single-use enrollment requires both `Authorization: Bearer
<deviceToken>` and `X-Fort-Installation-Id: <installationId>`. A device token
carries the single fixture owner's authority. There are no runner tokens accepted
by HTTP. Every browser Origin is denied; browser administration is not enabled.
No loopback exemption, anonymous status, enrollment-code generation route, or
credential-renewal route exists. Owner-local code issuance is a library method.

| Method | Route | Result |
| --- | --- | --- |
| POST | `/v1/devices/enroll` | Installation, device ID, token, expiry; consumes code once |
| GET | `/v1/installation` | Authenticated installation and recovery epoch |
| GET | `/v1/agents` | One stable Fixture Agent and its permanent Home/binding |
| POST | `/v1/devices/{deviceId}/revoke` | Revokes the exact device token |
| POST | `/v1/conversations/{conversationId}/messages` | Durable acceptance, 201 new / 200 exact retry |
| GET | `/v1/requests/{requestId}` | Original immutable receipt and acceptance snapshot |
| GET | `/v1/conversations/{conversationId}/snapshot` | Current atomic transcript, runs, read position and cursor |
| GET | `/v1/events?after=N&recoveryEpoch=...` | At most 256 durable events after N; returned cursor is last delivered |
| POST | `/v1/conversations/{conversationId}/read` | Monotonic shared read position for an existing message |

Submission fields are `requestId`, `bindingRevisionId`, `recoveryEpoch`, and
`text`. Reusing the ID with changed validated content returns 409. The original
acceptance response remains unchanged after execution; fetch a current snapshot
for current run state. Exact text is never trimmed, normalized or interpreted as
commands. The limit is 65,536 UTF-8 bytes; malformed Unicode is rejected.

Read fields are `recoveryEpoch`, `messageId`, and the observed `cursor`. A global
cursor cannot mark another Conversation or future message read. Events use
`message.accepted` (`data.message`, `data.run`), `run.updated` (`data.run`),
`message.created` (`data.message`) and `read.updated`
(`data.lastReadMessageSequence`). A client can fetch a snapshot after event replay.
The snapshot and event cursor belong to the exact installation and recovery epoch.

Errors are `{ "error": { "code": "...", "message": "..." } }`. Typical status
codes are 400 invalid input/cursor, 401 missing or denied device/enrollment
credential, 403 browser Origin, 404 absent entity/unsupported route, 409 changed
request/binding/epoch, and 413 size limit. Error messages contain no tokens, input
text, stack traces or environment values.

Only the loopback HTTP fixture is provided. TLS/private-network enrollment,
paginated transcript, SSE, public Agent/binding creation, separate read-only
scopes, approval/cancel/recovery actions, restore, contract minor-version upgrade
negotiation, skills and routines remain open acceptance work. An absent request
under a changed recovery epoch is not evidence that repeating it is safe.
