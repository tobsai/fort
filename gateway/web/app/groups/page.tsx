import Link from "next/link";

import { auth } from "@/auth";
import { createSignedFortControlAgentClientFromEnvironment } from "@/lib/v2-agent-client";
import { createFortControlGroupClient } from "@/lib/v2-group-client";
import { createFortControlServiceClientFromEnvironment } from "@/lib/v2-service-client";

export const dynamic = "force-dynamic";

export default async function GroupsPage() {
  const email = (await auth())?.user?.email?.trim().toLowerCase();
  let error: string | null = null;
  let groups = [] as Awaited<ReturnType<ReturnType<typeof createFortControlGroupClient>["list"]>>;
  let names = new Map<string, string>();
  if (!email) {
    error = "Your authenticated owner session is unavailable.";
  } else {
    const owner = { normalizedEmail: email };
    try {
      const [loadedGroups, agents] = await Promise.all([
        createFortControlGroupClient(createFortControlServiceClientFromEnvironment()).list({ owner }),
        createSignedFortControlAgentClientFromEnvironment().list({ owner }),
      ]);
      groups = loadedGroups;
      names = new Map(agents.map((agent) => [agent.agent.id, agent.profile.name]));
    } catch {
      error = "The Group roster is temporarily unavailable.";
    }
  }

  return (
    <div className="agent-shell">
      <section className="agent-shell-hero">
        <div className="agent-shell-copy">
          <span className="eyebrow">EXPLICIT COLLABORATION</span>
          <h1>Multi-Agent Groups</h1>
          <p className="subtitle">
            2–6 Agents, one frozen fan-out wave, up to 10 Agent messages, and durable Handoffs through depth 3.
          </p>
        </div>
        <Link href="/groups/new" className="btn btn-primary">New Group</Link>
      </section>

      {error ? (
        <div className="card agent-roster-state" role="alert">
          <p className="err">{error}</p>
          <p className="hint">Group history remains durable; reconnect after the v2 control service recovers.</p>
        </div>
      ) : groups.length === 0 ? (
        <div className="card agent-roster-state">
          <div className="empty">No Groups yet. <Link href="/groups/new">Create one</Link> from two or more Agents.</div>
        </div>
      ) : (
        <div className="group-grid" aria-label="Open Groups">
          {groups.map((group) => (
            <Link className="group-card" href={`/groups/${encodeURIComponent(group.group.id)}`} key={group.group.id}>
              <div className="group-card-heading">
                <span className="group-avatar" aria-hidden="true">{group.membership.members.length}</span>
                <div>
                  <strong>{group.conversation.title}</strong>
                  <span>Membership revision {group.membership.revision}</span>
                </div>
                <span className="agent-open-affordance" aria-hidden="true">→</span>
              </div>
              <div className="group-members">
                {group.membership.members.map((member) => (
                  <span key={member.agent_id}>{names.get(member.agent_id) ?? member.agent_id}</span>
                ))}
              </div>
              <span className="agent-stable-id">Stable Group · {group.group.id}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
