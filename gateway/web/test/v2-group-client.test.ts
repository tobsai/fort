import { describe, expect, it, vi } from "vitest";

import { createFortControlGroupClient } from "@/lib/v2-group-client";
import type { FortControlServiceClient } from "@/lib/v2-service-client";

const owner = { normalizedEmail: "owner@example.com" };

describe("fort-control Group client", () => {
  it("lists stable Groups with frozen membership over the signed service seam", async () => {
    const request = vi.fn(async () => ([{
      group: {
        id: "group:launch", account_id: "4af424a4-d81a-47d5-a495-400868883b86",
        conversation_id: "conversation:launch", state: "open",
        current_membership_revision_id: "membership:launch:1", created_at: "2026-08-21T20:00:00Z",
      },
      conversation: { id: "conversation:launch", title: "Product launch", state: "open" },
      membership: {
        id: "membership:launch:1", group_id: "group:launch", revision: 1,
        members: [{ agent_id: "agent:researcher", position: 0 }, { agent_id: "agent:builder", position: 1 }],
      },
      member_bindings: [
        { agent_id: "agent:researcher", behavior_revision_id: "behavior:r:1", binding_revision_id: "binding:r:1", participant_id: "participant:r" },
        { agent_id: "agent:builder", behavior_revision_id: "behavior:b:1", binding_revision_id: "binding:b:1", participant_id: "participant:b" },
      ],
    }]));
    const client = createFortControlGroupClient({ request } as FortControlServiceClient);

    const groups = await client.list({ owner });

    expect(groups[0]?.conversation.title).toBe("Product launch");
    expect(groups[0]?.membership.members).toHaveLength(2);
    expect(request).toHaveBeenCalledWith({
      owner,
      path: "/api/v2/groups?state=open",
      routeClass: "owner.groups.list",
      method: "GET",
    });
  });

  it("reads one exact Group projection with its single frozen fan-out wave", async () => {
    const record = {
      group: {
        id: "group:launch", account_id: "4af424a4-d81a-47d5-a495-400868883b86",
        conversation_id: "conversation:launch", state: "open",
        current_membership_revision_id: "membership:launch:1", created_at: "2026-08-21T20:00:00Z",
      },
      conversation: { id: "conversation:launch", title: "Product launch", state: "open" },
      membership: {
        id: "membership:launch:1", group_id: "group:launch", revision: 1,
        members: [{ agent_id: "agent:researcher", position: 0 }, { agent_id: "agent:builder", position: 1 }],
      },
      member_bindings: [
        { agent_id: "agent:researcher", behavior_revision_id: "behavior:r:1", binding_revision_id: "binding:r:1", participant_id: "participant:r" },
        { agent_id: "agent:builder", behavior_revision_id: "behavior:b:1", binding_revision_id: "binding:b:1", participant_id: "participant:b" },
      ],
    };
    const request = vi.fn(async () => ({
      group: record,
      messages: [
        { id: 1, conversation_id: "conversation:launch", turn_id: "turn:one", author_kind: "human", author_id: "human:owner", body: "Compare.", created_at: "2026-08-21T20:01:00Z" },
        { id: 2, conversation_id: "conversation:launch", turn_id: "turn:one", target_id: "target:r", author_kind: "agent", author_id: "agent:researcher", author_agent_id: "agent:researcher", body: "The evidence agrees.", created_at: "2026-08-21T20:02:00Z" },
      ],
      turns: [{
        message: { id: 1, conversation_id: "conversation:launch", turn_id: "turn:one", author_kind: "human", author_id: "human:owner", body: "Compare.", created_at: "2026-08-21T20:01:00Z" },
        envelope: {
          id: "turn:one", group_id: "group:launch", conversation_id: "conversation:launch",
          client_turn_id: "client:one", membership_revision_id: "membership:launch:1",
          selection: "everyone", concurrency_policy: "concurrent", max_agent_messages: 10,
          max_handoff_depth: 3, deadline: "2026-08-21T20:11:00Z", created_at: "2026-08-21T20:01:00Z",
        },
        recipients: record.member_bindings,
        initial_targets: [
          { id: "target:r", group_turn_id: "turn:one", wave: 0, agent_id: "agent:researcher", behavior_revision_id: "behavior:r:1", binding_revision_id: "binding:r:1", participant_id: "participant:r", state: "queued", created_at: "2026-08-21T20:01:00Z" },
          { id: "target:b", group_turn_id: "turn:one", wave: 0, agent_id: "agent:builder", behavior_revision_id: "behavior:b:1", binding_revision_id: "binding:b:1", participant_id: "participant:b", state: "queued", created_at: "2026-08-21T20:01:00Z" },
        ],
      }],
    }));
    const client = createFortControlGroupClient({ request } as FortControlServiceClient);

    const result = await client.read({ owner, groupID: "group:launch" });

    expect(result.turns[0]?.initial_targets.map((target) => target.agent_id)).toEqual(["agent:researcher", "agent:builder"]);
    expect(result.messages[1]).toMatchObject({ author_agent_id: "agent:researcher", body: "The evidence agrees." });
    expect(request).toHaveBeenCalledWith({
      owner,
      path: "/api/v2/groups/group%3Alaunch",
      routeClass: "owner.groups.read",
      method: "GET",
    });
  });

  it("preserves historical turn targets and Agent attribution after membership advances", async () => {
    const current = {
      group: {
        id: "group:launch", account_id: "4af424a4-d81a-47d5-a495-400868883b86",
        conversation_id: "conversation:launch", state: "open",
        current_membership_revision_id: "membership:launch:2", created_at: "2026-08-21T20:00:00Z",
      },
      conversation: { id: "conversation:launch", title: "Product launch", state: "open" },
      membership: {
        id: "membership:launch:2", group_id: "group:launch", revision: 2,
        members: [{ agent_id: "agent:builder", position: 0 }, { agent_id: "agent:reviewer", position: 1 }],
      },
      member_bindings: [
        { agent_id: "agent:builder", behavior_revision_id: "behavior:b:2", binding_revision_id: "binding:b:2", participant_id: "participant:b:2" },
        { agent_id: "agent:reviewer", behavior_revision_id: "behavior:v:1", binding_revision_id: "binding:v:1", participant_id: "participant:v:1" },
      ],
    };
    const historicalRecipient = {
      agent_id: "agent:researcher", behavior_revision_id: "behavior:r:1",
      binding_revision_id: "binding:r:1", participant_id: "participant:r:1",
    };
    const request = vi.fn(async () => ({
      group: current,
      messages: [
        { id: 1, conversation_id: "conversation:launch", turn_id: "turn:old", author_kind: "human", author_id: "human:owner", body: "Research this.", created_at: "2026-08-21T20:01:00Z" },
        { id: 2, conversation_id: "conversation:launch", turn_id: "turn:old", target_id: "target:old", author_kind: "agent", author_id: "agent:researcher", author_agent_id: "agent:researcher", body: "Historical answer.", created_at: "2026-08-21T20:02:00Z" },
      ],
      turns: [{
        message: { id: 1, conversation_id: "conversation:launch", turn_id: "turn:old", author_kind: "human", author_id: "human:owner", body: "Research this.", created_at: "2026-08-21T20:01:00Z" },
        envelope: {
          id: "turn:old", group_id: "group:launch", conversation_id: "conversation:launch",
          client_turn_id: "client:old", membership_revision_id: "membership:launch:1",
          selection: "explicit", concurrency_policy: "sequential", max_agent_messages: 10,
          max_handoff_depth: 3, deadline: "2026-08-21T20:11:00Z", created_at: "2026-08-21T20:01:00Z",
        },
        recipients: [historicalRecipient],
        initial_targets: [{
          id: "target:old", group_turn_id: "turn:old", wave: 0, ...historicalRecipient,
          state: "answered", created_at: "2026-08-21T20:01:00Z",
        }],
      }],
    }));

    const projection = await createFortControlGroupClient({ request } as FortControlServiceClient)
      .read({ owner, groupID: "group:launch" });

    expect(projection.group.membership.id).toBe("membership:launch:2");
    expect(projection.turns[0]?.envelope.membership_revision_id).toBe("membership:launch:1");
    expect(projection.turns[0]?.recipients[0]).toEqual(historicalRecipient);
    expect(projection.messages[1]?.author_agent_id).toBe("agent:researcher");
  });

  it("rejects malformed or single-Agent Group projections", async () => {
    const service = { request: async () => [{
      group: { id: "group:bad", conversation_id: "conversation:bad", state: "open" },
      conversation: { id: "conversation:bad", title: "Bad", state: "open" },
      membership: { id: "membership:bad:1", group_id: "group:bad", revision: 1, members: [{ agent_id: "agent:one", position: 0 }] },
      member_bindings: [],
    }] } as FortControlServiceClient;

    await expect(createFortControlGroupClient(service).list({ owner })).rejects.toThrow("fort-control Group read failed");
  });
});
