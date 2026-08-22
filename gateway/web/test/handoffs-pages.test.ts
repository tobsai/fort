import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

const root = new URL("..", import.meta.url);
const list = readFileSync(new URL("app/handoffs/page.tsx", root), "utf8");
const detail = readFileSync(new URL("app/handoffs/[handoffId]/page.tsx", root), "utf8");
const cancel = readFileSync(new URL("app/handoffs/HandoffCancelButton.tsx", root), "utf8");
const composer = readFileSync(new URL("app/handoffs/HandoffComposer.tsx", root), "utf8");
const direct = readFileSync(
  new URL("app/agents/[agentId]/conversations/[conversationId]/page.tsx", root),
  "utf8",
);
const group = readFileSync(new URL("app/groups/[groupId]/page.tsx", root), "utf8");

describe("durable Handoff pages", () => {
  it("shows recipient attribution, immutable revision evidence, bounds, and one authoritative result", () => {
    expect(list).toContain("recipient_agent_id");
    expect(list).toContain("recipient_binding_revision_id");
    expect(list).toContain("record.handoff.depth");
    expect(list).toContain("record.result.body");
    expect(detail).toContain("Source message");
    expect(detail).toContain("Authoritative result");
  });

  it("cancels only the persisted Handoff by opaque ID and idempotency key", () => {
    expect(cancel).toContain("encodeURIComponent(handoffID)");
    expect(cancel).toContain("idempotency_key");
    expect(cancel).not.toContain("target_id");
    expect(cancel).not.toContain("binding_revision_id");
  });

  it("creates a human-directed Handoff from an exact durable message and explicit recipient", () => {
    expect(direct).toContain("HandoffComposer");
    expect(group).toContain("HandoffComposer");
    expect(composer).toContain("source_conversation_id");
    expect(composer).toContain("source_message_id");
    expect(composer).toContain("recipient_agent_id");
    expect(composer).toContain("context_message_ids");
    expect(composer).toContain("requested_result");
    expect(composer).toContain("hard_deadline");
    expect(composer).not.toContain("binding_revision_id");
    expect(composer).not.toContain("behavior_revision_id");
    expect(composer).not.toContain("provider:");
    expect(composer).not.toContain("machine:");
  });

  it("inherits the exact persisted Group Turn deadline while direct messages retain a choice", () => {
    expect(group).toContain("turnDeadlineByID");
    expect(group).toContain("hardDeadline: turnDeadline");
    expect(composer).toContain("hardDeadline?: string");
    expect(composer).toContain("selectedMessage?.hardDeadline ??");
    expect(composer).toContain("Inherited from the source Group Turn");
    expect(composer).toContain("deadlineMinutes");
  });
});
