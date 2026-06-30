# Interface Contracts for `fort-ui`

Status: spike note (read-only extraction). Source of truth = the existing TypeScript
Fort core (`packages/core`). The future `fort-ui` will reimplement these contracts
against the new Go `fort-core`. Section by section below: the **existing TS contract**
(with `file:line` refs) and a **fort-ui will reimplement as** note.

> Transport reality check: the portal does **not** use SSE (`text/event-stream`) or a
> REST chat endpoint. Chat and the live event feed both ride a **single JSON WebSocket**
> on the portal port. REST (`/api/...`) is used only for setup, agent CRUD, history
> snapshots, and OpenClaw import. Two separate WS servers exist (see §2).

---

## 1. Chat API

Chat is a WebSocket request/response pair, not REST. The browser opens a WS to the
portal port (default **4077**, same port as the HTTP server — the server upgrades
`upgrade: websocket` requests) and sends a `chat` message.

**Send (client → server)** — `WSMessage` envelope
(`packages/core/src/server/index.ts:51`):

```jsonc
{
  "id": "<client-correlation-id>",
  "type": "chat",
  "payload": {
    "text": "what's on my plate today?",
    "agentId": "lewis",          // optional; falls back to default agent
    "hidden": false,             // optional; suppresses echo in UI
    "modelTier": "standard"      // optional: 'fast' | 'standard' | 'powerful'
  }
}
```

`payload` may also be a bare string (legacy) — handled at
`packages/core/src/server/index.ts:486-489`. The sentinel `text === "__greeting__"`
triggers a server-composed greeting/hatch prompt (`index.ts:490-509`).

**Reply (server → client)** — `WSResponse` (`index.ts:57`), emitted at
`index.ts:537-545`:

```jsonc
{
  "id": "<same correlation id>",
  "type": "chat.response",
  "payload": {
    "taskId": "<uuid>",
    "task": { /* full Task object, see §3 */ },
    "hidden": false
  }
}
```

Key behavior (`index.ts:484-545`, `orchestrator.ts:75-125`):
- Every chat **creates a Task** (`orchestrator.ts:87`, `metadata.type = 'chat'`).
- The handler `await`s the full task before replying, so `chat.response` arrives only
  after the agent finishes (or after a gate is answered — see §4). The actual reply text
  is `task.result`.
- The user message is persisted to a per-agent **thread** before dispatch
  (`index.ts:511-523`, `getOrCreateAgentThread`); the agent reply is persisted to the
  same thread when `task.status_changed` fires with a `result` (`index.ts:418-436`).

**History snapshot (REST):** `GET /api/chat-history` (`index.ts:208-228`) returns the
last 100 chat tasks (`metadata.type === 'chat'` and non-null `result`), each as
`{ id, shortId, title, description, result, status, source, assignedAgent, createdAt }`.

**Thread message shape** (`ThreadMessage`, `types.ts:83-90`):
`{ id, threadId, role: 'user'|'agent'|'system', content, agentId, createdAt }`.

**fort-ui will reimplement as:** a WS `chat` send + `chat.response` await keyed by a
client-generated `id`, rendering `task.result` as the agent message. Reuse `taskId` to
correlate with the live event stream (§2). Keep `GET /api/chat-history` (or a Go
equivalent) for cold-load hydration; render threads from the `ThreadMessage` shape.
fort-core may upgrade this to true token streaming, but the v1 contract is
"one request, one terminal response, live events in between."

---

## 2. Live Event Model (WS)

There are **two** WebSocket servers — this distinction matters for fort-ui:

| Server | Class | Port / path | Envelope | Audience |
|---|---|---|---|---|
| Portal WS | `FortServer` | `4077` (HTTP upgrade, no path) | `{ id, type, payload }` | Dashboard / fort-ui |
| Shell IPC WS | `IPCServer` | `4001` `/shell` | `{ type, data }` | Swift menu bar (§6) |

### Portal WS envelope (the one fort-ui consumes)

`WSResponse` (`index.ts:57`): `{ id, type, payload, error? }`. On connect the server
pushes an initial `{ id: "init", type: "state", payload: <state> }` snapshot
(`index.ts:397-401`). Bus events are bridged to broadcasts in
`setupEventBroadcast()` (`index.ts:405-482`) and the gate bridges at `index.ts:277-285`.

**Event `type` names emitted during a run** (each broadcast as
`{ id, type, payload }`):

