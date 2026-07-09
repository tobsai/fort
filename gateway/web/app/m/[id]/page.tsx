// /m/[id] — Board. A thin server wrapper that resolves the machine (name,
// fingerprint, daemon public key, online) from the worker and hands it to the
// client component, which runs the E2E Noise tunnel in the browser.

import Link from "next/link";

import BoardClient from "@/components/board-client";
import { getMachine } from "@/lib/worker";

export const dynamic = "force-dynamic";

export default async function BoardPage({ params }: { params: { id: string } }) {
  let machine = null;
  let error: string | null = null;
  try {
    machine = await getMachine(params.id);
  } catch (e) {
    error = e instanceof Error ? e.message : "failed to reach the gateway worker";
  }

  if (error) {
    return (
      <div className="card">
        <p className="err">Could not load machine: {error}</p>
        <p>
          <Link href="/">← Machines</Link>
        </p>
      </div>
    );
  }
  if (!machine) {
    return (
      <div className="card">
        <div className="empty">
          Unknown machine. <Link href="/">← Machines</Link>
        </div>
      </div>
    );
  }

  return (
    <div>
      <p className="hint">
        <Link href="/">← Machines</Link>
      </p>
      <h1>{machine.name || machine.machine_id}</h1>
      <p className="subtitle">
        <span className={`dot ${machine.online ? "online" : "offline"}`} />
        {machine.online ? "online" : "offline"} · connected end-to-end over the tunnel
      </p>
      <BoardClient
        machineId={machine.machine_id}
        name={machine.name || machine.machine_id}
        publicKey={machine.public_key}
        serverFingerprint={machine.fingerprint}
        online={machine.online}
      />
    </div>
  );
}
