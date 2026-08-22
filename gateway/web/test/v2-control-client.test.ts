import { describe, expect, it, vi } from "vitest";

import {
  createSignedFortControlCursorClient,
  createSignedFortControlCursorClientFromEnvironment,
  issueEventsReadAssertion,
} from "@/lib/v2-control-client";

const accountID = "4af424a4-d81a-47d5-a495-400868883b86";
const key = new TextEncoder().encode("0123456789abcdef0123456789abcdef");
const nonce = "908b3b526cf8472e91b1e6f71fb8df99";
// This literal was produced independently by controlapi.IssueServiceAssertion
// with the same body bytes and is the cross-language compatibility vector.
const expectedToken =
  "eyJhbGciOiJIUzI1NiIsImtpZCI6InNlcnZpY2UtMjAyNi0wOCJ9.eyJhY2NvdW50X2lkIjoiNGFmNDI0YTQtZDgxYS00N2Q1LWE0OTUtNDAwODY4ODgzYjg2Iiwicm91dGVfY2xhc3MiOiJvd25lci5ldmVudHMucmVhZCIsImF1ZCI6ImZvcnQtY29udHJvbCIsInJlcXVlc3RfZGlnZXN0IjoiYWY2YjJiODkyNmQ0ZWM1ZTM5ODliZWEzODRlNzIwOTVlMWUzYmI5NTE5ZjE3ZTkzZjU1MjE3NjE4MGY0ZmExYyIsImlhdCI6MTc4NzMzMTYwMCwiZXhwIjoxNzg3MzMxNjMwLCJub25jZSI6IjkwOGIzYjUyNmNmODQ3MmU5MWIxZTZmNzFmYjhkZjk5In0.BNSYUAo-6B19UVvQRcqVTaAbTXV73ZMe5Ul3qpcIXog";

