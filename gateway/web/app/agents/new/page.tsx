import Link from "next/link";

export const dynamic = "force-dynamic";

export default function NewAgentPage() {
  return (
    <div className="agent-shell">
      <section className="agent-shell-hero">
        <div className="agent-shell-copy">
          <span className="eyebrow">CLOSED EXECUTION BOUNDARY</span>
          <h1>New Agent</h1>
          <p className="subtitle">No eligible execution source is currently approved for Agent creation.</p>
        </div>
        <Link className="btn btn-secondary" href="/">Back to Agents</Link>
      </section>

      <section className="card agent-roster-state" aria-labelledby="agent-option-gate">
        <h2 id="agent-option-gate">Enroll and approve a source first</h2>
        <p>
          Fort creates an Agent only from an opaque eligible option resolved by the control service. That option
          must already pin one exact framework-native Agent, readiness contract, execution binding, and authority.
        </p>
        <p className="hint">
          Provider and machine choices never come from this page. Until an approved source inventory is available,
          Agent creation fails closed while existing Agents and their Home conversations remain usable.
        </p>
      </section>
    </div>
  );
}
