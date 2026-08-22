import { describe, expect, it, vi } from "vitest";

vi.mock("@/auth", () => ({ auth: vi.fn() }));
vi.mock("@/lib/v2-control-client", () => ({
  createSignedFortControlCursorClientFromEnvironment: () => ({
    readPage: async () => {
      throw new Error("fort-control cursor client unavailable");
    },
  }),
}));

import { auth } from "@/auth";
import {
  GET as productionEvents,
  maxDuration,
  runtime,
} from "@/app/api/v2/events/route";
import {
  createV2EventsHandler,
  type CursorPage,
  type CursorPageClient,
  type OwnerSession,
} from "@/lib/v2-events";
import { resolveGatewayOwnerSession } from "@/lib/v2-owner-session";

const owner: OwnerSession = { normalizedEmail: "owner@example.com" };

function request(cursor = "cursor-0"): Request {
  return new Request(
    `https://gateway.test/api/v2/events?cursor=${encodeURIComponent(cursor)}&account_id=forged-account`,
    {
      headers: {
        "last-event-id": cursor,
        "x-fort-account-id": "forged-account",
      },
    },
  );
}

describe("GET /api/v2/events", () => {
  it("starts a new durable stream at the canonical Postgres cursor", async () => {
    const readPage = vi.fn<CursorPageClient["readPage"]>(async () => ({
      events: [],
      nextCursor: "cursor-0",
    }));
    const handler = createV2EventsHandler({
      resolveOwnerSession: async () => owner,
      cursorPages: { readPage },
      cutoffMs: 1_000,
    });

    const response = await handler(new Request("https://gateway.test/api/v2/events"));
    await response.text();

    expect(readPage.mock.calls[0]?.[0].afterCursor).toBe("cursor-0");
  });

  it("uses the bounded Node.js production route and fails closed without signed-client configuration", async () => {
    process.env.FORT_ALLOWLIST = "owner@example.com";
    vi.mocked(auth).mockResolvedValueOnce({ user: { email: "owner@example.com" } } as never);

    const response = await productionEvents(request());
    const body = await response.text();

    expect(runtime).toBe("nodejs");
    expect(maxDuration).toBe(300);
    expect(body).toContain(
      "id: cursor-0\nevent: fort.reconnect\ndata: {\"cursor\":\"cursor-0\",\"reason\":\"control_unavailable\"}",
    );
  });

  it("derives a normalized owner from the authenticated gateway session", async () => {
    process.env.FORT_ALLOWLIST = "owner@example.com";
    vi.mocked(auth).mockResolvedValueOnce({ user: { email: " Owner@Example.COM " } } as never);

    const resolved = await resolveGatewayOwnerSession(request());

    expect(resolved).toEqual(owner);
  });

  it("fails closed before reading fort-control when there is no owner session", async () => {
    const readPage = vi.fn<CursorPageClient["readPage"]>();
    const handler = createV2EventsHandler({
      resolveOwnerSession: async () => null,
      cursorPages: { readPage },
      cutoffMs: 100,
    });

    const response = await handler(request());

    expect(response.status).toBe(401);
    expect(await response.json()).toEqual({ code: "unauthorized" });
    expect(readPage).not.toHaveBeenCalled();
  });

  it("streams ordered durable events for the authenticated owner and ends with a reconnect cursor", async () => {
    const pages: CursorPage[] = [
      {
        events: [{ cursor: "cursor-1", kind: "message.created", data: { message_id: "message-1" } }],
        nextCursor: "cursor-1",
      },
      {
        events: [{ cursor: "cursor-2", kind: "target.finished", data: { target_id: "target-1" } }],
        nextCursor: "cursor-2",
      },
    ];
    const readPage = vi.fn<CursorPageClient["readPage"]>().mockImplementation(async () => pages.shift()!);
    const times = [0, 0, 0, 101];
    const handler = createV2EventsHandler({
      resolveOwnerSession: async () => owner,
      cursorPages: { readPage },
      cutoffMs: 100,
      now: () => times.shift() ?? 101,
    });

    const response = await handler(request());
    const body = await response.text();

    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toBe("text/event-stream; charset=utf-8");
    expect(readPage).toHaveBeenCalledTimes(2);
    expect(readPage.mock.calls[0]?.[0]).toMatchObject({
      owner,
      afterCursor: "cursor-0",
    });
    expect(readPage.mock.calls[0]?.[0]).not.toHaveProperty("accountID");
    expect(readPage.mock.calls[1]?.[0]).toMatchObject({
      owner,
      afterCursor: "cursor-1",
    });
    expect(body).toContain("id: cursor-1\nevent: fort.event\ndata: {\"kind\":\"message.created\"");
    expect(body.indexOf("id: cursor-1")).toBeLessThan(body.indexOf("id: cursor-2"));
    expect(body).toContain(
      "id: cursor-2\nevent: fort.reconnect\ndata: {\"cursor\":\"cursor-2\",\"reason\":\"cutoff\"}",
    );
  });

  it("cuts off a blocked long poll and gives the client its exact durable reconnect cursor", async () => {
    const readPage = vi.fn<CursorPageClient["readPage"]>().mockImplementation(
      () => new Promise(() => undefined),
    );
    const handler = createV2EventsHandler({
      resolveOwnerSession: async () => owner,
      cursorPages: { readPage },
      cutoffMs: 10,
    });

    const response = await handler(request("cursor-9"));
    let timeout: ReturnType<typeof setTimeout> | undefined;
    const body = await Promise.race([
      response.text(),
      new Promise<never>((_resolve, reject) => {
        timeout = setTimeout(() => reject(new Error("stream did not enforce its cutoff")), 250);
      }),
    ]).finally(() => clearTimeout(timeout));

    expect(body).toContain(
      "id: cursor-9\nevent: fort.reconnect\ndata: {\"cursor\":\"cursor-9\",\"reason\":\"cutoff\"}",
    );
  });

  it("never inlines a cursor page whose encoded JSON exceeds 1 MiB", async () => {
    const oversized = "x".repeat(1024 * 1024);
    const handler = createV2EventsHandler({
      resolveOwnerSession: async () => owner,
      cursorPages: {
        readPage: async () => ({
          events: [{ cursor: "cursor-1", kind: "message.created", data: { body: oversized } }],
          nextCursor: "cursor-1",
        }),
      },
      cutoffMs: 100,
      now: () => 0,
    });

    const response = await handler(request());
    const body = await response.text();

    expect(body.length).toBeLessThan(1024);
    expect(body).not.toContain(oversized.slice(0, 100));
    expect(body).toContain(
      "id: cursor-0\nevent: fort.reconnect\ndata: {\"cursor\":\"cursor-0\",\"reason\":\"page_too_large\"}",
    );
  });

  it.each([
    [
      "duplicate event cursors",
      {
        events: [
          { cursor: "cursor-1", kind: "message.created", data: {} },
          { cursor: "cursor-1", kind: "target.finished", data: {} },
        ],
        nextCursor: "cursor-1",
      },
    ],
    [
      "a next cursor that does not identify the last event",
      {
        events: [{ cursor: "cursor-1", kind: "message.created", data: {} }],
        nextCursor: "cursor-2",
      },
    ],
  ])("rejects %s before emitting any part of that page", async (_name, page) => {
    const times = [0, 0, 101];
    const handler = createV2EventsHandler({
      resolveOwnerSession: async () => owner,
      cursorPages: { readPage: async () => page },
      cutoffMs: 100,
      now: () => times.shift() ?? 101,
    });

    const body = await (await handler(request())).text();

    expect(body).not.toContain("event: fort.event");
    expect(body).toContain(
      "id: cursor-0\nevent: fort.reconnect\ndata: {\"cursor\":\"cursor-0\",\"reason\":\"invalid_page\"}",
    );
  });

  it("does not emit the same durable event cursor again across cursor pages", async () => {
    const pages: CursorPage[] = [
      {
        events: [{ cursor: "cursor-1", kind: "message.created", data: {} }],
        nextCursor: "cursor-1",
      },
      {
        events: [{ cursor: "cursor-1", kind: "message.created", data: {} }],
        nextCursor: "cursor-1",
      },
    ];
    const handler = createV2EventsHandler({
      resolveOwnerSession: async () => owner,
      cursorPages: { readPage: async () => pages.shift()! },
      cutoffMs: 100,
      now: () => 0,
    });

    const body = await (await handler(request())).text();

    expect(body.match(/event: fort\.event/g)).toHaveLength(1);
    expect(body).toContain(
      "id: cursor-1\nevent: fort.reconnect\ndata: {\"cursor\":\"cursor-1\",\"reason\":\"invalid_page\"}",
    );
  });

  it("reconnects after one empty unchanged page instead of spinning until cutoff", async () => {
    const readPage = vi.fn<CursorPageClient["readPage"]>().mockResolvedValue({
      events: [],
      nextCursor: "cursor-0",
    });
    const times = [0, 0, 101];
    const handler = createV2EventsHandler({
      resolveOwnerSession: async () => owner,
      cursorPages: { readPage },
      cutoffMs: 100,
      now: () => times.shift() ?? 101,
    });

    const body = await (await handler(request())).text();

    expect(readPage).toHaveBeenCalledTimes(1);
    expect(body).toContain(
      "id: cursor-0\nevent: fort.reconnect\ndata: {\"cursor\":\"cursor-0\",\"reason\":\"idle\"}",
    );
  });
});
