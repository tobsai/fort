# SPEC-009: Notifications

**Status:** Draft  
**Author:** Lewis  
**Date:** 2026-03-25  
**Priority:** MEDIUM  
**Depends on:** Spec 001, Spec 005

---

## Problem

When agents complete tasks, fail, or need human input, there is no way for Fort to proactively notify the user. You must be actively watching the dashboard to see state changes. For an agentic system running scheduled tasks and background work, passive polling is insufficient.

## Goals

1. In-app notification bell with unread count in dashboard header
2. Notifications for: task completed, task failed, task needs approval (Tier 2/3 tools), agent started/stopped
3. Notification persist to DB — survive page refresh
4. Mark as read (individual + mark all read)
5. Push via WebSocket (real-time delivery when dashboard is open)

## Non-Goals (v1)

- Push notifications to mobile (APNs/FCM) — future
- Email notifications
- Per-notification-type settings
- Notification grouping/threading

---

## Design

### Data Model

```sql
CREATE TABLE notifications (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,         -- 'task.completed' | 'task.failed' | 'approval.required' | 'agent.started' | 'agent.stopped'
  title TEXT NOT NULL,
  body TEXT,
  entity_type TEXT,           -- 'task' | 'agent' | 'approval'
  entity_id TEXT,             -- ID of the related entity
  read INTEGER DEFAULT 0,
  created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX idx_notifications_read ON notifications(read);
CREATE INDEX idx_notifications_created ON notifications(created_at);
```

### NotificationStore

File: `packages/core/src/notifications/store.ts`

```typescript
export class NotificationStore {
  constructor(db: Database.Database) {}
  initSchema(): void
  create(notification: CreateNotificationInput): Notification
  markRead(id: string): void
  markAllRead(): void
  list(options?: { unreadOnly?: boolean; limit?: number }): Notification[]
  getUnreadCount(): number
}
```

### NotificationService

File: `packages/core/src/notifications/service.ts`

Subscribes to bus events and creates notifications:

| Bus Event | Notification |
|---|---|
| `task.completed` | "Task completed: {title}" |
| `task.failed` | "Task failed: {title}" (with error in body) |
| `approval.required` | "Approval needed: {tool} in {task}" |
| `agent.started` | "Agent {name} started" |
| `agent.stopped` | "Agent {name} stopped" |

### WebSocket Handlers

```
'notifications.list'         → { unreadOnly?: boolean, limit?: number }
'notifications.unread_count' → number
'notification.mark_read'     → { id: string }
'notifications.mark_all_read' → void
```

### Real-time Push

When NotificationService creates a notification, it also calls a registered push callback. Fort wires this to broadcast to all connected WebSocket clients:

```typescript
// In Fort.ts
notificationService.onNotification((n) => {
  wsServer.broadcast('notification.new', n);
});
```

### Dashboard: Notification Bell

In the dashboard header (DashboardPage/layout):
1. Bell icon with unread count badge (red dot with number)
2. Click → dropdown panel with last 20 notifications
3. Each notification: icon by type, title, body snippet, time ago
4. "Mark all as read" button
5. Click notification → navigate to relevant entity (task detail, approval prompt)

Unread count updated in real-time via `notification.new` WS push.

---

## Test Criteria

- NotificationStore.create() writes to DB
- NotificationStore.markRead() updates read flag
- NotificationStore.getUnreadCount() returns correct count
- NotificationService creates notification on task.completed bus event
- NotificationService creates notification on task.failed bus event
- Push callback invoked on new notification
- All existing 510 tests still pass

---

## Rollback

NotificationService is a passive subscriber. If not initialized, no notifications are generated. Feature flag: `FORT_NOTIFICATIONS_ENABLED=false`.
