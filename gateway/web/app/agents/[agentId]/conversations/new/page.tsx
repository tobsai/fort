import Link from "next/link";

import { auth } from "@/auth";
import NewConversationForm from "@/app/agents/[agentId]/conversations/new/NewConversationForm";
import { createFortControlAgentDetailClient } from "@/lib/v2-agent-detail-client";
import { createFortControlServiceClientFromEnvironment } from "@/lib/v2-service-client";

export const dynamic = "force-dynamic";

export default async function NewAgentConversationPage({ params }: { params: { agentId: string } }) {
  const email = (await auth())?.user?.email?.trim().toLowerCase();
  if (!email) return unavailable("Your authenticated owner session is unavailable.");
  const client = createFortControlAgentDetailClient(createFortControlServiceClientFromEnvironment());
  try {
    const agent = await client.get({ owner: { normalizedEmail: email }, agentID: params.agentId });
    return (
      <div className="new-conversation-page">
        <header>
          <Link className="rail-back" href={`/agents/${encodeURIComponent(agent.agent.id)}`}>← {agent.profile.name}</Link>
          <span className="eyebrow">SECONDARY CONVERSATION</span>
          <h1>New conversation</h1>
          <p>Start another focused transcript with {agent.profile.name}.</p>
        </header>
        <NewConversationForm agentID={agent.agent.id} />
      </div>
    );
  } catch {
    return unavailable("This Agent is unavailable or does not belong to your Fort account.");
  }
}

function unavailable(message: string) {
  return <div className="card agent-roster-state" role="alert"><p className="err">{message}</p></div>;
}
