// / — Secure control-plane entry points. A server component that lists relay
// daemons directly from the worker (server-side fetch with the shared secret).
// Opening any entry point reaches its authoritative all-machine Command Deck;
// these records are not task-target pins.

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
      <div className="page-heading">
        <div>
          <h1>Remote Command Deck</h1>
          <p className="subtitle">
            Choose a secure control-plane entry point. Its deck operates the whole connected mesh.
          </p>
        </div>
        <Link href="/add" className="btn btn-secondary">
          Add a Fort
        </Link>
      </div>

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
            No secure entry points are connected yet. <Link href="/add">Add one</Link> to get a join code.
          </div>
        </div>
      ) : (
        <div className="machine-grid">
          {machines.map((m) => (
            <article className="machine-card" key={m.machine_id}>
              <div className="machine-card-header">
                <span className={`status-dot ${m.online ? "accepted" : "idle"}`} />
                <strong>{m.name || m.machine_id}</strong>
                <span className={`status-pill ${m.online ? "state-delivered" : "state-idle"}`}>
                  {m.online ? "Online" : "Offline"}
                </span>
              </div>
              <p className="machine-entry-copy">
                Secure relay entry point for one authoritative all-machine deck—not a task target.
              </p>
              <div>
                <div className="hint">Relay daemon fingerprint</div>
                <div className="fingerprint">{m.fingerprint}</div>
              </div>
              <div className="machine-card-actions">
                <Link href={`/m/${m.machine_id}`} className="btn btn-primary">
                  Open all-machine deck
                </Link>
                <RevokeButton machineId={m.machine_id} name={m.name || m.machine_id} />
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  );
}
