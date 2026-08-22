import { describe, expect, it, vi } from "vitest";

import { createFortControlAgentConversationClient } from "@/lib/v2-agent-conversation-client";
import type { FortControlServiceClient } from "@/lib/v2-service-client";

const owner = { normalizedEmail: "owner@example.com" };

function projection() {
  return {
    conversation: {
      conversation: { id: "conversation:home", title: "Home", state: "open" },
      link: { agent_id: "agent:research", conversation_id: "conversation:home", kind: "canonical" },
      pinned: false,
    },
    messages: [{
      id: 1,
      conversation_id: "conversation:home",
      turn_id: "turn:one",
      author_kind: "human",
      author_id: "human:owner",
      body: "Investigate this.",
      created_at: "2026-08-21T15:00:00Z",
    }],
    turns: [{
      id: "turn:one",
      conversation_id: "conversation:home",
      client_turn_id: "client:one",
      prompt_message_id: 1,
      through_message_id: 1,
      membership_revision_id: "binding:one",
      context_manifest_id: "context:one",
      state: "queued",
      created_at: "2026-08-21T15:00:00Z",
    }],
    targets: [{
      id: "target:one",
      turn_id: "turn:one",
      conversation_id: "conversation:home",
      agent_id: "agent:research",
      behavior_revision_id: "behavior:one",
      binding_revision_id: "binding:one",
      participant_id: "participant:one",
      run_id: "run:one",
      state: "queued",
      attempt_count: 0,
      created_at: "2026-08-21T15:00:00Z",
      updated_at: "2026-08-21T15:00:00Z",
    }],
  };
}

describe("stable Agent conversation client", () => {
  it("reads one exact parent-scoped durable projection", async () => {
    const request = vi.fn<FortControlServiceClient["request"]>(async () => projection());
    const client = createFortControlAgentConversationClient({ request });

    const result = await client.read({ owner, agentID: "agent:research", conversationID: "conversation:home" });

    expect(result.messages[0]?.body).toBe("Investigate this.");
    expect(result.targets[0]?.binding_revision_id).toBe("binding:one");
    expect(request).toHaveBeenCalledWith({
      owner,
      path: "/api/v2/agents/agent%3Aresearch/conversations/conversation%3Ahome",
      routeClass: "owner.agent_conversations.read",
      method: "GET",
    });
  });

  it.each([
    ["foreign message", (value: ReturnType<typeof projection>) => { value.messages[0]!.conversation_id = "conversation:other"; }],
    ["foreign target", (value: ReturnType<typeof projection>) => { value.targets[0]!.agent_id = "agent:other"; }],
    ["unbound target", (value: ReturnType<typeof projection>) => { value.targets[0]!.binding_revision_id = ""; }],
    ["unknown author", (value: ReturnType<typeof projection>) => { value.messages[0]!.author_kind = "machine"; }],
  ])("rejects a %s in the control projection", async (_name, mutate) => {
    const value = projection();
    mutate(value);
    const client = createFortControlAgentConversationClient({ request: async () => value });

    await expect(client.read({ owner, agentID: "agent:research", conversationID: "conversation:home" }))
      .rejects.toThrow("fort-control Agent conversation read failed");
  });
});
