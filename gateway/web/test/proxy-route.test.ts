// Frame-proxy route shape: /api/req is a thin authenticated relay. It must (a)
// 401 with no session, (b) 400 when machine_id/frame is missing, and (c) on the
// happy path forward the opaque frame to the worker and return {frames}. The
// session guard and the worker client are mocked so this is a pure shape test.

import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/session", () => ({ requireSession: vi.fn() }));
vi.mock("@/lib/worker", () => ({ relayReq: vi.fn(), relaySse: vi.fn() }));

import { POST as postReq } from "@/app/api/req/route";
import { POST as postSse } from "@/app/api/sse/route";
import { requireSession } from "@/lib/session";
import { relayReq, relaySse } from "@/lib/worker";

const mReq = vi.mocked(requireSession);
const mRelay = vi.mocked(relayReq);
const mRelaySse = vi.mocked(relaySse);
const requestID = "018f3f1c-7d3a-7c1d-a176-9c52c606c6e4";
const canonicalRequestID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

function post(body: unknown, id?: string): Request {
  return new Request("https://web.test/api/req", {
    method: "POST",
    headers: {
      "content-type": "application/json",
      ...(id ? { "X-Fort-Request-ID": id } : {}),
    },
    body: JSON.stringify(body),
  });
}

describe("POST /api/req", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("401s when there is no session", async () => {
    mReq.mockResolvedValue(new Response("no", { status: 401 }));
    const res = await postReq(post({ machine_id: "m1", frame: { stream: "s", kind: "req" } }));
    expect(res.status).toBe(401);
    expect(mRelay).not.toHaveBeenCalled();
  });

  it("400s when machine_id/frame is missing", async () => {
    mReq.mockResolvedValue(null); // authenticated
    const res = await postReq(post({ machine_id: "m1" }));
    expect(res.status).toBe(400);
    expect(mRelay).not.toHaveBeenCalled();
  });

  it("forwards the frame and returns the daemon replies", async () => {
    mReq.mockResolvedValue(null);
    const reply = { stream: "s", kind: "hs2", b64: "AAAA" };
    mRelay.mockResolvedValue([reply]);

    const frame = { stream: "s", kind: "hs1", b64: "BBBB" };
    const res = await postReq(post({ machine_id: "m1", frame }, requestID));

    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ frames: [reply] });
    expect(res.headers.get("X-Fort-Request-ID")).toBe(requestID);
    expect(mRelay).toHaveBeenCalledWith("m1", frame, requestID);
  });

  it("generates a canonical correlation ID when an older caller omits it", async () => {
    mReq.mockResolvedValue(null);
    mRelay.mockResolvedValue([]);

    const res = await postReq(post({ machine_id: "m1", frame: { stream: "s", kind: "bye" } }));
    const generated = res.headers.get("X-Fort-Request-ID");

    expect(generated).toMatch(canonicalRequestID);
    expect(mRelay).toHaveBeenCalledWith("m1", { stream: "s", kind: "bye" }, generated);
  });

  it.each([503, 504])("preserves a worker %s so native clients can diagnose the relay", async (status) => {
    mReq.mockResolvedValue(null);
    mRelay.mockRejectedValue(Object.assign(new Error("daemon did not respond"), { status }));

    const res = await postReq(post({ machine_id: "m1", frame: { stream: "s", kind: "hs1" } }, requestID));

    expect(res.status).toBe(status);
    expect(res.headers.get("X-Fort-Request-ID")).toBe(requestID);
    expect(await res.json()).toEqual({ error: "relay request failed", request_id: requestID });
  });

  it("maps an unexpected worker failure to a bounded 502 without logging payloads or auth", async () => {
    mReq.mockResolvedValue(null);
    mRelay.mockRejectedValue(new Error("fetch failed: bearer secret; ciphertext AAAA; body private"));
    const log = vi.spyOn(console, "error").mockImplementation(() => undefined);

    const res = await postReq(post({ machine_id: "m1", frame: { stream: "s", kind: "hs1", b64: "AAAA" } }, requestID));

    expect(res.status).toBe(502);
    expect(await res.json()).toEqual({ error: "relay request failed", request_id: requestID });
    expect(JSON.stringify(log.mock.calls)).toContain(requestID);
    expect(JSON.stringify(log.mock.calls)).not.toContain("bearer secret");
    expect(JSON.stringify(log.mock.calls)).not.toContain("AAAA");
    expect(JSON.stringify(log.mock.calls)).not.toContain("private");
    log.mockRestore();
  });
});

describe("POST /api/sse", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("forwards and returns the same bounded request ID without inspecting the stream", async () => {
    mReq.mockResolvedValue(null);
    mRelaySse.mockResolvedValue(
      new Response('{"stream":"s","kind":"res","b64":"AAAA"}\n', {
        status: 200,
        headers: { "content-type": "application/x-ndjson" },
      }),
    );
    const frame = { stream: "s", kind: "req", b64: "SEALED" };

    const res = await postSse(post({ machine_id: "m1", frame }, requestID));

    expect(res.status).toBe(200);
    expect(res.headers.get("X-Fort-Request-ID")).toBe(requestID);
    expect(mRelaySse).toHaveBeenCalledWith("m1", frame, requestID);
  });

  it("returns a bounded correlated error without exposing the worker response", async () => {
    mReq.mockResolvedValue(null);
    mRelaySse.mockResolvedValue(new Response("private worker body AAAA", { status: 504 }));

    const res = await postSse(
      post({ machine_id: "m1", frame: { stream: "s", kind: "req", b64: "SEALED" } }, requestID),
    );

    expect(res.status).toBe(502);
    expect(res.headers.get("X-Fort-Request-ID")).toBe(requestID);
    expect(await res.json()).toEqual({ error: "relay stream failed", request_id: requestID });
  });
});
