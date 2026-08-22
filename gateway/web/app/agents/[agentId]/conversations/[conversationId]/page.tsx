import Link from "next/link";

import { auth } from "@/auth";
import DurableChatRefresh from "@/app/DurableChatRefresh";
import ConversationCommands from "@/app/agents/[agentId]/conversations/[conversationId]/ConversationCommands";
import HandoffComposer from "@/app/handoffs/HandoffComposer";
import { createSignedFortControlAgentClientFromEnvironment } from "@/lib/v2-agent-client";
import { createFortControlAgentConversationClient } from "@/lib/v2-agent-conversation-client";
import { createFortControlAgentDetailClient } from "@/lib/v2-agent-detail-client";
import { createFortControlServiceClientFromEnvironment } from "@/lib/v2-service-client";

export const dynamic = "force-dynamic";

export default async function AgentConversationPage({ params }: {
  params: { agentId: string; conversationId: string };
}) {
  const email = (await auth())?.user?.email?.trim().toLowerCase();
  if (!email) return unavailable("Your authenticated owner session is unavailable.");
  const owner = { normalizedEmail: email };
  const service = createFortControlServiceClientFromEnvironment();
  const conversationClient = createFortControlAgentConversationClient(service);
  const agentClient = createFortControlAgentDetailClient(service);
  try {
    const [agent, projection, agents] = await Promise.all([
      agentClient.get({ owner, agentID: params.agentId }),
      conversationClient.read({
        owner,
        agentID: params.agentId,
        conversationID: params.conversationId,
      }),
      createSignedFortControlAgentClientFromEnvironment().list({ owner }),
    ]);
    const activeTargets = projection.targets.filter((target) => target.state !== "answered");
    return (
      <div className="agent-conversation-page">
        <DurableChatRefresh />
        <header className="conversation-heading">
          <div>
            <Link className="rail-back" href={`/agents/${encodeURIComponent(agent.agent.id)}`}>← {agent.profile.name}</Link>
            <span className="eyebrow">{projection.conversation.link.kind === "canonical" ? "CANONICAL HOME" : "AGENT CONVERSATION"}</span>
            <h1>{projection.conversation.conversation.title}</h1>
          </div>
          <div className="conversation-binding-proof">
            <span>Current binding</span>
            <strong>{agent.binding.provider}</strong>
            <small>{agent.binding.requested_model}</small>
          </div>
        </header>

        <section className="message-ledger" aria-label="Durable messages">
          {projection.messages.length === 0 ? (
            <div className="empty conversation-empty">This is the beginning of this Agent conversation.</div>
          ) : projection.messages.map((message) => (
            <article className={`chat-message chat-message-${message.author_kind}`} key={message.id}>
              <div className="chat-message-meta">
                <strong>{message.author_kind === "human" ? "You" : message.author_kind === "agent" ? agent.profile.name : "Fort"}</strong>
                <time dateTime={message.created_at}>{formatTimestamp(message.created_at)}</time>
              </div>
              <p>{message.body}</p>
            </article>
          ))}
        </section>

        {projection.targets.length > 0 ? (
          <section className="execution-evidence" aria-label="Execution evidence">
            <h2>Execution evidence</h2>
            {projection.targets.map((target) => (
              <dl key={target.id}>
                <div><dt>State</dt><dd>{target.state}</dd></div>
                <div><dt>Attempts</dt><dd>{target.attempt_count}</dd></div>
                <div><dt>Binding revision</dt><dd>{target.binding_revision_id}</dd></div>
                <div><dt>Behavior revision</dt><dd>{target.behavior_revision_id}</dd></div>
              </dl>
            ))}
          </section>
        ) : null}

        <ConversationCommands
          agentID={agent.agent.id}
          conversationID={projection.conversation.conversation.id}
          targets={activeTargets.map(({ id, state }) => ({ id, state }))}
          archived={projection.conversation.conversation.state === "archived"}
          kind={projection.conversation.link.kind}
          pinned={projection.conversation.pinned}
          title={projection.conversation.conversation.title}
        />

        <HandoffComposer
          conversationID={projection.conversation.conversation.id}
          messages={projection.messages.map((message) => ({
            id: message.id,
            authorLabel: message.author_kind === "human" ? "You" : message.author_kind === "agent" ? agent.profile.name : "Fort",
            body: message.body,
            ...(message.author_kind === "agent" ? { authorAgentID: message.author_agent_id ?? agent.agent.id } : {}),
          }))}
          recipients={agents
            .filter((recipient) => recipient.agent.id !== agent.agent.id)
            .map((recipient) => ({ id: recipient.agent.id, name: recipient.profile.name }))}
        />
      </div>
    );
  } catch {
    return unavailable("This conversation is unavailable or does not belong to this Agent.");
  }
}

function unavailable(message: string) {
  return <div className="card agent-roster-state" role="alert"><p className="err">{message}</p></div>;
}

function formatTimestamp(value: string): string {
  return new Intl.DateTimeFormat("en", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "UTC",
  }).format(new Date(value));
}
