import Link from "next/link";

import { auth } from "@/auth";
import HandoffCancelButton from "@/app/handoffs/HandoffCancelButton";
import { createSignedFortControlAgentClientFromEnvironment } from "@/lib/v2-agent-client";
import { createFortControlHandoffClientFromEnvironment } from "@/lib/v2-handoff-client";

export const dynamic = "force-dynamic";

export default async function HandoffDetailPage({ params }: { params: { handoffId: string } }) {
  const email = (await auth())?.user?.email?.trim().toLowerCase();
  if (!email) return unavailable("Your authenticated owner session is unavailable.");
  const owner = { normalizedEmail: email };
  try {
    const [record, agents] = await Promise.all([
      createFortControlHandoffClientFromEnvironment().read({ owner, handoffID: params.handoffId }),
      createSignedFortControlAgentClientFromEnvironment().list({ owner }),
    ]);
    const name = agents.find((agent) => agent.agent.id === record.handoff.recipient_agent_id)?.profile.name ?? record.handoff.recipient_agent_id;
    return (
      <div className="handoff-detail-page">
        <header className="conversation-heading">
          <div>
            <Link className="rail-back" href="/handoffs">← Handoffs</Link>
            <span className="eyebrow">DEPTH {record.handoff.depth} OF {record.handoff.max_depth}</span>
            <h1>To {name}</h1>
          </div>
          <span className={`target-state target-state-${record.handoff.state}`}>{record.handoff.state.replace("_", " ")}</span>
        </header>
        <section className="card handoff-command-detail">
          <h2>Requested result</h2>
          <p>{record.handoff.requested_result}</p>
          <dl className="handoff-evidence">
            <div><dt>Source Conversation</dt><dd>{record.handoff.source_conversation_id}</dd></div>
            <div><dt>Source message</dt><dd>{record.handoff.source_message_id}</dd></div>
            <div><dt>Output Conversation</dt><dd>{record.handoff.output_conversation_id}</dd></div>
            <div><dt>Behavior revision</dt><dd>{record.handoff.recipient_behavior_revision_id}</dd></div>
            <div><dt>Binding revision</dt><dd>{record.handoff.recipient_binding_revision_id}</dd></div>
            <div><dt>Deadline</dt><dd>{formatTimestamp(record.handoff.deadline)}</dd></div>
          </dl>
        </section>
        {record.result ? (
          <section className="card handoff-result-detail">
            <h2>Authoritative result</h2>
            <p>{record.result.body}</p>
            <small>Message {record.result.message_id} in {record.result.output_conversation_id}</small>
          </section>
        ) : null}
        {record.handoff.state === "queued" || record.handoff.state === "working" || record.handoff.state === "needs_you" ? (
          <HandoffCancelButton handoffID={record.handoff.id} />
        ) : null}
      </div>
    );
  } catch {
    return unavailable("This Handoff is unavailable or does not belong to your account.");
  }
}

function unavailable(message: string) {
  return <div className="card agent-roster-state" role="alert"><p className="err">{message}</p></div>;
}

function formatTimestamp(value: string): string {
  return new Intl.DateTimeFormat("en", { dateStyle: "medium", timeStyle: "short", timeZone: "UTC" }).format(new Date(value));
}
