// /legacy — the bounded v1 rollback surface. Stable Agents are the primary
// product; this page keeps the encrypted machine Command Deck reachable during
// the published compatibility window.

import Link from "next/link";

import RevokeButton from "@/components/revoke-button";
import type { MachineSummary } from "@/lib/types";
import { listMachines } from "@/lib/worker";

export const dynamic = "force-dynamic";

export default async function LegacyMachinesPage() {
  let machines: MachineSummary[] = [];
  let error: string | null = null;
  try {
    machines = await listMachines();
  } catch (caught) {
    error = caught instanceof Error ? caught.message : "failed to reach the gateway worker";
  }

  return (
    <div>
      <div className="page-heading">
        <div>
          <span className="eyebrow">LEGACY V1</span>
          <h1>Remote Command Deck</h1>
          <p className="subtitle">
            Choose a secure control-plane entry point. Its deck operates the whole connected mesh.
          </p>
        </div>
        <Link href="/add" className="btn btn-secondary">
          Add a Fort
        </Link>
      </div>

      <div className="compatibility-banner" role="note">
        This machine-first surface remains available only for the signed v1 compatibility and rollback window.
      </div>

      {error ? (
        <div className="card">
          <p className="err">Could not load machines: {error}</p>
          <p className="hint">
            Check <code>WORKER_URL</code> and <code>GATEWAY_SECRET</code> in the deployment environment.
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
          {machines.map((machine) => (
            <article className="machine-card" key={machine.machine_id}>
              <div className="machine-card-header">
                <span className={`status-dot ${machine.online ? "accepted" : "idle"}`} />
                <strong>{machine.name || machine.machine_id}</strong>
                <span className={`status-pill ${machine.online ? "state-delivered" : "state-idle"}`}>
                  {machine.online ? "Online" : "Offline"}
                </span>
              </div>
              <p className="machine-entry-copy">
                Secure relay entry point for one authoritative all-machine deck—not a task target.
              </p>
              <div>
                <div className="hint">Relay daemon fingerprint</div>
                <div className="fingerprint">{machine.fingerprint}</div>
              </div>
              <div className="machine-card-actions">
                <Link href={`/m/${machine.machine_id}`} className="btn btn-primary">
                  Open all-machine deck
                </Link>
                <RevokeButton machineId={machine.machine_id} name={machine.name || machine.machine_id} />
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  );
}
