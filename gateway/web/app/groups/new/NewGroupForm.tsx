"use client";

import { FormEvent, useState, useTransition } from "react";
import { useRouter } from "next/navigation";

interface GroupAgentOption {
  id: string;
  name: string;
  title?: string;
}

export default function NewGroupForm({ agents }: { agents: GroupAgentOption[] }) {
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();

  function toggle(agentID: string) {
    setSelected((current) => current.includes(agentID)
      ? current.filter((id) => id !== agentID)
      : current.length < 6 ? [...current, agentID] : current);
  }

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const groupTitle = title.trim();
    if (!groupTitle || selected.length < 2 || selected.length > 6 || pending) return;
    setError(null);
    const commandID = crypto.randomUUID();
    let response: Response;
    try {
      response = await fetch("/api/v2/groups", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          idempotency_key: `group:${commandID}`,
          title: groupTitle,
          agent_ids: selected,
        }),
      });
    } catch {
      setError("control_unavailable");
      return;
    }
    if (!response.ok) {
      setError(await errorCode(response, "group_create_failed"));
      return;
    }
    const groupID = await createdGroupID(response);
    if (!groupID) {
      setError("group_create_failed");
      return;
    }
    startTransition(() => router.push(`/groups/${encodeURIComponent(groupID)}`));
  }

  return (
    <form className="new-group-form" onSubmit={create}>
      <label className="group-title-field" htmlFor="group-title">
        <span>Group title</span>
        <input
          id="group-title"
          value={title}
          disabled={pending}
          maxLength={512}
          placeholder="A durable purpose for this Group"
          onChange={(event) => setTitle(event.target.value)}
        />
      </label>

      <fieldset className="group-agent-picker" disabled={pending}>
        <legend>Agents <span>{selected.length}/6 selected</span></legend>
        <div className="group-agent-options">
          {agents.map((agent) => {
            const checked = selected.includes(agent.id);
            return (
              <label className={checked ? "group-agent-option selected" : "group-agent-option"} key={agent.id}>
                <input
                  type="checkbox"
                  checked={checked}
                  disabled={!checked && selected.length >= 6}
                  onChange={() => toggle(agent.id)}
                />
                <span className="agent-avatar" aria-hidden="true">{initials(agent.name)}</span>
                <span><strong>{agent.name}</strong><small>{agent.title || "Agent"}</small></span>
              </label>
            );
          })}
        </div>
      </fieldset>

      <div className="new-group-footer">
        <span>Only stable Agent IDs are sent. Fort resolves and freezes execution identity.</span>
        <button className="btn btn-primary" type="submit" disabled={pending || !title.trim() || selected.length < 2 || selected.length > 6}>
          {pending ? "Creating…" : "Create Group"}
        </button>
      </div>
      {error ? <p className="err" role="alert">Command failed: {error}</p> : null}
    </form>
  );
}

async function createdGroupID(response: Response): Promise<string | null> {
  try {
    const payload = await response.json() as unknown;
    if (isRecord(payload) && isRecord(payload.group) && typeof payload.group.id === "string" && payload.group.id.trim()) {
      return payload.group.id;
    }
  } catch {
    // The command succeeded without a usable bounded projection.
  }
  return null;
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

function initials(name: string): string {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]?.toUpperCase()).join("") || "A";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}
