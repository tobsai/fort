"use client";

import { FormEvent, useState, useTransition } from "react";
import { useRouter } from "next/navigation";

interface GroupMemberOption {
  agentID: string;
  name: string;
}

export default function GroupCommands({ groupID, members, archived }: {
  groupID: string;
  members: GroupMemberOption[];
  archived: boolean;
}) {
  const router = useRouter();
  const [text, setText] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const [concurrency, setConcurrency] = useState<"sequential" | "concurrent">("concurrent");
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();

  function toggle(agentID: string) {
    setSelected((current) => current.includes(agentID)
      ? current.filter((id) => id !== agentID)
      : [...current, agentID]);
  }

  async function send(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const prompt = text.trim();
    if (!prompt || selected.length === 0 || archived || pending) return;
    setError(null);
    const commandID = crypto.randomUUID();
    let response: Response;
    try {
      response = await fetch(`/api/v2/groups/${encodeURIComponent(groupID)}/turns`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          idempotency_key: `group-turn:${commandID}`,
          client_turn_id: `client:${commandID}`,
          text: prompt,
          selection: selected.length === members.length ? "everyone" : "explicit",
          recipient_agent_ids: selected,
          concurrency_policy: concurrency,
          hard_deadline: new Date(Date.now() + 10 * 60 * 1_000).toISOString(),
        }),
      });
    } catch {
      setError("control_unavailable");
      return;
    }
    if (!response.ok) {
      setError(await errorCode(response, "group_turn_failed"));
      return;
    }
    setText("");
    startTransition(() => router.refresh());
  }

  return (
    <div className="conversation-controls group-conversation-controls">
      <form className="agent-composer group-composer" onSubmit={send}>
        <div className="group-recipient-picker">
          <span>Address</span>
          {members.map((member) => (
            <label key={member.agentID}>
              <input type="checkbox" checked={selected.includes(member.agentID)} disabled={archived || pending} onChange={() => toggle(member.agentID)} />
              {member.name}
            </label>
          ))}
        </div>
        {selected.length === 0 ? <span className="group-recipient-required">Choose at least one Agent. Fort never assumes Everyone.</span> : null}
        <label htmlFor="group-message">Message this Group</label>
        <textarea
          id="group-message"
          value={text}
          disabled={archived || pending}
          maxLength={100_000}
          placeholder={archived ? "This Group is archived." : "Ask these Agents to collaborate…"}
          onChange={(event) => setText(event.target.value)}
          rows={4}
        />
        <div className="agent-composer-footer group-composer-footer">
          <label>Dispatch
            <select disabled={archived || pending} value={concurrency} onChange={(event) => setConcurrency(event.target.value as "sequential" | "concurrent")}>
              <option value="concurrent">Concurrent</option>
              <option value="sequential">Sequential</option>
            </select>
          </label>
          <span>{pending ? "Recording durable command…" : "One bounded wave · exact frozen revisions"}</span>
          <button className="btn btn-primary" disabled={archived || pending || !text.trim() || selected.length === 0} type="submit">Send</button>
        </div>
      </form>
      {error ? <p className="err" role="alert">Command failed: {error}</p> : null}
    </div>
  );
}

async function errorCode(response: Response, fallback: string): Promise<string> {
  try {
    const payload = await response.json() as unknown;
    if (isRecord(payload) && typeof payload.code === "string" && /^[a-z][a-z0-9_]{0,63}$/.test(payload.code)) return payload.code;
  } catch {
    // The gateway exposes only bounded machine-readable errors.
  }
  return fallback;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}
