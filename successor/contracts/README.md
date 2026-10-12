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
scopes, real-provider action control, restore, contract minor-version upgrade
negotiation, skills and routines remain open acceptance work. An absent request
under a changed recovery epoch is not evidence that repeating it is safe.

## Fixture approval, cancellation and uncertainty controls

Snapshots now include `approvals` (clients reading older snapshots default to an
empty array). `fixtures/pending-approval.json` supplies a shared pending example.
An approval pins installation, epoch, run, attempt, binding revision, canonical
action digest, exact resource scope and expiry. Its only permitted fixture action
is `Produce the fixture reply`, scoped to `['fixture:reply']`.

`POST /v1/approvals/{approvalId}/resolve` takes `recoveryEpoch`, `runId`,
`attemptId`, `bindingRevisionId`, `actionDigest`, `resourceScope`, and `decision`
(`approve` or `reject`). It returns `{approval,run}`. Exact authority is checked
before an atomic first-decision-wins transition. A second opposing decision
returns the already-recorded winner. Incorrect pins return 409. Expiry records
`expired` and cancels the effect-free parked fixture before returning 409.

`POST /v1/runs/{runId}/cancel` takes `{recoveryEpoch}` and returns `{run}`.
Unclaimed work and the effect-free parked fixture cancel immediately. Active
work first records `cancelRequested`; only observed fixture process termination
allows `canceled`. Correlated completion can win the race and remains completed.
A cancellation request cannot rewrite an unknown outcome as canceled.

`POST /v1/runs/{runId}/close-unknown` takes
`{recoveryEpoch,acknowledgeUncertainty:true}` and returns `{run}`. Only
`outcomeUnknown` can become `closedUnknown`. The owner's device and decision are
recorded in a durable event. This releases the Conversation barrier without
retrying the original request or claiming that its outcome is known. Clients
must explain that distinction before presenting “Close without retry”.

Run states additionally include `waitingForApproval`, `cancelRequested`,
`canceled`, and `closedUnknown`. Approval states are `pending`, `approved`,
`rejected`, `expired`, and `canceled`. `approval.requested` and
`approval.updated` events contain `data.approval`; run changes retain
`run.updated`. These commands always require owner-device authentication.
