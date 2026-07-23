// Frame-proxy route shape: /api/req is a thin authenticated relay. It must (a)
// 401 with no session, (b) 400 when machine_id/frame is missing, and (c) on the
// happy path forward the opaque frame to the worker and return {frames}. The
// session guard and the worker client are mocked so this is a pure shape test.

import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/session", () => ({ requireSession: vi.fn() }));
vi.mock("@/lib/worker", () => ({ relayReq: vi.fn() }));

import { POST } from "@/app/api/req/route";
import { requireSession } from "@/lib/session";
import { relayReq } from "@/lib/worker";

const mReq = vi.mocked(requireSession);
const mRelay = vi.mocked(relayReq);

function post(body: unknown): Request {
  return new Request("https://web.test/api/req", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

describe("POST /api/req", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("401s when there is no session", async () => {
    mReq.mockResolvedValue(new Response("no", { status: 401 }));
    const res = await POST(post({ machine_id: "m1", frame: { stream: "s", kind: "req" } }));
    expect(res.status).toBe(401);
    expect(mRelay).not.toHaveBeenCalled();
  });

  it("400s when machine_id/frame is missing", async () => {
    mReq.mockResolvedValue(null); // authenticated
    const res = await POST(post({ machine_id: "m1" }));
    expect(res.status).toBe(400);
    expect(mRelay).not.toHaveBeenCalled();
  });

  it("forwards the frame and returns the daemon replies", async () => {
    mReq.mockResolvedValue(null);
    const reply = { stream: "s", kind: "hs2", b64: "AAAA" };
    mRelay.mockResolvedValue([reply]);

    const frame = { stream: "s", kind: "hs1", b64: "BBBB" };
    const res = await POST(post({ machine_id: "m1", frame }));

    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ frames: [reply] });
    expect(mRelay).toHaveBeenCalledWith("m1", frame);
  });

  it.each([503, 504])("preserves a worker %s so native clients can diagnose the relay", async (status) => {
    mReq.mockResolvedValue(null);
    mRelay.mockRejectedValue(Object.assign(new Error("daemon did not respond"), { status }));

    const res = await POST(post({ machine_id: "m1", frame: { stream: "s", kind: "hs1" } }));

    expect(res.status).toBe(status);
    expect(await res.json()).toEqual({ error: "daemon did not respond" });
  });

  it("maps an unexpected worker failure to 502", async () => {
    mReq.mockResolvedValue(null);
    mRelay.mockRejectedValue(new Error("fetch failed"));

    const res = await POST(post({ machine_id: "m1", frame: { stream: "s", kind: "hs1" } }));

    expect(res.status).toBe(502);
  });
});
