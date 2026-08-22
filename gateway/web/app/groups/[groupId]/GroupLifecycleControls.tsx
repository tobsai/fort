"use client";

import { FormEvent, useState, useTransition } from "react";
import { useRouter } from "next/navigation";

interface AgentOption {
  agentID: string;
  name: string;
}

export default function GroupLifecycleControls({
  groupID,
  title: initialTitle,
  archived,
  membershipRevisionID,
  currentMembers,
  availableAgents,
}: {
  groupID: string;
  title: string;
  archived: boolean;
  membershipRevisionID: string;
  currentMembers: AgentOption[];
  availableAgents: AgentOption[];
}) {
  const router = useRouter();
  const [title, setTitle] = useState(initialTitle);
  const [orderedAgentIDs, setOrderedAgentIDs] = useState(currentMembers.map((member) => member.agentID));
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [refreshing, startTransition] = useTransition();
  const pending = submitting || refreshing;
  const names = new Map([...currentMembers, ...availableAgents].map((agent) => [agent.agentID, agent.name]));

  async function rename(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextTitle = title.trim();
    if (pending || nextTitle.length === 0 || nextTitle === initialTitle) return;
    await mutateGroup({
      idempotency_key: `group:rename:${crypto.randomUUID()}`,
      action: "rename",
      expected_title: initialTitle,
      title: nextTitle,
    });
  }

  async function changeState() {
    if (pending) return;
    await mutateGroup({
      idempotency_key: `group:state:${crypto.randomUUID()}`,
      action: archived ? "reopen" : "archive",
    });
  }

  function toggleMember(agentID: string) {
    setOrderedAgentIDs((current) => current.includes(agentID)
      ? current.filter((id) => id !== agentID)
      : [...current, agentID]);
  }

  function moveMember(agentID: string, offset: -1 | 1) {
    setOrderedAgentIDs((current) => {
      const from = current.indexOf(agentID);
      const to = from + offset;
      if (from < 0 || to < 0 || to >= current.length) return current;
      const next = [...current];
      [next[from], next[to]] = [next[to]!, next[from]!];
      return next;
    });
  }

  async function replaceMembers() {
    if (pending || archived || orderedAgentIDs.length < 2 || orderedAgentIDs.length > 6) return;
    await command(`/api/v2/groups/${encodeURIComponent(groupID)}/members`, "POST", {
      idempotency_key: `group:members:${crypto.randomUUID()}`,
      expected_membership_revision_id: membershipRevisionID,
      agent_ids: orderedAgentIDs,
    });
  }

  async function mutateGroup(body: Record<string, unknown>) {
    await command(`/api/v2/groups/${encodeURIComponent(groupID)}`, "PATCH", body);
  }

  async function command(path: string, method: "POST" | "PATCH", body: Record<string, unknown>) {
    setError(null);
    setSubmitting(true);
    try {
      const response = await fetch(path, {
        method,
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!response.ok) {
        const code = await errorCode(response, "group_update_failed");
        setError(response.status === 409
          ? `This Group changed or has active work (${code}). Refresh and try again.`
          : `Group update failed: ${code}`);
        return;
      }
      startTransition(() => router.refresh());
    } catch {
      setError("Group update failed: control_unavailable");
    } finally {
      setSubmitting(false);
    }
  }

  const membershipChanged = orderedAgentIDs.join("\0") !== currentMembers.map((member) => member.agentID).join("\0");

  return (
    <details className="card group-lifecycle-controls">
      <summary>Group settings</summary>
      <div className="group-lifecycle-grid">
        <form onSubmit={rename}>
          <label htmlFor="group-lifecycle-title">Name</label>
          <div>
            <input
              id="group-lifecycle-title"
              maxLength={120}
              value={title}
              disabled={pending}
              onChange={(event) => setTitle(event.target.value)}
            />
            <button className="btn" disabled={pending || title.trim().length === 0 || title.trim() === initialTitle} type="submit">
              Rename
            </button>
          </div>
        </form>

        <section aria-labelledby="group-membership-heading">
          <div className="group-lifecycle-section-heading">
            <div>
              <strong id="group-membership-heading">Ordered membership</strong>
              <span>{orderedAgentIDs.length}/6 Agents · minimum 2</span>
            </div>
            <button
              className="btn"
              disabled={pending || archived || !membershipChanged || orderedAgentIDs.length < 2 || orderedAgentIDs.length > 6}
              onClick={replaceMembers}
              type="button"
            >
              Replace membership
            </button>
          </div>
          <ol className="group-member-order">
            {orderedAgentIDs.map((agentID, position) => (
              <li key={agentID}>
                <span><b>{position + 1}</b>{names.get(agentID) ?? agentID}</span>
                <div>
                  <button aria-label={`Move ${names.get(agentID) ?? agentID} earlier`} disabled={pending || archived || position === 0} onClick={() => moveMember(agentID, -1)} type="button">↑</button>
                  <button aria-label={`Move ${names.get(agentID) ?? agentID} later`} disabled={pending || archived || position === orderedAgentIDs.length - 1} onClick={() => moveMember(agentID, 1)} type="button">↓</button>
                  <button aria-label={`Remove ${names.get(agentID) ?? agentID}`} disabled={pending || archived} onClick={() => toggleMember(agentID)} type="button">Remove</button>
                </div>
              </li>
            ))}
          </ol>
          <div className="group-member-add" aria-label="Available open Agents">
            {availableAgents.filter((agent) => !orderedAgentIDs.includes(agent.agentID)).map((agent) => (
              <button
                className="btn"
                disabled={pending || archived || orderedAgentIDs.length >= 6}
                key={agent.agentID}
                onClick={() => toggleMember(agent.agentID)}
                type="button"
              >
                + {agent.name}
              </button>
            ))}
          </div>
          {archived ? <p>Reopen this Group before replacing its membership.</p> : null}
        </section>

        <div className="group-lifecycle-state">
          <div>
            <strong>{archived ? "Archived" : "Open"}</strong>
            <span>{archived ? "History stays readable; new work is paused." : "Archive only when no Group work or Handoff chain is active."}</span>
          </div>
          <button className="btn" disabled={pending} onClick={changeState} type="button">
            {archived ? "Reopen Group" : "Archive Group"}
          </button>
        </div>
      </div>
      {error ? <p className="err" role="alert">{error}</p> : null}
    </details>
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
