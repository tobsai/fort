import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

const root = new URL("..", import.meta.url);
const page = readFileSync(
  new URL("app/agents/[agentId]/conversations/[conversationId]/page.tsx", root),
  "utf8",
);
const commands = readFileSync(new URL("app/agents/[agentId]/conversations/[conversationId]/ConversationCommands.tsx", root), "utf8");

describe("durable stable-Agent conversation page", () => {
  it("renders the full parent-scoped projection and pinned execution evidence", () => {
    expect(page).toContain("createFortControlAgentConversationClient");
    expect(page).toContain("projection.messages");
    expect(page).toContain("projection.targets");
    expect(page).toContain("binding_revision_id");
    expect(page).toContain("attempt_count");
    expect(page).toContain("ConversationCommands");
  });

  it("sends, retries, and cancels only through exact authenticated v2 child routes", () => {
    expect(commands).toContain('"use client"');
    expect(commands).toContain("/api/v2/agents/");
    expect(commands).toContain("hard_deadline");
    expect(commands).toContain('action: "retry" | "cancel"');
    expect(commands).toContain("/${action}");
    expect(commands).toContain("router.refresh()");
    expect(commands).not.toContain("provider:");
    expect(commands).not.toContain("model:");
    expect(commands).not.toContain("machine:");
    expect(commands).not.toContain("binding_revision_id:");
  });
});
