"use client";

import { FormEvent, useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";

interface ConversationCommandsProps {
  agentID: string;
  conversationID: string;
  targets: Array<{ id: string; state: string }>;
  archived: boolean;
  kind: "canonical" | "secondary";
  pinned: boolean;
  title: string;
}

export default function ConversationCommands({
  agentID,
  conversationID,
  targets,
  archived,
  kind,
  pinned,
  title,
}: ConversationCommandsProps) {
  const router = useRouter();
  const [text, setText] = useState("");
  const [renameTitle, setRenameTitle] = useState(title);
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();
  const conversationPath = `/api/v2/agents/${encodeURIComponent(agentID)}/conversations/${encodeURIComponent(conversationID)}`;

  useEffect(() => setRenameTitle(title), [title]);

  async function send(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const prompt = text.trim();
    if (!prompt || archived || pending) return;
    setError(null);
    const id = crypto.randomUUID();
    const response = await fetch(`${conversationPath}/turns`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        idempotency_key: `send:${id}`,
        client_turn_id: `client:${id}`,
        text: prompt,
        hard_deadline: new Date(Date.now() + 10 * 60 * 1_000).toISOString(),
      }),
    });
    if (!response.ok) {
      setError(await errorCode(response, "send_failed"));
      return;
    }
    setText("");
    startTransition(() => router.refresh());
  }

  async function targetCommand(targetID: string, action: "retry" | "cancel") {
    if (pending) return;
    setError(null);
    const response = await fetch(
      `${conversationPath}/targets/${encodeURIComponent(targetID)}/${action}`,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ idempotency_key: `${action}:${crypto.randomUUID()}` }),
      },
    );
    if (!response.ok) {
      setError(await errorCode(response, `${action}_failed`));
      return;
    }
    startTransition(() => router.refresh());
  }

  async function conversationCommand(
    action: "rename" | "pin" | "unpin" | "archive" | "reopen",
    fields: Record<string, string> = {},
  ) {
    if (pending) return;
    setError(null);
    try {
      const response = await fetch(conversationPath, {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          idempotency_key: `conversation:${action}:${crypto.randomUUID()}`,
          action,
          ...fields,
        }),
      });
      if (!response.ok) {
        setError(await errorCode(response, `${action}_failed`));
        return;
      }
      startTransition(() => router.refresh());
    } catch {
      setError(`${action}_failed`);
    }
  }

  return (
    <div className="conversation-controls">
      {kind === "secondary" ? (
        <section className="secondary-conversation-actions" aria-label="Conversation settings">
          <form onSubmit={(event) => {
            event.preventDefault();
            const normalized = renameTitle.trim();
            if (!normalized || normalized !== renameTitle || normalized === title) return;
            void conversationCommand("rename", { expected_title: title, title: normalized });
          }}>
            <label htmlFor="conversation-rename">Title</label>
            <input
              id="conversation-rename"
              value={renameTitle}
              maxLength={512}
              disabled={pending}
              onChange={(event) => setRenameTitle(event.target.value)}
            />
            <button className="btn btn-secondary" type="submit" disabled={pending || !renameTitle.trim() || renameTitle.trim() !== renameTitle || renameTitle === title}>Rename</button>
          </form>
          <div>
            <button className="btn btn-secondary" type="button" disabled={pending} onClick={() => void conversationCommand(pinned ? "unpin" : "pin")}>
              {pinned ? "Unpin" : "Pin"}
            </button>
            <button className={archived ? "btn btn-secondary" : "btn-danger"} type="button" disabled={pending} onClick={() => void conversationCommand(archived ? "reopen" : "archive")}>
              {archived ? "Reopen" : "Archive"}
            </button>
          </div>
        </section>
      ) : null}

      {targets.length > 0 ? (
        <div className="target-actions" aria-label="Target actions">
          {targets.map((target) => (
            <div key={target.id}>
              <span className={`target-state target-state-${target.state}`}>{target.state}</span>
              {target.state === "failed" || target.state === "canceled" ? (
                <button className="btn btn-secondary" type="button" disabled={pending} onClick={() => targetCommand(target.id, "retry")}>Retry</button>
              ) : null}
              {target.state === "queued" || target.state === "working" ? (
                <button className="btn-danger" type="button" disabled={pending} onClick={() => targetCommand(target.id, "cancel")}>Cancel</button>
              ) : null}
            </div>
          ))}
        </div>
      ) : null}

      <form className="agent-composer" onSubmit={send}>
        <label htmlFor="agent-message">Message this Agent</label>
        <textarea
          id="agent-message"
          value={text}
          disabled={archived || pending}
          maxLength={100_000}
          placeholder={archived ? "This conversation is archived." : "Ask, delegate, or continue the conversation…"}
          onChange={(event) => setText(event.target.value)}
          rows={4}
        />
        <div className="agent-composer-footer">
          <span>{pending ? "Recording durable command…" : "Send uses this Agent's current accepted binding."}</span>
          <button className="btn btn-primary" disabled={archived || pending || !text.trim()} type="submit">Send</button>
        </div>
      </form>
      {error ? <p className="err" role="alert">Command failed: {error}</p> : null}
    </div>
  );
}

async function errorCode(response: Response, fallback: string): Promise<string> {
  try {
    const payload = await response.json() as unknown;
    if (isRecord(payload) && typeof payload.code === "string" && /^[a-z][a-z0-9_]{0,63}$/.test(payload.code)) {
      return payload.code;
    }
  } catch {
    // The gateway intentionally exposes only bounded machine-readable errors.
  }
  return fallback;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}
