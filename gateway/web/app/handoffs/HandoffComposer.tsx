"use client";

import { useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";

interface HandoffSourceMessage {
  id: number | string;
  authorLabel: string;
  body: string;
  authorAgentID?: string;
  hardDeadline?: string;
}

interface HandoffRecipient {
  id: string;
  name: string;
}

export default function HandoffComposer({
  conversationID,
  messages,
  recipients,
}: {
  conversationID: string;
  messages: HandoffSourceMessage[];
  recipients: HandoffRecipient[];
}) {
  const router = useRouter();
  const newestMessageID = messages.length > 0 ? String(messages[messages.length - 1]!.id) : "";
  const [sourceMessageID, setSourceMessageID] = useState(newestMessageID);
  const [contextMessageIDs, setContextMessageIDs] = useState<Set<string>>(
    () => new Set(newestMessageID ? [newestMessageID] : []),
  );
  const [recipientAgentID, setRecipientAgentID] = useState("");
  const [requestedResult, setRequestedResult] = useState("");
  const [deadlineMinutes, setDeadlineMinutes] = useState(10);
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();

  const selectedMessage = useMemo(
    () => messages.find((message) => String(message.id) === sourceMessageID),
    [messages, sourceMessageID],
  );
  const eligibleRecipients = recipients.filter((recipient) => recipient.id !== selectedMessage?.authorAgentID);

  function selectSource(messageID: string) {
    setSourceMessageID(messageID);
    setContextMessageIDs((current) => new Set([...current, messageID]));
    setRecipientAgentID((current) => {
      const source = messages.find((message) => String(message.id) === messageID);
      return source?.authorAgentID === current ? "" : current;
    });
  }

  function toggleContext(messageID: string) {
    if (messageID === sourceMessageID) return;
    setContextMessageIDs((current) => {
      const next = new Set(current);
      if (next.has(messageID)) next.delete(messageID);
      else next.add(messageID);
      return next;
    });
  }

  async function createHandoff(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending || !sourceMessageID || !recipientAgentID || !requestedResult.trim()) return;
    setError(null);
    const orderedContext = messages
      .map((message) => String(message.id))
      .filter((messageID) => contextMessageIDs.has(messageID) || messageID === sourceMessageID);
    try {
      const response = await fetch("/api/v2/handoffs", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          idempotency_key: `handoff:create:${crypto.randomUUID()}`,
          source_conversation_id: conversationID,
          source_message_id: sourceMessageID,
          recipient_agent_id: recipientAgentID,
          context_message_ids: orderedContext,
          requested_result: requestedResult.trim(),
          hard_deadline: selectedMessage?.hardDeadline ?? new Date(Date.now() + deadlineMinutes * 60_000).toISOString(),
        }),
      });
      if (!response.ok) {
        setError(await errorCode(response));
        return;
      }
      const value = await response.json() as unknown;
      if (!isRecord(value) || !isRecord(value.handoff) || typeof value.handoff.id !== "string") {
        setError("handoff_create_failed");
        return;
      }
      const handoffID = value.handoff.id;
      startTransition(() => router.push(`/handoffs/${encodeURIComponent(handoffID)}`));
    } catch {
      setError("handoff_create_failed");
    }
  }

  if (messages.length === 0 || recipients.length === 0) return null;

  return (
    <details className="card handoff-composer">
      <summary>Hand off a message</summary>
      <form onSubmit={createHandoff}>
        <label>
          Source message
          <select value={sourceMessageID} onChange={(event) => selectSource(event.target.value)}>
            {messages.map((message) => (
              <option key={String(message.id)} value={String(message.id)}>
                {message.authorLabel}: {summary(message.body)}
              </option>
            ))}
          </select>
        </label>
        <fieldset>
          <legend>Exact context</legend>
          {messages.map((message) => {
            const messageID = String(message.id);
            return (
              <label className="handoff-context-option" key={messageID}>
                <input
                  type="checkbox"
                  checked={contextMessageIDs.has(messageID) || messageID === sourceMessageID}
                  disabled={messageID === sourceMessageID}
                  onChange={() => toggleContext(messageID)}
                />
                <span><strong>{message.authorLabel}</strong> {summary(message.body)}</span>
              </label>
            );
          })}
        </fieldset>
        <label>
          Recipient Agent
          <select required value={recipientAgentID} onChange={(event) => setRecipientAgentID(event.target.value)}>
            <option value="">Choose one Agent</option>
            {eligibleRecipients.map((recipient) => (
              <option key={recipient.id} value={recipient.id}>{recipient.name}</option>
            ))}
          </select>
        </label>
        <label>
          Requested result
          <textarea
            required
            maxLength={4_000}
            value={requestedResult}
            onChange={(event) => setRequestedResult(event.target.value)}
            placeholder="What exact result should this Agent return?"
          />
        </label>
        {selectedMessage?.hardDeadline ? (
          <div className="handoff-inherited-deadline">
            <span>Hard deadline</span>
            <time dateTime={selectedMessage.hardDeadline}>{formatDeadline(selectedMessage.hardDeadline)}</time>
            <small>Inherited from the source Group Turn</small>
          </div>
        ) : (
          <label>
            Hard deadline
            <select value={deadlineMinutes} onChange={(event) => setDeadlineMinutes(Number(event.target.value))}>
              <option value={10}>10 minutes</option>
              <option value={30}>30 minutes</option>
              <option value={60}>1 hour</option>
            </select>
          </label>
        )}
        <div className="handoff-composer-actions">
          <button className="btn btn-primary" type="submit" disabled={pending || eligibleRecipients.length === 0}>
            {pending ? "Creating Handoff…" : "Create Handoff"}
          </button>
          {error ? <span className="err" role="alert">{error}</span> : null}
        </div>
      </form>
    </details>
  );
}

function summary(body: string): string {
  const compact = body.replace(/\s+/g, " ").trim();
  return compact.length > 80 ? `${compact.slice(0, 77)}…` : compact || "Empty message";
}

function formatDeadline(value: string): string {
  return new Intl.DateTimeFormat("en", { dateStyle: "medium", timeStyle: "short", timeZone: "UTC" }).format(new Date(value));
}

async function errorCode(response: Response): Promise<string> {
  try {
    const value = await response.json() as unknown;
    if (isRecord(value) && typeof value.code === "string" && /^[a-z][a-z0-9_]{0,63}$/.test(value.code)) return value.code;
  } catch {
    // The gateway intentionally exposes only bounded machine-readable errors.
  }
  return "handoff_create_failed";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}
