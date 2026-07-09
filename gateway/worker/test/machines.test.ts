// Machines-list tests (miniflare): the list reflects a joined machine with the
// fingerprint computed by @fort/gateway-shared, and is secret-gated.

import { describe, expect, it } from "vitest";

import { fingerprint } from "@fort/gateway-shared";
import { listMachines, mintAndJoin, newDaemonKey } from "./helpers";
import type { MachineSummary } from "../src/types";

describe("GET /api/relay/machines (internal)", () => {
  it("rejects a missing secret with 401", async () => {
    const res = await listMachines(null);
    expect(res.status).toBe(401);
  });

  it("lists a joined machine with the shared-computed fingerprint, offline", async () => {
    const kp = newDaemonKey();
    const { machine_id } = await mintAndJoin("workstation", kp);

    const res = await listMachines();
    expect(res.status).toBe(200);
    const { machines } = (await res.json()) as { machines: MachineSummary[] };

    const mine = machines.find((m) => m.machine_id === machine_id);
    expect(mine).toBeDefined();
    expect(mine!.name).toBe("workstation");
    expect(mine!.fingerprint).toBe(fingerprint(kp.publicKey)); // derived, not stored
    expect(mine!.online).toBe(false); // no daemon socket attached
  });
});
