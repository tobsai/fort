import Link from "next/link";

import { auth } from "@/auth";
import NewGroupForm from "@/app/groups/new/NewGroupForm";
import { createSignedFortControlAgentClientFromEnvironment } from "@/lib/v2-agent-client";

export const dynamic = "force-dynamic";

export default async function NewGroupPage() {
  const email = (await auth())?.user?.email?.trim().toLowerCase();
  if (!email) return unavailable("Your authenticated owner session is unavailable.");

  try {
    const agents = await createSignedFortControlAgentClientFromEnvironment().list({
      owner: { normalizedEmail: email },
    });
    const openAgents = agents.filter((agent) => agent.agent.state === "open");
    return (
      <div className="new-group-page">
        <header className="conversation-heading">
          <div>
            <Link className="rail-back" href="/groups">← Groups</Link>
            <span className="eyebrow">FROZEN MEMBERSHIP</span>
            <h1>New multi-Agent Group</h1>
            <p className="subtitle">Choose 2–6 stable Agents. Fort freezes their current accepted revisions when the Group is created.</p>
          </div>
        </header>
        {openAgents.length < 2 ? (
          <div className="card agent-roster-state" role="alert">
            <p className="err">At least two open Agents are required to create a Group.</p>
          </div>
        ) : (
          <NewGroupForm agents={openAgents.map((agent) => ({
            id: agent.agent.id,
            name: agent.profile.name,
            title: agent.profile.title,
          }))} />
        )}
      </div>
    );
  } catch {
    return unavailable("The Agent roster is temporarily unavailable.");
  }
}

function unavailable(message: string) {
  return (
    <div className="new-group-page">
      <Link className="rail-back" href="/groups">← Groups</Link>
      <div className="card agent-roster-state" role="alert"><p className="err">{message}</p></div>
    </div>
  );
}
