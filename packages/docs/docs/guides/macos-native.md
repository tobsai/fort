---
sidebar_position: 17
title: macOS Native
description: Swift menu bar app, native notifications, and macOS system integration
---

# macOS Native

Fort includes a Swift menu bar app that provides native macOS integration. The app connects to the TypeScript core over WebSocket and surfaces system status, notifications, and quick actions without opening a terminal.

## Menu Bar App

The menu bar app lives in `packages/swift-shell/` and displays a status dot:

| Color | Meaning |
|-------|---------|
| **Green** | All systems healthy. No active issues. |
| **Yellow** | One or more modules degraded. Fort is operational but something needs attention. |
| **Red** | Critical failure. One or more modules are unhealthy. |
| **Gray** | Disconnected. The TypeScript core is not running or WebSocket connection lost. |

Click the menu bar icon to see:

- Current system status summary
- Active task count
- Recent notifications
- Quick actions (new task, run diagnostics, open dashboard)

## WebSocket Connection

The Swift app connects to the TypeScript core's WebSocket server:

```
ws://localhost:9473/shell
```

The connection carries:

- **Status updates** — Module health changes push to the menu bar in real time.
- **Notifications** — Events flagged for user attention are forwarded as native macOS notifications.
- **Commands** — Quick actions from the menu bar are sent back to the core for execution.

If the connection drops, the status dot turns gray. The app retries every 5 seconds until the core is reachable again.

## Native Notifications

Fort sends macOS notifications for important events:

- Task completed or failed
- Permission approval requests (Tier 3 actions waiting for confirmation)
- Feature flag promoted or rolled back
- Harness self-coding run finished
- Integration errors (OAuth expired, API failures)

Notifications are actionable — click to open the relevant task in the dashboard or approve a pending action.

Configure notification categories in `.fort/data/notifications.yaml`:

```yaml
notifications:
  task:completed: true
  task:failed: true
  permission:approval: true
  flag:promoted: false
  flag:rolledback: true
  harness:completed: true
  integration:error: true
```

## Planned macOS Integrations

The following integrations are stubbed in `packages/swift-shell/` and under active development.

### Spotlight Indexing

Fort will index its data for Spotlight search:

- **Tasks** — Search tasks by title, status, or content.
- **Threads** — Find conversation threads by topic.
- **Memory** — Search memory entities and relationships.

This uses `CSSearchableItem` and `CSSearchableIndex` from CoreSpotlight. When complete, you will be able to type task names or memory entities into Spotlight and jump directly to them.

### Shortcuts Intents

Fort will expose Siri Shortcuts intents:

| Intent | Description |
|--------|-------------|
| **Ask Fort** | Send a natural language query and get a response. |
| **Get Status** | Return current system health and active task count. |
| **Run Routine** | Trigger a named routine from the scheduler. |
| **Search Memory** | Query the memory graph and return matching entities. |
| **Create Task** | Create a new task with a title and optional description. |

These intents enable building Shortcuts automations that incorporate Fort. For example, a morning routine shortcut that asks Fort for your schedule and pending tasks.

### Finder Quick Action

Right-click files in Finder to send them to Fort:

- **Analyze file** — Fort reads and summarizes the file contents.
- **Add to memory** — Store the file's key information in the memory graph.
- **Create task from file** — Generate a task based on the file.

This uses a Finder Sync extension.

### Global Hotkey

**Cmd+Shift+F** opens a floating input window for quick queries:

- Type a question or command.
- Fort responds in the same floating window.
- Press Escape to dismiss.

This provides terminal-free access to Fort from any application.

### Focus Mode Awareness

Fort reads the system's Focus (Do Not Disturb) state:

- During Focus mode, suppress non-critical notifications.
- Only Tier 3 approval requests and critical errors break through.
- When Focus ends, deliver a digest of suppressed notifications.

This uses `NSDoNotDisturbEnabled` from UserDefaults.

### Voice Input and Output

Fort will support voice interaction using Apple's Speech framework:

- **Input** — Dictate commands and queries using system speech recognition.
- **Output** — Fort reads responses aloud using `AVSpeechSynthesizer`.

Voice mode is activated via the menu bar or the global hotkey window.

## Building the Menu Bar App

```bash
cd packages/swift-shell
swift build
```

Run in development mode with verbose logging:

```bash
swift run FortShell --verbose
```

The app requires macOS 13+ and connects to the TypeScript core, which must be running separately.

## Events

The Swift shell publishes and subscribes to ModuleBus events via WebSocket:

- Subscribes: `module:health`, `task:completed`, `task:failed`, `permission:pending`, `flag:promoted`, `flag:rolledback`
- Publishes: `shell:command` (when user triggers a quick action)