describe("signed fort-control cursor client", () => {
  it("matches the deterministic token issued by the Go verifier implementation", () => {
    expect(
      issueEventsReadAssertion({
        accountID,
        body: '{"after_cursor":"cursor-9"}',
        issuedAtSeconds: 1_787_331_600,
        expiresAtSeconds: 1_787_331_630,
        key,
        keyID: "service-2026-08",
        nonce,
      }),
    ).toBe(expectedToken);
  });

  it("maps the authenticated email server-side and signs the exact bounded cursor body", async () => {
    const fetcher = vi.fn(
      async (_input: string | URL | Request, _init?: RequestInit): Promise<Response> =>
        Response.json({
          events: [{ cursor: "cursor-10", kind: "message.created", data: { message_id: "message-1" } }],
          next_cursor: "cursor-10",
        }),
    );
    const client = createSignedFortControlCursorClient({
      origin: "https://control.test",
      accountByEmail: { "owner@example.com": accountID },
      key,
      keyID: "service-2026-08",
      ttlSeconds: 30,
      nowSeconds: () => 1_787_331_600,
      nonce: () => nonce,
      fetch: fetcher,
    });
    const signal = new AbortController().signal;

    const page = await client.readPage({
      owner: { normalizedEmail: "owner@example.com" },
      afterCursor: "cursor-9",
      signal,
    });

    expect(page).toEqual({
      events: [{ cursor: "cursor-10", kind: "message.created", data: { message_id: "message-1" } }],
      nextCursor: "cursor-10",
    });
    expect(fetcher).toHaveBeenCalledTimes(1);
    const [url, options] = fetcher.mock.calls[0]!;
    expect(url).toBe("https://control.test/api/v2/events/cursor");
    expect(options).toMatchObject({
      method: "POST",
      body: '{"after_cursor":"cursor-9"}',
      cache: "no-store",
      signal,
    });
    const headers = new Headers(options?.headers);
    expect(headers.get("x-fort-service-assertion")).toBe(expectedToken);
    expect(headers.get("content-type")).toBe("application/json");
    expect(JSON.stringify(options)).not.toContain(accountID);
  });

  it("fails before fetch when the authenticated email has no server-side account mapping", async () => {
    const fetcher = vi.fn();
    const client = createSignedFortControlCursorClient({
      origin: "https://control.test",
      accountByEmail: {},
      key,
      keyID: "service-2026-08",
      ttlSeconds: 30,
      fetch: fetcher,
    });

    await expect(
      client.readPage({
        owner: { normalizedEmail: "owner@example.com" },
        afterCursor: "cursor-9",
        signal: new AbortController().signal,
      }),
    ).rejects.toThrow("fort-control cursor client unavailable");
    expect(fetcher).not.toHaveBeenCalled();
  });

  it.each([
    ["non-OK", new Response("private", { status: 502 })],
    ["invalid JSON", new Response("not-json", { headers: { "content-type": "application/json" } })],
    ["more than 1 MiB", new Response("x".repeat(1024 * 1024 + 1))],
    ["unsafe cursor", Response.json({ events: [], next_cursor: "bad\ncursor" })],
    [
      "duplicate cursor page",
      Response.json({
        events: [
          { cursor: "cursor-10", kind: "one", data: {} },
          { cursor: "cursor-10", kind: "two", data: {} },
        ],
        next_cursor: "cursor-10",
      }),
    ],
    [
      "mismatched next cursor",
      Response.json({
        events: [{ cursor: "cursor-10", kind: "one", data: {} }],
        next_cursor: "cursor-11",
      }),
    ],
  ])("rejects a %s control response", async (_name, response) => {
    const client = createSignedFortControlCursorClient({
      origin: "https://control.test",
      accountByEmail: { "owner@example.com": accountID },
      key,
      keyID: "service-2026-08",
      ttlSeconds: 30,
      fetch: async () => response,
    });

    await expect(
      client.readPage({
        owner: { normalizedEmail: "owner@example.com" },
        afterCursor: "cursor-9",
        signal: new AbortController().signal,
      }),
    ).rejects.toThrow("fort-control cursor read failed");
  });

  it("fails closed when signed-client environment configuration is absent", async () => {
    const client = createSignedFortControlCursorClientFromEnvironment({});

    await expect(
      client.readPage({
        owner: { normalizedEmail: "owner@example.com" },
        afterCursor: "cursor-9",
        signal: new AbortController().signal,
      }),
    ).rejects.toThrow("fort-control cursor client unavailable");
  });

  it("builds the production client only from server-side environment configuration", async () => {
    const fetcher = vi.fn(
      async (_input: string | URL | Request, _init?: RequestInit): Promise<Response> =>
        Response.json({ events: [], next_cursor: "cursor-9" }),
    );
    vi.stubGlobal("fetch", fetcher);
    try {
      const client = createSignedFortControlCursorClientFromEnvironment({
        FORT_CONTROL_ORIGIN: "https://control.test",
        FORT_CONTROL_ACCOUNT_MAP: JSON.stringify({ "owner@example.com": accountID }),
        FORT_CONTROL_ASSERTION_KID: "service-2026-08",
        FORT_CONTROL_ASSERTION_KEY_B64URL: Buffer.from(key).toString("base64url"),
        FORT_CONTROL_ASSERTION_TTL_SECONDS: "30",
      });

      await expect(
        client.readPage({
          owner: { normalizedEmail: "owner@example.com" },
          afterCursor: "cursor-9",
          signal: new AbortController().signal,
        }),
      ).resolves.toEqual({ events: [], nextCursor: "cursor-9" });
      expect(fetcher).toHaveBeenCalledTimes(1);
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("refuses an assertion lifetime longer than the Go verifier's 60-second maximum", () => {
    expect(() =>
      createSignedFortControlCursorClient({
        origin: "https://control.test",
        accountByEmail: { "owner@example.com": accountID },
        key,
        keyID: "service-2026-08",
        ttlSeconds: 61,
      }),
    ).toThrow("fort-control cursor client unavailable");
  });
});
