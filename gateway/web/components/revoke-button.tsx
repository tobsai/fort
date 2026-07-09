"use client";

// RevokeButton — DELETE /api/machines/:id, then refresh the list. Confirms
// first because a revoke drops the daemon's socket and invalidates its token.

import { useRouter } from "next/navigation";
import { useState } from "react";

export default function RevokeButton({ machineId, name }: { machineId: string; name: string }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function onRevoke() {
    if (!confirm(`Revoke "${name}"? Its device token is invalidated and the tunnel drops.`)) {
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      const res = await fetch(`/api/machines/${encodeURIComponent(machineId)}`, {
        method: "DELETE",
      });
      if (!res.ok) throw new Error(`revoke failed (${res.status})`);
      router.refresh();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "revoke failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <span className="row">
      <button className="btn-danger" onClick={onRevoke} disabled={busy}>
        {busy ? "Revoking…" : "Revoke"}
      </button>
      {err ? <span className="err">{err}</span> : null}
    </span>
  );
}
