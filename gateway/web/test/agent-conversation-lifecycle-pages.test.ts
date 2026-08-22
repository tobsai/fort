import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

const detail = readFileSync(new URL("../app/agents/[agentId]/page.tsx", import.meta.url), "utf8");
const conversation = readFileSync(new URL("../app/agents/[agentId]/conversations/[conversationId]/page.tsx", import.meta.url), "utf8");
const commands = readFileSync(new URL("../app/agents/[agentId]/conversations/[conversationId]/ConversationCommands.tsx", import.meta.url), "utf8");
const newPage = readFileSync(new URL("../app/agents/[agentId]/conversations/new/page.tsx", import.meta.url), "utf8");
const newForm = readFileSync(new URL("../app/agents/[agentId]/conversations/new/NewConversationForm.tsx", import.meta.url), "utf8");
const proxyRoute = readFileSync(new URL("../app/api/v2/[...segments]/route.ts", import.meta.url), "utf8");

describe("secondary Agent Conversation lifecycle", () => {
  it("offers a parent-scoped New conversation flow without execution identity", () => {
    expect(detail).toContain("conversations/new");
    expect(newPage).toContain("createFortControlAgentDetailClient");
    expect(newForm).toContain("idempotency_key");
    expect(newForm).toContain("title");
    expect(newForm).not.toMatch(/provider|model|machine|binding_revision|behavior_revision/);
    expect(newForm).toContain("router.push");
  });

  it("renders rename, pin, archive, and reopen only for secondary Conversations", () => {
    expect(conversation).toContain("kind={projection.conversation.link.kind}");
    expect(conversation).toContain("pinned={projection.conversation.pinned}");
    expect(commands).toContain('kind === "secondary"');
    for (const action of ["rename", "pin", "unpin", "archive", "reopen"]) {
      expect(commands).toContain(`"${action}"`);
    }
    expect(commands).toContain('method: "PATCH"');
    expect(proxyRoute).toContain("export const PATCH = ownerProxy");
    expect(commands).not.toMatch(/"(?:provider|model|machine_id|binding_revision_id|behavior_revision_id)"\s*:/);
  });
});
