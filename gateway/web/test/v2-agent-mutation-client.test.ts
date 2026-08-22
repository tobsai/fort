import { describe, expect, it, vi } from "vitest";

import { createAgentMutationClient } from "@/lib/v2-agent-mutation-client";

type Fetcher = (input: string | URL | Request, init?: RequestInit) => Promise<Response>;

describe("stable Agent mutation client", () => {
  it("sends only the closed profile revision payload to the exact Agent", async () => {
    const fetcher = vi.fn<Fetcher>(async () => new Response(JSON.stringify({ agent: { id: "agent:researcher" } }), {
      status: 200,
      headers: { "content-type": "application/json" },
    }));
    const client = createAgentMutationClient(fetcher);

    await client.update({
      action: "profile",
      agentID: "agent:researcher",
      idempotencyKey: "profile:edit:one",
      expectedProfileRevisionID: "profile:researcher:1",
      profile: {
        name: "Research Lead",
        title: "Primary-source research",
        avatarURL: "https://images.example/researcher.png",
        hidden: false,
        pinned: true,
        sortOrder: 2,
      },
      // A forged caller may have an open-ended object at runtime. The client
      // still constructs its wire body from the closed presentation fields.
      provider: "forged",
      machine: "forged",
    } as Parameters<typeof client.update>[0] & { provider: string; machine: string });

    expect(fetcher).toHaveBeenCalledOnce();
    const [path, init] = fetcher.mock.calls[0]!;
    expect(path).toBe("/api/v2/agents/agent%3Aresearcher");
    expect(init).toMatchObject({ method: "PATCH", headers: { "content-type": "application/json" } });
    expect(JSON.parse(String(init?.body))).toEqual({
      action: "profile",
      idempotency_key: "profile:edit:one",
      expected_profile_revision_id: "profile:researcher:1",
      profile: {
        name: "Research Lead",
        title: "Primary-source research",
        avatar_url: "https://images.example/researcher.png",
        hidden: false,
        pinned: true,
        sort_order: 2,
      },
    });
  });

  it("sends Behavior intent while keeping the current Binding opaque", async () => {
    const fetcher = vi.fn<Fetcher>(async () => new Response(JSON.stringify({
      agent: { agent: { id: "agent:researcher" } },
      transition: { successor_binding_revision_id: "binding:researcher:2" },
    }), { status: 200, headers: { "content-type": "application/json" } }));
    const client = createAgentMutationClient(fetcher);

    await client.update({
      action: "behavior",
      agentID: "agent:researcher",
      idempotencyKey: "behavior:edit:one",
      expectedBehaviorRevisionID: "behavior:researcher:1",
      expectedBindingRevisionID: "binding:researcher:1",
      behavior: {
        role: "Researcher",
        standingInstructions: "Cite primary sources.",
        enabledSkills: ["web", "documents"],
        enabledTools: ["browser"],
        promptMaterial: "Be concise.",
      },
      adapterID: "forged",
      model: "forged",
    } as Parameters<typeof client.update>[0] & { adapterID: string; model: string });

    const [, init] = fetcher.mock.calls[0]!;
    expect(JSON.parse(String(init?.body))).toEqual({
      action: "behavior",
      idempotency_key: "behavior:edit:one",
      expected_behavior_revision_id: "behavior:researcher:1",
      expected_binding_revision_id: "binding:researcher:1",
      behavior: {
        role: "Researcher",
        standing_instructions: "Cite primary sources.",
        enabled_skills: ["web", "documents"],
        enabled_tools: ["browser"],
        prompt_material: "Be concise.",
      },
    });
  });

  it("fails closed when the response belongs to another Agent", async () => {
    const fetcher = vi.fn<Fetcher>(async () => new Response(JSON.stringify({ agent: { id: "agent:other" } }), {
      status: 202,
      headers: { "content-type": "application/json" },
    }));
    const client = createAgentMutationClient(fetcher);

    await expect(client.update({
      action: "profile",
      agentID: "agent:researcher",
      idempotencyKey: "profile:edit:two",
      expectedProfileRevisionID: "profile:researcher:1",
      profile: {
        name: "Researcher", title: "", avatarURL: "", hidden: false, pinned: false, sortOrder: 0,
      },
    })).rejects.toThrow("agent_update_failed");
    expect(fetcher).toHaveBeenCalledOnce();
  });
});
