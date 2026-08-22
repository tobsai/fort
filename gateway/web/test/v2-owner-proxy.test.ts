import { describe, expect, it, vi } from "vitest";

import {
  createV2OwnerProxyHandler,
  type ResolveOwnerSession,
} from "@/lib/v2-owner-proxy";
import {
  FortControlResponseError,
  type FortControlServiceClient,
} from "@/lib/v2-service-client";

const owner = { normalizedEmail: "owner@example.com" };

function fixture(input?: {
  resolvedOwner?: typeof owner | null;
  response?: unknown;
  error?: Error;
}) {
  const resolveOwnerSession = vi.fn<ResolveOwnerSession>(
    async () => input?.resolvedOwner === undefined ? owner : input.resolvedOwner,
  );
  const request = vi.fn<FortControlServiceClient["request"]>(async () => {
    if (input?.error) throw input.error;
    return input?.response ?? { ok: true };
  });
  return {
    resolveOwnerSession,
    service: { request },
    request,
    handler: createV2OwnerProxyHandler({ resolveOwnerSession, service: { request } }),
  };
}

describe("authenticated /api/v2 owner proxy", () => {
  it.each([
    ["https://gateway.test/api/v2/agents?state=open", "/api/v2/agents?state=open", "owner.agents.list"],
    ["https://gateway.test/api/v2/agents/agent%3Aresearch", "/api/v2/agents/agent%3Aresearch", "owner.agents.read"],
    ["https://gateway.test/api/v2/agents/agent:research", "/api/v2/agents/agent%3Aresearch", "owner.agents.read"],
    [
      "https://gateway.test/api/v2/agents/agent%3Aresearch/conversations",
      "/api/v2/agents/agent%3Aresearch/conversations",
      "owner.agent_conversations.list",
    ],
    [
      "https://gateway.test/api/v2/agents/agent%3Aresearch/conversations/canonical",
      "/api/v2/agents/agent%3Aresearch/conversations/canonical",
      "owner.agent_conversations.canonical",
    ],
    [
      "https://gateway.test/api/v2/agents/agent%3Aresearch/conversations/conversation%3Ahome",
      "/api/v2/agents/agent%3Aresearch/conversations/conversation%3Ahome",
      "owner.agent_conversations.read",
    ],
    ["https://gateway.test/api/v2/groups?state=open", "/api/v2/groups?state=open", "owner.groups.list"],
    ["https://gateway.test/api/v2/groups/group:launch", "/api/v2/groups/group%3Alaunch", "owner.groups.read"],
    ["https://gateway.test/api/v2/handoffs", "/api/v2/handoffs", "owner.handoffs.list"],
    ["https://gateway.test/api/v2/handoffs/handoff:one", "/api/v2/handoffs/handoff%3Aone", "owner.handoffs.read"],
    [
      "https://gateway.test/api/v2/agents/agent%3Aresearch/routines",
      "/api/v2/agents/agent%3Aresearch/routines",
      "owner.routines.list",
    ],
    [
      "https://gateway.test/api/v2/agents/agent%3Aresearch/routines/routine%3Aweekly/runs",
      "/api/v2/agents/agent%3Aresearch/routines/routine%3Aweekly/runs",
      "owner.routines.runs",
    ],
  ])("maps GET %s to its exact signed control route", async (url, path, routeClass) => {
    const { handler, request } = fixture({ response: [] });

    const response = await handler(new Request(url, {
      headers: { "x-fort-account-id": "forged", "x-fort-service-assertion": "forged" },
    }));

    expect(response.status).toBe(200);
    expect(response.headers.get("cache-control")).toBe("no-store");
    expect(request).toHaveBeenCalledWith({ owner, path, routeClass, method: "GET" });
    expect(request.mock.calls[0]?.[0]).not.toHaveProperty("accountID");
  });

  it.each([
    ["https://gateway.test/api/v2/agents", "owner.agents.create"],
    ["https://gateway.test/api/v2/agents/agent%3Aresearch/rebind", "owner.agents.rebind"],
    ["https://gateway.test/api/v2/agents/agent%3Aresearch/conversations", "owner.agent_conversations.create"],
    ["https://gateway.test/api/v2/groups", "owner.groups.create"],
    ["https://gateway.test/api/v2/groups/group%3Alaunch/members", "owner.group_members.replace"],
    ["https://gateway.test/api/v2/groups/group%3Alaunch/turns", "owner.group_turns.send"],
    ["https://gateway.test/api/v2/handoffs", "owner.handoffs.create"],
    ["https://gateway.test/api/v2/handoffs/handoff%3Aone/cancel", "owner.handoffs.cancel"],
    ["https://gateway.test/api/v2/agents/agent%3Aresearch/routines", "owner.routines.create"],
    ["https://gateway.test/api/v2/agents/agent%3Aresearch/routines/routine%3Aweekly/test", "owner.routines.test"],
    [
      "https://gateway.test/api/v2/agents/agent%3Aresearch/conversations/conversation%3Ahome/turns",
      "owner.agent_turns.send",
    ],
    [
      "https://gateway.test/api/v2/agents/agent%3Aresearch/conversations/conversation%3Ahome/targets/target%3A1/retry",
      "owner.agent_targets.retry",
    ],
    [
      "https://gateway.test/api/v2/agents/agent%3Aresearch/conversations/conversation%3Ahome/targets/target%3A1/cancel",
      "owner.agent_targets.cancel",
    ],
  ])("forwards the exact bounded JSON command body for %s", async (url, routeClass) => {
    const body = JSON.stringify({ idempotency_key: "idem-1", client_turn_id: "client-1", text: "hello" });
    const { handler, request } = fixture({ response: { accepted: true } });

    const response = await handler(new Request(url, {
      method: "POST",
      headers: { "content-type": "application/json", "x-fort-account-id": "forged" },
      body,
    }));

    expect(response.status).toBe(200);
    expect(request).toHaveBeenCalledWith({
      owner,
      path: new URL(url).pathname,
      routeClass,
      method: "POST",
      body,
    });
  });

  it("forwards only the exact secondary Conversation mutation route as PATCH", async () => {
    const body = JSON.stringify({
      idempotency_key: "conversation:rename:1",
      action: "rename",
      expected_title: "Market map",
      title: "Market landscape",
    });
    const { handler, request } = fixture({ response: { pinned: false } });
    const response = await handler(new Request(
      "https://gateway.test/api/v2/agents/agent:research/conversations/conversation:market",
      { method: "PATCH", headers: { "content-type": "application/json" }, body },
    ));

    expect(response.status).toBe(200);
    expect(request).toHaveBeenCalledWith({
      owner,
      path: "/api/v2/agents/agent%3Aresearch/conversations/conversation%3Amarket",
      routeClass: "owner.agent_conversations.mutate",
      method: "PATCH",
      body,
    });
  });

  it("forwards only the exact closed Agent mutation route as PATCH", async () => {
    const body = JSON.stringify({
      action: "profile",
      idempotency_key: "agent:profile:2",
      expected_profile_revision_id: "profile:researcher:1",
      profile: {
        name: "Research Lead",
        title: "Primary-source research",
        avatar_url: "",
        hidden: false,
        pinned: true,
        sort_order: 1,
      },
    });
    const { handler, request } = fixture({ response: { profile: { name: "Research Lead" } } });
    const response = await handler(new Request(
      "https://gateway.test/api/v2/agents/agent:research",
      { method: "PATCH", headers: { "content-type": "application/json" }, body },
    ));

    expect(response.status).toBe(200);
    expect(request).toHaveBeenCalledWith({
      owner,
      path: "/api/v2/agents/agent%3Aresearch",
      routeClass: "owner.agents.mutate",
      method: "PATCH",
      body,
    });
  });

  it("forwards only the exact closed Group lifecycle route as PATCH", async () => {
    const body = JSON.stringify({
      action: "rename",
      idempotency_key: "group:rename:2",
      expected_title: "Launch crew",
      title: "Launch council",
    });
    const { handler, request } = fixture({ response: { conversation: { title: "Launch council" } } });
    const response = await handler(new Request(
      "https://gateway.test/api/v2/groups/group:launch",
      { method: "PATCH", headers: { "content-type": "application/json" }, body },
    ));

    expect(response.status).toBe(200);
    expect(request).toHaveBeenCalledWith({
      owner,
      path: "/api/v2/groups/group%3Alaunch",
      routeClass: "owner.groups.mutate",
      method: "PATCH",
      body,
    });
  });

  it("forwards only the exact Agent-owned Routine mutation route as PATCH", async () => {
    const body = JSON.stringify({ idempotency_key: "routine:revalidate:2", action: "revalidate" });
    const { handler, request } = fixture({ response: { state: "active" } });
    const response = await handler(new Request(
      "https://gateway.test/api/v2/agents/agent:research/routines/routine:weekly",
      { method: "PATCH", headers: { "content-type": "application/json" }, body },
    ));

    expect(response.status).toBe(200);
    expect(request).toHaveBeenCalledWith({
      owner,
      path: "/api/v2/agents/agent%3Aresearch/routines/routine%3Aweekly",
      routeClass: "owner.routines.mutate",
      method: "PATCH",
      body,
    });
  });

  it("fails closed before control access without an authenticated owner", async () => {
    const { handler, request } = fixture({ resolvedOwner: null });

    const response = await handler(new Request("https://gateway.test/api/v2/agents"));

    expect(response.status).toBe(401);
    expect(await response.json()).toEqual({ code: "unauthorized" });
    expect(request).not.toHaveBeenCalled();
  });

  it.each([
    "https://gateway.test/api/v2/agents/a%2Fb",
    "https://gateway.test/api/v2/agents/a/conversations/b?account_id=forged",
    "https://gateway.test/api/v2/handoffs/handoff:one?target_id=forged",
    "https://gateway.test/api/v2/unknown",
  ])("rejects an unrecognized or ambiguous child route: %s", async (url) => {
    const { handler, request } = fixture();

    const response = await handler(new Request(url));

    expect(response.status).toBe(404);
    expect(await response.json()).toEqual({ code: "not_found" });
    expect(request).not.toHaveBeenCalled();
  });

  it("rejects a non-JSON or oversized command before control access", async () => {
    const { handler, request } = fixture();
    const url = "https://gateway.test/api/v2/agents/a/conversations/b/turns";

    const wrongType = await handler(new Request(url, { method: "POST", body: "{}" }));
    const oversized = await handler(new Request(url, {
      method: "POST",
      headers: { "content-type": "application/json", "content-length": String(4 * 1024 * 1024 + 1) },
      body: "{}",
    }));

    expect(wrongType.status).toBe(415);
    expect(oversized.status).toBe(413);
    expect(request).not.toHaveBeenCalled();
  });

  it("preserves an allowlisted semantic control error and hides an internal failure", async () => {
    const first = fixture({ error: new FortControlResponseError(409, "idempotency_conflict") });
    const second = fixture({ error: new Error("database password leaked") });

    const semantic = await first.handler(new Request("https://gateway.test/api/v2/groups"));
    const hidden = await second.handler(new Request("https://gateway.test/api/v2/groups"));

    expect(semantic.status).toBe(409);
    expect(await semantic.json()).toEqual({ code: "idempotency_conflict" });
    expect(hidden.status).toBe(502);
    expect(await hidden.json()).toEqual({ code: "control_unavailable" });
  });
});