| `type` | Source line | Payload (abridged) |
|---|---|---|
| `task.created` | `index.ts:407-409` | `{ task }` |
| `task.status_changed` | `index.ts:411-416` | the full `Task` object (flattened) |
| `agent.acknowledged` | `index.ts:439` | `{ ... }` |
| `agent.started` | `index.ts:443` | `{ ... }` |
| `agent.error` | `index.ts:447` | `{ ... }` |
| `agent.classifying` / `agent.classified` | `index.ts:453-464` | classifier lifecycle |
| `agent.decomposing` / `agent.decomposed` / `agent.decomposed_failed` | `index.ts:453-464` | plan lifecycle |
| `triager.reclassified` | `index.ts:453-464` | `{ taskId, from, to, newBoard }` |
| `reflection.insight` | `index.ts:466-468` | `{ ... }` |
| `tool.executed` | `index.ts:471-473` | tool-call result |
| `tool.denied` | `index.ts:475-477` | `{ ... }` |
| `tool.error` | `index.ts:479-481` | `{ ... }` |
| `approval.new` | `index.ts:278-280` | approval gate (see §4) |
| `model-choice.new` | `index.ts:283-285` | model-choice gate (see §4) |

Underlying bus event names (the strings the server subscribes to) come from
`TaskGraph` (`task.created` `task-graph/index.ts:78`; `task.status_changed`
`:93,:127`; `task.assigned` `:146`; `task.review_completed` `:170,:208`) and the tool
executor (`approval.required` `tools/executor.ts:259`).

**fort-ui will reimplement as:** a single persistent WS to fort-core, dispatching on the
`type` string. The envelope is `{ id, type, payload }` (no SSE framing). Note the
asymmetry to preserve: `task.status_changed` broadcasts the **flattened Task** (not
`{ task }`) — see the unwrap at `index.ts:414-416`. fort-ui should treat the WS as the
live feed and the `chat.response`/REST snapshots as the source-of-record reconciliation.

---

## 3. Board / Task Model

### Task entity (`Task`, `types.ts:28-48`)

```ts
{
  id: string;                 // uuid
  shortId: string;            // "FORT-001" (task-graph/index.ts:46)
  parentId: string | null;    // subtask linkage
  title: string;
  description: string;
  status: TaskStatus;         // see state machine below
  source: TaskSource;
  assignedAgent: string | null;
  sourceAgentId: string | null;
  createdAt: Date;
  updatedAt: Date;
  completedAt: Date | null;
  result: string | null;      // the agent's reply text for chat tasks
  assignedTo: 'agent' | 'user' | null;
  metadata: Record<string, unknown>;  // holds board, classification, type, modelTier…
  subtaskIds: string[];
  threadId: string | null;
  goalId?: string | null;
}
```

### State machine (`TaskStatus`, `types.ts:10-17`)

`pending → created → in_progress → (blocked | needs_review) → completed | failed`

- `created` on `createTask` (`task-graph/index.ts:50`).
- `in_progress` when work starts / thread opens (`threads/index.ts:117`).
- `blocked` while awaiting a gate (spec 020; `types.ts` note `:14`).
- `needs_review` when LLM completion review rejects (`task-graph/index.ts:175,217`) or a
  subtask failed (`:462-466`).
- `completed` / `failed` set `completedAt` (`:113-115`).
- Parent auto-completes when all subtasks terminal (`checkParentCompletion`
  `:451-468`).

### Boards (columns) — `metadata.board`

Boards are a metadata tag, **not** a status: `'main'` (work board) vs `'questions'`
(quick-answer board); default `'main'` (`task-graph/index.ts:102-105`,
`setBoard`). Query/filter contract via WS `tasks.query`:

- **Request** (`index.ts:564-574`): `{ status?, assignedAgent?, since?, limit?, offset?,
  board?: 'main'|'questions'|'all' }`.
- **Response** `tasks.query.response` (`index.ts:599-603`): each task enriched with a
  `subtasks` array (its direct children from the store).
- Other read messages: `tasks` (last 100, `index.ts:550-555`), `tasks.active`
  (`:557-562`), `triager.reclassify` to move a task between boards + record a memory
  correction (`:606-656`, emits `triager.reclassified`).

**fort-ui will reimplement as:** a Kanban whose **columns are `metadata.board` values**
(`main` / `questions`), with the `TaskStatus` enum driving per-card state/coloring
(`created`/`in_progress` = active, `blocked` = paused/awaiting gate, `needs_review` =
attention, `completed`/`failed` = terminal). Use `shortId` for display, `parentId` /
`subtasks` for nesting. Reclassification is a `triager.reclassify` WS round-trip.

---

## 4. Gate / Approval Surface

