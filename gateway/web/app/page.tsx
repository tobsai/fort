// / — the stable-Agent roster. Agents are durable chat relationships; their
// exact execution bindings are evidence, never their product identity.

import Link from "next/link";

import { auth } from "@/auth";
import {
  createSignedFortControlAgentClientFromEnvironment,
  type AgentRecordWire,
} from "@/lib/v2-agent-client";

export const dynamic = "force-dynamic";

export default async function AgentRosterPage() {
  const session = await auth();
  const email = session?.user?.email?.trim().toLowerCase();
  let agents: AgentRecordWire[] = [];
  let error: string | null = null;
  if (!email) {
    error = "Your authenticated owner session is unavailable.";
  } else {
    try {
      agents = await createSignedFortControlAgentClientFromEnvironment().list({
        owner: { normalizedEmail: email },
      });
    } catch {
      error = "The Agent roster is temporarily unavailable.";
    }
  }

  return (
    <div className="agent-shell">
      <section className="agent-shell-hero">
        <div className="agent-shell-copy">
          <span className="eyebrow">YOUR FORT</span>
          <h1>Your Agents</h1>
          <p className="subtitle">Pick up a durable conversation, no matter which framework or computer runs it.</p>
        </div>
        <div className="agent-shell-actions">
          <Link href="/groups" className="btn btn-secondary">Groups</Link>
          <Link href="/agents/new" className="btn btn-primary">New Agent</Link>
        </div>
      </section>

      {error ? (
        <div className="card agent-roster-state" role="alert">
          <p className="err">{error}</p>
          <p className="hint">
            Fort keeps identities visible when workers are offline, but this page requires the v2 control service.
          </p>
        </div>
      ) : agents.length === 0 ? (
        <div className="card agent-roster-state">
          <div className="empty">
            No Agents yet. <Link href="/agents/new">Create an Agent</Link> to establish its permanent Home conversation.
          </div>
        </div>
      ) : (
        <div className="agent-grid" aria-label="Open Agents">
          {agents.map((agent) => (
            <Link className="agent-card" href={`/agents/${encodeURIComponent(agent.agent.id)}`} key={agent.agent.id}>
              <div className="agent-card-topline">
                <span className="agent-avatar" aria-hidden="true">{initials(agent.profile.name)}</span>
                <div className="agent-identity">
                  <strong>{agent.profile.name}</strong>
                  <span>{agent.profile.title || "Fort Agent"}</span>
                </div>
                {agent.profile.pinned ? <span className="status-pill agent-pinned">Pinned</span> : null}
              </div>
              <div className="agent-home-row">
                <span className="agent-home-badge">Home</span>
                <span>{agent.home.title}</span>
                <span className="agent-open-affordance" aria-hidden="true">→</span>
              </div>
              <dl className="agent-binding">
                <div>
                  <dt>Framework</dt>
                  <dd>{agent.binding.provider}</dd>
                </div>
                <div>
                  <dt>Model</dt>
                  <dd>{agent.binding.resolved_model || agent.binding.requested_model}</dd>
                </div>
                <div>
                  <dt>Execution</dt>
                  <dd>{agent.binding.computer_id || agent.binding.cloud_runtime || "Unavailable"}</dd>
                </div>
              </dl>
              <span className="agent-stable-id">Stable Agent · {agent.agent.id}</span>
            </Link>
          ))}
        </div>
      )}

      <section className="group-launch-card">
        <div>
          <span className="eyebrow">MULTI-AGENT</span>
          <h2>Groups</h2>
          <p>Bring two to six Agents into one explicit, bounded conversation with durable Handoffs.</p>
        </div>
        <Link href="/groups" className="btn btn-secondary">Open Groups</Link>
      </section>
    </div>
  );
}

function initials(name: string): string {
  return name.trim().split(/\s+/).slice(0, 2).map((part) => part[0]?.toUpperCase() ?? "").join("") || "A";
}
