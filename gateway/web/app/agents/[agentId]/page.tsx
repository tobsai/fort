import Link from "next/link";

import { auth } from "@/auth";
import {
  createFortControlAgentDetailClient,
  type AgentConversationWire,
} from "@/lib/v2-agent-detail-client";
import {
  createFortControlRoutineClient,
  type RoutineRecordWire,
  type RoutineRunRecordWire,
} from "@/lib/v2-routine-client";
import { createFortControlServiceClientFromEnvironment } from "@/lib/v2-service-client";

import AgentSettings from "./AgentSettings";
import RoutineManager from "./RoutineManager";

export const dynamic = "force-dynamic";

export default async function AgentDetailPage({ params }: { params: { agentId: string } }) {
  const email = (await auth())?.user?.email?.trim().toLowerCase();
  const service = createFortControlServiceClientFromEnvironment();
  const client = createFortControlAgentDetailClient(service);
  const routineClient = createFortControlRoutineClient(service);
  let error: string | null = null;
  let loaded: Awaited<ReturnType<typeof client.get>> | null = null;
  let conversations: Awaited<ReturnType<typeof client.listConversations>> = [];
  let routines: RoutineRecordWire[] = [];
  let runsByRoutine: Record<string, RoutineRunRecordWire[]> = {};
  if (!email) {
    error = "Your authenticated owner session is unavailable.";
  } else {
    const owner = { normalizedEmail: email };
    try {
      [loaded, conversations, routines] = await Promise.all([
        client.get({ owner, agentID: params.agentId }),
        client.listConversations({ owner, agentID: params.agentId }),
        routineClient.list({ owner, agentID: params.agentId }),
      ]);
      runsByRoutine = Object.fromEntries(await Promise.all(routines.map(async (record) => [
        record.routine.id,
        await routineClient.listRuns({ owner, agentID: params.agentId, routineID: record.routine.id }),
      ])));
    } catch {
      error = "This Agent is unavailable or does not belong to your Fort account.";
    }
  }

  if (error || !loaded) {
    return <div className="card agent-roster-state" role="alert"><p className="err">{error}</p></div>;
  }
  const agent = loaded;
  const canonical = conversations.find((item) => item.link.kind === "canonical");
  const pinned = conversations.filter((item) => item.link.kind === "secondary" && item.pinned);
  const secondary = conversations.filter((item) => item.link.kind === "secondary" && !item.pinned);

  return (
    <div className="agent-detail-shell">
      <aside className="agent-conversation-rail">
        <Link href="/" className="rail-back">← All Agents</Link>
        <div className="rail-agent">
          <span className="agent-avatar" aria-hidden="true">{initials(agent.profile.name)}</span>
          <div><strong>{agent.profile.name}</strong><span>{agent.profile.title || "Fort Agent"}</span></div>
        </div>

        <nav className="conversation-nav" aria-label={`${agent.profile.name} conversations`}>
          <span className="conversation-nav-label">Canonical Home</span>
          {canonical ? <ConversationLink agentID={agent.agent.id} item={canonical} home /> : null}
          {pinned.length > 0 ? <span className="conversation-nav-label">Pinned conversations</span> : null}
          {pinned.map((item) => <ConversationLink agentID={agent.agent.id} item={item} key={item.conversation.id} />)}
          {secondary.length > 0 ? <span className="conversation-nav-label">Conversations</span> : null}
          {secondary.map((item) => <ConversationLink agentID={agent.agent.id} item={item} key={item.conversation.id} />)}
        </nav>
        <Link className="btn btn-secondary rail-new-conversation" href={`/agents/${encodeURIComponent(agent.agent.id)}/conversations/new`}>
          New conversation
        </Link>
      </aside>

      <section className="agent-home-panel">
        <header className="agent-home-header">
          <div>
            <span className="eyebrow">HOME</span>
            <h1>{agent.profile.name}</h1>
            <p>{agent.home.title} is this Agent&apos;s permanent canonical conversation.</p>
          </div>
          <Link className="btn btn-primary" href={`/agents/${encodeURIComponent(agent.agent.id)}/conversations/${encodeURIComponent(agent.home.id)}`}>
            Open Home
          </Link>
        </header>
        <dl className="agent-binding agent-binding-detail">
          <div><dt>Framework</dt><dd>{agent.binding.provider}</dd></div>
          <div><dt>Requested model</dt><dd>{agent.binding.requested_model}</dd></div>
          <div><dt>Execution</dt><dd>{agent.binding.computer_id || agent.binding.cloud_runtime || "Unavailable"}</dd></div>
        </dl>
        <div className="binding-truth-note">
          This exact binding is visible execution evidence. The stable Agent, Home, Groups, Routines, and history survive an explicitly approved Rebind.
        </div>
        <RoutineManager
          agentID={agent.agent.id}
          routines={routines}
          runsByRoutine={runsByRoutine}
          resultConversations={conversations
            .filter((conversation) => conversation.conversation.state === "open")
            .map((conversation) => ({
              id: conversation.conversation.id,
              title: conversation.conversation.title,
              kind: conversation.link.kind,
            }))}
        />
        <AgentSettings
          key={`${agent.agent.current_profile_revision_id}:${agent.agent.current_behavior_revision_id}`}
          agentID={agent.agent.id}
          profileRevisionID={agent.agent.current_profile_revision_id}
          behaviorRevisionID={agent.agent.current_behavior_revision_id}
          bindingRevisionID={agent.agent.current_binding_revision_id}
          profile={{
            name: agent.profile.name,
            title: agent.profile.title ?? "",
            avatarURL: agent.profile.avatar_url ?? "",
            hidden: agent.profile.hidden,
            pinned: agent.profile.pinned,
            sortOrder: agent.profile.sort_order,
          }}
          behavior={{
            role: agent.behavior.role,
            standingInstructions: agent.behavior.standing_instructions ?? "",
            enabledSkills: agent.behavior.enabled_skills,
            enabledTools: agent.behavior.enabled_tools,
            promptMaterial: agent.behavior.prompt_material ?? "",
          }}
        />
      </section>
    </div>
  );
}

function ConversationLink({ agentID, item, home = false }: {
  agentID: string;
  item: AgentConversationWire;
  home?: boolean;
}) {
  return (
    <Link className="conversation-nav-item" href={`/agents/${encodeURIComponent(agentID)}/conversations/${encodeURIComponent(item.conversation.id)}`}>
      <span>{item.conversation.title}</span>
      {home ? <small>Home</small> : item.pinned ? <small>Pinned</small> : null}
    </Link>
  );
}

function initials(name: string): string {
  return name.trim().split(/\s+/).slice(0, 2).map((part) => part[0]?.toUpperCase() ?? "").join("") || "A";
}
