"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";

export default function NewConversationForm({ agentID }: { agentID: string }) {
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalized = title.trim();
    if (!normalized || normalized !== title || pending) return;
    setPending(true);
    setError(null);
    try {
      const response = await fetch(`/api/v2/agents/${encodeURIComponent(agentID)}/conversations`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          idempotency_key: `conversation:create:${crypto.randomUUID()}`,
          title: normalized,
        }),
      });
      if (!response.ok) {
        setError(await errorCode(response, "create_failed"));
        return;
      }
      const payload = await response.json() as unknown;
      const conversationID = createdConversationID(payload, agentID);
      if (!conversationID) {
        setError("invalid_response");
        return;
      }
      router.push(`/agents/${encodeURIComponent(agentID)}/conversations/${encodeURIComponent(conversationID)}`);
      router.refresh();
    } catch {
      setError("create_failed");
    } finally {
      setPending(false);
    }
  }

  return (
    <form className="new-conversation-form" onSubmit={create}>
      <label htmlFor="conversation-title">Conversation title</label>
      <input
        id="conversation-title"
        name="title"
        value={title}
        maxLength={512}
        autoFocus
        disabled={pending}
        placeholder="Market map"
        onChange={(event) => setTitle(event.target.value)}
      />
      <p>This creates a separate durable transcript while Home remains permanent.</p>
      <div className="new-conversation-actions">
        <button className="btn btn-primary" disabled={pending || !title.trim() || title !== title.trim()} type="submit">
          {pending ? "Creating…" : "Create conversation"}
        </button>
      </div>
      {error ? <p className="err" role="alert">Creation failed: {error}</p> : null}
    </form>
  );
}

function createdConversationID(value: unknown, agentID: string): string | null {
  if (!isRecord(value) || !isRecord(value.conversation) || !isRecord(value.link)) return null;
  const id = value.conversation.id;
  if (typeof id !== "string" || !id || id.length > 512 || /[\/\\\r\n\0]/.test(id)) return null;
  if (value.link.agent_id !== agentID || value.link.conversation_id !== id || value.link.kind !== "secondary") return null;
  return id;
}

async function errorCode(response: Response, fallback: string): Promise<string> {
  try {
    const payload = await response.json() as unknown;
    if (isRecord(payload) && typeof payload.code === "string" && /^[a-z][a-z0-9_]{0,63}$/.test(payload.code)) return payload.code;
  } catch {
    // Only bounded error codes cross the gateway.
  }
  return fallback;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}
