import { describe, expect, it, vi } from "vitest";

import { createFortControlHandoffClient } from "@/lib/v2-handoff-client";
import type { FortControlServiceClient } from "@/lib/v2-service-client";

const owner = { normalizedEmail: "owner@example.com" };

function record() {
  return {
    handoff: {
      id: "handoff:one", account_id: "4af424a4-d81a-47d5-a495-400868883b86", state: "completed",
      created_by_kind: "human", created_by_id: "human:owner", group_turn_id: "turn:group:one",
      source_message_id: "41", recipient_agent_id: "agent:builder",
      recipient_behavior_revision_id: "behavior:builder:1", recipient_binding_revision_id: "binding:builder:1",
      source_conversation_id: "conversation:launch", output_conversation_id: "conversation:launch",
      requested_result: "Review the evidence.", max_agent_messages: 10, max_depth: 3, depth: 1,
      deadline: "2026-08-21T20:10:00Z", created_at: "2026-08-21T20:00:00Z",
    },
    target: {
      id: "target:handoff:one", handoff_id: "handoff:one", conversation_id: "conversation:launch",
      agent_id: "agent:builder", behavior_revision_id: "behavior:builder:1", binding_revision_id: "binding:builder:1",
      participant_id: "participant:builder:group", state: "answered", created_at: "2026-08-21T20:00:00Z",
    },
    projections: [],
    result: { handoff_id: "handoff:one", output_conversation_id: "conversation:launch", message_id: "42", body: "Reviewed." },
  };
}

describe("fort-control Handoff client", () => {
  it("lists and reads one bounded attributed Handoff through signed service routes", async () => {
    const request = vi.fn(async ({ path }: { path: string }) => path.endsWith("handoff%3Aone") ? record() : [record()]);
    const client = createFortControlHandoffClient({ request } as unknown as FortControlServiceClient);

    expect((await client.list({ owner }))[0]?.result?.body).toBe("Reviewed.");
    expect((await client.read({ owner, handoffID: "handoff:one" })).handoff.recipient_agent_id).toBe("agent:builder");
    expect(request).toHaveBeenCalledWith({ owner, path: "/api/v2/handoffs", routeClass: "owner.handoffs.list", method: "GET" });
    expect(request).toHaveBeenCalledWith({ owner, path: "/api/v2/handoffs/handoff%3Aone", routeClass: "owner.handoffs.read", method: "GET" });
  });

  it("rejects copied results, unpinned targets, and malformed parent identity", async () => {
    const malformed = record();
    malformed.target.agent_id = "agent:other";
    await expect(createFortControlHandoffClient({ request: async () => [malformed] } as FortControlServiceClient).list({ owner }))
      .rejects.toThrow("fort-control Handoff read failed");
    await expect(createFortControlHandoffClient({ request: async () => record() } as FortControlServiceClient).read({ owner, handoffID: "../one" }))
      .rejects.toThrow("fort-control Handoff read failed");
  });
});
