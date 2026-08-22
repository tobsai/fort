import Link from "next/link";

import { auth } from "@/auth";
import DurableChatRefresh from "@/app/DurableChatRefresh";
import GroupCommands from "@/app/groups/[groupId]/GroupCommands";
import GroupLifecycleControls from "@/app/groups/[groupId]/GroupLifecycleControls";
import HandoffComposer from "@/app/handoffs/HandoffComposer";
import { createSignedFortControlAgentClientFromEnvironment } from "@/lib/v2-agent-client";
import { createFortControlGroupClient } from "@/lib/v2-group-client";
import { createFortControlServiceClientFromEnvironment } from "@/lib/v2-service-client";

export const dynamic = "force-dynamic";

export default async function GroupPage({ params }: { params: { groupId: string } }) {
  const email = (await auth())?.user?.email?.trim().toLowerCase();
  if (!email) return unavailable("Your authenticated owner session is unavailable.");
  const owner = { normalizedEmail: email };
  const client = createFortControlGroupClient(createFortControlServiceClientFromEnvironment());
  try {
    const [projection, agents] = await Promise.all([
      client.read({ owner, groupID: params.groupId }),
      createSignedFortControlAgentClientFromEnvironment().list({ owner }),
    ]);
    const names = new Map(agents.map((agent) => [agent.agent.id, agent.profile.name]));
    const turnDeadlineByID = new Map(projection.turns.map((turn) => [turn.envelope.id, turn.envelope.deadline]));
    return (
      <div className="agent-conversation-page group-conversation-page">
        <DurableChatRefresh />
        <header className="conversation-heading">
          <div>
            <Link className="rail-back" href="/groups">← Groups</Link>
            <span className="eyebrow">MEMBERSHIP REVISION {projection.group.membership.revision}</span>
            <h1>{projection.group.conversation.title}</h1>
          </div>
          <div className="group-heading-members" aria-label="Frozen Group members">
            {projection.group.membership.members.map((member) => (
              <span key={member.agent_id}>{names.get(member.agent_id) ?? member.agent_id}</span>
            ))}
          </div>
        </header>

        <GroupLifecycleControls
          groupID={projection.group.group.id}
          title={projection.group.conversation.title}
          archived={projection.group.group.state === "archived"}
          membershipRevisionID={projection.group.membership.id}
          currentMembers={projection.group.membership.members.map((member) => ({
            agentID: member.agent_id,
            name: names.get(member.agent_id) ?? member.agent_id,
          }))}
          availableAgents={agents.map((agent) => ({ agentID: agent.agent.id, name: agent.profile.name }))}
        />

        <section className="message-ledger group-message-ledger" aria-label="Durable Group transcript">
          {projection.messages.length === 0 ? (
            <div className="empty conversation-empty">This is the beginning of this multi-Agent Group.</div>
          ) : projection.messages.map((message) => (
            <article className={`chat-message chat-message-${message.author_kind}`} key={message.id}>
              <div className="chat-message-meta">
                <strong>{message.author_kind === "human" ? "You" : message.author_kind === "agent" ? names.get(message.author_agent_id ?? "") ?? message.author_agent_id : "Fort"}</strong>
                <time dateTime={message.created_at}>{formatTimestamp(message.created_at)}</time>
              </div>
              <p>{message.body}</p>
            </article>
          ))}
        </section>

        {projection.turns.length > 0 ? (
          <section className="group-execution-ledger" aria-label="Group execution evidence">
            {projection.turns.map((turn) => (
              <article className="group-turn" key={turn.envelope.id}>
                <div className="group-turn-heading">
                  <strong>Turn {turn.envelope.client_turn_id}</strong>
                  <time dateTime={turn.envelope.created_at}>{formatTimestamp(turn.envelope.created_at)}</time>
                </div>
              <div className="group-wave" aria-label="Initial Agent targets">
                {turn.initial_targets.map((target) => (
                  <div className="group-wave-target" key={target.id}>
                    <div><strong>{names.get(target.agent_id) ?? target.agent_id}</strong><span className={`target-state target-state-${target.state}`}>{target.state}</span></div>
                    <dl>
                      <div><dt>Wave</dt><dd>{target.wave}</dd></div>
                      <div><dt>Binding revision</dt><dd>{target.binding_revision_id}</dd></div>
                      <div><dt>Behavior revision</dt><dd>{target.behavior_revision_id}</dd></div>
                    </dl>
                  </div>
                ))}
              </div>
              </article>
            ))}
          </section>
        ) : null}

        <GroupCommands
          groupID={projection.group.group.id}
          members={projection.group.member_bindings.map((member) => ({
            agentID: member.agent_id,
            name: names.get(member.agent_id) ?? member.agent_id,
          }))}
          archived={projection.group.group.state === "archived"}
        />

        <HandoffComposer
          conversationID={projection.group.conversation.id}
          messages={projection.messages.map((message) => {
            const turnDeadline = message.turn_id ? turnDeadlineByID.get(message.turn_id) : undefined;
            return {
              id: message.id,
              authorLabel: message.author_kind === "human"
                ? "You"
                : message.author_kind === "agent"
                  ? names.get(message.author_agent_id ?? "") ?? message.author_agent_id ?? "Agent"
                  : "Fort",
              body: message.body,
              ...(message.author_agent_id ? { authorAgentID: message.author_agent_id } : {}),
              ...(turnDeadline ? { hardDeadline: turnDeadline } : {}),
            };
          })}
          recipients={agents.map((agent) => ({ id: agent.agent.id, name: agent.profile.name }))}
        />
      </div>
    );
  } catch {
    return unavailable("This Group is unavailable or does not belong to your account.");
  }
}

function unavailable(message: string) {
  return <div className="card agent-roster-state" role="alert"><p className="err">{message}</p></div>;
}

function formatTimestamp(value: string): string {
  return new Intl.DateTimeFormat("en", { dateStyle: "medium", timeStyle: "short", timeZone: "UTC" }).format(new Date(value));
}
