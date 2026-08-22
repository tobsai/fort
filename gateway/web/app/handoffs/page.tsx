import Link from "next/link";

import { auth } from "@/auth";
import HandoffCancelButton from "@/app/handoffs/HandoffCancelButton";
import { createSignedFortControlAgentClientFromEnvironment } from "@/lib/v2-agent-client";
import { createFortControlHandoffClientFromEnvironment } from "@/lib/v2-handoff-client";

export const dynamic = "force-dynamic";

export default async function HandoffsPage() {
  const email = (await auth())?.user?.email?.trim().toLowerCase();
  if (!email) return unavailable("Your authenticated owner session is unavailable.");
  const owner = { normalizedEmail: email };
  try {
    const [records, agents] = await Promise.all([
      createFortControlHandoffClientFromEnvironment().list({ owner }),
      createSignedFortControlAgentClientFromEnvironment().list({ owner }),
    ]);
    const names = new Map(agents.map((agent) => [agent.agent.id, agent.profile.name]));
    return (
      <div className="handoffs-page">
        <header className="page-heading">
          <div>
            <span className="eyebrow">DURABLE DELEGATION</span>
            <h1>Handoffs</h1>
            <p className="subtitle">One exact recipient, bounded context, and one authoritative result.</p>
          </div>
        </header>
        {records.length === 0 ? (
          <div className="card empty">No Handoffs yet. Start one from an Agent or Group message.</div>
        ) : (
          <section className="handoff-list" aria-label="Handoff history">
            {records.map((record) => (
              <article className="card handoff-card" key={record.handoff.id}>
                <div className="handoff-card-heading">
                  <div>
                    <span className={`target-state target-state-${record.handoff.state}`}>{record.handoff.state.replace("_", " ")}</span>
                    <h2>{names.get(record.handoff.recipient_agent_id) ?? record.handoff.recipient_agent_id}</h2>
                  </div>
                  <time dateTime={record.handoff.created_at}>{formatTimestamp(record.handoff.created_at)}</time>
                </div>
                <p>{record.handoff.requested_result}</p>
                <dl className="handoff-evidence">
                  <div><dt>Output</dt><dd>{record.handoff.output_conversation_id}</dd></div>
                  <div><dt>Binding revision</dt><dd>{record.handoff.recipient_binding_revision_id}</dd></div>
                  <div><dt>Depth</dt><dd>{record.handoff.depth} / {record.handoff.max_depth}</dd></div>
                  <div><dt>Deadline</dt><dd>{formatTimestamp(record.handoff.deadline)}</dd></div>
                </dl>
                {record.result ? <blockquote className="handoff-result">{record.result.body}</blockquote> : null}
                <div className="handoff-card-actions">
                  <Link className="btn btn-secondary" href={`/handoffs/${encodeURIComponent(record.handoff.id)}`}>Inspect</Link>
                  {record.handoff.state === "queued" || record.handoff.state === "working" || record.handoff.state === "needs_you" ? (
                    <HandoffCancelButton handoffID={record.handoff.id} />
                  ) : null}
                </div>
              </article>
            ))}
          </section>
        )}
      </div>
    );
  } catch {
    return unavailable("Handoffs are unavailable for this account.");
  }
}

function unavailable(message: string) {
  return <div className="card agent-roster-state" role="alert"><p className="err">{message}</p></div>;
}

function formatTimestamp(value: string): string {
  return new Intl.DateTimeFormat("en", { dateStyle: "medium", timeStyle: "short", timeZone: "UTC" }).format(new Date(value));
}