Two blocking gate types, both modeled on the same resolver-map + bus-event + 10-min
timeout pattern, both surfaced over the portal WS as inline chat cards. The chat request
stays open (§1) until the gate resolves.

### 4a. Tool approval (Tier 2/3 tools)

- Raised: `tools/executor.ts:259` publishes `approval.required`
  `{ approvalId, toolName, tier, taskId, agentId, parameters }`; 10-min timeout
  (`:269-274`).
- Broadcast to UI: `approval.new` (`index.ts:278-280`).
- Persistent record `ApprovalRequest` (`tools/approval-store.ts:14-25`):
  `{ id, taskId, agentId, toolName, toolTier, parameters, status, rejectionReason,
  createdAt, resolvedAt }`.
- Read: WS `approvals.list` → `{ pending }` (`index.ts:1144-1146`);
  `approvals.for_task` → `{ approvals }` (`:1284-1290`).
- Answer: WS `approval.respond` `{ id, approved: boolean, rejectionReason? }`
  (`index.ts:1293-1325`) → resolves the executor promise.

### 4b. Model-choice gate (gated/rate-limited model, spec 020)

- Service: `ModelChoiceService` (`services/model-choice.ts`). `requestChoice` publishes
  `model-choice.required` `{ id, taskId, agentId, gatedModel, options }`
  (`model-choice.ts:42-61`); 10-min timeout → `{ action: 'fallback' }` (`:50-54`).
- Broadcast to UI: `model-choice.new` (`index.ts:283-285`). Task goes `blocked` while
  pending (spec 020 §"Task status").
- `options` (`ChoiceOption`, `model-choice.ts:6-9`) — discriminated union:
  - `{ action: 'switch_provider', providerId, label }`
  - `{ action: 'lighter_model', tier: 'fast'|'standard', label }`
  - `{ action: 'use_api_key', providerId, label }`
- Answer: WS `model-choice.respond`
  `{ id, action: 'switch_provider'|'lighter_model'|'use_api_key'|'fallback',
  providerId?, tier?, apiKey?, remember? }` (`index.ts:1327-1343`) →
  `resolveChoice` → `{ ok }`. `remember: true` persists the choice to the agent's
  `identity.yaml` (`model-choice.ts:resolve` + `persist` `:69-75`).

**fort-ui will reimplement as:** a generic "blocking inline card" component keyed off
`approval.new` / `model-choice.new` (both carry the `taskId` to slot the card into the
right chat stream). Answer over WS with `approval.respond` / `model-choice.respond`. The
card collapses to a one-line summary once answered; the held-open `chat.response` then
delivers the real reply. Non-interactive runs (`source !== 'user_chat'`) never gate —
they auto-fall-back server-side, so fort-ui only renders gates for interactive chats.

---

## 5. OpenClaw Channel

**Important scope note:** in the current TS core, OpenClaw is a **one-shot import
(migration), not a live inbound message channel.** There is no running OpenClaw socket
mapping inbound messages to tasks. The `'imessage'` value exists in `TaskSource`
(`types.ts:21`) and there is an `IMessageIntegration` for *sending*
(`integrations/imessage.ts`, Tier-2 `compose_imessage`), but no inbound-OpenClaw→task
producer is wired. Treat this section as "the import contract + the reserved channel
slot."

### Import contract (spec 017, `index.ts:156-163`, `:1483-1500`)

- `GET /api/import/openclaw/preview` → `scanOpenClaw(fort)` → `OpenClawPreview`
  (`{ found, agents[], providers[], warnings[] }`; `{ found: false }` if absent).
- `POST /api/import/openclaw` → `importOpenClaw(fort, options)` → result JSON.
- Mapping (spec 017): OpenClaw `agents.list[]` → `identity.yaml`; workspace `SOUL.md` /
  `MEMORY.md` → agent files; `models.providers` + env keys →
  `llmProviders.addProvider()`. Sessions/tasks are **excluded** in v1.

### Reserved inbound-channel shape (how a live channel *would* map)

Any future inbound channel (OpenClaw, iMessage) lands as a Task via
`orchestrator.chat(message, source, agentId?, modelTier?)` (`orchestrator.ts:75-125`),
with `source` set to the channel (`'imessage'` / a future `'openclaw'`). That is the
single funnel — every external message becomes a Task exactly like a portal chat,
carrying `assignedAgent` and emitting `task.created` / `task.status_changed`.

