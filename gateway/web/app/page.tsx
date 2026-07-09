// / — Machines. A server component that lists registered machines directly
// from the worker (server-side fetch with the shared secret). Each row shows
// the name, the daemon key fingerprint (the string to verify against
// `fort relay join` output), an online dot, a link to the board, and Revoke.

import Link from "next/link";

import RevokeButton from "@/components/revoke-button";
import type { MachineSummary } from "@/lib/types";
import { listMachines } from "@/lib/worker";

export const dynamic = "force-dynamic";

export default async function MachinesPage() {
  let machines: MachineSummary[] = [];
  let error: string | null = null;
  try {
    machines = await listMachines();
  } catch (e) {
    error = e instanceof Error ? e.message : "failed to reach the gateway worker";
  }

  return (
    <div>
      <h1>Machines</h1>
      <p className="subtitle">Forts registered to this gateway.</p>

      {error ? (
        <div className="card">
          <p className="err">Could not load machines: {error}</p>
          <p className="hint">
            Check <code>WORKER_URL</code> and <code>GATEWAY_SECRET</code> in the deployment env.
          </p>
        </div>
      ) : machines.length === 0 ? (
        <div className="card">
          <div className="empty">
            No machines yet. <Link href="/add">Add one</Link> to get a join code.
          </div>
        </div>
      ) : (
        machines.map((m) => (
          <div className="card" key={m.machine_id}>
            <div className="machine-row">
              <div className="machine-main">
                <div className="machine-name">
                  <Link href={`/m/${m.machine_id}`}>{m.name || m.machine_id}</Link>
                </div>
                <div className="fingerprint">{m.fingerprint}</div>
              </div>
              <div className="row">
                <span>
                  <span className={`dot ${m.online ? "online" : "offline"}`} />
                  <span className="status-label">{m.online ? "online" : "offline"}</span>
                </span>
                <Link href={`/m/${m.machine_id}`} className="btn">
                  Board
                </Link>
                <RevokeButton machineId={m.machine_id} name={m.name || m.machine_id} />
              </div>
            </div>
          </div>
        ))
      )}
    </div>
  );
}
