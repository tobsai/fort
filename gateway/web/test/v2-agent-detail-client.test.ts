import { describe, expect, it, vi } from "vitest";

import { createFortControlAgentDetailClient } from "@/lib/v2-agent-detail-client";
import type { FortControlServiceClient } from "@/lib/v2-service-client";

const owner = { normalizedEmail: "owner@example.com" };
const agent = {
  agent: {
    id: "agent:researcher", account_id: "4af424a4-d81a-47d5-a495-400868883b86", state: "open",
    canonical_conversation_id: "conversation:researcher:home",
    current_profile_revision_id: "profile:researcher:1",
    current_behavior_revision_id: "behavior:researcher:1",
    current_binding_revision_id: "binding:researcher:1",
  },
  profile: { id: "profile:researcher:1", agent_id: "agent:researcher", revision: 1, name: "Researcher", title: "", avatar_url: "", hidden: false, pinned: false, sort_order: 0 },
  behavior: {
    id: "behavior:researcher:1", agent_id: "agent:researcher", revision: 1, role: "Researcher",
    standing_instructions: "Cite primary sources.", enabled_skills: ["web"], enabled_tools: ["browser"], prompt_material: "",
  },
  binding: {
    id: "binding:researcher:1", agent_id: "agent:researcher", behavior_revision_id: "behavior:researcher:1",
    provider: "openclaw", requested_model: "main", computer_id: "worker:studio",
  },
  home: { id: "conversation:researcher:home", title: "Home", state: "open" },
};

describe("fort-control Agent detail client", () => {
  it("reads the exact stable Agent and parent-scoped conversation list", async () => {
    const request = vi.fn(async (input: { path: string }) => input.path.endsWith("/conversations") ? [
      {
        conversation: { id: "conversation:researcher:home", title: "Home", state: "open" },
        link: { agent_id: "agent:researcher", conversation_id: "conversation:researcher:home", kind: "canonical" },
        pinned: false,
      },
      {
        conversation: { id: "conversation:researcher:market", title: "Market map", state: "open" },
        link: { agent_id: "agent:researcher", conversation_id: "conversation:researcher:market", kind: "secondary" },
        pinned: true,
        pinned_at: "2026-08-21T21:00:00Z",
      },
    ] : agent);
    const client = createFortControlAgentDetailClient({ request } as FortControlServiceClient);

    const [record, conversations] = await Promise.all([
      client.get({ owner, agentID: "agent:researcher" }),
      client.listConversations({ owner, agentID: "agent:researcher" }),
    ]);

    expect(record.profile.name).toBe("Researcher");
    expect(conversations.map((item) => item.conversation.title)).toEqual(["Home", "Market map"]);
    expect(request.mock.calls.map(([input]) => input.path)).toEqual([
      "/api/v2/agents/agent%3Aresearcher",
      "/api/v2/agents/agent%3Aresearcher/conversations",
    ]);
  });

  it("rejects a child Conversation that belongs to another Agent", async () => {
    const service = { request: async () => [{
      conversation: { id: "conversation:other", title: "Other", state: "open" },
      link: { agent_id: "agent:other", conversation_id: "conversation:other", kind: "secondary" },
      pinned: false,
    }] } as FortControlServiceClient;

    await expect(createFortControlAgentDetailClient(service).listConversations({
      owner, agentID: "agent:researcher",
    })).rejects.toThrow("fort-control Agent detail read failed");
  });

  it("rejects Agent edit state whose current revisions do not match the returned records", async () => {
    const malformed = structuredClone(agent);
    malformed.behavior.id = "behavior:other:1";
    const client = createFortControlAgentDetailClient({ request: async () => malformed });

    await expect(client.get({ owner, agentID: "agent:researcher" }))
      .rejects.toThrow("fort-control Agent detail read failed");
  });
});
