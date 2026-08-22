import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const refresh = readFileSync(new URL("../app/DurableChatRefresh.tsx", import.meta.url), "utf8");
const agentConversation = readFileSync(
  new URL("../app/agents/[agentId]/conversations/[conversationId]/page.tsx", import.meta.url),
  "utf8",
);
const groupConversation = readFileSync(new URL("../app/groups/[groupId]/page.tsx", import.meta.url), "utf8");

describe("durable chat refresh", () => {
  it("resumes the owner event cursor and refreshes chat projections on durable events", () => {
    expect(refresh).toContain("new EventSource");
    expect(refresh).toContain("/api/v2/events?cursor=");
    expect(refresh).toContain('addEventListener("fort.event"');
    expect(refresh).toContain("event.lastEventId");
    expect(refresh).toContain("sessionStorage");
    expect(refresh).toContain("router.refresh()");
  });

  it("mounts the same reconnecting feed in Agent and Group chat pages", () => {
    expect(agentConversation).toContain("<DurableChatRefresh />");
    expect(groupConversation).toContain("<DurableChatRefresh />");
  });
});
