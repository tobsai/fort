// /tunnel authentication tests (miniflare): the daemon socket upgrade is gated
// by a valid device-token Bearer.

import { describe, expect, it } from "vitest";
import { SELF } from "cloudflare:test";

import { BASE, mintAndJoin, newDaemonKey } from "./helpers";

function upgrade(headers: Record<string, string>): Promise<Response> {
  return SELF.fetch(BASE + "/tunnel", { headers: { Upgrade: "websocket", ...headers } });
}

describe("GET /tunnel", () => {
  it("requires a websocket upgrade (426 otherwise)", async () => {
    const res = await SELF.fetch(BASE + "/tunnel", { headers: { Authorization: "Bearer x" } });
    expect(res.status).toBe(426);
  });

  it("rejects a missing bearer with 401", async () => {
    const res = await upgrade({});
    expect(res.status).toBe(401);
  });

  it("rejects an unknown token with 401", async () => {
    const res = await upgrade({ Authorization: "Bearer not-a-real-token" });
    expect(res.status).toBe(401);
  });

  it("upgrades with a valid device token", async () => {
    const { device_token } = await mintAndJoin("daemon-box", newDaemonKey());
    const res = await upgrade({ Authorization: "Bearer " + device_token });
    expect(res.status).toBe(101);
    expect(res.webSocket).toBeTruthy();
    res.webSocket!.accept();
    res.webSocket!.close();
  });
});
