"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";

export default function HandoffCancelButton({ handoffID }: { handoffID: string }) {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();

  async function cancel() {
    if (pending) return;
    setError(null);
    try {
      const response = await fetch(`/api/v2/handoffs/${encodeURIComponent(handoffID)}/cancel`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ idempotency_key: `handoff:cancel:${crypto.randomUUID()}` }),
      });
      if (!response.ok) {
        setError(await errorCode(response));
        return;
      }
      startTransition(() => router.refresh());
    } catch {
      setError("handoff_cancel_failed");
    }
  }

  return (
    <div className="handoff-cancel-action">
      <button className="btn-danger" type="button" disabled={pending} onClick={cancel}>
        {pending ? "Canceling…" : "Cancel Handoff"}
      </button>
      {error ? <span className="err" role="alert">{error}</span> : null}
    </div>
  );
}

async function errorCode(response: Response): Promise<string> {
  try {
    const value = await response.json() as unknown;
    if (isRecord(value) && typeof value.code === "string" && /^[a-z][a-z0-9_]{0,63}$/.test(value.code)) return value.code;
  } catch {
    // The gateway intentionally exposes only bounded machine-readable errors.
  }
  return "handoff_cancel_failed";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}
