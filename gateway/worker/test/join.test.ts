// Join-flow tests (miniflare): invite auth, code redemption, single-use 409.

import { describe, expect, it } from "vitest";

import { invite, join, newDaemonKey, pubB64 } from "./helpers";

describe("POST /api/relay/invite (internal)", () => {
  it("rejects a missing GATEWAY_SECRET with 401", async () => {
    const res = await invite(null);
    expect(res.status).toBe(401);
  });

  it("rejects a wrong GATEWAY_SECRET with 401", async () => {
    const res = await invite("nope");
    expect(res.status).toBe(401);
  });

  it("mints an XXXX-XXXX code with the right secret", async () => {
    const res = await invite();
    expect(res.status).toBe(200);
    const { code } = (await res.json()) as { code: string };
    expect(code).toMatch(/^[A-Z2-9]{4}-[A-Z2-9]{4}$/);
  });
});

describe("POST /api/relay/join (public)", () => {
  it("redeems a fresh code for a device token + machine id", async () => {
    const { code } = (await (await invite()).json()) as { code: string };
    const res = await join(code, "laptop", pubB64(newDaemonKey()));
    expect(res.status).toBe(200);
    const body = (await res.json()) as { device_token: string; machine_id: string };
    expect(body.device_token).toBeTruthy();
    expect(body.machine_id).toBeTruthy();
  });

  it("is single-use: reusing a code returns 409", async () => {
    const { code } = (await (await invite()).json()) as { code: string };
    const first = await join(code, "a", pubB64(newDaemonKey()));
    expect(first.status).toBe(200);
    const second = await join(code, "b", pubB64(newDaemonKey()));
    expect(second.status).toBe(409);
  });

  it("rejects an unknown code with 409", async () => {
    const res = await join("ZZZZ-ZZZZ", "ghost", pubB64(newDaemonKey()));
    expect(res.status).toBe(409);
  });

  it("rejects a non-32-byte public key with 400", async () => {
    const { code } = (await (await invite()).json()) as { code: string };
    const res = await join(code, "bad", "dG9vc2hvcnQ="); // "tooshort"
    expect(res.status).toBe(400);
  });

  it("survives a concurrent double-redeem: exactly one wins", async () => {
    const { code } = (await (await invite()).json()) as { code: string };
    const [a, b] = await Promise.all([
      join(code, "x", pubB64(newDaemonKey())),
      join(code, "y", pubB64(newDaemonKey())),
    ]);
    const statuses = [a.status, b.status].sort();
    expect(statuses).toEqual([200, 409]);
  });
});
