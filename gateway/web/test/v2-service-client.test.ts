import { createHash } from "node:crypto";

import { describe, expect, it, vi } from "vitest";

import {
  createFortControlServiceClient,
  FortControlResponseError,
} from "@/lib/v2-service-client";

const accountID = "4af424a4-d81a-47d5-a495-400868883b86";
const key = new TextEncoder().encode("0123456789abcdef0123456789abcdef");

describe("generic signed fort-control service client", () => {
  it("binds owner, route class, path body, audience, nonce, and lifetime", async () => {
    const fetcher = vi.fn(async (_input: string | URL | Request, _init?: RequestInit) =>
      Response.json({ groups: [] }),
    );
    const client = createFortControlServiceClient({
      origin: "https://control.test",
      accountByEmail: { "owner@example.com": accountID },
      key,
      keyID: "service-2026-08",
      ttlSeconds: 30,
      nowSeconds: () => 1_787_331_600,
      nonce: () => "908b3b526cf8472e91b1e6f71fb8df99",
      fetch: fetcher,
    });

    await client.request({
      owner: { normalizedEmail: "owner@example.com" },
      path: "/api/v2/groups?state=open",
      routeClass: "owner.groups.list",
      method: "GET",
    });

    const [url, init] = fetcher.mock.calls[0]!;
    expect(url).toBe("https://control.test/api/v2/groups?state=open");
    const token = new Headers(init?.headers).get("x-fort-service-assertion")!;
    const claims = JSON.parse(Buffer.from(token.split(".")[1]!, "base64url").toString("utf8")) as Record<string, unknown>;
    expect(claims).toEqual({
      account_id: accountID,
      route_class: "owner.groups.list",
      aud: "fort-control",
      request_digest: createHash("sha256").update("").digest("hex"),
      iat: 1_787_331_600,
      exp: 1_787_331_630,
      nonce: "908b3b526cf8472e91b1e6f71fb8df99",
    });
  });

  it("rejects untrusted paths, unknown owners, oversized bodies, and oversized responses before parsing", async () => {
    const fetcher = vi.fn(async () => new Response("x".repeat(4 * 1024 * 1024 + 1)));
    const client = createFortControlServiceClient({
      origin: "https://control.test",
      accountByEmail: { "owner@example.com": accountID },
      key,
      keyID: "service-2026-08",
      ttlSeconds: 30,
      fetch: fetcher,
    });

    await expect(client.request({
      owner: { normalizedEmail: "owner@example.com" }, path: "https://evil.test/api/v2/groups",
      routeClass: "owner.groups.list", method: "GET",
    })).rejects.toThrow("fort-control service client unavailable");
    for (const path of [
      "/api/v2/groups/../agents",
      "/api/v2/groups/%2e%2e/agents",
      "/api/v2/groups/%2Fcontrol",
      "/api/v2/groups/%5ccontrol",
    ]) {
      await expect(client.request({
        owner: { normalizedEmail: "owner@example.com" }, path,
        routeClass: "owner.groups.list", method: "GET",
      })).rejects.toThrow("fort-control service client unavailable");
    }
    await expect(client.request({
      owner: { normalizedEmail: "other@example.com" }, path: "/api/v2/groups",
      routeClass: "owner.groups.list", method: "GET",
    })).rejects.toThrow("fort-control service client unavailable");
    await expect(client.request({
      owner: { normalizedEmail: "owner@example.com" }, path: "/api/v2/groups",
      routeClass: "owner.groups.create", method: "POST", body: "x".repeat(4 * 1024 * 1024 + 1),
    })).rejects.toThrow("fort-control request failed");
    await expect(client.request({
      owner: { normalizedEmail: "owner@example.com" }, path: "/api/v2/groups",
      routeClass: "owner.groups.list", method: "GET",
    })).rejects.toThrow("fort-control request failed");
  });

  it.each([
    [400, "invalid_request"],
    [404, "not_found"],
    [409, "conflict"],
    [413, "payload_too_large"],
    [503, "seat_unready"],
  ])("preserves the bounded semantic %i response for the authenticated gateway", async (status, code) => {
    const client = createFortControlServiceClient({
      origin: "https://control.test",
      accountByEmail: { "owner@example.com": accountID },
      key,
      keyID: "service-2026-08",
      ttlSeconds: 30,
      fetch: async () => Response.json({ code }, { status }),
    });

    const request = client.request({
      owner: { normalizedEmail: "owner@example.com" },
      path: "/api/v2/groups",
      routeClass: "owner.groups.list",
      method: "GET",
    });

    await expect(request).rejects.toEqual(new FortControlResponseError(status, code));
  });

  it.each([
    [401, "unauthorized"],
    [500, "database_error"],
    [404, "not a machine-readable code"],
  ])("does not expose the control service's %i failure", async (status, code) => {
    const client = createFortControlServiceClient({
      origin: "https://control.test",
      accountByEmail: { "owner@example.com": accountID },
      key,
      keyID: "service-2026-08",
      ttlSeconds: 30,
      fetch: async () => Response.json({ code }, { status }),
    });

    await expect(client.request({
      owner: { normalizedEmail: "owner@example.com" },
      path: "/api/v2/groups",
      routeClass: "owner.groups.list",
      method: "GET",
    })).rejects.toThrow("fort-control request failed");
  });
});
