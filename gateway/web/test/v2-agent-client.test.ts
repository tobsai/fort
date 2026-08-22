import { createHash } from "node:crypto";

import { describe, expect, it, vi } from "vitest";

import { createSignedFortControlAgentClient } from "@/lib/v2-agent-client";

const key = new TextEncoder().encode("0123456789abcdef0123456789abcdef");

describe("signed fort-control Agent client", () => {
  it("maps the authenticated owner server-side and signs an empty GET body for the Agent roster", async () => {
    const fetcher = vi.fn(async (_input: string | URL | Request, _init?: RequestInit) =>
      Response.json([
        {
          agent: { id: "agent:researcher", account_id: "4af424a4-d81a-47d5-a495-400868883b86", state: "open" },
          profile: { name: "Researcher", title: "Evidence and synthesis", pinned: true },
          binding: { provider: "openclaw", requested_model: "main", computer_id: "computer:studio" },
          home: { id: "conversation:researcher:home", title: "Home", state: "open" },
        },
      ]),
    );
    const client = createSignedFortControlAgentClient({
      origin: "https://control.test",
      accountByEmail: { "owner@example.com": "4af424a4-d81a-47d5-a495-400868883b86" },
      key,
      keyID: "service-2026-08",
      ttlSeconds: 30,
      nowSeconds: () => 1_787_331_600,
      nonce: () => "908b3b526cf8472e91b1e6f71fb8df99",
      fetch: fetcher,
    });

    const records = await client.list({ owner: { normalizedEmail: "owner@example.com" } });

    expect(records[0]?.profile.name).toBe("Researcher");
    const [url, init] = fetcher.mock.calls[0]!;
    expect(url).toBe("https://control.test/api/v2/agents?state=open");
    expect(init?.method).toBe("GET");
    expect(init?.body).toBeUndefined();
    const token = new Headers(init?.headers).get("x-fort-service-assertion")!;
    const claims = JSON.parse(Buffer.from(token.split(".")[1]!, "base64url").toString("utf8")) as Record<string, unknown>;
    expect(claims).toMatchObject({
      account_id: "4af424a4-d81a-47d5-a495-400868883b86",
      route_class: "owner.agents.list",
      aud: "fort-control",
      request_digest: createHash("sha256").update("").digest("hex"),
      iat: 1_787_331_600,
      exp: 1_787_331_630,
      nonce: "908b3b526cf8472e91b1e6f71fb8df99",
    });
  });

  it("fails closed for an owner that is not in the server-only account map", async () => {
    const client = createSignedFortControlAgentClient({
      origin: "https://control.test",
      accountByEmail: { "owner@example.com": "4af424a4-d81a-47d5-a495-400868883b86" },
      key,
      keyID: "service-2026-08",
      ttlSeconds: 30,
      fetch: vi.fn(async (_input: string | URL | Request, _init?: RequestInit) => new Response()),
    });

    await expect(client.list({ owner: { normalizedEmail: "other@example.com" } })).rejects.toThrow(
      "fort-control Agent client unavailable",
    );
  });
});
