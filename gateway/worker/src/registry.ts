// registry: the authoritative, strongly-consistent registry for the gateway.
//
// One singleton instance (addressed by idFromName("registry")) owns:
//   • join codes            code:<CODE>   -> { expires_at }
//   • the machine registry  machine:<id>  -> MachineRecord
//   • a token index         token:<tok>   -> machine_id
//
// A Durable Object (not KV) because join-code consumption must be exactly-once:
// the DO input gate keeps the read-check-delete of `join` atomic, so a code can
// never be redeemed twice even under a burst of concurrent posts (409 on reuse).
// Callable over Durable Object RPC — the router awaits these methods directly.

import { DurableObject } from "cloudflare:workers";

import { newJoinCode, normalizeCode } from "./codes";
import type { Env, MachineRecord } from "./types";
import { randomToken } from "./types";

interface CodeRecord {
  expires_at: number;
}

/** Outcome of a join attempt. `ok:false` distinguishes bad vs expired codes. */
export type JoinResult =
  | { ok: true; device_token: string; machine_id: string }
  | { ok: false; reason: "unknown" | "expired" };

export class RegistryDO extends DurableObject<Env> {
  private get storage(): DurableObjectStorage {
    return this.ctx.storage;
  }

  /** mintCode creates a fresh single-use join code with a TTL (seconds). */
  async mintCode(ttlSeconds: number): Promise<string> {
    // Collisions are astronomically unlikely; loop anyway to guarantee unique.
    for (let attempt = 0; attempt < 5; attempt++) {
      const code = newJoinCode();
      const existing = await this.storage.get<CodeRecord>("code:" + code);
      if (existing) continue;
      const rec: CodeRecord = { expires_at: Date.now() + ttlSeconds * 1000 };
      await this.storage.put("code:" + code, rec);
      return code;
    }
    throw new Error("registry: could not mint a unique code");
  }

  /**
   * join redeems a code for a device token + machine id and registers the
   * machine. Single-use: the code is deleted in the same gated section, so a
   * replay finds nothing and returns {ok:false}. Expired codes are also purged.
   */
  async join(code: string, name: string, publicKey: string): Promise<JoinResult> {
    const key = "code:" + normalizeCode(code);
    const rec = await this.storage.get<CodeRecord>(key);
    if (!rec) return { ok: false, reason: "unknown" };
    // Consume first (atomic with the read under the input gate) so no second
    // caller can also observe it as valid.
    await this.storage.delete(key);
    if (Date.now() > rec.expires_at) return { ok: false, reason: "expired" };

    const machine_id = randomToken(16);
    const device_token = randomToken(32);
    const machine: MachineRecord = {
      machine_id,
      name,
      public_key: publicKey,
      device_token,
      created_at: Date.now(),
    };
    await this.storage.put("machine:" + machine_id, machine);
    await this.storage.put("token:" + device_token, machine_id);
    return { ok: true, device_token, machine_id };
  }

  /** listMachines returns every registered machine record. */
  async listMachines(): Promise<MachineRecord[]> {
    const map = await this.storage.list<MachineRecord>({ prefix: "machine:" });
    return [...map.values()];
  }

  /** machineIdByToken resolves a device token to its machine id, or null. */
  async machineIdByToken(token: string): Promise<string | null> {
    return (await this.storage.get<string>("token:" + token)) ?? null;
  }

  /** getMachine returns one machine record (for revoke authorization). */
  async getMachine(machineId: string): Promise<MachineRecord | null> {
    return (await this.storage.get<MachineRecord>("machine:" + machineId)) ?? null;
  }

  /** removeMachine deletes a machine and its token index. Returns true if present. */
  async removeMachine(machineId: string): Promise<boolean> {
    const machine = await this.storage.get<MachineRecord>("machine:" + machineId);
    if (!machine) return false;
    await this.storage.delete("machine:" + machineId);
    await this.storage.delete("token:" + machine.device_token);
    return true;
  }
}