**fort-ui will reimplement as:** consume the import endpoints as-is for migration. For
inbound, expect channel messages to surface as ordinary Tasks distinguished by
`task.source` (e.g. `imessage` / `openclaw`); render them in the same board/stream and
key the channel badge off `source`. If fort-core adds a live OpenClaw socket, the UI
contract is unchanged — it is still "Task in, `task.status_changed` out." No new UI
primitive needed beyond a source filter/badge.

---

## 6. iOS / Native Shell

The Swift shell (`packages/swift-shell`) is a thin **macOS menu-bar client** of the
**shell IPC WS** (`IPCServer`, port **4001**, path `/shell`) — a different server/
envelope from the portal WS (§2).

### What the shell consumes today

`WebSocketClient.swift` connects to `ws://localhost:4001/shell`
(`WebSocketClient.swift:17-19`), sends `{ action, payload? }` (`:49-65`), and handles
inbound `{ type, ... }` for three types (`:105-128`): `status`, `tasks`, `notification`.

### Server side — IPC envelope `{ type, data }` (`ipc/index.ts:17-25`)

- **Inbound actions** (`handleAction`, `ipc/index.ts:184-307`): `get_status`,
  `get_tasks`, `get_agents`, `run_doctor`, `run_routine`, `spotlight_query`,
  `shortcut_action`, `file_action`, `voice_input`, `notification_policy`.
- **Outbound messages** (`subscribeToBus` + builders, `ipc/index.ts:310-459`):
  - `status` → `{ agents: [{ id, name, status, taskCount }], health: 'green'|'yellow'|'red' }` (`:395-421`)
  - `tasks` → `{ active, queued, history }` counts (`:423-441`)
  - `agents` → `{ agents: [{ id, name, type, status, taskCount, errorCount, capabilities }] }` (`:443-459`)
  - `notification` → `{ title, body, category }` on `task.completed` / `task.failed` /
    `agent.created` / `flow.completed` (`:341-392`)

The shell features (Spotlight, Shortcuts, Global Hotkey, Voice) are scaffolded but
commented out in `App.swift:22-71`; only menu bar + WS + native notifications are live.

### What an iOS client needs from the server

An iOS app would target the **portal WS + REST** (§1–§4), not the macOS shell IPC,
because:
- Chat, gates, and the board live on the portal WS (`4077`); the IPC server (`4001`)
  only exposes aggregate counts + notifications + OS-integration actions.
- Auth is portal-side: `GET /health` is public; `/auth/google`, `/auth/google/callback`,
  `/auth/logout` handle login; all `/api/*` and the WS upgrade are gated by
  `authConfig.authEnabled` (401 for API/WS, redirect for browsers)
  (`index.ts:80-110`). An iOS client needs the Google OAuth flow + session cookie/token
  to reach the portal.

**fort-ui will reimplement as:** for an iOS client, talk **portal WS + REST** for the
real product surface (chat, board, gates, history) and reuse the IPC `{ type, data }`
contract only if a lightweight "ambient status / notification" surface (badge counts,
push) is wanted — that maps cleanly to the `status` / `tasks` / `notification` IPC
messages. Honor the `/health` + `/auth/google*` auth contract; the WS upgrade is
401-gated when auth is enabled.

---

## Quick reference — endpoints & event names

**Portal (4077):**
- HTTP: `GET /health`, `/auth/google`, `/auth/google/callback`, `/auth/logout`,
  `GET /api/chat-history`, `GET|POST /api/import/openclaw[/preview]`,
  `POST /api/agents/create`, `GET /api/agents`, `/api/providers/*`, `/api/hatch/*`,
  `/api/goals*`, `/api/setup-status`, `/api/llm-status`.
- WS in: `chat`, `status`, `tasks`, `tasks.active`, `tasks.query`, `triager.reclassify`,
  `agents`/`agents.list`, `agent.get`, `agent.create`, `approvals.list`,
  `approvals.for_task`, `approval.respond`, `model-choice.respond`.
- WS out: `state` (init), `chat.response`, `task.created`, `task.status_changed`,
  `agent.started/acknowledged/error/classifying/classified/decomposing/decomposed`,
  `triager.reclassified`, `tool.executed/denied/error`, `reflection.insight`,
  `approval.new`, `model-choice.new`.

**Shell IPC (4001 `/shell`):**
- In (`action`): `get_status`, `get_tasks`, `get_agents`, `run_doctor`, `run_routine`,
  `spotlight_query`, `shortcut_action`, `file_action`, `voice_input`,
  `notification_policy`.
- Out (`type`): `status`, `tasks`, `agents`, `notification`, `doctor`, `routine_result`,
  `spotlight_results`, `shortcut_result`, `file_action_result`, `voice_result`,
  `notification_policy`, `error`.
